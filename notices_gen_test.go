package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// noticeFilePrefixes are the module-root files that carry a module's notices.
var noticeFilePrefixes = []string{"LICENSE", "COPYING", "NOTICE", "PATENTS"}

func isNoticeFile(name string) bool {
	for _, p := range noticeFilePrefixes {
		if strings.HasPrefix(strings.ToUpper(name), p) {
			return true
		}
	}
	return false
}

func readNoticeFile(t *testing.T, dir, name string) NoticeFile {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	return NoticeFile{Name: name, Text: strings.ReplaceAll(string(data), "\r\n", "\n")}
}

func goOutput(t *testing.T, args ...string) string {
	t.Helper()
	cmd := exec.Command("go", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go %s: %v\n%s", strings.Join(args, " "), err, stderr.String())
	}
	return string(out)
}

// buildGoNotices builds the notices from the module graph of the main package
// (test-only imports never count): the standard library first, then each
// linked module by path.
func buildGoNotices(t *testing.T) []GoNotice {
	t.Helper()
	root := strings.TrimSpace(goOutput(t, "env", "GOROOT"))
	std := GoNotice{Path: stdlibNoticePath}
	for _, name := range []string{"LICENSE", "PATENTS"} {
		std.Files = append(std.Files, readNoticeFile(t, root, name))
	}
	notices := []GoNotice{std}

	list := goOutput(t, "list", "-deps", "-f", `{{with .Module}}{{if not .Main}}{{.Path}}{{"\t"}}{{.Version}}{{"\t"}}{{.Dir}}{{end}}{{end}}`, ".")
	seen := map[string]bool{}
	var mods []GoNotice
	for _, line := range strings.Split(list, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		f := strings.Split(line, "\t")
		if len(f) != 3 {
			t.Fatalf("unexpected go list line %q", line)
		}
		if seen[f[0]] {
			continue
		}
		seen[f[0]] = true
		n := GoNotice{Path: f[0], Version: f[1]}
		entries, err := os.ReadDir(f[2])
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			if e.Type().IsRegular() && isNoticeFile(e.Name()) {
				n.Files = append(n.Files, readNoticeFile(t, f[2], e.Name()))
			}
		}
		sort.Slice(n.Files, func(i, j int) bool { return n.Files[i].Name < n.Files[j].Name })
		if len(n.Files) == 0 {
			t.Fatalf("module %s %s has no license files in %s", n.Path, n.Version, f[2])
		}
		mods = append(mods, n)
	}
	sort.Slice(mods, func(i, j int) bool { return mods[i].Path < mods[j].Path })
	return append(notices, mods...)
}

func marshalGoNotices(t *testing.T, notices []GoNotice) []byte {
	t.Helper()
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(notices); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func noticeFilesEqual(a, b []NoticeFile) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// diffGoNotices names every module that is missing from, extra in or at
// another version in the committed notices. The standard library is compared
// by its texts, not by the toolchain version (LN-6.2).
func diffGoNotices(committed, want []GoNotice) []string {
	have := map[string]GoNotice{}
	for _, n := range committed {
		have[n.Path] = n
	}
	wantBy := map[string]GoNotice{}
	var problems []string
	for _, w := range want {
		wantBy[w.Path] = w
		h, ok := have[w.Path]
		switch {
		case !ok:
			problems = append(problems, w.Path+" "+w.Version+" is missing")
		case w.Path == stdlibNoticePath:
			if !noticeFilesEqual(h.Files, w.Files) {
				problems = append(problems, w.Path+" license files changed")
			}
		case h.Version != w.Version:
			problems = append(problems, w.Path+" is "+w.Version+" in go.mod but "+h.Version+" in the notices")
		}
	}
	for _, h := range committed {
		if _, ok := wantBy[h.Path]; !ok {
			problems = append(problems, h.Path+" "+h.Version+" is extra")
		}
	}
	sort.Strings(problems)
	return problems
}

// TestGoNoticesFile checks that notices/go-notices.json matches the modules
// the server links (LN-6.1, LN-6.2). With UPDATE_FIXTURES=1 it writes the file.
func TestGoNoticesFile(t *testing.T) {
	want := buildGoNotices(t)
	if os.Getenv("UPDATE_FIXTURES") == "1" {
		if err := os.MkdirAll("notices", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(goNoticesFile, marshalGoNotices(t, want), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	committed, err := loadGoNotices()
	if err != nil {
		t.Fatalf("%v\nregenerate with: %s", err, noticesRegenerateCommand)
	}
	if problems := diffGoNotices(committed, want); len(problems) > 0 {
		t.Fatalf("%s is out of date:\n  %s\nregenerate with: %s", goNoticesFile, strings.Join(problems, "\n  "), noticesRegenerateCommand)
	}
}

// TestGoNoticesDiff covers the failure messages of TestGoNoticesFile without
// touching the committed file.
func TestGoNoticesDiff(t *testing.T) {
	f := []NoticeFile{{Name: "LICENSE", Text: "x"}}
	committed := []GoNotice{
		{Path: stdlibNoticePath, Files: f},
		{Path: "example.com/a", Version: "v1.0.0", Files: f},
		{Path: "example.com/gone", Version: "v1.0.0", Files: f},
	}
	want := []GoNotice{
		{Path: stdlibNoticePath, Files: f},
		{Path: "example.com/a", Version: "v1.1.0", Files: f},
		{Path: "example.com/new", Version: "v2.0.0", Files: f},
	}
	got := strings.Join(diffGoNotices(committed, want), "\n")
	for _, s := range []string{
		"example.com/a is v1.1.0 in go.mod but v1.0.0 in the notices",
		"example.com/new v2.0.0 is missing",
		"example.com/gone v1.0.0 is extra",
	} {
		if !strings.Contains(got, s) {
			t.Errorf("diff missing %q in:\n%s", s, got)
		}
	}
	if d := diffGoNotices(committed[:1], want[:1]); len(d) != 0 {
		t.Errorf("identical stdlib entries differ: %v", d)
	}
}
