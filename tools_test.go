package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestListImages(t *testing.T) {
	s := mcpClient(t, newFakeService(t))

	var all ListImagesOutput
	mustCall(t, s, "list_images", nil, &all)
	if all.Total != 11 || all.Count != 11 {
		t.Fatalf("want 11 images, got total=%d count=%d", all.Total, all.Count)
	}
	for _, img := range all.Images {
		if img.Free == nil {
			t.Fatalf("%s: free status missing", img.Name)
		}
		if want := img.Name == "python" || img.Name == "node" || img.Name == "go" || img.Name == "jre" || img.Name == "static" || img.Name == "wolfi-base"; *img.Free != want {
			t.Errorf("%s: free=%v, want %v", img.Name, *img.Free, want)
		}
	}

	var node ListImagesOutput
	mustCall(t, s, "list_images", map[string]any{"query": "NODE", "limit": 2, "offset": 1}, &node)
	if node.Total != 3 || node.Count != 2 || node.Images[0].Name != "node-fips" {
		t.Fatalf("unexpected paging result: %+v", node)
	}

	var free ListImagesOutput
	mustCall(t, s, "list_images", map[string]any{"query": "node", "free_only": true}, &free)
	if free.Total != 1 || free.Images[0].Name != "node" || free.Images[0].Reference != "cgr.dev/chainguard/node" {
		t.Fatalf("unexpected free_only result: %+v", free)
	}
}

func TestGetImageTags(t *testing.T) {
	s := mcpClient(t, newFakeService(t))

	var out GetImageTagsOutput
	mustCall(t, s, "get_image_tags", map[string]any{"image": "python"}, &out)
	if !slices.Equal(out.Tags, []string{"latest", "latest-dev"}) {
		t.Fatalf("want [latest latest-dev] (attestation tags filtered), got %v", out.Tags)
	}

	if msg := callTool(t, s, "get_image_tags", map[string]any{"image": "loki-fips"}, nil); !strings.Contains(msg, "not publicly accessible") {
		t.Fatalf("expected not-public error, got %q", msg)
	}
	if msg := callTool(t, s, "get_image_tags", map[string]any{"image": "../etc"}, nil); !strings.Contains(msg, "invalid image name") {
		t.Fatalf("expected validation error, got %q", msg)
	}
}

func TestGetImageDetails(t *testing.T) {
	s := mcpClient(t, newFakeService(t))

	var d ImageDetails
	mustCall(t, s, "get_image_details", map[string]any{"image": "python"}, &d)
	if len(d.Platforms) != 2 || d.Platforms[0].Platform != "linux/amd64" || d.Platforms[0].SizeBytes <= 1000 {
		t.Errorf("unexpected platforms: %+v", d.Platforms)
	}
	if d.User != "65532" || d.RunsAsRoot || d.ConfigPlatform != "linux/amd64" {
		t.Errorf("unexpected user info: %+v", d)
	}
	if !slices.Equal(d.Entrypoint, []string{"/usr/bin/python"}) || d.Source != "https://example.com/python" {
		t.Errorf("unexpected config: %+v", d)
	}
	if !strings.HasPrefix(d.PinnedReference, "cgr.dev/chainguard/python:latest@sha256:") {
		t.Errorf("unexpected pinned reference %q", d.PinnedReference)
	}

	var dev ImageDetails
	mustCall(t, s, "get_image_details", map[string]any{"image": "python", "tag": "latest-dev"}, &dev)
	if !dev.RunsAsRoot {
		t.Errorf("latest-dev runs as root in the fixture: %+v", dev)
	}

	if msg := callTool(t, s, "get_image_details", map[string]any{"image": "python", "tag": "3.99"}, nil); !strings.Contains(msg, "not found") {
		t.Fatalf("expected not-found error, got %q", msg)
	}
}

func TestPinImage(t *testing.T) {
	s := mcpClient(t, newFakeService(t))

	var p PinResult
	mustCall(t, s, "pin_image", map[string]any{"image": "static"}, &p)
	if !strings.HasPrefix(p.Digest, "sha256:") ||
		p.PinnedReference != "cgr.dev/chainguard/static:latest@"+p.Digest ||
		p.DigestReference != "cgr.dev/chainguard/static@"+p.Digest {
		t.Fatalf("unexpected pin result: %+v", p)
	}
}

func TestGetImagePackages(t *testing.T) {
	s := mcpClient(t, newFakeService(t))

	var out GetImagePackagesOutput
	mustCall(t, s, "get_image_packages", map[string]any{"image": "python"}, &out)
	if out.Total != 3 || len(out.Packages) != 3 {
		t.Fatalf("want 3 deduplicated packages, got %+v", out.Packages)
	}
	want := Package{Name: "libssl3", Version: "3.6.5-r1", Origin: "openssl", License: "MIT", Distro: "wolfi"}
	if out.Packages[1] != want {
		t.Errorf("got %+v, want %+v", out.Packages[1], want)
	}
	if out.HasShell || out.HasAPK {
		t.Errorf("latest should have no shell or apk: %+v", out)
	}

	var filtered GetImagePackagesOutput
	mustCall(t, s, "get_image_packages", map[string]any{"image": "python", "query": "openssl"}, &filtered)
	if filtered.Total != 3 || len(filtered.Packages) != 2 {
		t.Errorf("query should match by origin: %+v", filtered.Packages)
	}

	var dev GetImagePackagesOutput
	mustCall(t, s, "get_image_packages", map[string]any{"image": "python", "tag": "latest-dev"}, &dev)
	if !dev.HasShell || !dev.HasAPK {
		t.Errorf("latest-dev should have a shell and apk: %+v", dev)
	}

	if msg := callTool(t, s, "get_image_packages", map[string]any{"image": "python", "arch": "riscv64"}, nil); !strings.Contains(msg, "no riscv64 variant") {
		t.Errorf("expected missing-arch error, got %q", msg)
	}
}

func TestCheckVulnerabilities(t *testing.T) {
	s := mcpClient(t, newFakeService(t))

	var summary VulnReport
	mustCall(t, s, "check_vulnerabilities", map[string]any{"image": "python"}, &summary)
	// openssl: CVE-2022-3602 fixed, CVE-2023-0466 not affected, CVE-2099-0001 pending.
	// python-3.13: two IDs fixed in 3.13.1.
	if summary.TotalFixed != 3 || summary.TotalNotAffected != 1 || len(summary.Packages) != 2 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	if !slices.Equal(summary.Packages[0].PendingFixes, []string{"CVE-2099-0001"}) {
		t.Errorf("want pending CVE-2099-0001 for openssl, got %+v", summary.Packages[0])
	}

	for id, want := range map[string]string{
		"cve-2022-3602":       StatusFixed,
		"CVE-2023-0466":       StatusNotAffected,
		"CVE-2099-0001":       StatusVulnerable,
		"GHSA-aaaa-bbbb-cccc": StatusFixed,
	} {
		var r VulnReport
		mustCall(t, s, "check_vulnerabilities", map[string]any{"image": "python", "id": id}, &r)
		if len(r.Matches) != 1 || r.Matches[0].Status != want {
			t.Errorf("%s: want %s, got %+v", id, want, r.Matches)
		}
	}

	var none VulnReport
	mustCall(t, s, "check_vulnerabilities", map[string]any{"image": "python", "id": "CVE-1999-0001"}, &none)
	if len(none.Matches) != 0 || !strings.HasPrefix(none.Note, "No record of CVE-1999-0001") {
		t.Errorf("unexpected result for unknown CVE: %+v", none)
	}
}

func TestFindAlternative(t *testing.T) {
	s := mcpClient(t, newFakeService(t))

	var node AlternativesResult
	mustCall(t, s, "find_alternative", map[string]any{"image": "docker.io/library/node:20-alpine"}, &node)
	if node.Recommended == nil || node.Recommended.Image != "node" || node.Recommended.Free == nil || !*node.Recommended.Free {
		t.Fatalf("unexpected recommendation: %+v", node.Recommended)
	}
	if node.DevTag != "latest-dev" {
		t.Errorf("want dev tag latest-dev, got %q", node.DevTag)
	}
	if len(node.Alternatives) != 2 || node.Alternatives[0].Image != "node-lts" {
		t.Errorf("want [node-lts node-fips] (non-FIPS first), got %+v", node.Alternatives)
	}
	if !slices.ContainsFunc(node.Notes, func(n string) bool { return strings.Contains(n, "only provides the latest version") }) {
		t.Errorf("expected a version-tag note: %v", node.Notes)
	}

	for in, want := range map[string]string{
		"golang:1.23":                       "go",
		"eclipse-temurin:21-jre":            "jre",
		"eclipse-temurin:21":                "jdk",
		"gcr.io/distroless/static-debian12": "static",
		"ubuntu:24.04":                      "wolfi-base",
		"python@sha256:abc":                 "python",
	} {
		var out AlternativesResult
		mustCall(t, s, "find_alternative", map[string]any{"image": in}, &out)
		if out.Recommended == nil || out.Recommended.Image != want {
			t.Errorf("%s: want %s, got %+v", in, want, out.Recommended)
		}
	}

	var unknown AlternativesResult
	mustCall(t, s, "find_alternative", map[string]any{"image": "acme/widget:1.0"}, &unknown)
	if unknown.Recommended != nil || len(unknown.Notes) == 0 {
		t.Errorf("expected no recommendation: %+v", unknown)
	}
}

func TestMigratePrompt(t *testing.T) {
	s := mcpClient(t, newFakeService(t))
	res, err := s.GetPrompt(context.Background(), &mcp.GetPromptParams{
		Name:      "migrate_dockerfile",
		Arguments: map[string]string{"dockerfile": "FROM node:20\nCMD node app.js"},
	})
	if err != nil {
		t.Fatal(err)
	}
	text := res.Messages[0].Content.(*mcp.TextContent).Text
	if !strings.Contains(text, "find_alternative") || !strings.Contains(text, "FROM node:20") {
		t.Fatalf("unexpected prompt text: %s", text)
	}
}

func TestSaveSBOM(t *testing.T) {
	svc := newFakeService(t)
	svc.SBOMDir = t.TempDir()
	s := mcpClient(t, svc)

	var saved SavedSBOM
	mustCall(t, s, "save_sbom", map[string]any{"image": "python"}, &saved)
	wantPath := filepath.Join(svc.SBOMDir, "python-latest-amd64.spdx.json")
	if saved.Path != wantPath || saved.PackageCount != 3 || saved.SPDXVersion != "SPDX-2.3" || !strings.HasPrefix(saved.Digest, "sha256:") {
		t.Fatalf("unexpected result: %+v", saved)
	}

	// The file is the full, pretty-printed SPDX document.
	b, err := os.ReadFile(saved.Path)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		SPDXVersion   string           `json:"spdxVersion"`
		Packages      []map[string]any `json:"packages"`
		Relationships []map[string]any `json:"relationships"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatalf("saved file isn't valid JSON: %v", err)
	}
	if doc.SPDXVersion != "SPDX-2.3" || len(doc.Packages) != 9 || len(doc.Relationships) != 3 || len(b) != saved.SizeBytes {
		t.Fatalf("saved file is incomplete: version=%q packages=%d relationships=%d", doc.SPDXVersion, len(doc.Packages), len(doc.Relationships))
	}
	if !strings.Contains(string(b), "\n  \"") {
		t.Error("saved file should be indented")
	}

	// Existing files are only replaced with overwrite.
	if msg := callTool(t, s, "save_sbom", map[string]any{"image": "python"}, nil); !strings.Contains(msg, "already exists") {
		t.Fatalf("expected already-exists error, got %q", msg)
	}
	mustCall(t, s, "save_sbom", map[string]any{"image": "python", "overwrite": true}, nil)

	// Custom names get a .json extension; paths are rejected.
	var custom SavedSBOM
	mustCall(t, s, "save_sbom", map[string]any{"image": "python", "tag": "latest-dev", "filename": "python-dev"}, &custom)
	if filepath.Base(custom.Path) != "python-dev.json" || custom.PackageCount != 3 {
		t.Fatalf("unexpected result: %+v", custom)
	}
	for _, bad := range []string{"../escape.json", "sub/dir.json", `..\escape.json`, ".hidden.json", "C:evil.json"} {
		if msg := callTool(t, s, "save_sbom", map[string]any{"image": "python", "filename": bad}, nil); !strings.Contains(msg, "invalid filename") {
			t.Errorf("%q: expected invalid-filename error, got %q", bad, msg)
		}
	}
	entries, _ := os.ReadDir(svc.SBOMDir)
	if len(entries) != 2 {
		t.Errorf("want exactly 2 files in the SBOM folder, got %d", len(entries))
	}

	if msg := callTool(t, s, "save_sbom", map[string]any{"image": "loki-fips"}, nil); !strings.Contains(msg, "not publicly accessible") {
		t.Errorf("expected not-public error, got %q", msg)
	}
}

func TestSaveSBOMDisabledWithoutDir(t *testing.T) {
	s := mcpClient(t, newFakeService(t)) // SBOMDir unset, as in HTTP mode
	res, err := s.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range res.Tools {
		if tool.Name == "save_sbom" {
			t.Fatal("save_sbom should not be offered without an SBOM folder")
		}
	}
}
