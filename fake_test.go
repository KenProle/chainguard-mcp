package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeChainguard is an in-process stand-in for the sitemap, the cgr.dev
// registry and the Wolfi security database.
type fakeChainguard struct {
	t         *testing.T
	images    []string                     // catalog, in sitemap order
	public    map[string]bool              // images that get anonymous tokens
	tags      map[string]map[string]string // image → tag → manifest digest
	manifests map[string]fakeBlob          // digest → manifest
	blobs     map[string][]byte            // digest → blob
	secdb     string
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

func (f *fakeChainguard) addBlob(v any) Descriptor {
	b := mustJSON(f.t, v)
	d := digestOf(b)
	f.blobs[d] = b
	return Descriptor{MediaType: "application/octet-stream", Digest: d, Size: int64(len(b))}
}

func (f *fakeChainguard) addManifest(mediaType string, v any) Descriptor {
	b := mustJSON(f.t, v)
	d := digestOf(b)
	f.manifests[d] = fakeBlob{mediaType, b}
	return Descriptor{MediaType: mediaType, Digest: d, Size: int64(len(b))}
}

func (f *fakeChainguard) tag(image, tag, digest string) {
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
		if arch == "amd64" {
			f.addSBOM(image, m.Digest, pkgs)
		}
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
	f := &fakeChainguard{
		t: t,
		images: []string{
			"go", "jdk", "jre", "loki-fips", "node", "node-fips", "node-lts",
			"python", "python-fips", "static", "wolfi-base",
		},
		public: map[string]bool{
			"go": true, "jre": true, "node": true, "python": true, "static": true, "wolfi-base": true,
		},
		tags:      make(map[string]map[string]string),
		manifests: make(map[string]fakeBlob),
		blobs:     make(map[string][]byte),
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
	return svc
}
