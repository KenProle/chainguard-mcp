package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
)

const defaultEndOfLifeURL = "https://endoflife.date"

// eolProducts maps official Docker Hub image names to their endoflife.date
// product and display name. Only confident mappings are listed; an image not
// here gets no end-of-life note (UC-8.2).
var eolProducts = map[string]struct{ slug, name string }{
	"node":      {"nodejs", "Node.js"},
	"python":    {"python", "Python"},
	"golang":    {"go", "Go"},
	"postgres":  {"postgresql", "PostgreSQL"},
	"nginx":     {"nginx", "nginx"},
	"redis":     {"redis", "Redis"},
	"mysql":     {"mysql", "MySQL"},
	"mariadb":   {"mariadb", "MariaDB"},
	"ruby":      {"ruby", "Ruby"},
	"php":       {"php", "PHP"},
	"mongo":     {"mongodb", "MongoDB"},
	"memcached": {"memcached", "Memcached"},
	"rabbitmq":  {"rabbitmq", "RabbitMQ"},
	"haproxy":   {"haproxy", "HAProxy"},
	"traefik":   {"traefik", "Traefik"},
	"tomcat":    {"tomcat", "Tomcat"},
	"ubuntu":    {"ubuntu", "Ubuntu"},
	"debian":    {"debian", "Debian"},
	"alpine":    {"alpine", "Alpine Linux"},
}

// EndOfLife looks up support end dates on endoflife.date.
type EndOfLife struct {
	HTTP *http.Client
	URL  string
	TTL  time.Duration
	// Now is the clock, replaceable in tests.
	Now func() time.Time

	mu    sync.Mutex
	cache map[string]eolCacheEntry
}

type eolCacheEntry struct {
	cycles  []eolCycle
	fetched time.Time
}

type eolCycle struct {
	Cycle string `json:"cycle"`
	// EOL is a date, or true/false for releases with no date; only dates are used.
	EOL json.RawMessage `json:"eol"`
}

func NewEndOfLife(client *http.Client) *EndOfLife {
	return &EndOfLife{HTTP: client, URL: defaultEndOfLifeURL, TTL: 24 * time.Hour, Now: time.Now}
}

// EndOfLifeNote says an upstream release is no longer supported.
type EndOfLifeNote struct {
	Product string `json:"product" jsonschema:"product name, e.g. Node.js"`
	Cycle   string `json:"cycle" jsonschema:"release cycle, e.g. 20"`
	Date    string `json:"date" jsonschema:"end-of-life date, YYYY-MM-DD"`
}

// tagVersion matches the version a tag starts with, e.g. "20" in "20-alpine".
var tagVersion = regexp.MustCompile(`^v?([0-9]+(?:\.[0-9]+)*)`)

// Note returns a note when the image's release has passed its end-of-life
// date, or nil when the image or tag doesn't map confidently to a release,
// the release is still supported, or the lookup fails: a missing note never
// fails a comparison (UC-11).
func (e *EndOfLife) Note(ctx context.Context, repo, tag string) *EndOfLifeNote {
	name, official := strings.CutPrefix(repo, "library/")
	product, mapped := eolProducts[name]
	m := tagVersion.FindStringSubmatch(tag)
	if !official || !mapped || m == nil {
		return nil
	}
	cycles, err := e.cycles(ctx, product.slug)
	if err != nil {
		log.Printf("endoflife.date lookup for %s failed: %v", product.slug, err)
		return nil
	}
	// A tag may be more precise than the cycle (20.11.1 → 20), so try shorter
	// prefixes of its version until one matches.
	version := strings.Split(m[1], ".")
	for n := len(version); n > 0; n-- {
		want := strings.Join(version[:n], ".")
		for _, c := range cycles {
			if c.Cycle != want {
				continue
			}
			var date string
			if json.Unmarshal(c.EOL, &date) != nil || date == "" {
				return nil
			}
			end, err := time.Parse(time.DateOnly, date)
			if err != nil || end.After(e.Now()) {
				return nil
			}
			return &EndOfLifeNote{Product: product.name, Cycle: c.Cycle, Date: date}
		}
	}
	return nil
}

func (e *EndOfLife) cycles(ctx context.Context, slug string) ([]eolCycle, error) {
	e.mu.Lock()
	if c, ok := e.cache[slug]; ok && time.Since(c.fetched) < e.TTL {
		e.mu.Unlock()
		return c.cycles, nil
	}
	e.mu.Unlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSuffix(e.URL, "/")+"/api/"+slug+".json", nil)
	if err != nil {
		return nil, err
	}
	resp, err := e.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %s", resp.Status)
	}
	var cycles []eolCycle
	if err := json.NewDecoder(resp.Body).Decode(&cycles); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}
	e.mu.Lock()
	if e.cache == nil {
		e.cache = make(map[string]eolCacheEntry)
	}
	e.cache[slug] = eolCacheEntry{cycles: cycles, fetched: time.Now()}
	e.mu.Unlock()
	return cycles, nil
}
