package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPM15_2_EmbeddedMappingsRead(t *testing.T) {
	m, err := loadDFCMappings()
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Debian) == 0 {
		t.Fatal("no Debian mappings")
	}
	if got := m.Debian["build-essential"]; len(got) != 1 || got[0] != "build-base" {
		t.Errorf("build-essential = %v, want [build-base]", got)
	}
	if got, ok := m.Debian["software-properties-common"]; !ok || len(got) != 0 {
		t.Errorf("software-properties-common = %v, %v; want an empty list", got, ok)
	}
	if len(m.Source.Commit) != 40 {
		t.Errorf("snapshot commit = %q", m.Source.Commit)
	}
	if b, err := dfcFiles.ReadFile("mappings/" + dfcLicenseFile); err != nil || !strings.Contains(string(b), "Apache License") {
		t.Errorf("license file missing or not Apache: %v", err)
	}
}

func TestPM15_2_StrictReaderErrors(t *testing.T) {
	const head = "images:\n    a: b\npackages:\n    debian:\n        curl:\n            - curl\n"
	tests := []struct {
		name, data, want string
	}{
		{"tab", "images:\n\ta: b\n", "line 2"},
		{"quoted value", "images:\n    a: \"b\"\n", "line 2"},
		{"6-space line", head + "      x: y\n", "line 7"},
		{"unknown top-level key", head + "extra:\n", "line 7"},
		{"list outside a package", "packages:\n    debian:\n            - x\n", "line 3"},
		{"no debian section", "images:\n    a: b\n", "no Debian"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseDFCMappings([]byte(tt.data))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("got %v, want an error containing %q", err, tt.want)
			}
		})
	}
	got, err := parseDFCMappings([]byte(head + "        empty: []\n"))
	if err != nil || len(got["curl"]) != 1 || got["empty"] == nil || len(got["empty"]) != 0 {
		t.Errorf("good input: %v, %v", got, err)
	}
}

// fakeDFC serves a commit list and raw files like GitHub.
func fakeDFC(t *testing.T, sha, mappings string) *DFCRefresher {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/commits", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]map[string]string{{"sha": sha}})
	})
	mux.HandleFunc("/raw/", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "builtin-mappings.yaml"):
			fmt.Fprint(w, mappings)
		case strings.HasSuffix(r.URL.Path, "/LICENSE"):
			fmt.Fprint(w, "Apache License fake")
		default:
			http.NotFound(w, r)
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	r := NewDFCRefresher(srv.Client())
	r.CommitsURL = srv.URL + "/commits"
	r.RawURL = srv.URL + "/raw/%s/%s"
	r.Now = func() string { return "2026-10-20" }
	return r
}

func TestPM16_1_Refresh(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, dfcMappingsFile)
	old := "packages:\n    debian:\n        curl:\n            - curl\n        gone:\n            - x\n        same:\n            - s\n"
	os.WriteFile(file, []byte(old), 0o644)
	sha := strings.Repeat("c", 40)
	next := "packages:\n    debian:\n        curl:\n            - curl2\n        same:\n            - s\n        fresh:\n            - f\n"
	report, err := fakeDFC(t, sha, next).Refresh(context.Background(), file)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(file); string(got) != next {
		t.Errorf("file not updated: %q", got)
	}
	src, _ := os.ReadFile(filepath.Join(dir, dfcSourceFile))
	if !strings.Contains(string(src), sha) || !strings.Contains(string(src), "2026-10-20") {
		t.Errorf("source file = %s", src)
	}
	if lic, _ := os.ReadFile(filepath.Join(dir, dfcLicenseFile)); !strings.Contains(string(lic), "Apache") {
		t.Errorf("license = %s", lic)
	}
	for _, want := range []string{"added: fresh", "removed: gone", "changed: curl"} {
		if !strings.Contains(report, want) {
			t.Errorf("report lacks %q:\n%s", want, report)
		}
	}
}

func TestPM16_2_RefreshRefusesBadFormat(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, dfcMappingsFile)
	old := "packages:\n    debian:\n        curl:\n            - curl\n"
	os.WriteFile(file, []byte(old), 0o644)
	_, err := fakeDFC(t, strings.Repeat("d", 40), "packages:\n  debian: [1]\n").Refresh(context.Background(), file)
	if err == nil {
		t.Fatal("want an error")
	}
	if got, _ := os.ReadFile(file); string(got) != old {
		t.Errorf("existing file changed: %q", got)
	}
	if _, err := os.Stat(filepath.Join(dir, dfcSourceFile)); err == nil {
		t.Error("source file was written")
	}
}
