package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"testing"
	"testing/fstest"
)

func mustGoNotices(t *testing.T) []GoNotice {
	t.Helper()
	n, err := loadGoNotices()
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func findGoNotice(t *testing.T, notices []GoNotice, path string) GoNotice {
	t.Helper()
	for _, n := range notices {
		if n.Path == path {
			return n
		}
	}
	t.Fatalf("no notice for %s", path)
	return GoNotice{}
}

func noticeText(n GoNotice, file string) (string, bool) {
	for _, f := range n.Files {
		if f.Name == file {
			return f.Text, true
		}
	}
	return "", false
}

func viteFixture(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/vite-licenses-2026-11-10.json")
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// builtUI is a UI file system as `npm run build` leaves it.
func builtUI(t *testing.T) fstest.MapFS {
	t.Helper()
	return fstest.MapFS{
		"index.html":    {Data: []byte("<html></html>")},
		"licenses.json": {Data: viteFixture(t)},
	}
}

func TestLN_1_1_PatentsFileOfLinkedModule(t *testing.T) {
	n := findGoNotice(t, mustGoNotices(t), "golang.org/x/sync")
	if n.Version != "v0.20.0" {
		t.Fatalf("version = %s", n.Version)
	}
	lic, ok := noticeText(n, "LICENSE")
	if !ok || !strings.Contains(lic, "Copyright 2009 The Go Authors") {
		t.Errorf("LICENSE missing or wrong: %q", lic)
	}
	if pat, ok := noticeText(n, "PATENTS"); !ok || !strings.Contains(pat, "patent") {
		t.Errorf("PATENTS missing or wrong: %q", pat)
	}
}

func TestLN_1_2_DualLicenseTransitionNotice(t *testing.T) {
	n := findGoNotice(t, mustGoNotices(t), "github.com/modelcontextprotocol/go-sdk")
	if n.Version != "v1.8.0" {
		t.Fatalf("version = %s", n.Version)
	}
	lic, _ := noticeText(n, "LICENSE")
	if !strings.HasPrefix(lic, "The MCP project is undergoing a licensing transition from the MIT License") {
		t.Errorf("LICENSE does not start with the transition paragraph: %.80q", lic)
	}
	if !strings.Contains(lic, "Apache License") || !strings.Contains(lic, "MIT License") {
		t.Error("LICENSE lacks the license texts")
	}
}

func TestLN_1_3_StandardLibraryEntry(t *testing.T) {
	n := findGoNotice(t, mustGoNotices(t), stdlibNoticePath)
	if n.Version != "" {
		t.Errorf("stdlib entry has version %q", n.Version)
	}
	for _, f := range []string{"LICENSE", "PATENTS"} {
		if text, ok := noticeText(n, f); !ok || strings.TrimSpace(text) == "" {
			t.Errorf("stdlib %s missing", f)
		}
	}
}

func TestLN_2_2_NoDevelopmentTooling(t *testing.T) {
	text, err := renderNotices("dev", mustGoNotices(t), dfcFiles, builtUI(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range mustGoNotices(t) {
		for _, bad := range []string{"vite", "vitest", "typescript", "jsdom", "oxlint"} {
			if strings.Contains(n.Path, bad) {
				t.Errorf("Go notices name %s", bad)
			}
		}
	}
	for _, line := range strings.Split(text, "\n") {
		if strings.HasSuffix(line, "(web UI)") {
			for _, bad := range []string{"vite", "vitest", "typescript", "jsdom", "oxlint"} {
				if strings.HasPrefix(line, bad+" ") {
					t.Errorf("entry %q is development tooling", line)
				}
			}
		}
	}
}

func TestLN_1_5_BundledUIPackage(t *testing.T) {
	notices, err := parseUINotices(viteFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range notices {
		if n.Name == "react-router" {
			if n.Version != "8.4.0" || n.Identifier != "MIT" || !strings.Contains(n.Text, "MIT License") {
				t.Errorf("react-router entry = %+v", n)
			}
			return
		}
	}
	t.Fatal("react-router missing")
}

func TestLN_2_1_TreeShakenPackagesAbsent(t *testing.T) {
	notices, err := parseUINotices(viteFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(notices) != 6 {
		t.Errorf("%d packages, want 6", len(notices))
	}
	for _, n := range notices {
		if n.Name == "@remix-run/route-pattern" || n.Name == "cookie-es" {
			t.Errorf("%s should not be listed", n.Name)
		}
	}
}

func TestUINoticesRejectBadInput(t *testing.T) {
	if _, err := parseUINotices([]byte(`[{"name":"left-pad","version":"1.0.0","identifier":"MIT","text":"  "}]`)); err == nil || !strings.Contains(err.Error(), "left-pad") {
		t.Errorf("empty text: err = %v", err)
	}
	if _, err := parseUINotices([]byte(`[{"name":"a","version":"1","identifier":"MIT","text":"t","extra":1}]`)); err == nil {
		t.Error("unknown field accepted")
	}
	if _, err := parseUINotices([]byte(`{"name":"a"}`)); err == nil {
		t.Error("object instead of array accepted")
	}
	if _, err := parseUINotices([]byte(`[] []`)); err == nil {
		t.Error("trailing data accepted")
	}
}

func TestLN_1_4_DFCEntryNamesCommit(t *testing.T) {
	text, err := renderNotices("dev", mustGoNotices(t), dfcFiles, builtUI(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "Chainguard dfc package mappings, commit 84e5c1cd7648d4896dbc56c49a7a7bdd1638b0cd") {
		t.Error("dfc entry with the commit missing")
	}
	if !strings.Contains(text, "Apache License") {
		t.Error("dfc license text missing")
	}
}

func TestLN_7_1_BuiltUIWithoutLicenseData(t *testing.T) {
	ui := fstest.MapFS{"index.html": {Data: []byte("x")}}
	_, err := renderNotices("dev", mustGoNotices(t), dfcFiles, ui, nil)
	if err == nil || !strings.Contains(err.Error(), "license notices") {
		t.Errorf("err = %v", err)
	}
}

func TestLN_7_2_NoUI(t *testing.T) {
	ui := fstest.MapFS{".gitkeep": {Data: nil}}
	text, err := renderNotices("dev", mustGoNotices(t), dfcFiles, ui, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, noUIMessage) || !strings.Contains(text, "golang.org/x/sync") || !strings.Contains(text, "dfc") {
		t.Error("notices without UI incomplete")
	}
}

func TestRenderNoticesIsDeterministic(t *testing.T) {
	a, err := renderNotices("1.2.3", mustGoNotices(t), dfcFiles, builtUI(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := renderNotices("1.2.3", mustGoNotices(t), dfcFiles, builtUI(t), nil)
	if a != b {
		t.Error("two renders differ")
	}
	if !strings.HasPrefix(a, "chainguard-mcp 1.2.3 third-party notices\n") {
		t.Errorf("header = %.60q", a)
	}
	if !strings.Contains(a, "--- license text (MIT) ---") {
		t.Error("UI entries lack the license text label")
	}
	if strings.Contains(a, "\r") {
		t.Error("output contains carriage returns")
	}
}

func TestLN_2_3_BuildInfoMatchesNotices(t *testing.T) {
	notices := []GoNotice{
		{Path: stdlibNoticePath},
		{Path: "example.com/a", Version: "v1.0.0"},
		{Path: "example.com/b", Version: "v2.0.0"},
	}
	deps := func(m ...*debug.Module) *debug.BuildInfo { return &debug.BuildInfo{Deps: m} }
	a := &debug.Module{Path: "example.com/a", Version: "v1.0.0"}
	b := &debug.Module{Path: "example.com/b", Version: "v2.0.0"}

	if err := checkBuildInfo(deps(a, b), notices); err != nil {
		t.Errorf("equal lists: %v", err)
	}
	replaced := &debug.Module{Path: "example.com/b", Version: "v2.0.0", Replace: &debug.Module{Path: "example.com/fork", Version: "v9.9.9"}}
	if err := checkBuildInfo(deps(a, replaced), notices); err != nil {
		t.Errorf("replaced module keyed on its original path and version: %v", err)
	}
	if err := checkBuildInfo(nil, notices); err != nil {
		t.Errorf("nil build info: %v", err)
	}
	cases := []struct {
		name string
		bi   *debug.BuildInfo
		want string
	}{
		{"missing", deps(a), "example.com/b v2.0.0 has a notice but is not in the binary"},
		{"extra", deps(a, b, &debug.Module{Path: "example.com/c", Version: "v3.0.0"}), "example.com/c v3.0.0 is in the binary but has no notice"},
		{"version", deps(a, &debug.Module{Path: "example.com/b", Version: "v2.1.0"}), "example.com/b is v2.1.0 in the binary but its notice is for v2.0.0"},
	}
	for _, c := range cases {
		err := checkBuildInfo(c.bi, notices)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v", c.name, err)
		}
	}
}

// TestLN_3_1_LicensesFlag builds the binary and runs it with -licenses, so the
// header, the exit status and the absence of a server are checked end to end.
func TestLN_3_1_LicensesFlag(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the binary")
	}
	exe := filepath.Join(t.TempDir(), "chainguard-mcp")
	if runtime.GOOS == "windows" {
		exe += ".exe"
	}
	build := exec.Command("go", "build", "-ldflags", "-X main.version=1.4.0", "-o", exe, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	// Checks the exit status, the header and the output of the real binary. It
	// does not prove the flag avoids the network.
	cmd := exec.Command(exe, "-licenses")
	cmd.Env = []string{"SystemRoot=" + os.Getenv("SystemRoot")}
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("-licenses: %v", err)
	}
	first, _, _ := strings.Cut(string(out), "\n")
	if first != "chainguard-mcp 1.4.0 third-party notices" {
		t.Errorf("first line = %q", first)
	}
	if !strings.Contains(string(out), "golang.org/x/sync") {
		t.Error("output lacks golang.org/x/sync")
	}
}
