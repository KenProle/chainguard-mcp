package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"slices"
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
		{"/api/nope", 404, "not_found"},          // WA-3.1
		{"/api/images/python", 404, "not_found"}, // WA-3.2
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

func TestAPIFamilies(t *testing.T) {
	svc, fake := newFakeBackend(t)
	srv := httptest.NewServer(newWebHandler(svc, fakeUI))
	t.Cleanup(srv.Close)

	t.Run("CM-3.1 whole catalog", func(t *testing.T) {
		var out ImageFamiliesOutput
		if resp := getJSON(t, srv, "/api/families", &out); resp.StatusCode != 200 {
			t.Fatalf("status %d", resp.StatusCode)
		}
		var names []string
		for _, f := range out.Families {
			names = append(names, f.Name)
		}
		want := []string{"node", "python", "go", "jdk", "jre", "loki", "static", "wolfi-base"}
		if out.Total != 11 || !slices.Equal(names, want) {
			t.Errorf("got %d images in %v, want 11 in %v", out.Total, names, want)
		}
		if !slices.Equal(out.Families[0].Images, []string{"node", "node-fips", "node-lts"}) {
			t.Errorf("node family: %v", out.Families[0].Images)
		}
		if loki := out.Families[5]; !slices.Equal(loki.Images, []string{"loki-fips"}) {
			t.Errorf("loki family: %+v", loki)
		}
	})
	t.Run("CM-3.3 no free-tier checks", func(t *testing.T) {
		before := fake.tokenRequests.Load()
		getJSON(t, srv, "/api/families", nil)
		if n := fake.tokenRequests.Load() - before; n != 0 {
			t.Errorf("made %d token requests", n)
		}
	})
	t.Run("query", func(t *testing.T) {
		var out ImageFamiliesOutput
		getJSON(t, srv, "/api/families?query=fips", &out)
		var names []string
		for _, f := range out.Families {
			names = append(names, f.Name)
		}
		if out.Total != 3 || !slices.Equal(names, []string{"loki", "node", "python"}) {
			t.Errorf("got %d images in %v", out.Total, names)
		}
	})
}

func TestAPIFamiliesCatalogUnavailable(t *testing.T) {
	t.Run("CM-3.4 catalog unavailable", func(t *testing.T) {
		svc := newFakeService(t)
		svc.Catalog.SitemapURL += ".missing"
		srv := httptest.NewServer(newWebHandler(svc, fakeUI))
		t.Cleanup(srv.Close)
		var e apiError
		if resp := getJSON(t, srv, "/api/families", &e); resp.StatusCode != http.StatusBadGateway {
			t.Errorf("status %d %+v, want 502", resp.StatusCode, e)
		}
	})
}

func TestAPIGroups(t *testing.T) {
	svc, fake := newFakeBackend(t)
	srv := httptest.NewServer(newWebHandler(svc, fakeUI))
	t.Cleanup(srv.Close)
	labels := func(out ImageGroupsOutput) []string {
		var l []string
		for _, g := range out.Groups {
			l = append(l, fmt.Sprintf("%s:%d", g.Label, len(g.Images)))
		}
		return l
	}

	t.Run("CM-16.1 default grouping", func(t *testing.T) {
		var out ImageGroupsOutput
		if resp := getJSON(t, srv, "/api/groups", &out); resp.StatusCode != 200 {
			t.Fatalf("status %d", resp.StatusCode)
		}
		// 11 images: names need 2 images, so lts (node-lts) folds into Other.
		want := []string{"Base images:7", "FIPS:3", "Other variants:1"}
		if out.GroupBy != "variant" || out.Total != 11 || !slices.Equal(labels(out), want) {
			t.Errorf("got %s %d %v, want %v", out.GroupBy, out.Total, labels(out), want)
		}
	})
	t.Run("CM-16.2 other group", func(t *testing.T) {
		var out ImageGroupsOutput
		getJSON(t, srv, "/api/groups?group_by=prefix", &out)
		want := []string{"Other:6", "node:3", "python:2"}
		if !slices.Equal(labels(out), want) || out.Groups[0].Folded != 6 {
			t.Errorf("got %v (folded %d), want %v", labels(out), out.Groups[0].Folded, want)
		}
	})
	t.Run("CM-16.3 invalid grouping", func(t *testing.T) {
		var e apiError
		if resp := getJSON(t, srv, "/api/groups?group_by=size", &e); resp.StatusCode != 400 || e.Code != "invalid_input" {
			t.Errorf("got %d %+v", resp.StatusCode, e)
		}
	})
	t.Run("CM-16.4 no free-tier checks", func(t *testing.T) {
		before := fake.tokenRequests.Load()
		getJSON(t, srv, "/api/groups?query=node", nil)
		if n := fake.tokenRequests.Load() - before; n != 0 {
			t.Errorf("made %d token requests", n)
		}
	})
}

func TestAPIIndex(t *testing.T) {
	srv := newWebServer(t, fakeUI)
	var index apiIndex
	if resp := getJSON(t, srv, "/api/", &index); resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	byPath := map[string]apiRoute{}
	for _, e := range index.Endpoints {
		if _, dup := byPath[e.Path]; dup {
			t.Errorf("%s listed twice", e.Path)
		}
		byPath[e.Path] = e
	}
	param := func(path, name string) (apiParam, bool) {
		for _, p := range byPath[path].Parameters {
			if p.Name == name {
				return p, true
			}
		}
		return apiParam{}, false
	}

	t.Run("WA-1.1 index lists images parameters", func(t *testing.T) {
		e := byPath["/api/images"]
		var names []string
		for _, p := range e.Parameters {
			names = append(names, p.Name)
			if p.In != "query" || p.Required || p.Description == "" {
				t.Errorf("/api/images %s: %+v", p.Name, p)
			}
		}
		if e.Method != "GET" || strings.Join(names, ",") != "query,free_only,limit,offset" {
			t.Errorf("/api/images: %+v", e)
		}
	})

	t.Run("WA-1.2 index marks path and required parameters", func(t *testing.T) {
		const vulns = "/api/images/{name}/vulnerabilities"
		want := map[string]apiParam{
			"name": {In: "path", Required: true},
			"tag":  {In: "query"},
			"id":   {In: "query"},
		}
		if len(byPath[vulns].Parameters) != len(want) {
			t.Errorf("%s parameters: %+v", vulns, byPath[vulns].Parameters)
		}
		for name, w := range want {
			if p, ok := param(vulns, name); !ok || p.In != w.In || p.Required != w.Required {
				t.Errorf("%s %s: %+v", vulns, name, p)
			}
		}
		if p, ok := param("/api/alternatives", "image"); !ok || p.In != "query" || !p.Required {
			t.Errorf("/api/alternatives image: %+v", p)
		}
	})

	t.Run("WA-1.3 index examples are relative", func(t *testing.T) {
		for _, e := range index.Endpoints {
			if !strings.HasPrefix(e.Example, "/api/") || strings.Contains(e.Example, "://") {
				t.Errorf("%s example %q", e.Path, e.Example)
			}
		}
		// Raw output should be readable with curl, so & isn't escaped.
		if _, body := get(t, srv, "/api/"); !strings.Contains(body, "python&free_only") {
			t.Errorf("example URLs are escaped in the raw index: %.300s", body)
		}
	})

	t.Run("WA-2.1 index lists exactly the expected endpoints", func(t *testing.T) {
		want := []string{
			"/api/images",
			"/api/groups",
			"/api/families",
			"/api/images/{name}/tags",
			"/api/images/{name}/details",
			"/api/images/{name}/pin",
			"/api/images/{name}/packages",
			"/api/images/{name}/vulnerabilities",
			"/api/images/{name}/sbom",
			"/api/alternatives",
		}
		var got []string
		for _, e := range index.Endpoints {
			got = append(got, e.Path)
		}
		if strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Errorf("index paths:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
		}
	})

	t.Run("WA-2.2 every example reaches its endpoint", func(t *testing.T) {
		for _, e := range index.Endpoints {
			resp, body := get(t, srv, e.Example)
			if resp.StatusCode != 200 || strings.Contains(body, "unknown API endpoint") {
				t.Errorf("%s: %d %.200s", e.Example, resp.StatusCode, body)
			}
		}
	})

	// ServeMux can't list its patterns, so scan the source for routes
	// registered outside apiRoutes.
	t.Run("WA-2.3 every /api route literal is in the index", func(t *testing.T) {
		allowed := map[string]bool{"GET /api/{$}": true, "/api/": true}
		files, err := filepath.Glob("*.go")
		if err != nil {
			t.Fatal(err)
		}
		pattern := regexp.MustCompile(`\.Handle(?:Func)?\(\s*"([^"]*)"`)
		for _, f := range files {
			if strings.HasSuffix(f, "_test.go") {
				continue
			}
			src, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			for _, m := range pattern.FindAllStringSubmatch(string(src), -1) {
				if strings.Contains(m[1], "/api") && !allowed[m[1]] {
					t.Errorf("%s registers %q outside apiRoutes; add it to the table instead", f, m[1])
				}
			}
		}
	})

	t.Run("WA-3.3 index only answers GET", func(t *testing.T) {
		resp, err := http.Post(srv.URL+"/api/", "application/json", nil)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode == 200 || strings.Contains(string(body), "endpoints") {
			t.Errorf("POST /api/: %d %s", resp.StatusCode, body)
		}
	})

	t.Run("WA-4.1 README lists every index endpoint", func(t *testing.T) {
		readme, err := os.ReadFile("README.md")
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range index.Endpoints {
			if !strings.Contains(string(readme), "| `"+e.Path+"`") {
				t.Errorf("README.md has no table row for `%s`", e.Path)
			}
		}
	})
}
