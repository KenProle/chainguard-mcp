package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"
)

const (
	defaultOSVURL = "https://api.osv.dev"
	// osvMaxInFlight bounds concurrent requests to OSV.dev (UC-10).
	osvMaxInFlight = 10
)

// OSV is a client for the OSV.dev vulnerability database. It queries one
// package at a time with POST /v1/query, which returns full records (the
// batch endpoint returns only IDs), and caches each package's result.
type OSV struct {
	HTTP *http.Client
	URL  string
	// TTL is how long a package's result is cached; ChainguardTTL replaces it
	// for the Wolfi and Chainguard ecosystems, whose open advisories change
	// during the day (OA-12).
	TTL, ChainguardTTL time.Duration
	// Now is the clock, replaced in tests.
	Now func() time.Time

	sem chan struct{}

	mu    sync.Mutex
	cache map[OSVPackage]osvCacheEntry
}

type osvCacheEntry struct {
	vulns   []OSVVuln
	fetched time.Time
}

func (o *OSV) now() time.Time {
	if o.Now != nil {
		return o.Now()
	}
	return time.Now()
}

// ttl returns how long a result for the package's ecosystem stays fresh.
func (o *OSV) ttl(pkg OSVPackage) time.Duration {
	if pkg.Ecosystem == "Wolfi" || pkg.Ecosystem == "Chainguard" {
		return o.ChainguardTTL
	}
	return o.TTL
}

func NewOSV(client *http.Client) *OSV {
	return &OSV{HTTP: client, URL: defaultOSVURL, TTL: 24 * time.Hour, ChainguardTTL: time.Hour, sem: make(chan struct{}, osvMaxInFlight)}
}

// OSVVuln is the part of an OSV record used to classify a vulnerability.
type OSVVuln struct {
	ID string `json:"id"`
	// Aliases and Upstream name the same vulnerability in other databases.
	// Distribution records (DEBIAN-CVE-…, DSA-…, CGA-…) put the CVE in Upstream.
	Aliases  []string      `json:"aliases"`
	Upstream []string      `json:"upstream"`
	Affected []OSVAffected `json:"affected"`
}

// fixedVersion returns the first fixed version the record names for the
// package, if any.
func (v OSVVuln) fixedVersion(pkg OSVPackage) string {
	for _, a := range v.Affected {
		if a.Package.Name != pkg.Name || a.Package.Ecosystem != pkg.Ecosystem {
			continue
		}
		for _, r := range a.Ranges {
			for _, e := range r.Events {
				if e.Fixed != "" {
					return e.Fixed
				}
			}
		}
	}
	return ""
}

// vulnKeys returns the vulnerabilities an OSV record stands for, so records
// for the same vulnerability count once (UC-4.2): Chainguard publishes a CGA
// record per package per CVE, and Debian's DSA records repeat CVEs that also
// have DEBIAN-CVE records. The keys are the record's CVEs, else its GHSAs,
// else its own ID.
func vulnKeys(v OSVVuln) []string {
	ids := slices.Concat([]string{v.ID}, v.Upstream, v.Aliases)
	for _, prefix := range []string{"CVE-", "GHSA-"} {
		var keys []string
		for _, id := range ids {
			if strings.HasPrefix(id, prefix) && !slices.Contains(keys, id) {
				keys = append(keys, id)
			}
		}
		if len(keys) > 0 {
			return keys
		}
	}
	return []string{v.ID}
}

type OSVAffected struct {
	Package struct {
		Name      string `json:"name"`
		Ecosystem string `json:"ecosystem"`
	} `json:"package"`
	Ranges []struct {
		Events []struct {
			Introduced string `json:"introduced"`
			Fixed      string `json:"fixed"`
		} `json:"events"`
	} `json:"ranges"`
	EcosystemSpecific struct {
		// Urgency is Debian's triage, e.g. "unimportant" or "end-of-life".
		Urgency    string `json:"urgency"`
		Components []struct {
			// ComponentName and ComponentType name what the advisory is
			// about: the package itself ("apk") or software compiled or
			// bundled into it ("rust-crate", "python", "go-module", …).
			ComponentName string `json:"component_name"`
			ComponentType string `json:"component_type"`
			// LatestEventStatus is Chainguard's, e.g. "pending_upstream_fix".
			LatestEventStatus string `json:"latest_event_status"`
		} `json:"components"`
	} `json:"ecosystem_specific"`
}

type osvQueryRequest struct {
	Package struct {
		Name      string `json:"name"`
		Ecosystem string `json:"ecosystem"`
	} `json:"package"`
	Version   string `json:"version"`
	PageToken string `json:"page_token,omitempty"`
}

type osvQueryResponse struct {
	Vulns         []OSVVuln `json:"vulns"`
	NextPageToken string    `json:"next_page_token"`
}

// Query returns the vulnerabilities OSV.dev lists for a package version,
// following pagination. Results are cached by ecosystem, name and version.
func (o *OSV) Query(ctx context.Context, pkg OSVPackage) ([]OSVVuln, error) {
	vulns, _, err := o.query(ctx, pkg)
	return vulns, err
}

// query is Query that also returns when the result was fetched.
func (o *OSV) query(ctx context.Context, pkg OSVPackage) ([]OSVVuln, time.Time, error) {
	o.mu.Lock()
	if e, ok := o.cache[pkg]; ok && o.now().Sub(e.fetched) < o.ttl(pkg) {
		o.mu.Unlock()
		return e.vulns, e.fetched, nil
	}
	o.mu.Unlock()

	var vulns []OSVVuln
	token := ""
	for {
		page, err := o.queryPage(ctx, pkg, token)
		if err != nil {
			return nil, time.Time{}, err
		}
		vulns = append(vulns, page.Vulns...)
		if page.NextPageToken == "" {
			break
		}
		token = page.NextPageToken
	}

	o.mu.Lock()
	if o.cache == nil {
		o.cache = make(map[OSVPackage]osvCacheEntry)
	}
	if len(o.cache)%1000 == 999 {
		for k, e := range o.cache {
			if o.now().Sub(e.fetched) >= o.ttl(k) {
				delete(o.cache, k)
			}
		}
	}
	fetched := o.now()
	o.cache[pkg] = osvCacheEntry{vulns: vulns, fetched: fetched}
	o.mu.Unlock()
	return vulns, fetched, nil
}

// queryPage makes one request, retrying once on a server error.
func (o *OSV) queryPage(ctx context.Context, pkg OSVPackage, token string) (*osvQueryResponse, error) {
	var req osvQueryRequest
	req.Package.Name, req.Package.Ecosystem = pkg.Name, pkg.Ecosystem
	req.Version, req.PageToken = pkg.Version, token
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	var lastErr error
	for attempt := range 2 {
		if attempt > 0 {
			select {
			case <-time.After(300 * time.Millisecond):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		page, retry, err := o.post(ctx, body)
		if err == nil {
			return page, nil
		}
		lastErr = fmt.Errorf("%s %s@%s: %w", pkg.Ecosystem, pkg.Name, pkg.Version, err)
		if !retry {
			break
		}
	}
	return nil, lastErr
}

func (o *OSV) post(ctx context.Context, body []byte) (*osvQueryResponse, bool, error) {
	select {
	case o.sem <- struct{}{}:
		defer func() { <-o.sem }()
	case <-ctx.Done():
		return nil, false, ctx.Err()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimSuffix(o.URL, "/")+"/v1/query", bytes.NewReader(body))
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := o.HTTP.Do(req)
	if err != nil {
		return nil, ctx.Err() == nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
		return nil, resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests, fmt.Errorf("unexpected status %s", resp.Status)
	}
	var page osvQueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&page); err != nil {
		return nil, false, fmt.Errorf("decoding response: %w", err)
	}
	return &page, false, nil
}

// QueryAll looks up every package and returns the vulnerabilities by
// package. progress, if set, is called after each package with the number
// done and the total. The first failure cancels the rest, since a partial
// result must not be shown as complete (UC-11).
func (o *OSV) QueryAll(ctx context.Context, pkgs []OSVPackage, progress func(done, total int)) (map[OSVPackage][]OSVVuln, error) {
	out, _, err := o.QueryAllAt(ctx, pkgs, progress)
	return out, err
}

// QueryAllAt is QueryAll that also returns the oldest time any of the results
// was fetched, so a cached answer reports its real age (OA-12.1).
func (o *OSV) QueryAllAt(ctx context.Context, pkgs []OSVPackage, progress func(done, total int)) (map[OSVPackage][]OSVVuln, time.Time, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var (
		mu       sync.Mutex
		wg       sync.WaitGroup
		out      = make(map[OSVPackage][]OSVVuln, len(pkgs))
		done     int
		firstErr error
		oldest   time.Time
		ch       = make(chan OSVPackage)
	)
	for range osvMaxInFlight {
		wg.Go(func() {
			for pkg := range ch {
				vulns, fetched, err := o.query(ctx, pkg)
				mu.Lock()
				if err != nil {
					if firstErr == nil {
						firstErr = err
						cancel()
					}
				} else {
					out[pkg] = vulns
					if oldest.IsZero() || fetched.Before(oldest) {
						oldest = fetched
					}
					done++
					if progress != nil {
						progress(done, len(pkgs))
					}
				}
				mu.Unlock()
			}
		})
	}
feed:
	for _, pkg := range pkgs {
		select {
		case ch <- pkg:
		case <-ctx.Done():
			break feed
		}
	}
	close(ch)
	wg.Wait()
	if firstErr != nil {
		return nil, time.Time{}, &SourceError{Source: "OSV.dev", Err: firstErr}
	}
	if err := ctx.Err(); err != nil {
		return nil, time.Time{}, err
	}
	return out, oldest, nil
}

// FixStatus says whether a known vulnerability can be fixed by updating.
type FixStatus string

const (
	// FixAvailable: the distribution has released a fixed version newer than
	// the one installed.
	FixAvailable FixStatus = "fix_available"
	// NoFixYet: no fixed version exists, including Chainguard advisories
	// pending an upstream fix.
	NoFixYet FixStatus = "no_fix_yet"
	// WontFix: the distribution has marked it as one it won't fix, such as
	// Debian's "unimportant" and "end-of-life" urgencies.
	WontFix FixStatus = "wont_fix"
)

// classify reads a vulnerability's fix status from the advisory for the
// package's own name and ecosystem, so a fix released only for another
// release (Debian 13's) doesn't count for this one (UC-5.3). OSV's query
// already returns only records that affect the installed version.
func classify(v OSVVuln, pkg OSVPackage) FixStatus {
	for _, a := range v.Affected {
		if a.Package.Name != pkg.Name || a.Package.Ecosystem != pkg.Ecosystem {
			continue
		}
		for _, r := range a.Ranges {
			for _, e := range r.Events {
				if e.Fixed != "" {
					return FixAvailable
				}
			}
		}
		switch a.EcosystemSpecific.Urgency {
		case "unimportant", "end-of-life":
			return WontFix
		}
		return NoFixYet
	}
	return NoFixYet
}

// VulnCounts counts distinct known vulnerabilities by fix status.
type VulnCounts struct {
	Total        int `json:"total"`
	FixAvailable int `json:"fix_available" jsonschema:"a fixed version exists that is newer than the one installed"`
	NoFixYet     int `json:"no_fix_yet" jsonschema:"no fixed version exists yet, including advisories pending an upstream fix"`
	WontFix      int `json:"wont_fix" jsonschema:"the distribution has marked these as ones it won't fix"`
}

// fixRank orders statuses when one vulnerability has different statuses for
// different packages: a fix for any package wins (UC-5).
var fixRank = map[FixStatus]int{FixAvailable: 3, NoFixYet: 2, WontFix: 1}

// classifyAll gives each vulnerability (see vulnKeys) across all packages and
// records one status, however many packages or records it appears in
// (UC-4.2), with the best fix status winning.
func classifyAll(results map[OSVPackage][]OSVVuln) map[string]FixStatus {
	status := make(map[string]FixStatus)
	for pkg, vulns := range results {
		for _, v := range vulns {
			s := classify(v, pkg)
			for _, key := range vulnKeys(v) {
				if prev, ok := status[key]; !ok || fixRank[s] > fixRank[prev] {
					status[key] = s
				}
			}
		}
	}
	return status
}

// summarize counts the distinct vulnerabilities by fix status.
func summarize(results map[OSVPackage][]OSVVuln) VulnCounts {
	status := classifyAll(results)
	c := VulnCounts{Total: len(status)}
	for _, s := range status {
		switch s {
		case FixAvailable:
			c.FixAvailable++
		case WontFix:
			c.WontFix++
		default:
			c.NoFixYet++
		}
	}
	return c
}
