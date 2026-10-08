package main

import (
	"slices"
	"testing"
)

func TestCompareAPKVersions(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"3.6.5-r1", "3.6.5-r1", 0},
		{"3.6.5-r1", "3.0.7-r0", 1},
		{"3.0.7-r0", "3.0.7-r1", -1},
		{"3.0.10-r0", "3.0.9-r0", 1}, // numeric, not lexical
		{"1.0", "1.0.1", -1},
		{"1.0a", "1.0", 1},
		{"1.0_rc1", "1.0", -1},
		{"1.0_alpha2", "1.0_beta1", -1},
		{"1.0_p1", "1.0", 1},
		{"0_git20240101", "0_git20240201", -1},
		{"20230201-r30", "20230201-r4", 1},
		{"007-r0", "7-r0", 0},
		{"99999999999999999999.0", "1.0", 1}, // no integer overflow
		{"weird", "weird", 0},                // unparseable falls back to string compare
	}
	for _, tt := range tests {
		if got := compareAPKVersions(tt.a, tt.b); got != tt.want {
			t.Errorf("compareAPKVersions(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
		if got := compareAPKVersions(tt.b, tt.a); got != -tt.want {
			t.Errorf("compareAPKVersions(%q, %q) = %d, want %d", tt.b, tt.a, got, -tt.want)
		}
	}
}

func TestParseImageRef(t *testing.T) {
	tests := []struct {
		ref  string
		path []string
		tag  string
	}{
		{"python", []string{"python"}, ""},
		{"Node:20-Alpine", []string{"node"}, "20-alpine"},
		{"docker.io/library/python:3.12-slim", []string{"library", "python"}, "3.12-slim"},
		{"localhost:5000/team/app:v1", []string{"team", "app"}, "v1"},
		{"mcr.microsoft.com/dotnet/aspnet:8.0", []string{"dotnet", "aspnet"}, "8.0"},
		{"redis@sha256:abc", []string{"redis"}, ""},
		{"bitnami/redis", []string{"bitnami", "redis"}, ""},
	}
	for _, tt := range tests {
		got := parseImageRef(tt.ref)
		if !slices.Equal(got.path, tt.path) || got.tag != tt.tag {
			t.Errorf("parseImageRef(%q) = %v %q, want %v %q", tt.ref, got.path, got.tag, tt.path, tt.tag)
		}
	}
}

func TestCandidateNames(t *testing.T) {
	tests := map[string][]string{
		"golang:1.23":                         {"go", "golang"},
		"eclipse-temurin:21-jre":              {"jre", "jdk", "eclipse-temurin"},
		"mcr.microsoft.com/dotnet/aspnet:8.0": {"aspnet-runtime", "aspnet"},
		"redis":                               {"redis"},
	}
	for ref, want := range tests {
		if got := parseImageRef(ref).candidateNames(); !slices.Equal(got, want) {
			t.Errorf("candidateNames(%q) = %v, want %v", ref, got, want)
		}
	}
}

func TestIsRootUser(t *testing.T) {
	for user, want := range map[string]bool{
		"": true, "root": true, "0": true, "0:0": true, "root:root": true,
		"65532": false, "nonroot": false, "65532:65532": false,
	} {
		if got := isRootUser(user); got != want {
			t.Errorf("isRootUser(%q) = %v, want %v", user, got, want)
		}
	}
}

func TestImageAndTag(t *testing.T) {
	if img, tag, err := imageAndTag(" python ", ""); err != nil || img != "python" || tag != "latest" {
		t.Errorf("got %q %q %v", img, tag, err)
	}
	for _, bad := range []string{"", "Python", "a/b", "../x", "py thon", "-x"} {
		if _, _, err := imageAndTag(bad, ""); err == nil {
			t.Errorf("imageAndTag(%q) should fail", bad)
		}
	}
	if _, _, err := imageAndTag("python", "bad/tag"); err == nil {
		t.Error("tag with slash should fail")
	}
}

func TestAnalyzeVulnsUncovered(t *testing.T) {
	pkgs := []Package{
		{Name: "libssl3", Version: "3.6.5-r1", Origin: "openssl", Distro: "wolfi"},
		{Name: "private-tool", Version: "1.0-r0", Origin: "private-tool", Distro: "chainguard"},
	}
	r := analyzeVulns(pkgs, map[string]map[string][]string{}, "")
	if !slices.Equal(r.Uncovered, []string{"private-tool"}) || len(r.Packages) != 1 {
		t.Fatalf("unexpected report: %+v", r)
	}
}

func TestIsVariant(t *testing.T) {
	// Real catalog names: variants vs. unrelated projects sharing a prefix.
	tests := []struct {
		img, base string
		want      bool
	}{
		{"node-fips", "node", true},
		{"postgres-iamguarded-fips", "postgres", true},
		{"jdk-crac", "jdk", true},
		{"go-msft-fips", "go", true},
		{"node-local-dns", "node", false},
		{"node-problem-detector-fips", "node", false},
		{"postgres-operator", "postgres", false},
		{"go-ipfs", "go", false},
		{"nodejs", "node", false}, // no separator
		{"python", "python", false},
	}
	for _, tt := range tests {
		if got := isVariant(tt.img, tt.base); got != tt.want {
			t.Errorf("isVariant(%q, %q) = %v, want %v", tt.img, tt.base, got, tt.want)
		}
	}
}

func TestAnalyzeVulnsVersionStreamFallback(t *testing.T) {
	// Wolfi keeps OpenSSL's history under "openssl" while images install the
	// versioned "openssl-4.0" package.
	fixes := map[string]map[string][]string{
		"openssl": {
			"0":        {"CVE-2023-0466"},
			"3.0.7-r0": {"CVE-2022-3602"},
			"4.1.0-r0": {"CVE-2099-0002"}, // a newer stream's fix
		},
		"python-3.13": {"3.13.9-r0": {"CVE-2099-0003"}},
	}
	pkgs := []Package{
		{Name: "openssl-4.0-libssl", Version: "4.0.3-r5", Origin: "openssl-4.0", Distro: "wolfi"},
		{Name: "python-3.13", Version: "3.13.7-r0", Origin: "python-3.13", Distro: "wolfi"},
	}

	r := analyzeVulns(pkgs, fixes, "CVE-2022-3602")
	if len(r.Matches) != 1 || r.Matches[0].Status != StatusFixed || r.Matches[0].RecordsFrom != "openssl" {
		t.Fatalf("want CVE-2022-3602 fixed via openssl records, got %+v", r.Matches)
	}

	// A newer-stream fix found via the fallback is skipped, not reported as vulnerable.
	if r := analyzeVulns(pkgs, fixes, "CVE-2099-0002"); len(r.Matches) != 0 {
		t.Errorf("fallback should skip newer-stream fixes, got %+v", r.Matches)
	}

	// A package with its own records still reports pending fixes.
	summary := analyzeVulns(pkgs, fixes, "")
	if summary.TotalFixed != 1 || summary.TotalNotAffected != 1 {
		t.Errorf("unexpected totals: fixed=%d not_affected=%d", summary.TotalFixed, summary.TotalNotAffected)
	}
	py := summary.Packages[1]
	if py.RecordsFrom != "" || !slices.Equal(py.PendingFixes, []string{"CVE-2099-0003"}) {
		t.Errorf("python-3.13 should use its own records: %+v", py)
	}
}
