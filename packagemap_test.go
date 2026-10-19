package main

import (
	"context"
	"errors"
	"net/http/httptest"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func mapNames(t *testing.T, svc *Service, names ...string) *PackageMappingResult {
	t.Helper()
	out, err := svc.MapPackages(context.Background(), MapPackagesInput{Names: names})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func mapOne(t *testing.T, name string) PackageMapping {
	t.Helper()
	out := mapNames(t, newFakeService(t), name)
	if len(out.Results) != 1 {
		t.Fatalf("%d results for %s", len(out.Results), name)
	}
	return out.Results[0]
}

func TestPM1_1_SeveralNames(t *testing.T) {
	out := mapNames(t, newFakeService(t), "curl", "build-essential", "libpq-dev")
	var got []string
	for _, r := range out.Results {
		got = append(got, r.Name)
	}
	if !slices.Equal(got, []string{"curl", "build-essential", "libpq-dev"}) {
		t.Errorf("results = %v", got)
	}
	if out.IndexFetched.IsZero() {
		t.Error("no index fetch time")
	}
	if out.MappingCommit != "84e5c1cd7648d4896dbc56c49a7a7bdd1638b0cd" {
		t.Errorf("mapping commit = %q", out.MappingCommit)
	}
}

func TestPM1_2_DuplicateNames(t *testing.T) {
	out := mapNames(t, newFakeService(t), "curl", "curl")
	if len(out.Results) != 1 || out.Results[0].Name != "curl" {
		t.Errorf("results = %+v", out.Results)
	}
}

func TestPM2_1_StaleDFCHint(t *testing.T) {
	// dfc maps libpq-dev to postgresql-dev, which is not in the index.
	m := mapOne(t, "libpq-dev")
	for _, p := range append(slices.Clone(m.Packages), m.Alternatives...) {
		if p == "postgresql-dev" {
			t.Errorf("result names postgresql-dev: %+v", m)
		}
	}
}

func TestPM2_2_EveryResultIsIndexed(t *testing.T) {
	idx := loadFixtureIndex(t)
	out := mapNames(t, newFakeService(t), packageSample...)
	if len(out.Results) != len(packageSample) {
		t.Fatalf("%d results for %d names", len(out.Results), len(packageSample))
	}
	for _, r := range out.Results {
		for _, p := range append(slices.Clone(r.Packages), r.Alternatives...) {
			if _, ok := idx.Pkgs[p]; !ok {
				t.Errorf("%s: %s is not in the index", r.Name, p)
			}
		}
	}
}

func TestPM3_1_Exact(t *testing.T) {
	m := mapOne(t, "curl")
	if m.Status != "exact" || !slices.Equal(m.Packages, []string{"curl"}) || m.Source != "Wolfi package index" {
		t.Errorf("got %+v", m)
	}
}

func TestPM3_2_Unknown(t *testing.T) {
	m := mapOne(t, "dnsutils")
	if m.Status != "unknown" || len(m.Packages) != 0 {
		t.Errorf("got %+v", m)
	}
}

func TestPM4_1_NameExists(t *testing.T) {
	m := mapOne(t, "libffi-dev")
	if m.Status != "exact" {
		t.Errorf("got %+v", m)
	}
	// Even with a dfc entry for it, the index wins.
	got := mapPackage(loadFixtureIndex(t), map[string][]string{"libffi-dev": {"curl"}}, "libffi-dev")
	if got.Status != "exact" || got.Packages[0] != "libffi-dev" {
		t.Errorf("with a dfc entry: %+v", got)
	}
}

func TestPM4_2_DFCMapping(t *testing.T) {
	m := mapOne(t, "build-essential")
	if m.Status != "mapped" || !slices.Equal(m.Packages, []string{"build-base"}) || m.Source != "dfc" {
		t.Errorf("got %+v", m)
	}
}

func TestPM4_3_SeveralPackages(t *testing.T) {
	m := mapOne(t, "ssh")
	if m.Status != "mapped" || !slices.Equal(m.Packages, []string{"openssh-client", "openssh-server"}) {
		t.Errorf("got %+v", m)
	}
}

func TestPM4_4_FallsThroughStaleMapping(t *testing.T) {
	m := mapOne(t, "libpq-dev")
	if m.Status != "candidates" {
		t.Errorf("got %+v", m)
	}
}

func TestPM4_5_NoReplacementNeeded(t *testing.T) {
	m := mapOne(t, "software-properties-common")
	if m.Status != "mapped" || m.Packages == nil || len(m.Packages) != 0 || !strings.Contains(m.Note, "dfc lists no replacement") {
		t.Errorf("got %+v", m)
	}
}

func TestPM5_1_LibsslDev(t *testing.T) {
	m := mapOne(t, "libssl-dev")
	if m.Status != "mapped" || !slices.Equal(m.Packages, []string{"openssl-dev"}) || !strings.Contains(m.Source, "corrected") {
		t.Errorf("got %+v", m)
	}
}

func TestPM5_2_LibsqliteDev(t *testing.T) {
	m := mapOne(t, "libsqlite3-dev")
	if !slices.Equal(m.Packages, []string{"sqlite-dev"}) {
		t.Errorf("got %+v", m)
	}
}

func TestPM6_1_PkgConfigModule(t *testing.T) {
	m := mapOne(t, "libpq-dev")
	if m.Status != "candidates" || !strings.Contains(m.Source, "pkg-config module libpq") {
		t.Errorf("got %+v", m)
	}
}

func TestPM6_2_SharedLibrary(t *testing.T) {
	m := mapOne(t, "libgomp1")
	if m.Status != "candidates" || !slices.Equal(m.Packages, []string{"libgomp"}) || !strings.Contains(m.Source, "libgomp.so.1") {
		t.Errorf("got %+v", m)
	}
}

func TestPM6_3_Command(t *testing.T) {
	m := mapOne(t, "iputils-ping")
	if m.Status != "candidates" || !slices.Equal(m.Packages, []string{"iputils"}) || !strings.Contains(m.Source, "command ping") {
		t.Errorf("got %+v", m)
	}
}

func TestPM6_4_WithoutDefault(t *testing.T) {
	m := mapOne(t, "default-libmysqlclient-dev")
	if m.Status != "candidates" || len(m.Packages) != 1 || !regexp.MustCompile(`^mysql-[0-9.]+-dev$`).MatchString(m.Packages[0]) {
		t.Errorf("got %+v", m)
	}
}

func TestPM7_1_PostgresClientHeaders(t *testing.T) {
	m := mapOne(t, "libpq-dev")
	if !slices.Equal(m.Packages, []string{"postgresql-18-dev"}) ||
		!slices.Contains(m.Alternatives, "postgresql-17-dev") || !slices.Contains(m.Alternatives, "postgresql-16-dev") ||
		!strings.Contains(m.Note, "version") {
		t.Errorf("got %+v", m)
	}
}

func TestPM7_2_PrereleaseStreamNotChosen(t *testing.T) {
	idx := loadFixtureIndex(t)
	if v := idx.Pkgs["python-3.15"].Version; !strings.HasPrefix(v, "3.15.0_git") {
		t.Fatalf("the fixture's python-3.15 is %s, not a prerelease snapshot", v)
	}
	m := mapOne(t, "python3")
	if slices.Contains(m.Packages, "python-3.15") || !slices.Contains(m.Alternatives, "python-3.15") {
		t.Errorf("got %+v", m)
	}
	// 3.14.8_git… is a build of a stable stream, so it can be chosen.
	if !slices.Equal(m.Packages, []string{"python-3.14"}) {
		t.Errorf("chose %v, want python-3.14", m.Packages)
	}
}

func TestPM7_3_StaleStreamNotChosen(t *testing.T) {
	if _, ok := loadFixtureIndex(t).Pkgs["postgresql-12-client"]; !ok {
		t.Fatal("the fixture lacks postgresql-12-client")
	}
	m := mapOne(t, "postgresql-client")
	if !slices.Equal(m.Packages, []string{"postgresql-18-client"}) {
		t.Errorf("got %+v", m)
	}
}

func TestPM8_1_Dnsutils(t *testing.T) {
	m := mapOne(t, "dnsutils")
	if m.Status != "unknown" || len(m.Packages) != 0 || len(m.Alternatives) != 0 {
		t.Errorf("got %+v", m)
	}
}

func TestPM9_1_InvalidName(t *testing.T) {
	_, err := newFakeService(t).MapPackages(context.Background(), MapPackagesInput{Names: []string{"curl", "Curl;rm"}})
	var in *InputError
	if !errors.As(err, &in) || !strings.Contains(err.Error(), "Curl;rm") {
		t.Errorf("got %v", err)
	}
}

func TestPM9_2_TooManyNames(t *testing.T) {
	names := make([]string, 101)
	for i := range names {
		names[i] = "pkg" + string(rune('a'+i%26)) + string(rune('a'+i/26))
	}
	_, err := newFakeService(t).MapPackages(context.Background(), MapPackagesInput{Names: names})
	var in *InputError
	if !errors.As(err, &in) {
		t.Errorf("got %v", err)
	}
}

func TestPM9_3_NoNames(t *testing.T) {
	_, err := newFakeService(t).MapPackages(context.Background(), MapPackagesInput{})
	var in *InputError
	if !errors.As(err, &in) {
		t.Errorf("got %v", err)
	}
}

func TestPM9_4_NameBoundaries(t *testing.T) {
	for _, bad := range []string{"a", "-curl", "curl_x", "Curl", ""} {
		if _, err := validateMapNames([]string{bad}); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
	for _, good := range []string{"g++", "libstdc++-dev", "libglib2.0-0", "7zip"} {
		if _, err := validateMapNames([]string{good}); err != nil {
			t.Errorf("%q rejected: %v", good, err)
		}
	}
}

func TestPM11_2_MapPackagesWithoutIndex(t *testing.T) {
	svc := newFakeService(t)
	svc.Packages.URL = "http://127.0.0.1:1/APKINDEX.tar.gz"
	_, err := svc.MapPackages(context.Background(), MapPackagesInput{Names: []string{"curl"}})
	var srcErr *SourceError
	if !errors.As(err, &srcErr) || !strings.Contains(srcErr.Source, "Wolfi package index") {
		t.Errorf("got %v", err)
	}
}

func TestPM12_1_MapPackagesTool(t *testing.T) {
	s := mcpClient(t, newFakeService(t))
	var out PackageMappingResult
	mustCall(t, s, "map_packages", map[string]any{"names": []string{"build-essential"}}, &out)
	if len(out.Results) != 1 || out.Results[0].Name != "build-essential" || out.Results[0].Status != "mapped" ||
		!slices.Equal(out.Results[0].Packages, []string{"build-base"}) {
		t.Errorf("got %+v", out.Results)
	}
	tools, err := s.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range tools.Tools {
		if tool.Name != "map_packages" {
			continue
		}
		for _, want := range []string{"package index", "candidates", "build"} {
			if !strings.Contains(tool.Description, want) {
				t.Errorf("description lacks %q: %s", want, tool.Description)
			}
		}
		if tool.Annotations == nil || !tool.Annotations.ReadOnlyHint {
			t.Error("map_packages should be read-only")
		}
		return
	}
	t.Error("map_packages is not registered")
}

func TestPM12_2_MapPackagesToolError(t *testing.T) {
	s := mcpClient(t, newFakeService(t))
	msg := callTool(t, s, "map_packages", map[string]any{"names": []string{"Curl;rm"}}, nil)
	if !strings.Contains(msg, "Curl;rm") {
		t.Errorf("error = %q, want it to name Curl;rm", msg)
	}
}

func TestPM14_1_MigratePromptUsesMapping(t *testing.T) {
	s := mcpClient(t, newFakeService(t))
	res, err := s.GetPrompt(context.Background(), &mcp.GetPromptParams{
		Name:      "migrate_dockerfile",
		Arguments: map[string]string{"dockerfile": "FROM debian\nRUN apt-get install -y libpq-dev"},
	})
	if err != nil {
		t.Fatal(err)
	}
	text := res.Messages[0].Content.(*mcp.TextContent).Text
	for _, want := range []string{"map_packages", "candidates", "unknown"} {
		if !strings.Contains(text, want) {
			t.Errorf("prompt lacks %q", want)
		}
	}
}

func TestPM13_1_APIRequest(t *testing.T) {
	srv := newWebServer(t, fakeUI)
	var out PackageMappingResult
	resp := getJSON(t, srv, "/api/packages/map?names=curl,build-essential", &out)
	if resp.StatusCode != 200 || len(out.Results) != 2 ||
		out.Results[0].Name != "curl" || out.Results[0].Status != "exact" ||
		out.Results[1].Name != "build-essential" || out.Results[1].Status != "mapped" {
		t.Errorf("status %d, %+v", resp.StatusCode, out)
	}
}

func TestPM13_2_APIInvalidName(t *testing.T) {
	srv := newWebServer(t, fakeUI)
	var e apiError
	// Go drops a query pair containing a raw ";", leaving no names; the
	// encoded form reaches name validation.
	for _, path := range []string{"/api/packages/map?names=Curl;rm", "/api/packages/map?names=Curl%3Brm", "/api/packages/map"} {
		resp := getJSON(t, srv, path, &e)
		if resp.StatusCode != 400 || e.Code != "invalid_input" {
			t.Errorf("%s: status %d, %+v", path, resp.StatusCode, e)
		}
	}
}

func TestPM13_3_APIIndexUnavailable(t *testing.T) {
	svc := newFakeService(t)
	svc.Packages.URL = "http://127.0.0.1:1/APKINDEX.tar.gz"
	srv := httptest.NewServer(newWebHandler(svc, fakeUI))
	t.Cleanup(srv.Close)
	var e apiError
	resp := getJSON(t, srv, "/api/packages/map?names=curl", &e)
	if resp.StatusCode != 502 || e.Code != "source_failed" || !strings.Contains(e.Source, "Wolfi package index") {
		t.Errorf("status %d, %+v", resp.StatusCode, e)
	}
}

func TestPM13_4_ListedInIndex(t *testing.T) {
	srv := newWebServer(t, fakeUI)
	var index apiIndex
	getJSON(t, srv, "/api/", &index)
	for _, e := range index.Endpoints {
		if e.Path != "/api/packages/map" {
			continue
		}
		if len(e.Parameters) != 1 || e.Parameters[0].Name != "names" || !e.Parameters[0].Required || e.Parameters[0].In != "query" {
			t.Errorf("parameters = %+v", e.Parameters)
		}
		return
	}
	t.Error("/api/packages/map is not in the index")
}

func TestPM6_5_VirtualPackageOfLastWord(t *testing.T) {
	idx := loadFixtureIndex(t)
	if _, ok := idx.Pkgs["gcc-go"]; !ok {
		t.Fatal("the fixture lacks gcc-go, which provides cmd:go")
	}
	m := mapOne(t, "golang-go")
	if m.Status != "candidates" || len(m.Packages) != 1 || !regexp.MustCompile(`^go-1\.\d+$`).MatchString(m.Packages[0]) ||
		slices.Contains(m.Packages, "gcc-go") || m.Source != "virtual package go" {
		t.Errorf("got %+v", m)
	}
}

func TestPM7_4_ChoicesAreStable(t *testing.T) {
	// The ranking is one consistent order, so the choices don't depend on
	// the order the matches were found in.
	want := map[string]string{
		"python3": "python-3.14", "nodejs": "nodejs-26",
		"libpq-dev": "postgresql-18-dev", "postgresql-client": "postgresql-18-client",
	}
	for name, pkg := range want {
		if m := mapOne(t, name); !slices.Equal(m.Packages, []string{pkg}) {
			t.Errorf("%s = %+v, want %s", name, m, pkg)
		}
	}
	m := mapOne(t, "python3")
	if !slices.Equal(m.Alternatives, []string{"python-3.15", "python-3.13", "python-3.12", "python-3.11", "python-3.10"}) {
		t.Errorf("python3 alternatives = %v", m.Alternatives)
	}
}
