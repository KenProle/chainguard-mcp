package main

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"sync"
	"time"
)

const defaultSitemapURL = "https://images.chainguard.dev/sitemap.xml"

// imagePath matches image overview pages in the directory sitemap.
var imagePath = regexp.MustCompile(`^/directory/image/([^/]+)/overview$`)

// Catalog lists Chainguard images from the public images directory. The
// cgr.dev registry doesn't support the catalog API, so the directory's
// sitemap is the source of truth. The list is cached for TTL.
type Catalog struct {
	HTTP       *http.Client
	SitemapURL string
	TTL        time.Duration

	mu        sync.Mutex
	images    []string
	fetchedAt time.Time
}

func NewCatalog(client *http.Client) *Catalog {
	return &Catalog{HTTP: client, SitemapURL: defaultSitemapURL, TTL: time.Hour}
}

// Images returns the sorted list of image names in the Chainguard directory.
func (c *Catalog) Images(ctx context.Context) ([]string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.images != nil && time.Since(c.fetchedAt) < c.TTL {
		return c.images, nil
	}
	images, err := c.fetchImages(ctx)
	if err != nil {
		if c.images != nil {
			// Serve stale data rather than failing outright.
			return c.images, nil
		}
		return nil, err
	}
	c.images, c.fetchedAt = images, time.Now()
	return images, nil
}

func (c *Catalog) fetchImages(ctx context.Context) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.SitemapURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching sitemap: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching sitemap: unexpected status %s", resp.Status)
	}

	return parseSitemap(resp.Body)
}

// parseSitemap returns the sorted, de-duplicated image names listed in an
// images.chainguard.dev sitemap. The live catalog and the history backfill
// share it so both agree on what counts as an image.
func parseSitemap(r io.Reader) ([]string, error) {
	var sitemap struct {
		URLs []struct {
			Loc string `xml:"loc"`
		} `xml:"url"`
	}
	if err := xml.NewDecoder(r).Decode(&sitemap); err != nil {
		return nil, fmt.Errorf("decoding sitemap: %w", err)
	}

	seen := make(map[string]bool)
	var images []string
	for _, u := range sitemap.URLs {
		parsed, err := url.Parse(u.Loc)
		if err != nil {
			continue
		}
		m := imagePath.FindStringSubmatch(parsed.Path)
		if m == nil || seen[m[1]] {
			continue
		}
		seen[m[1]] = true
		images = append(images, m[1])
	}
	if len(images) == 0 {
		return nil, errors.New("no images found in sitemap")
	}
	sort.Strings(images)
	return images, nil
}

// Contains reports whether image is in the catalog.
func (c *Catalog) Contains(ctx context.Context, image string) (bool, error) {
	images, err := c.Images(ctx)
	if err != nil {
		return false, err
	}
	_, found := sort.Find(len(images), func(i int) int {
		switch {
		case image < images[i]:
			return -1
		case image > images[i]:
			return 1
		}
		return 0
	})
	return found, nil
}
