package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"
	"time"
)

// TestLive exercises every tool against the real Chainguard endpoints.
// It's skipped unless CHAINGUARD_LIVE=1, so `go test` works offline.
func TestLive(t *testing.T) {
	if os.Getenv("CHAINGUARD_LIVE") != "1" {
		t.Skip("set CHAINGUARD_LIVE=1 to run against the live Chainguard endpoints")
	}
	svc := NewService()
	svc.SBOMDir = t.TempDir()
	s := mcpClient(t, svc)

	t.Run("list_images", func(t *testing.T) {
		var out ListImagesOutput
		mustCall(t, s, "list_images", map[string]any{"query": "python", "limit": 5}, &out)
		t.Logf("total=%d page=%+v", out.Total, out.Images)
		if out.Total == 0 || len(out.Images) == 0 || out.Images[0].Free == nil {
			t.Fatalf("unexpected result: %+v", out)
		}
		var free ListImagesOutput
		mustCall(t, s, "list_images", map[string]any{"query": "python", "free_only": true}, &free)
		t.Logf("free python images: %d", free.Total)
		if !slices.ContainsFunc(free.Images, func(i ImageSummary) bool { return i.Name == "python" }) {
			t.Fatalf("python missing from free images: %+v", free.Images)
		}
	})

	t.Run("get_image_tags", func(t *testing.T) {
		var out GetImageTagsOutput
		mustCall(t, s, "get_image_tags", map[string]any{"image": "static"}, &out)
		if !slices.Contains(out.Tags, "latest") {
			t.Fatalf("expected latest tag, got %v", out.Tags)
		}
		if msg := callTool(t, s, "get_image_tags", map[string]any{"image": "loki-fips"}, nil); !strings.Contains(msg, "not publicly accessible") {
			t.Fatalf("expected not-public error, got %q", msg)
		}
	})

	t.Run("get_image_details", func(t *testing.T) {
		var out ImageDetails
		mustCall(t, s, "get_image_details", map[string]any{"image": "python"}, &out)
		t.Logf("%+v", out)
		if len(out.Platforms) < 2 || out.RunsAsRoot || len(out.Entrypoint) == 0 {
			t.Fatalf("unexpected details: %+v", out)
		}
	})

	t.Run("pin_image", func(t *testing.T) {
		var out PinResult
		mustCall(t, s, "pin_image", map[string]any{"image": "static"}, &out)
		t.Logf("%s", out.PinnedReference)
		if !strings.HasPrefix(out.PinnedReference, "cgr.dev/chainguard/static:latest@sha256:") {
			t.Fatalf("unexpected pin: %+v", out)
		}
	})

	t.Run("get_image_packages", func(t *testing.T) {
		var out GetImagePackagesOutput
		mustCall(t, s, "get_image_packages", map[string]any{"image": "python"}, &out)
		t.Logf("total=%d shell=%v apk=%v", out.Total, out.HasShell, out.HasAPK)
		// Package names change upstream (libssl3 became openssl-4.0-libssl), so
		// only check that an OpenSSL library is traced to an OpenSSL origin.
		i := slices.IndexFunc(out.Packages, func(p Package) bool { return strings.Contains(p.Name, "libssl") })
		if i < 0 || !strings.HasPrefix(out.Packages[i].Origin, "openssl") || out.Packages[i].Origin == out.Packages[i].Name {
			t.Fatalf("expected an OpenSSL library with an openssl origin, got %+v", out.Packages)
		}
		var dev GetImagePackagesOutput
		mustCall(t, s, "get_image_packages", map[string]any{"image": "python", "tag": "latest-dev", "arch": "arm64"}, &dev)
		if !dev.HasShell || !dev.HasAPK {
			t.Fatalf("expected latest-dev to have a shell and apk: %+v", dev)
		}
	})

	t.Run("save_sbom", func(t *testing.T) {
		var out SavedSBOM
		mustCall(t, s, "save_sbom", map[string]any{"image": "python", "arch": "arm64"}, &out)
		t.Logf("%s: %d bytes, %s, %d packages", out.Path, out.SizeBytes, out.SPDXVersion, out.PackageCount)
		if out.PackageCount == 0 || out.SizeBytes < 10000 || !strings.HasPrefix(out.SPDXVersion, "SPDX-") {
			t.Fatalf("unexpected result: %+v", out)
		}
	})

	t.Run("check_vulnerabilities", func(t *testing.T) {
		var out VulnReport
		mustCall(t, s, "check_vulnerabilities", map[string]any{"image": "python"}, &out)
		t.Logf("fixed=%d not_affected=%d packages=%d", out.TotalFixed, out.TotalNotAffected, len(out.Packages))
		if out.TotalFixed == 0 {
			t.Fatalf("expected recorded fixes: %+v", out)
		}
		// CVE-2022-3602 ("Spooky SSL") was fixed in openssl 3.0.7.
		var cve VulnReport
		mustCall(t, s, "check_vulnerabilities", map[string]any{"image": "python", "id": "cve-2022-3602"}, &cve)
		t.Logf("%+v", cve.Matches)
		if len(cve.Matches) == 0 || cve.Matches[0].Status != StatusFixed {
			t.Fatalf("expected CVE-2022-3602 fixed: %+v", cve)
		}
	})

	t.Run("find_alternative", func(t *testing.T) {
		var node AlternativesResult
		mustCall(t, s, "find_alternative", map[string]any{"image": "node:20-alpine"}, &node)
		for _, a := range node.Alternatives {
			if !isVariant(a.Image, "node") {
				t.Errorf("unrelated alternative for node: %s", a.Image)
			}
		}

		for in, want := range map[string]string{
			"node:20-alpine":                      "node",
			"docker.io/library/golang:1.23":       "go",
			"eclipse-temurin:21-jre":              "jre",
			"gcr.io/distroless/static-debian12":   "static",
			"mcr.microsoft.com/dotnet/aspnet:8.0": "aspnet-runtime",
		} {
			var out AlternativesResult
			mustCall(t, s, "find_alternative", map[string]any{"image": in}, &out)
			if out.Recommended == nil || out.Recommended.Image != want {
				t.Errorf("%s: want %s, got %+v", in, want, out.Recommended)
			}
		}
	})

	t.Run("upstream", func(t *testing.T) {
		// UC-13.1: the vulnerabilities found for Docker Hub's node:20 include at
		// least 95% of those grype finds in its OS packages, kernel excluded.
		grype, err := exec.LookPath("grype")
		if err != nil {
			t.Skip("grype is not on PATH")
		}
		ctx := context.Background()

		// The comparison itself, whose figures task 8.2 records.
		start := time.Now()
		cmp, err := svc.CompareUpstream(ctx, CompareUpstreamInput{Image: "node:20"})
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("compared in %s", time.Since(start).Round(time.Second))
		t.Logf("upstream:   %d OS packages, %v language packages, %d bytes, built %s, %+v",
			cmp.Upstream.OSPackages, cmp.Upstream.LanguagePackages, cmp.Upstream.SizeBytes, cmp.Upstream.Created, *cmp.Upstream.Vulnerabilities)
		t.Logf("chainguard: %d OS packages, %d bytes, built %s, %+v",
			cmp.Chainguard.OSPackages, cmp.Chainguard.SizeBytes, cmp.Chainguard.Created, *cmp.Chainguard.Vulnerabilities)
		t.Logf("end of life: %+v", cmp.EndOfLife)

		// The same pipeline, keeping the IDs.
		m, digest, _, top, err := svc.Upstream.resolvePlatform(ctx, "library/node", "20", "amd64")
		if err != nil || m == nil {
			t.Fatal(err)
		}
		doc, err := svc.Upstream.sbomForPlatform(ctx, "library/node", "20", "amd64", top, digest)
		if err != nil {
			t.Fatal(err)
		}
		pkgs, err := parseUpstreamPackages(doc)
		if err != nil {
			t.Fatal(err)
		}
		results, err := svc.OSV.QueryAll(ctx, pkgs.Lookups, nil)
		if err != nil {
			t.Fatal(err)
		}
		found := map[string]bool{}
		for id := range classifyAll(results) {
			found[id] = true
		}
		if len(found) != cmp.Upstream.Vulnerabilities.Total {
			t.Errorf("%d distinct IDs, but the comparison counts %d", len(found), cmp.Upstream.Vulnerabilities.Total)
		}

		scanCtx, cancel := context.WithTimeout(ctx, 8*time.Minute)
		defer cancel()
		out, err := exec.CommandContext(scanCtx, grype, "registry:docker.io/library/node:20", "--platform", "linux/amd64", "-o", "json", "-q").Output()
		if err != nil {
			t.Fatalf("grype: %v", err)
		}
		var scan struct {
			Matches []struct {
				Vulnerability struct{ ID string }
				Artifact      struct {
					Name      string
					Type      string
					Upstreams []struct{ Name string }
				}
			}
		}
		if err := json.Unmarshal(out, &scan); err != nil {
			t.Fatal(err)
		}
		grypeIDs := map[string]bool{}
		for _, match := range scan.Matches {
			a := match.Artifact
			source := a.Name
			if len(a.Upstreams) > 0 {
				source = a.Upstreams[0].Name
			}
			if (a.Type == "deb" || a.Type == "apk") && source != kernelSource && strings.HasPrefix(match.Vulnerability.ID, "CVE-") {
				grypeIDs[match.Vulnerability.ID] = true
			}
		}
		var missing []string
		for id := range grypeIDs {
			if !found[id] {
				missing = append(missing, id)
			}
		}
		slices.Sort(missing)
		t.Logf("grype: %d CVEs in OS packages (kernel excluded); comparison: %d; missing from the comparison: %d %v",
			len(grypeIDs), len(found), len(missing), missing[:min(len(missing), 10)])
		if len(grypeIDs) == 0 {
			t.Fatal("grype found no CVEs; is its database present?")
		}
		if covered := float64(len(grypeIDs)-len(missing)) / float64(len(grypeIDs)); covered < 0.95 {
			t.Errorf("the comparison covers %.1f%% of grype's CVEs, want at least 95%%", covered*100)
		}
	})

	t.Run("archive_captures", func(t *testing.T) {
		// CH-8.1 as a stable property: the Archive keeps at least the 17 months
		// of sitemap captures that existed on 2026-10-10, from 2024-04.
		a := NewArchive(&http.Client{Timeout: 60 * time.Second})
		captures, err := a.Captures(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		months := map[string]bool{}
		for _, ts := range captures {
			if ts >= "202404" {
				months[ts[:6]] = true
			}
		}
		if len(months) < 17 {
			t.Errorf("captures cover %d months since 2024-04, want at least 17", len(months))
		}
	})
}
