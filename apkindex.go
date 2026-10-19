package main

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultPackageIndexURL = "https://packages.wolfi.dev/os/x86_64/APKINDEX.tar.gz"
	packageIndexSource     = "Wolfi package index"
	// maxIndexBytes caps the decompressed index (43 MB in 2026-10) so a bad
	// response can't fill memory.
	maxIndexBytes = 512 << 20
)

// pkgInfo is what the lookup keeps of a package's newest build.
type pkgInfo struct {
	Version string
	Built   int64 // build time, Unix seconds (0 when the index has none)
	Origin  string
}

// IndexData is a parsed Wolfi package index, reduced to the newest build of
// each package name and what each package provides.
type IndexData struct {
	Pkgs map[string]pkgInfo
	// provides maps a provided token (such as "pc:libpq", "so:libssl.so.3",
	// "cmd:ping" or a virtual name), without its version constraint, to the
	// package names that provide it, in name order.
	provides map[string][]string
	// soFold maps a lower-cased "so:" token to the tokens that fold to it, so
	// "libGL.so.1" is found from "libgl.so.1".
	soFold map[string][]string
}

// Providers returns the packages providing token.
func (d *IndexData) Providers(token string) []string { return d.provides[token] }

// ProvidersFold is Providers, also matching "so:" tokens that differ only in case.
func (d *IndexData) ProvidersFold(token string) []string {
	if got := d.provides[token]; len(got) > 0 {
		return got
	}
	var out []string
	for _, tok := range d.soFold[strings.ToLower(token)] {
		out = append(out, d.provides[tok]...)
	}
	return out
}

// PackageIndex fetches Wolfi's public x86_64 package index and keeps the
// parsed result for TTL. If a refresh fails it serves the previous copy.
type PackageIndex struct {
	HTTP *http.Client
	URL  string
	TTL  time.Duration
	// Now is the clock; nil means time.Now.
	Now func() time.Time

	mu        sync.Mutex
	data      *IndexData
	fetchedAt time.Time
}

func NewPackageIndex(client *http.Client) *PackageIndex {
	return &PackageIndex{HTTP: client, URL: defaultPackageIndexURL, TTL: 6 * time.Hour}
}

func (p *PackageIndex) now() time.Time {
	if p.Now != nil {
		return p.Now()
	}
	return time.Now()
}

// Get returns the index and when it was fetched. With no usable copy and a
// failed download it returns a *SourceError.
func (p *PackageIndex) Get(ctx context.Context) (*IndexData, time.Time, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.data != nil && p.now().Sub(p.fetchedAt) < p.TTL {
		return p.data, p.fetchedAt, nil
	}
	data, err := p.fetch(ctx)
	if err != nil {
		if p.data != nil {
			return p.data, p.fetchedAt, nil
		}
		return nil, time.Time{}, &SourceError{Source: packageIndexSource, Err: err}
	}
	p.data, p.fetchedAt = data, p.now()
	return p.data, p.fetchedAt, nil
}

func (p *PackageIndex) fetch(ctx context.Context) (*IndexData, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.URL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := p.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching package index: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching package index: unexpected status %s", resp.Status)
	}
	return parseAPKIndex(resp.Body)
}

// parseAPKIndex reads an APKINDEX.tar.gz as a stream: gzip, then tar, then
// the APKINDEX member's line-based records (blank-line separated, one letter
// per field). Only P (name), V (version), o (origin), t (build time) and p
// (provides) are used.
func parseAPKIndex(r io.Reader) (*IndexData, error) {
	zr, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("reading package index: %w", err)
	}
	tr := tar.NewReader(zr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil, errors.New("package index has no APKINDEX file")
		}
		if err != nil {
			return nil, fmt.Errorf("reading package index: %w", err)
		}
		if hdr.Name == "APKINDEX" {
			return parseAPKIndexText(io.LimitReader(tr, maxIndexBytes))
		}
	}
}

type indexBuild struct {
	pkgInfo
	provides []string
}

func parseAPKIndexText(r io.Reader) (*IndexData, error) {
	newest := map[string]*indexBuild{}
	var name, version, origin string
	var built int64
	var provides []string
	flush := func() {
		if name == "" {
			return
		}
		b := &indexBuild{pkgInfo{Version: version, Built: built, Origin: origin}, provides}
		if cur, ok := newest[name]; !ok || b.Built > cur.Built || (b.Built == cur.Built && compareAPKVersions(b.Version, cur.Version) > 0) {
			newest[name] = b
		}
		name, version, origin, built, provides = "", "", "", 0, nil
	}

	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1<<20), 8<<20)
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			flush()
			continue
		}
		if len(line) < 2 || line[1] != ':' {
			continue
		}
		val := line[2:]
		switch line[0] {
		case 'P':
			name = val
		case 'V':
			version = val
		case 'o':
			origin = val
		case 't':
			built, _ = strconv.ParseInt(val, 10, 64)
		case 'p':
			provides = strings.Fields(val)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("reading package index: %w", err)
	}
	flush()
	if len(newest) == 0 {
		return nil, errors.New("package index lists no packages")
	}

	d := &IndexData{
		Pkgs:     make(map[string]pkgInfo, len(newest)),
		provides: map[string][]string{},
		soFold:   map[string][]string{},
	}
	for n, b := range newest {
		d.Pkgs[n] = b.pkgInfo
		for _, tok := range b.provides {
			if i := strings.IndexAny(tok, "=<>~"); i >= 0 {
				tok = tok[:i]
			}
			if tok == "" || tok == n {
				continue
			}
			d.provides[tok] = append(d.provides[tok], n)
		}
	}
	for tok, names := range d.provides {
		sort.Strings(names)
		d.provides[tok] = names
		if strings.HasPrefix(tok, "so:") {
			low := strings.ToLower(tok)
			d.soFold[low] = append(d.soFold[low], tok)
		}
	}
	for low, toks := range d.soFold {
		sort.Strings(toks)
		d.soFold[low] = toks
	}
	return d, nil
}
