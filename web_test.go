package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

// fakeUI stands in for the built React app.
var fakeUI = fstest.MapFS{
	"index.html":         {Data: []byte(`<!doctype html><div id="root"></div>`)},
	"assets/app-1a2b.js": {Data: []byte(`console.log("app")`)},
	"favicon.svg":        {Data: []byte(`<svg/>`)},
}

func newWebServer(t *testing.T, ui fstest.MapFS) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(newWebHandler(newFakeService(t), ui))
	t.Cleanup(srv.Close)
	return srv
}

// getJSON fetches path and decodes the JSON body into out (if non-nil).
func getJSON(t *testing.T, srv *httptest.Server, path string, out any) *http.Response {
	t.Helper()
	resp, err := http.Get(srv.URL + path)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("%s: Content-Type %q, want JSON", path, ct)
	}
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			t.Fatalf("%s: decoding: %v", path, err)
		}
	}
	return resp
}

func TestAPIEndpoints(t *testing.T) {
	srv := newWebServer(t, fakeUI)

	var list ListImagesOutput
	if resp := getJSON(t, srv, "/api/images?query=node&free_only=true", &list); resp.StatusCode != 200 {
		t.Fatalf("list: status %d", resp.StatusCode)
	}
	if list.Total != 1 || list.Images[0].Name != "node" {
		t.Errorf("list: %+v", list)
	}

	var paged ListImagesOutput
	getJSON(t, srv, "/api/images?limit=2&offset=1", &paged)
	if paged.Count != 2 || paged.Offset != 1 || paged.Total != 11 {
		t.Errorf("paging: %+v", paged)
	}

	var tags GetImageTagsOutput
	getJSON(t, srv, "/api/images/python/tags", &tags)
	if len(tags.Tags) != 2 {
		t.Errorf("tags: %+v", tags)
	}

	var details ImageDetails
	getJSON(t, srv, "/api/images/python/details?tag=latest-dev", &details)
	if details.Tag != "latest-dev" || !details.RunsAsRoot || len(details.Platforms) != 2 {
		t.Errorf("details: %+v", details)
	}

	var pin PinResult
	getJSON(t, srv, "/api/images/static/pin", &pin)
	if !strings.HasPrefix(pin.PinnedReference, "cgr.dev/chainguard/static:latest@sha256:") {
		t.Errorf("pin: %+v", pin)
	}

	// "ssl" matches libssl3 by name and libcrypto3 by its openssl origin.
	var pkgs GetImagePackagesOutput
	getJSON(t, srv, "/api/images/python/packages?query=ssl", &pkgs)
	if pkgs.Arch != "amd64" || pkgs.Total != 3 || len(pkgs.Packages) != 2 {
		t.Errorf("packages: %+v", pkgs)
	}

	var vulns VulnReport
	getJSON(t, srv, "/api/images/python/vulnerabilities?id=CVE-2022-3602", &vulns)
	if len(vulns.Matches) != 1 || vulns.Matches[0].Status != StatusFixed {
		t.Errorf("vulnerabilities: %+v", vulns)
	}

	var alt AlternativesResult
	getJSON(t, srv, "/api/alternatives?image=golang:1.23", &alt)
	if alt.Recommended == nil || alt.Recommended.Image != "go" {
		t.Errorf("alternatives: %+v", alt)
	}
}

func TestAPIErrors(t *testing.T) {
	srv := newWebServer(t, fakeUI)
	tests := []struct {
		path   string
		status int
		code   string
	}{
		{"/api/images/loki-fips/details", 403, "not_public"},
		{"/api/images/python/details?tag=3.99", 404, "not_found"},
		{"/api/images/python/packages?arch=riscv64", 404, "not_found"},
		{"/api/images/Python/tags", 400, "invalid_input"},
		{"/api/images/python/packages?arch=x86%2F64", 400, "invalid_input"},
		{"/api/images?limit=abc", 400, "invalid_input"},
		{"/api/alternatives", 400, "invalid_input"},
		{"/api/nope", 404, "not_found"},
	}
	for _, tt := range tests {
		var e apiError
		resp := getJSON(t, srv, tt.path, &e)
		if resp.StatusCode != tt.status || e.Code != tt.code || e.Error == "" {
			t.Errorf("%s: got %d %+v, want %d %s", tt.path, resp.StatusCode, e, tt.status, tt.code)
		}
	}

	resp, err := http.Post(srv.URL+"/api/images", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		t.Error("POST /api/images should not succeed")
	}
}

func TestAPISBOMDownload(t *testing.T) {
	srv := newWebServer(t, fakeUI)
	resp, err := http.Get(srv.URL + "/api/images/python/sbom")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 ||
		resp.Header.Get("Content-Type") != "application/spdx+json" ||
		resp.Header.Get("Content-Disposition") != `attachment; filename="python-latest-amd64.spdx.json"` {
		t.Fatalf("unexpected response: %d %v", resp.StatusCode, resp.Header)
	}
	var doc struct {
		SPDXVersion string `json:"spdxVersion"`
		Packages    []any  `json:"packages"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil || doc.SPDXVersion != "SPDX-2.3" || len(doc.Packages) != 9 {
		t.Fatalf("bad SBOM body: %v %+v", err, doc)
	}

	var e apiError
	if r := getJSON(t, srv, "/api/images/loki-fips/sbom", &e); r.StatusCode != 403 {
		t.Errorf("paid image SBOM: status %d", r.StatusCode)
	}
}

func get(t *testing.T, srv *httptest.Server, path string) (*http.Response, string) {
	t.Helper()
	resp, err := http.Get(srv.URL + path)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp, string(body)
}

func TestUIServing(t *testing.T) {
	srv := newWebServer(t, fakeUI)

	// Client-side routes fall back to index.html.
	for _, p := range []string{"/", "/images/python", "/alternatives"} {
		resp, body := get(t, srv, p)
		if resp.StatusCode != 200 || !strings.Contains(body, `<div id="root">`) || resp.Header.Get("Cache-Control") != "no-cache" {
			t.Errorf("%s: %d %q %q", p, resp.StatusCode, resp.Header.Get("Cache-Control"), body)
		}
	}

	resp, body := get(t, srv, "/assets/app-1a2b.js")
	if resp.StatusCode != 200 || body != `console.log("app")` || !strings.Contains(resp.Header.Get("Cache-Control"), "immutable") {
		t.Errorf("asset: %d %q %q", resp.StatusCode, resp.Header.Get("Cache-Control"), body)
	}
	if resp, _ := get(t, srv, "/assets/missing.js"); resp.StatusCode != 404 {
		t.Errorf("missing asset: status %d, want 404", resp.StatusCode)
	}
	if resp, _ := get(t, srv, "/../../etc/passwd"); resp.StatusCode != 200 {
		t.Errorf("path traversal should be cleaned to an SPA route, got %d", resp.StatusCode)
	}

	// Every response carries the security headers.
	for _, p := range []string{"/", "/api/images/python/tags"} {
		resp, _ := get(t, srv, p)
		if !strings.Contains(resp.Header.Get("Content-Security-Policy"), "default-src 'self'") ||
			resp.Header.Get("X-Content-Type-Options") != "nosniff" {
			t.Errorf("%s: missing security headers: %v", p, resp.Header)
		}
	}
}

func TestUINotBuilt(t *testing.T) {
	srv := newWebServer(t, fstest.MapFS{".gitkeep": {}})
	resp, body := get(t, srv, "/")
	if resp.StatusCode != http.StatusServiceUnavailable || !strings.Contains(body, "npm run build") {
		t.Errorf("got %d %q", resp.StatusCode, body)
	}
	// The API still works without the UI.
	var tags GetImageTagsOutput
	if resp := getJSON(t, srv, "/api/images/python/tags", &tags); resp.StatusCode != 200 {
		t.Errorf("API without UI: status %d", resp.StatusCode)
	}
}

func TestEmbeddedUIFS(t *testing.T) {
	// The embedded FS must at least contain the placeholder, so the Go
	// build works before the UI has been built.
	if _, err := embeddedUI().Open(".gitkeep"); err != nil {
		t.Fatalf("embedded UI is missing .gitkeep: %v", err)
	}
}
