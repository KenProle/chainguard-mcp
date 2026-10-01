package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	defaultRegistryURL = "https://cgr.dev"
	registryHost       = "cgr.dev"
	defaultNamespace   = "chainguard"
)

// Media types accepted when fetching manifests.
const (
	mediaOCIIndex      = "application/vnd.oci.image.index.v1+json"
	mediaDockerList    = "application/vnd.docker.distribution.manifest.list.v2+json"
	mediaOCIManifest   = "application/vnd.oci.image.manifest.v1+json"
	mediaDockerV2      = "application/vnd.docker.distribution.manifest.v2+json"
	acceptAnyManifest  = mediaOCIIndex + ", " + mediaDockerList + ", " + mediaOCIManifest + ", " + mediaDockerV2
	acceptManifestOnly = mediaOCIManifest + ", " + mediaDockerV2
)

var (
	// ErrNotPublic is returned when the registry refuses anonymous access to
	// an image, which is the case for images outside Chainguard's free tier.
	ErrNotPublic = errors.New("image is not publicly accessible (likely requires a Chainguard subscription)")
	// ErrNotFound is returned when a tag, manifest or blob doesn't exist.
	ErrNotFound = errors.New("not found")
)

// Registry is a minimal anonymous client for the cgr.dev OCI registry.
type Registry struct {
	HTTP      *http.Client
	BaseURL   string
	Namespace string
	// PublicTTL is how long a free/paid result from IsPublic is cached.
	PublicTTL time.Duration

	mu     sync.Mutex
	tokens map[string]cachedToken
	public map[string]publicStatus
}

type cachedToken struct {
	token   string
	expires time.Time
}

type publicStatus struct {
	public  bool
	checked time.Time
}

func NewRegistry(client *http.Client) *Registry {
	return &Registry{
		HTTP:      client,
		BaseURL:   defaultRegistryURL,
		Namespace: defaultNamespace,
		PublicTTL: 24 * time.Hour,
	}
}

// Ref returns the pullable reference for an image, e.g. cgr.dev/chainguard/python.
func (r *Registry) Ref(image string) string {
	return registryHost + "/" + r.Namespace + "/" + image
}

func (r *Registry) repo(image string) string { return r.Namespace + "/" + image }

// token returns an anonymous pull token for image, cached until shortly
// before it expires. The registry refuses tokens for non-public images.
func (r *Registry) token(ctx context.Context, image string) (string, error) {
	r.mu.Lock()
	if t, ok := r.tokens[image]; ok && time.Now().Before(t.expires) {
		r.mu.Unlock()
		return t.token, nil
	}
	r.mu.Unlock()

	q := url.Values{"service": {registryHost}, "scope": {"repository:" + r.repo(image) + ":pull"}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.BaseURL+"/token?"+q.Encode(), nil)
	if err != nil {
		return "", err
	}
	resp, err := r.HTTP.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching registry token: %w", err)
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound:
		r.setPublic(image, false)
		return "", ErrNotPublic
	default:
		return "", fmt.Errorf("fetching registry token: unexpected status %s", resp.Status)
	}

	var body struct {
		Token     string `json:"token"`
		ExpiresIn int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", fmt.Errorf("decoding registry token: %w", err)
	}
	lifetime := time.Duration(body.ExpiresIn) * time.Second
	if lifetime <= 0 {
		lifetime = 5 * time.Minute
	}
	r.mu.Lock()
	if r.tokens == nil {
		r.tokens = make(map[string]cachedToken)
	}
	// Refresh a minute early so a token never expires mid-request.
	r.tokens[image] = cachedToken{token: body.Token, expires: time.Now().Add(lifetime - time.Minute)}
	r.mu.Unlock()
	r.setPublic(image, true)
	return body.Token, nil
}

func (r *Registry) setPublic(image string, public bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.public == nil {
		r.public = make(map[string]publicStatus)
	}
	r.public[image] = publicStatus{public: public, checked: time.Now()}
}

// IsPublic reports whether image can be pulled anonymously (free tier).
func (r *Registry) IsPublic(ctx context.Context, image string) (bool, error) {
	r.mu.Lock()
	s, ok := r.public[image]
	r.mu.Unlock()
	if ok && time.Since(s.checked) < r.PublicTTL {
		return s.public, nil
	}
	_, err := r.token(ctx, image)
	if errors.Is(err, ErrNotPublic) {
		return false, nil
	}
	return err == nil, err
}

// PublicStatus checks many images concurrently and returns name → free.
// Images whose check fails are left out of the result.
func (r *Registry) PublicStatus(ctx context.Context, images []string) map[string]bool {
	const workers = 8
	var (
		mu  sync.Mutex
		wg  sync.WaitGroup
		out = make(map[string]bool, len(images))
		ch  = make(chan string)
	)
	for range workers {
		wg.Go(func() {
			for image := range ch {
				public, err := r.IsPublic(ctx, image)
				if err != nil {
					continue
				}
				mu.Lock()
				out[image] = public
				mu.Unlock()
			}
		})
	}
	for _, image := range images {
		ch <- image
	}
	close(ch)
	wg.Wait()
	return out
}

// get performs an authenticated GET against the image's repository, e.g.
// path "tags/list" or "manifests/latest". The caller closes the body.
func (r *Registry) get(ctx context.Context, image, path, accept string) (*http.Response, error) {
	token, err := r.token(ctx, image)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.BaseURL+"/v2/"+r.repo(image)+"/"+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	resp, err := r.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching %s: %w", path, err)
	}
	switch resp.StatusCode {
	case http.StatusOK:
		return resp, nil
	case http.StatusUnauthorized, http.StatusForbidden:
		resp.Body.Close()
		return nil, ErrNotPublic
	case http.StatusNotFound:
		resp.Body.Close()
		return nil, fmt.Errorf("%s/%s: %w", image, path, ErrNotFound)
	default:
		resp.Body.Close()
		return nil, fmt.Errorf("fetching %s: unexpected status %s", path, resp.Status)
	}
}

// getJSON GETs path, decodes the JSON body into v and returns the
// Docker-Content-Digest header (empty for non-manifest responses).
func (r *Registry) getJSON(ctx context.Context, image, path, accept string, v any) (string, error) {
	resp, err := r.get(ctx, image, path, accept)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		return "", fmt.Errorf("decoding %s: %w", path, err)
	}
	return resp.Header.Get("Docker-Content-Digest"), nil
}

// Tags returns the human-readable tags for an image. Signature, attestation
// and SBOM tags (sha256-*) are filtered out.
func (r *Registry) Tags(ctx context.Context, image string) ([]string, error) {
	var body struct {
		Tags []string `json:"tags"`
	}
	if _, err := r.getJSON(ctx, image, "tags/list", "", &body); err != nil {
		return nil, err
	}
	tags := []string{}
	for _, t := range body.Tags {
		if !strings.HasPrefix(t, "sha256-") {
			tags = append(tags, t)
		}
	}
	sort.Strings(tags)
	return tags, nil
}

// Descriptor points at a blob or manifest.
type Descriptor struct {
	MediaType   string            `json:"mediaType"`
	Digest      string            `json:"digest"`
	Size        int64             `json:"size"`
	Platform    *OCIPlatform      `json:"platform,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

type OCIPlatform struct {
	OS           string `json:"os"`
	Architecture string `json:"architecture"`
	Variant      string `json:"variant,omitempty"`
}

func (p *OCIPlatform) String() string {
	s := p.OS + "/" + p.Architecture
	if p.Variant != "" {
		s += "/" + p.Variant
	}
	return s
}

// Manifest is either an image index (Manifests set) or an image manifest
// (Config and Layers set).
type Manifest struct {
	MediaType   string            `json:"mediaType"`
	Manifests   []Descriptor      `json:"manifests,omitempty"`
	Config      Descriptor        `json:"config"`
	Layers      []Descriptor      `json:"layers,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

func (m *Manifest) IsIndex() bool {
	return m.MediaType == mediaOCIIndex || m.MediaType == mediaDockerList || len(m.Manifests) > 0
}

// Manifest fetches a manifest by tag or digest and returns it with its digest.
func (r *Registry) Manifest(ctx context.Context, image, ref string, accept string) (*Manifest, string, error) {
	var m Manifest
	digest, err := r.getJSON(ctx, image, "manifests/"+ref, accept, &m)
	if err != nil {
		return nil, "", err
	}
	return &m, digest, nil
}

// Blob fetches a blob by digest and returns its raw bytes.
func (r *Registry) Blob(ctx context.Context, image, digest string, maxBytes int64) ([]byte, error) {
	resp, err := r.get(ctx, image, "blobs/"+digest, "")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("reading blob %s: %w", digest, err)
	}
	if int64(len(b)) > maxBytes {
		return nil, fmt.Errorf("blob %s exceeds %d bytes", digest, maxBytes)
	}
	return b, nil
}

// PlatformManifest resolves tag to the image manifest for arch (e.g. "amd64").
// It returns the manifest, its digest and the tag's top-level digest.
func (r *Registry) PlatformManifest(ctx context.Context, image, tag, arch string) (*Manifest, string, string, error) {
	top, topDigest, err := r.Manifest(ctx, image, tag, acceptAnyManifest)
	if err != nil {
		return nil, "", "", err
	}
	if !top.IsIndex() {
		return top, topDigest, topDigest, nil
	}
	var available []string
	for _, d := range top.Manifests {
		if d.Platform == nil {
			continue
		}
		available = append(available, d.Platform.Architecture)
		if d.Platform.Architecture == arch {
			m, digest, err := r.Manifest(ctx, image, d.Digest, acceptManifestOnly)
			if err != nil {
				return nil, "", "", err
			}
			if digest == "" {
				digest = d.Digest
			}
			return m, digest, topDigest, nil
		}
	}
	return nil, "", "", fmt.Errorf("%s:%s has no %s variant (available: %s)", image, tag, arch, strings.Join(available, ", "))
}
