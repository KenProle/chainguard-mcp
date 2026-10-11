package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
)

const defaultAdvisoryFeedURL = "https://advisories.cgr.dev"

// advisoryFeedSource names the feed in a *SourceError.
const advisoryFeedSource = "Chainguard advisories"

// advisoryID is the shape of a Chainguard advisory ID. Only IDs of this shape
// are put in a feed URL.
var advisoryID = regexp.MustCompile(`^CGA-[0-9a-z]{4}-[0-9a-z]{4}-[0-9a-z]{4}$`)

// AdvisoryFeed reads Chainguard's own advisories, one JSON file per advisory
// in the OSV schema. OSV.dev serves copies of them that can be older, so an
// advisory's current status comes from here (OA-17). It is anonymous, like
// the registry, and caches each advisory, including a missing one.
type AdvisoryFeed struct {
	HTTP *http.Client
	URL  string
	// TTL is how long an advisory is cached (OA-12).
	TTL time.Duration
	// Now is the clock, replaced in tests.
	Now func() time.Time

	sem chan struct{}

	mu    sync.Mutex
	cache map[string]feedEntry
}

type feedEntry struct {
	vuln    *OSVVuln // nil when the feed has no such advisory
	fetched time.Time
}

func NewAdvisoryFeed(client *http.Client) *AdvisoryFeed {
	return &AdvisoryFeed{HTTP: client, URL: defaultAdvisoryFeedURL, TTL: time.Hour, sem: make(chan struct{}, osvMaxInFlight)}
}

func (f *AdvisoryFeed) now() time.Time {
	if f.Now != nil {
		return f.Now()
	}
	return time.Now()
}

// fetch returns an advisory's current record and when it was fetched. The
// record is nil if the feed doesn't have the advisory (a 404).
func (f *AdvisoryFeed) fetch(ctx context.Context, id string) (*OSVVuln, time.Time, error) {
	if !advisoryID.MatchString(id) {
		return nil, time.Time{}, fmt.Errorf("%q is not a Chainguard advisory ID", id)
	}
	f.mu.Lock()
	if e, ok := f.cache[id]; ok && f.now().Sub(e.fetched) < f.TTL {
		f.mu.Unlock()
		return e.vuln, e.fetched, nil
	}
	f.mu.Unlock()

	var vuln *OSVVuln
	var err error
	for attempt := range 2 {
		if attempt > 0 {
			select {
			case <-time.After(300 * time.Millisecond):
			case <-ctx.Done():
				return nil, time.Time{}, ctx.Err()
			}
		}
		var retry bool
		vuln, retry, err = f.get(ctx, id)
		if err == nil || !retry {
			break
		}
	}
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("%s: %w", id, err)
	}

	f.mu.Lock()
	if f.cache == nil {
		f.cache = make(map[string]feedEntry)
	}
	if len(f.cache)%1000 == 999 {
		for k, e := range f.cache {
			if f.now().Sub(e.fetched) >= f.TTL {
				delete(f.cache, k)
			}
		}
	}
	fetched := f.now()
	f.cache[id] = feedEntry{vuln: vuln, fetched: fetched}
	f.mu.Unlock()
	return vuln, fetched, nil
}

// get makes one request. The bool says whether a retry may help.
func (f *AdvisoryFeed) get(ctx context.Context, id string) (*OSVVuln, bool, error) {
	select {
	case f.sem <- struct{}{}:
		defer func() { <-f.sem }()
	case <-ctx.Done():
		return nil, false, ctx.Err()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSuffix(f.URL, "/")+"/chainguard/v3/osv/"+id+".json", nil)
	if err != nil {
		return nil, false, err
	}
	resp, err := f.HTTP.Do(req)
	if err != nil {
		return nil, ctx.Err() == nil, err
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusNotFound:
		io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
		return nil, false, nil
	case resp.StatusCode != http.StatusOK:
		io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
		return nil, resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests, fmt.Errorf("unexpected status %s", resp.Status)
	}
	var v OSVVuln
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&v); err != nil {
		return nil, false, fmt.Errorf("decoding advisory: %w", err)
	}
	if v.ID != id {
		return nil, false, fmt.Errorf("the record is for %q", v.ID)
	}
	return &v, false, nil
}

// FetchAll reads every advisory and returns them by ID, with nil for those
// the feed doesn't have, and the oldest time any of them was fetched. The first
// failure cancels the rest, since a partial result must not be shown as
// complete (OA-13); at most osvMaxInFlight requests run at once (OA-12).
func (f *AdvisoryFeed) FetchAll(ctx context.Context, ids []string) (map[string]*OSVVuln, time.Time, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var (
		mu       sync.Mutex
		wg       sync.WaitGroup
		out      = make(map[string]*OSVVuln, len(ids))
		oldest   time.Time
		firstErr error
		ch       = make(chan string)
	)
	for range osvMaxInFlight {
		wg.Go(func() {
			for id := range ch {
				v, fetched, err := f.fetch(ctx, id)
				mu.Lock()
				if err != nil {
					if firstErr == nil {
						firstErr = err
						cancel()
					}
				} else {
					out[id] = v
					if oldest.IsZero() || fetched.Before(oldest) {
						oldest = fetched
					}
				}
				mu.Unlock()
			}
		})
	}
feed:
	for _, id := range ids {
		select {
		case ch <- id:
		case <-ctx.Done():
			break feed
		}
	}
	close(ch)
	wg.Wait()
	if firstErr != nil {
		return nil, time.Time{}, &SourceError{Source: advisoryFeedSource, Err: firstErr}
	}
	if err := ctx.Err(); err != nil {
		return nil, time.Time{}, err
	}
	return out, oldest, nil
}
