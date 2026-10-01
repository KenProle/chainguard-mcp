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
