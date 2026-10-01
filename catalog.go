package main

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	defaultSitemapURL  = "https://images.chainguard.dev/sitemap.xml"
	defaultRegistryURL = "https://cgr.dev"
	defaultRepoPrefix  = "chainguard"
)

// ErrNotPublic is returned when the registry refuses anonymous access to an
// image's tags, which is the case for images outside Chainguard's free tier.
var ErrNotPublic = errors.New("image tags are not publicly accessible (likely requires a Chainguard subscription)")

// imagePath matches image overview pages in the directory sitemap.
var imagePath = regexp.MustCompile(`^/directory/image/([^/]+)/overview$`)

// Catalog lists Chainguard images from the public images directory and looks
// up tags from the cgr.dev registry. The image list is cached for ttl.
type Catalog struct {
	HTTP        *http.Client
	SitemapURL  string
	RegistryURL string
	TTL         time.Duration

	mu        sync.Mutex
	images    []string
	fetchedAt time.Time
}

func NewCatalog() *Catalog {
	return &Catalog{
		HTTP:        &http.Client{Timeout: 30 * time.Second},
		SitemapURL:  defaultSitemapURL,
		RegistryURL: defaultRegistryURL,
		TTL:         time.Hour,
	}
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

	var sitemap struct {
		URLs []struct {
			Loc string `xml:"loc"`
		} `xml:"url"`
	}
	if err := xml.NewDecoder(resp.Body).Decode(&sitemap); err != nil {
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

// Tags returns the human-readable tags for an image (signature, attestation
// and SBOM tags are filtered out). Only publicly pullable images work.
func (c *Catalog) Tags(ctx context.Context, image string) ([]string, error) {
	repo := defaultRepoPrefix + "/" + image
	token, err := c.anonymousToken(ctx, repo)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.RegistryURL+"/v2/"+repo+"/tags/list", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("listing tags: %w", err)
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound:
		return nil, ErrNotPublic
	default:
		return nil, fmt.Errorf("listing tags: unexpected status %s", resp.Status)
	}

	var body struct {
		Tags []string `json:"tags"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("decoding tags: %w", err)
	}
	var tags []string
	for _, t := range body.Tags {
		if !strings.HasPrefix(t, "sha256-") {
			tags = append(tags, t)
		}
	}
	sort.Strings(tags)
	return tags, nil
}

func (c *Catalog) anonymousToken(ctx context.Context, repo string) (string, error) {
	q := url.Values{"service": {"cgr.dev"}, "scope": {"repository:" + repo + ":pull"}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.RegistryURL+"/token?"+q.Encode(), nil)
	if err != nil {
		return "", err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching registry token: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", ErrNotPublic
	}
	var body struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", fmt.Errorf("decoding registry token: %w", err)
	}
	return body.Token, nil
}
