package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeChainguard is an in-process stand-in for the sitemap, the cgr.dev
// registry and the Wolfi security database.
// fakeStore holds the manifests, blobs and tags a fake registry serves.
type fakeStore struct {
	t         *testing.T
	tags      map[string]map[string]string // image → tag → manifest digest
	manifests map[string]fakeBlob          // digest → manifest
	blobs     map[string][]byte            // digest → blob
}

func newFakeStore(t *testing.T) fakeStore {
	return fakeStore{t: t, tags: map[string]map[string]string{}, manifests: map[string]fakeBlob{}, blobs: map[string][]byte{}}
}

type fakeChainguard struct {
	fakeStore
	images []string        // catalog, in sitemap order
	public map[string]bool // images that get anonymous tokens
	secdb  string

	tokenRequests atomic.Int64 // requests to /token, for asserting none were made
	blobRequests  atomic.Int64 // blob requests, one per SBOM read
}

type fakeBlob struct {
	mediaType string
	body      []byte
}

func digestOf(b []byte) string { return fmt.Sprintf("sha256:%x", sha256.Sum256(b)) }

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func (f *fakeStore) addBlob(v any) Descriptor {
	b := mustJSON(f.t, v)
	d := digestOf(b)
	f.blobs[d] = b
	return Descriptor{MediaType: "application/octet-stream", Digest: d, Size: int64(len(b))}
}

func (f *fakeStore) addManifest(mediaType string, v any) Descriptor {
	b := mustJSON(f.t, v)
	d := digestOf(b)
	f.manifests[d] = fakeBlob{mediaType, b}
	return Descriptor{MediaType: mediaType, Digest: d, Size: int64(len(b))}
}

func (f *fakeStore) tag(image, tag, digest string) {
	if f.tags[image] == nil {
		f.tags[image] = make(map[string]string)
	}
	f.tags[image][tag] = digest
}

// addImage publishes a two-platform image whose amd64 variant carries an
// SBOM attestation listing pkgs (each "name@version" from origin).
func (f *fakeChainguard) addImage(image, tag, user string, pkgs []fakePkg) string {
	var platforms []Descriptor
	for _, arch := range []string{"amd64", "arm64"} {
		cfg := f.addBlob(map[string]any{
			"os": "linux", "architecture": arch, "created": "2026-01-02T03:04:05Z",
			"config": map[string]any{"User": user, "Entrypoint": []string{"/usr/bin/" + image}, "Env": []string{"PATH=/usr/bin"}},
		})
		layer := Descriptor{MediaType: "application/vnd.oci.image.layer.v1.tar+gzip", Digest: "sha256:" + strings.Repeat("a", 64), Size: 1000}
		m := f.addManifest(mediaOCIManifest, Manifest{MediaType: mediaOCIManifest, Config: cfg, Layers: []Descriptor{layer}})
		m.Platform = &OCIPlatform{OS: "linux", Architecture: arch}
		platforms = append(platforms, m)
		f.addSBOM(image, m.Digest, pkgs)
	}
	idx := f.addManifest(mediaOCIIndex, Manifest{
		MediaType:   mediaOCIIndex,
		Manifests:   platforms,
		Annotations: map[string]string{"org.opencontainers.image.source": "https://example.com/" + image},
	})
	f.tag(image, tag, idx.Digest)
	return idx.Digest
}

type fakePkg struct{ name, version, origin string }

// addSBOM attaches an SPDX attestation shaped like apko's: each package is
// listed twice and linked to its origin's melange file via DESCRIBED_BY.
func (f *fakeChainguard) addSBOM(image, digest string, pkgs []fakePkg) {
	var packages, relationships []map[string]any
	purl := func(s string) []map[string]string {
		return []map[string]string{{"referenceType": "purl", "referenceLocator": s}}
	}
	for i, p := range pkgs {
		id := fmt.Sprintf("SPDXRef-Package-apk-%s", p.name)
		melange := fmt.Sprintf("SPDXRef-Package-Melange-%s-%d", p.origin, i)
		packages = append(packages,
			map[string]any{"SPDXID": id, "name": p.name, "versionInfo": p.version, "licenseDeclared": "MIT",
				"externalRefs": purl("pkg:apk/wolfi/" + p.name + "@" + p.version + "?arch=x86_64&distro=wolfi")},
			map[string]any{"SPDXID": id + "-dup", "name": p.name, "versionInfo": p.version, "licenseDeclared": "NOASSERTION",
				"externalRefs": purl("pkg:apk/wolfi/" + p.name + "@" + p.version + "?arch=x86_64&origin=" + p.name)},
			map[string]any{"SPDXID": melange, "name": p.origin + ".yaml", "versionInfo": "abc123",
				"externalRefs": purl("pkg:github/chainguard-dev/stereo@abc123#" + p.origin + ".yaml")},
		)
		relationships = append(relationships, map[string]any{"spdxElementId": id, "relationshipType": "DESCRIBED_BY", "relatedSpdxElement": melange})
	}
	statement := map[string]any{
		"_type":         "https://in-toto.io/Statement/v0.1",
		"predicateType": predicateSPDX,
		"predicate":     map[string]any{"spdxVersion": "SPDX-2.3", "packages": packages, "relationships": relationships},
	}
	envelope := f.addBlob(map[string]any{
		"payloadType": "application/vnd.in-toto+json",
		"payload":     base64.StdEncoding.EncodeToString(mustJSON(f.t, statement)),
	})
	provenance := f.addBlob(map[string]any{"payload": ""})
	provenance.Annotations = map[string]string{"predicateType": "https://slsa.dev/provenance/v1"}
	envelope.Annotations = map[string]string{"predicateType": predicateSPDX}
	att := f.addManifest(mediaOCIManifest, Manifest{MediaType: mediaOCIManifest, Layers: []Descriptor{provenance, envelope}})
	f.tag(image, strings.Replace(digest, ":", "-", 1)+".att", att.Digest)
}

func (f *fakeChainguard) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	switch {
	case path == "/sitemap.xml":
		fmt.Fprint(w, `<?xml version="1.0"?><urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">`)
		fmt.Fprint(w, `<url><loc>https://images.chainguard.dev/directory</loc></url>`)
		for _, img := range f.images {
			for _, page := range []string{"overview", "versions", "provenance"} {
				fmt.Fprintf(w, `<url><loc>https://images.chainguard.dev/directory/image/%s/%s</loc></url>`, img, page)
			}
		}
		fmt.Fprint(w, `</urlset>`)

	case path == "/security.json":
		fmt.Fprint(w, f.secdb)

	case path == "/token":
		f.tokenRequests.Add(1)
		image := strings.TrimSuffix(strings.TrimPrefix(r.URL.Query().Get("scope"), "repository:chainguard/"), ":pull")
		if !f.public[image] {
			http.Error(w, `{"errors":[{"code":"DENIED"}]}`, http.StatusForbidden)
			return
		}
		fmt.Fprintf(w, `{"token":"tok-%s","expires_in":3600}`, image)

	case strings.HasPrefix(path, "/v2/chainguard/"):
		rest := strings.TrimPrefix(path, "/v2/chainguard/")
		image, kind, _ := strings.Cut(rest, "/")
		if r.Header.Get("Authorization") != "Bearer tok-"+image {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		kind, ref, _ := strings.Cut(kind, "/")
		switch kind {
		case "tags":
			var tags []string
			for t := range f.tags[image] {
				tags = append(tags, t)
			}
			w.Write(mustJSON(f.t, map[string]any{"name": "chainguard/" + image, "tags": tags}))
		case "manifests":
			if d, ok := f.tags[image][ref]; ok {
				ref = d
			}
			m, ok := f.manifests[ref]
			if !ok {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", m.mediaType)
			w.Header().Set("Docker-Content-Digest", ref)
			w.Write(m.body)
		case "blobs":
			f.blobRequests.Add(1)
			b, ok := f.blobs[ref]
			if !ok {
				http.NotFound(w, r)
				return
			}
			w.Write(b)
		default:
			http.NotFound(w, r)
		}

	default:
		http.NotFound(w, r)
	}
}

// newFakeService starts a fake Chainguard backend with a few images and
// returns a Service wired to it.
func newFakeService(t *testing.T) *Service {
	t.Helper()
	svc, _ := newFakeBackend(t)
	return svc
}

// newFakeBackend is newFakeService that also returns the fake, so tests can
// inspect the requests it received.
func newFakeBackend(t *testing.T) (*Service, *fakeChainguard) {
	t.Helper()
	f := &fakeChainguard{
		fakeStore: newFakeStore(t),
		images: []string{
			"go", "jdk", "jre", "loki-fips", "node", "node-fips", "node-lts",
			"python", "python-fips", "static", "wolfi-base",
		},
		public: map[string]bool{
			"go": true, "jre": true, "node": true, "python": true, "static": true, "wolfi-base": true,
		},
		secdb: `{"packages":[
			{"pkg":{"name":"openssl","secfixes":{"0":["CVE-2023-0466"],"3.0.7-r0":["CVE-2022-3602"],"9.9.9-r0":["CVE-2099-0001"]}}},
			{"pkg":{"name":"python-3.13","secfixes":{"3.13.1-r0":["CVE-2024-0001","GHSA-aaaa-bbbb-cccc"]}}}
		]}`,
	}
	f.addImage("python", "latest", "65532", []fakePkg{
		{"libcrypto3", "3.6.5-r1", "openssl"},
		{"libssl3", "3.6.5-r1", "openssl"},
		{"python-3.13", "3.13.7-r0", "python-3.13"},
	})
	f.addImage("python", "latest-dev", "root", []fakePkg{
		{"apk-tools", "2.14.0-r0", "apk-tools"},
		{"busybox", "1.37.0-r0", "busybox"},
		{"python-3.13", "3.13.7-r0", "python-3.13"},
	})
	f.addImage("node", "latest", "65532", nil)
	f.addImage("node", "latest-dev", "65532", nil)
	f.addImage("static", "latest", "65532", nil)

	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)

	svc := NewService()
	svc.Catalog.SitemapURL = srv.URL + "/sitemap.xml"
	svc.Registry.BaseURL = srv.URL
	svc.SecDB.URL = srv.URL + "/security.json"
	// The upstream comparison's sources get unreachable addresses, so a test
	// that forgets to fake them fails instead of reaching the real services.
	const unreachable = "http://127.0.0.1:1"
	svc.Upstream.BaseURL, svc.Upstream.TokenURL = unreachable, unreachable+"/token"
	svc.OSV.URL, svc.EOL.URL, svc.Feed.URL = unreachable, unreachable, unreachable
	return svc, f
}

// fakeDockerHub is an in-process stand-in for Docker Hub's token service and
// registry. Repositories are full paths such as "library/node".
type fakeDockerHub struct {
	fakeStore
	limited       atomic.Bool  // answer 429 to registry requests
	tokenRequests atomic.Int64 // requests to /token
	requests      atomic.Int64 // manifest and blob requests
	blobRequests  atomic.Int64 // blob requests, one of which is the SBOM
	lastService   atomic.Value // service parameter of the last token request
}

// dockerImage describes an image added to a fakeDockerHub.
type dockerImage struct {
	created string          // image config creation time
	sbom    json.RawMessage // SPDX document; nil publishes no attestations
}

// addImage publishes a two-platform image index. Like BuildKit's, it carries
// an attestation manifest per platform image when d.sbom is set.
func (f *fakeDockerHub) addImage(repo, tag string, d dockerImage) string {
	var entries []Descriptor
	for _, arch := range []string{"amd64", "arm64"} {
		cfg := f.addBlob(map[string]any{"os": "linux", "architecture": arch, "created": d.created})
		layer := Descriptor{MediaType: "application/vnd.oci.image.layer.v1.tar+gzip", Digest: "sha256:" + strings.Repeat("b", 64), Size: 4000}
		m := f.addManifest(mediaOCIManifest, Manifest{MediaType: mediaOCIManifest, Config: cfg, Layers: []Descriptor{layer}})
		m.Platform = &OCIPlatform{OS: "linux", Architecture: arch}
		entries = append(entries, m)
		if d.sbom == nil {
			continue
		}
		// The statement is bare, not wrapped in DSSE as cosign's are.
		layerBlob := f.addBlob(map[string]any{"_type": "https://in-toto.io/Statement/v0.1", "predicateType": predicateSPDX, "predicate": d.sbom})
		layerBlob.Annotations = map[string]string{dockerPredicateType: predicateSPDX}
		att := f.addManifest(mediaOCIManifest, Manifest{MediaType: mediaOCIManifest, Layers: []Descriptor{layerBlob}})
		att.Platform = &OCIPlatform{OS: "unknown", Architecture: "unknown"}
		att.Annotations = map[string]string{"vnd.docker.reference.type": "attestation-manifest", dockerRefDigest: m.Digest}
		entries = append(entries, att)
	}
	idx := f.addManifest(mediaOCIIndex, Manifest{MediaType: mediaOCIIndex, Manifests: entries})
	f.tag(repo, tag, idx.Digest)
	return idx.Digest
}

func (f *fakeDockerHub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/token" {
		f.tokenRequests.Add(1)
		f.lastService.Store(r.URL.Query().Get("service"))
		repo := strings.TrimSuffix(strings.TrimPrefix(r.URL.Query().Get("scope"), "repository:"), ":pull")
		fmt.Fprintf(w, `{"token":"dh-%s","expires_in":300}`, repo)
		return
	}
	rest, ok := strings.CutPrefix(r.URL.Path, "/v2/")
	if !ok {
		http.NotFound(w, r)
		return
	}
	f.requests.Add(1)
	if f.limited.Load() {
		w.Header().Set("Retry-After", "3600")
		http.Error(w, `{"errors":[{"code":"TOOMANYREQUESTS"}]}`, http.StatusTooManyRequests)
		return
	}
	var repo, kind, ref string
	for _, k := range []string{"/manifests/", "/blobs/"} {
		if before, after, found := strings.Cut(rest, k); found {
			repo, kind, ref = before, strings.Trim(k, "/"), after
		}
	}
	if r.Header.Get("Authorization") != "Bearer dh-"+repo {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	switch kind {
	case "manifests":
		if d, ok := f.tags[repo][ref]; ok {
			ref = d
		}
		m, ok := f.manifests[ref]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", m.mediaType)
		w.Header().Set("Docker-Content-Digest", ref)
		w.Write(m.body)
	case "blobs":
		f.blobRequests.Add(1)
		b, ok := f.blobs[ref]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Write(b)
	default:
		http.NotFound(w, r)
	}
}

// newFakeDockerHub starts a fake Docker Hub and returns a Registry for it.
func newFakeDockerHub(t *testing.T) (*Registry, *fakeDockerHub) {
	t.Helper()
	f := &fakeDockerHub{fakeStore: newFakeStore(t)}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	reg := NewDockerHubRegistry(srv.Client())
	reg.BaseURL = srv.URL
	reg.TokenURL = srv.URL + "/token"
	return reg, f
}

// fakeDebPkg is an installed Debian package in a fake SBOM. An empty source
// means the package is its own source and has no GENERATED_FROM link.
type fakeDebPkg struct{ name, version, source, sourceVersion string }

// debianSPDX builds an SPDX document shaped like BuildKit's for a Debian 12
// image: binary and source packages as separate entries linked by
// GENERATED_FROM, plus the given npm package names.
func debianSPDX(t *testing.T, pkgs []fakeDebPkg, npm ...string) json.RawMessage {
	t.Helper()
	const suffix = "?os_distro=bookworm&os_name=debian&os_version=12"
	var packages, relationships []map[string]any
	sources := map[string]string{} // name@version → SPDXID
	entry := func(id, name, version, purl string) map[string]any {
		return map[string]any{"SPDXID": id, "name": name, "versionInfo": version,
			"externalRefs": []map[string]string{{"referenceType": "purl", "referenceLocator": purl}}}
	}
	for i, p := range pkgs {
		id := fmt.Sprintf("SPDXRef-Package-bin-%d", i)
		packages = append(packages, entry(id, p.name, p.version, "pkg:deb/debian/"+p.name+"@"+p.version+suffix))
		if p.source == "" {
			continue
		}
		key := p.source + "@" + p.sourceVersion
		srcID, ok := sources[key]
		if !ok {
			srcID = fmt.Sprintf("SPDXRef-Package-src-%d", len(sources))
			sources[key] = srcID
			packages = append(packages, entry(srcID, p.source, p.sourceVersion, "pkg:deb/debian/"+p.source+"@"+p.sourceVersion+suffix))
		}
		relationships = append(relationships, map[string]any{"spdxElementId": id, "relationshipType": "GENERATED_FROM", "relatedSpdxElement": srcID})
	}
	for i, n := range npm {
		packages = append(packages, entry(fmt.Sprintf("SPDXRef-Package-npm-%d", i), n, "1.0.0", "pkg:npm/"+n+"@1.0.0"))
	}
	return mustJSON(t, map[string]any{"spdxVersion": "SPDX-2.3", "packages": packages, "relationships": relationships})
}

// fakeOSV is an in-process stand-in for OSV.dev's /v1/query. Each package
// has a list of vulnerability records, returned pageSize at a time when set.
type fakeOSV struct {
	vulns    map[OSVPackage][]json.RawMessage
	pageSize int
	delay    time.Duration   // per-request delay, to observe concurrency
	failName map[string]bool // package names answered with a 500

	requests    atomic.Int64
	inFlight    atomic.Int64
	maxInFlight atomic.Int64
}

func (f *fakeOSV) add(pkg OSVPackage, records ...any) {
	if f.vulns == nil {
		f.vulns = make(map[OSVPackage][]json.RawMessage)
	}
	for _, r := range records {
		b, err := json.Marshal(r)
		if err != nil {
			panic(err)
		}
		f.vulns[pkg] = append(f.vulns[pkg], b)
	}
}

func (f *fakeOSV) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || r.URL.Path != "/v1/query" {
		http.NotFound(w, r)
		return
	}
	f.requests.Add(1)
	n := f.inFlight.Add(1)
	defer f.inFlight.Add(-1)
	for {
		m := f.maxInFlight.Load()
		if n <= m || f.maxInFlight.CompareAndSwap(m, n) {
			break
		}
	}
	time.Sleep(f.delay)

	var req osvQueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if f.failName[req.Package.Name] {
		http.Error(w, "boom", http.StatusInternalServerError)
		return
	}
	all := f.vulns[OSVPackage{Ecosystem: req.Package.Ecosystem, Name: req.Package.Name, Version: req.Version}]
	start := 0
	if req.PageToken != "" {
		start, _ = strconv.Atoi(req.PageToken)
	}
	end := len(all)
	next := ""
	if f.pageSize > 0 && start+f.pageSize < end {
		end = start + f.pageSize
		next = strconv.Itoa(end)
	}
	resp := map[string]any{"vulns": all[start:end]}
	if len(all) == 0 {
		resp = map[string]any{}
	}
	if next != "" {
		resp["next_page_token"] = next
	}
	b, _ := json.Marshal(resp)
	w.Write(b)
}

// newFakeOSV starts a fake OSV.dev and returns a client for it.
func newFakeOSV(t *testing.T) (*OSV, *fakeOSV) {
	t.Helper()
	f := &fakeOSV{}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	o := NewOSV(srv.Client())
	o.URL = srv.URL
	return o, f
}

// fakeEndOfLife is an in-process stand-in for endoflife.date's API.
type fakeEndOfLife struct {
	products map[string]string // product slug → JSON body
	requests atomic.Int64
	fail     atomic.Bool
}

func (f *fakeEndOfLife) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.requests.Add(1)
	slug := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/"), ".json")
	body, ok := f.products[slug]
	switch {
	case f.fail.Load():
		http.Error(w, "boom", http.StatusInternalServerError)
	case !ok:
		http.NotFound(w, r)
	default:
		fmt.Fprint(w, body)
	}
}

// newFakeEndOfLife starts a fake endoflife.date with Node.js's release cycles
// (as of 2026-10-11) and returns a client for it, with its clock set to that day.
func newFakeEndOfLife(t *testing.T) (*EndOfLife, *fakeEndOfLife) {
	t.Helper()
	f := &fakeEndOfLife{products: map[string]string{
		"nodejs": `[{"cycle":"24","eol":"2028-04-30"},{"cycle":"22","eol":"2027-04-30"},{"cycle":"20","eol":"2026-04-30"},{"cycle":"18","eol":"2025-04-30"},{"cycle":"17","eol":true}]`,
		"python": `[{"cycle":"3.14","eol":"2030-10-31"},{"cycle":"3.12","eol":"2028-10-31"}]`,
	}}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	e := NewEndOfLife(srv.Client())
	e.URL = srv.URL
	e.Now = func() time.Time { return time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC) }
	return e, f
}

// fakeUpstream bundles the fakes behind the upstream comparison.
type fakeUpstream struct {
	Chainguard *fakeChainguard
	Hub        *fakeDockerHub
	OSV        *fakeOSV
	EOL        *fakeEndOfLife
}

// debianVuln is an OSV record for a Debian 12 source package, fixed in
// 12.0-1 when fixed is set, otherwise with the given Debian urgency. As on
// OSV.dev, a DEBIAN-CVE-… record names its CVE in upstream; a DSA-… record
// is given the IDs it covers.
func debianVuln(id, source string, fixed bool, urgency string, upstream ...string) map[string]any {
	events := []map[string]string{{"introduced": "0"}}
	if fixed {
		events = append(events, map[string]string{"fixed": "99:9.9-1~deb12u1"})
	}
	if cve, ok := strings.CutPrefix(id, "DEBIAN-"); ok {
		upstream = []string{cve}
	}
	return map[string]any{"id": id, "upstream": upstream, "affected": []map[string]any{{
		"package":            map[string]string{"name": source, "ecosystem": "Debian:12"},
		"ranges":             []map[string]any{{"type": "ECOSYSTEM", "events": events}},
		"ecosystem_specific": map[string]string{"urgency": urgency},
	}}}
}

// chainguardVuln is a Chainguard advisory (one per package per CVE, as
// Chainguard publishes them) pending an upstream fix.
func chainguardVuln(id, pkg, cve string) map[string]any {
	return map[string]any{"id": id, "upstream": []string{cve}, "affected": []map[string]any{{
		"package":            map[string]string{"name": pkg, "ecosystem": "Wolfi"},
		"ranges":             []map[string]any{{"type": "ECOSYSTEM", "events": []map[string]string{{"introduced": "0"}}}},
		"ecosystem_specific": map[string]any{"components": []map[string]string{{"latest_event_status": "pending_upstream_fix"}}},
	}}}
}

// Versions of the fake node:20 image's packages.
const (
	fakeOpenSSLVersion = "3.0.19-1~deb12u2"
	fakeGlibcVersion   = "2.36-9+deb12u13"
)

// newFakeUpstreamService wires a Service to the fake Chainguard backend plus
// fakes for Docker Hub, OSV.dev and endoflife.date. Docker Hub has:
//   - library/node:20 (built 2026-04-22): libssl3 and libc6 with their sources
//     openssl and glibc, linux-libc-dev from the kernel source, 2 npm packages;
//   - library/python:3.12-slim, which publishes no SBOM.
//
// Chainguard's node image has glibc-2.44 and its subpackage ld-linux, busybox
// and c-ares. The node:20 vulnerabilities: openssl A (fix available, listed
// again by DSA-0001-1) and B (won't fix), glibc C (no fix yet) and A again;
// the kernel has D, which must never be counted. Chainguard has one advisory
// each for glibc-2.44 and ld-linux, both for one CVE pending an upstream fix.
func newFakeUpstreamService(t *testing.T) (*Service, *fakeUpstream) {
	t.Helper()
	svc, cg := newFakeBackend(t)
	cg.addImage("node", "latest", "65532", []fakePkg{
		{"busybox", "1.38.0-r3", "busybox"},
		{"c-ares", "1.34.8-r3", "c-ares"},
		{"glibc-2.44", "2.44-r8", "glibc-2.44"},
		{"ld-linux", "2.44-r8", "glibc-2.44"},
	})

	reg, hub := newFakeDockerHub(t)
	hub.addImage("library/node", "20", dockerImage{created: "2026-04-22T03:00:00Z", sbom: debianSPDX(t, []fakeDebPkg{
		{"libssl3", fakeOpenSSLVersion, "openssl", fakeOpenSSLVersion},
		{"libc6", fakeGlibcVersion, "glibc", fakeGlibcVersion},
		{"linux-libc-dev", "6.1.164-1", "linux", "6.1.164-1"},
	}, "@isaacs/cliui", "semver")})
	hub.addImage("library/python", "3.12-slim", dockerImage{created: "2026-09-01T00:00:00Z"})

	osv, osvFake := newFakeOSV(t)
	osvFake.add(OSVPackage{"Debian:12", "openssl", fakeOpenSSLVersion},
		debianVuln("DEBIAN-CVE-A", "openssl", true, "not yet assigned"),
		debianVuln("DEBIAN-CVE-B", "openssl", false, "unimportant"),
		debianVuln("DSA-0001-1", "openssl", true, "", "CVE-A", "DEBIAN-CVE-A"))
	osvFake.add(OSVPackage{"Debian:12", "glibc", fakeGlibcVersion},
		debianVuln("DEBIAN-CVE-C", "glibc", false, "not yet assigned"),
		debianVuln("DEBIAN-CVE-A", "glibc", false, "not yet assigned"))
	osvFake.add(OSVPackage{"Debian:12", "linux", "6.1.164-1"}, debianVuln("DEBIAN-CVE-D", "linux", true, "not yet assigned"))
	osvFake.add(OSVPackage{"Wolfi", "glibc-2.44", "2.44-r8"}, chainguardVuln("CGA-0000-0000-0001", "glibc-2.44", "CVE-2026-0001"))
	osvFake.add(OSVPackage{"Wolfi", "ld-linux", "2.44-r8"}, chainguardVuln("CGA-0000-0000-0002", "ld-linux", "CVE-2026-0001"))

	eol, eolFake := newFakeEndOfLife(t)
	svc.Upstream, svc.OSV, svc.EOL = reg, osv, eol
	return svc, &fakeUpstream{Chainguard: cg, Hub: hub, OSV: osvFake, EOL: eolFake}
}

// advisoryVuln is a Chainguard advisory in the shape OSV.dev serves it: the
// tracked IDs in upstream and one component per (name, type) pair, which
// defaults to the package itself as an apk. A non-empty fixed adds a range
// with that fixed version.
func advisoryVuln(id, pkg, status, fixed string, upstream []string, comps ...[2]string) map[string]any {
	events := []map[string]string{{"introduced": "0"}}
	if fixed != "" {
		events = append(events, map[string]string{"fixed": fixed})
	}
	if len(comps) == 0 {
		comps = [][2]string{{pkg, "apk"}}
	}
	var components []map[string]string
	for _, c := range comps {
		components = append(components, map[string]string{"component_name": c[0], "component_type": c[1], "latest_event_status": status})
	}
	return map[string]any{"id": id, "upstream": upstream, "affected": []map[string]any{{
		"package":            map[string]string{"name": pkg, "ecosystem": "Wolfi"},
		"ranges":             []map[string]any{{"type": "ECOSYSTEM", "events": events}},
		"ecosystem_specific": map[string]any{"components": components},
	}}}
}

// fakeFeed is an in-process stand-in for Chainguard's advisory feed
// (advisories.cgr.dev): one JSON file per advisory, 404 for any other ID.
type fakeFeed struct {
	records  map[string][]byte
	failID   map[string]bool // advisories answered with a 500
	delay    time.Duration
	requests atomic.Int64

	inFlight    atomic.Int64
	maxInFlight atomic.Int64
	mu          sync.Mutex
	requested   map[string]int // requests per advisory ID
}

// add publishes the current record of an advisory; its ID comes from the record.
func (f *fakeFeed) add(record map[string]any) {
	if f.records == nil {
		f.records = make(map[string][]byte)
	}
	b, err := json.Marshal(record)
	if err != nil {
		panic(err)
	}
	f.records[record["id"].(string)] = b
}

func (f *fakeFeed) requestsFor(id string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.requested[id]
}

func (f *fakeFeed) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	id, ok := strings.CutSuffix(strings.TrimPrefix(r.URL.Path, "/chainguard/v3/osv/"), ".json")
	if !ok || r.URL.Path == "/chainguard/v3/osv/"+id {
		http.NotFound(w, r)
		return
	}
	f.requests.Add(1)
	f.mu.Lock()
	if f.requested == nil {
		f.requested = make(map[string]int)
	}
	f.requested[id]++
	f.mu.Unlock()
	n := f.inFlight.Add(1)
	defer f.inFlight.Add(-1)
	for {
		m := f.maxInFlight.Load()
		if n <= m || f.maxInFlight.CompareAndSwap(m, n) {
			break
		}
	}
	time.Sleep(f.delay)
	if f.failID[id] {
		http.Error(w, "boom", http.StatusInternalServerError)
		return
	}
	rec, ok := f.records[id]
	if !ok {
		// Like the real bucket: an XML error body.
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `<?xml version='1.0' encoding='UTF-8'?><Error><Code>NoSuchKey</Code></Error>`)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(rec)
}

// newFakeFeed starts a fake advisory feed and returns a client for it.
func newFakeFeed(t *testing.T) (*AdvisoryFeed, *fakeFeed) {
	t.Helper()
	f := &fakeFeed{}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	feed := NewAdvisoryFeed(srv.Client())
	feed.URL = srv.URL
	return feed, f
}

// newFakeAdvisoryService wires a Service to the fake Chainguard backend, a
// fake OSV.dev holding Chainguard-shaped advisories for the fake python image,
// and a fake advisory feed with their current records (newFakeBackend leaves
// both unreachable). OSV.dev's copies can be older than the feed's.
//
// python latest has four vulnerabilities: CVE-2026-0101 not yet assessed
// (python-3.13), CVE-2026-0102 pending an upstream fix (two advisories on
// libssl3, one on libcrypto3), CVE-2026-0103 with a fix available (libssl3,
// fixed in 3.6.6-r0) and CVE-2026-0104 with no fix planned (python-3.13). A
// fifth, CVE-2026-0106 on libssl3, is pending in OSV.dev's copy but fixed in
// the installed version in the feed, so it is not open.
// latest-dev shares python-3.13's two and adds CVE-2026-0105 in a Rust crate
// bundled into busybox, and CVE-2026-0107 on busybox, which the feed has no
// record of (OSV.dev's copy is used).
func newFakeAdvisoryService(t *testing.T) (*Service, *fakeOSV, *fakeChainguard, *fakeFeed) {
	t.Helper()
	svc, cg := newFakeBackend(t)
	osv, fake := newFakeOSV(t)
	feed, feedFake := newFakeFeed(t)

	// both: the same record in OSV.dev and the feed; stale: pending in
	// OSV.dev, fixed in the feed.
	both := func(pkg OSVPackage, rec map[string]any) {
		fake.add(pkg, rec)
		feedFake.add(rec)
	}
	python := OSVPackage{"Wolfi", "python-3.13", "3.13.7-r0"}
	both(python, advisoryVuln("CGA-aaaa-0000-0001", "python-3.13", "detection", "", []string{"CVE-2026-0101"}))
	both(python, advisoryVuln("CGA-aaaa-0000-0004", "python-3.13", "fix_not_planned", "", []string{"CVE-2026-0104"}))
	libssl := OSVPackage{"Wolfi", "libssl3", "3.6.5-r1"}
	both(libssl, advisoryVuln("CGA-bbbb-0000-0001", "libssl3", "pending_upstream_fix", "", []string{"CVE-2026-0102", "GHSA-aaaa-bbbb-cccc"}))
	both(libssl, advisoryVuln("CGA-bbbb-0000-0002", "libssl3", "pending_upstream_fix", "", []string{"CVE-2026-0102"}))
	both(libssl, advisoryVuln("CGA-bbbb-0000-0003", "libssl3", "fixed", "3.6.6-r0", []string{"CVE-2026-0103"}))
	both(OSVPackage{"Wolfi", "libcrypto3", "3.6.5-r1"}, advisoryVuln("CGA-bbbb-0000-0004", "libcrypto3", "pending_upstream_fix", "", []string{"CVE-2026-0102"}))
	fake.add(libssl, advisoryVuln("CGA-bbbb-0000-0005", "libssl3", "pending_upstream_fix", "", []string{"CVE-2026-0106"}))
	feedFake.add(advisoryVuln("CGA-bbbb-0000-0005", "libssl3", "fixed", "3.6.5-r1", []string{"CVE-2026-0106"}))
	busybox := OSVPackage{"Wolfi", "busybox", "1.37.0-r0"}
	both(busybox, advisoryVuln("CGA-cccc-0000-0001", "busybox", "detection", "", []string{"CVE-2026-0105"}, [2]string{"tiny", "rust-crate"}))
	fake.add(busybox, advisoryVuln("CGA-cccc-0000-0002", "busybox", "pending_upstream_fix", "", []string{"CVE-2026-0107"}))

	svc.OSV, svc.Feed = osv, feed
	return svc, fake, cg, feedFake
}
