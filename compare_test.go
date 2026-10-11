package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"
)

func compareNode20(t *testing.T, svc *Service) *UpstreamComparison {
	t.Helper()
	cmp, err := svc.CompareUpstream(context.Background(), CompareUpstreamInput{Image: "node:20"})
	if err != nil {
		t.Fatal(err)
	}
	return cmp
}

func TestUC1_1_ComparesTheEnteredImageWithChainguardLatest(t *testing.T) {
	svc, _ := newFakeUpstreamService(t)
	cmp := compareNode20(t, svc)

	if cmp.Input != "node:20" {
		t.Errorf("input = %q", cmp.Input)
	}
	if cmp.Upstream.Reference != "docker.io/library/node:20" || cmp.Upstream.Platform != "linux/amd64" {
		t.Errorf("upstream = %+v", cmp.Upstream)
	}
	if cmp.Chainguard.Reference != "cgr.dev/chainguard/node:latest" || cmp.Chainguard.Platform != "linux/amd64" {
		t.Errorf("chainguard = %+v", cmp.Chainguard)
	}
	if !strings.HasPrefix(cmp.Upstream.Digest, "sha256:") || !strings.HasPrefix(cmp.Chainguard.Digest, "sha256:") {
		t.Errorf("digests: %q %q", cmp.Upstream.Digest, cmp.Chainguard.Digest)
	}

	arm, err := svc.CompareUpstream(context.Background(), CompareUpstreamInput{Image: "docker.io/library/node:20", Arch: "arm64"})
	if err != nil {
		t.Fatal(err)
	}
	if arm.Upstream.Platform != "linux/arm64" || arm.Chainguard.Platform != "linux/arm64" || arm.Upstream.Digest == cmp.Upstream.Digest {
		t.Errorf("arm64: %+v / %+v", arm.Upstream, arm.Chainguard)
	}
}

func TestUC1_2_NoAlternative(t *testing.T) {
	svc, _ := newFakeUpstreamService(t)
	_, err := svc.CompareUpstream(context.Background(), CompareUpstreamInput{Image: "some-unknown-app:1"})
	if !errors.Is(err, ErrNoAlternative) {
		t.Fatalf("got %v, want ErrNoAlternative", err)
	}
}

func TestUC2_3_OtherRegistry(t *testing.T) {
	svc, f := newFakeUpstreamService(t)
	_, err := svc.CompareUpstream(context.Background(), CompareUpstreamInput{Image: "ghcr.io/example/node:1"})
	if !errors.Is(err, ErrUnsupportedRegistry) || !strings.Contains(err.Error(), "only Docker Hub images") {
		t.Fatalf("got %v, want the Docker Hub-only error", err)
	}
	if n := f.Hub.requests.Load() + f.OSV.requests.Load(); n != 0 {
		t.Errorf("%d requests were made for a reference that can't be compared", n)
	}
}

func TestUC2_1_UC3_1_UC4_UC5_NodeCounts(t *testing.T) {
	svc, f := newFakeUpstreamService(t)
	cmp := compareNode20(t, svc)

	// UC-3.1: installed binaries, not the openssl/glibc/linux source packages;
	// language packages are counted separately.
	if !cmp.Upstream.SBOMPublished || cmp.Upstream.OSPackages != 3 {
		t.Errorf("upstream packages = %d (sbom %v), want 3", cmp.Upstream.OSPackages, cmp.Upstream.SBOMPublished)
	}
	if len(cmp.Upstream.LanguagePackages) != 1 || cmp.Upstream.LanguagePackages[0] != (LanguageCount{"npm", 2}) {
		t.Errorf("language packages = %+v", cmp.Upstream.LanguagePackages)
	}
	if cmp.Chainguard.OSPackages != 3 {
		t.Errorf("chainguard packages = %d, want 3", cmp.Chainguard.OSPackages)
	}

	// UC-4.2 and UC-5: A is fixable (and no fix for glibc) so it counts once as
	// fix available; B won't be fixed; C has no fix yet; the kernel's D isn't counted.
	if got, want := *cmp.Upstream.Vulnerabilities, (VulnCounts{Total: 3, FixAvailable: 1, NoFixYet: 1, WontFix: 1}); got != want {
		t.Errorf("upstream vulnerabilities = %+v, want %+v", got, want)
	}
	// UC-5.2: Chainguard's advisory is pending an upstream fix.
	if got, want := *cmp.Chainguard.Vulnerabilities, (VulnCounts{Total: 1, NoFixYet: 1}); got != want {
		t.Errorf("chainguard vulnerabilities = %+v, want %+v", got, want)
	}
	// 2 upstream sources (the kernel is skipped) + 3 Chainguard packages.
	if n := f.OSV.requests.Load(); n != 5 {
		t.Errorf("OSV requests = %d, want 5", n)
	}
}

func TestUC7_1_SizesAndBuildDates(t *testing.T) {
	svc, f := newFakeUpstreamService(t)
	cmp := compareNode20(t, svc)

	m, _, _, err := svc.Upstream.PlatformManifest(context.Background(), "library/node", "20", "amd64")
	if err != nil {
		t.Fatal(err)
	}
	if want := m.Config.Size + 4000; cmp.Upstream.SizeBytes != want {
		t.Errorf("upstream size = %d, want %d (config + layers)", cmp.Upstream.SizeBytes, want)
	}
	if cmp.Upstream.Created != "2026-04-22T03:00:00Z" {
		t.Errorf("upstream created = %q", cmp.Upstream.Created)
	}
	d, err := svc.ImageDetails(context.Background(), "node", "latest")
	if err != nil {
		t.Fatal(err)
	}
	var want int64
	for _, p := range d.Platforms {
		if p.Platform == "linux/amd64" {
			want = p.SizeBytes
		}
	}
	if want == 0 || cmp.Chainguard.SizeBytes != want || cmp.Chainguard.Created != d.Created || d.Created == "" {
		t.Errorf("chainguard size/created = %d %q, want %d %q", cmp.Chainguard.SizeBytes, cmp.Chainguard.Created, want, d.Created)
	}
	_ = f
}

func TestUC8_1_EndOfLifeInComparison(t *testing.T) {
	svc, _ := newFakeUpstreamService(t)
	cmp := compareNode20(t, svc)
	if cmp.EndOfLife == nil || cmp.EndOfLife.Date != "2026-04-30" || cmp.EndOfLife.Product != "Node.js" {
		t.Errorf("end of life = %+v", cmp.EndOfLife)
	}
}

func TestUC6_SourceAndCheckedAt(t *testing.T) {
	svc, _ := newFakeUpstreamService(t)
	cmp := compareNode20(t, svc)
	if !strings.Contains(cmp.Source, "OSV.dev") || !strings.Contains(cmp.Source, "known vulnerabilities") && !strings.Contains(cmp.Source, "Known vulnerabilities") {
		t.Errorf("source = %q", cmp.Source)
	}
	if _, err := time.Parse(time.RFC3339, cmp.CheckedAt); err != nil {
		t.Errorf("checked_at = %q: %v", cmp.CheckedAt, err)
	}
	all := strings.ToLower(cmp.Source + strings.Join(cmp.Notes, " "))
	for _, bad := range []string{"vulnerability-free", "vulnerability free", "is secure"} {
		if strings.Contains(all, bad) {
			t.Errorf("wording %q must not appear: %s", bad, all)
		}
	}
}

func TestUC2_2_NoSBOMEndToEnd(t *testing.T) {
	svc, _ := newFakeUpstreamService(t)
	cmp, err := svc.CompareUpstream(context.Background(), CompareUpstreamInput{Image: "python:3.12-slim"})
	if err != nil {
		t.Fatal(err)
	}
	up := cmp.Upstream
	if up.SBOMPublished || up.OSPackages != 0 || up.Vulnerabilities != nil || len(up.LanguagePackages) != 0 {
		t.Errorf("an image without an SBOM must show no counts: %+v", up)
	}
	if up.SizeBytes == 0 || up.Created != "2026-09-01T00:00:00Z" {
		t.Errorf("size and build date are still shown: %+v", up)
	}
	if cmp.Chainguard.Vulnerabilities == nil || cmp.Chainguard.Reference != "cgr.dev/chainguard/python:latest" {
		t.Errorf("chainguard side: %+v", cmp.Chainguard)
	}
	if cmp.EndOfLife != nil {
		t.Errorf("end of life = %+v, want none for python 3.12", cmp.EndOfLife)
	}
}

// uncheckedUpstream compares a node tag whose SBOM is sbom and checks that the
// upstream side reports reason and no counts, never "0 known vulnerabilities".
func uncheckedUpstream(t *testing.T, tag string, sbom json.RawMessage, reason, note string) {
	t.Helper()
	svc, f := newFakeUpstreamService(t)
	f.Hub.addImage("library/node", tag, dockerImage{created: "2026-09-01T00:00:00Z", sbom: sbom})
	cmp, err := svc.CompareUpstream(context.Background(), CompareUpstreamInput{Image: "node:" + tag})
	if err != nil {
		t.Fatal(err)
	}
	up := cmp.Upstream
	if !up.SBOMPublished || up.PackagesChecked || up.UncheckedReason != reason || up.Vulnerabilities != nil || up.OSPackages != 0 {
		t.Errorf("upstream side = %+v, want an unchecked side (%s) without counts", up, reason)
	}
	if up.SizeBytes == 0 || up.Created == "" {
		t.Errorf("size and build date are still shown: %+v", up)
	}
	if !cmp.Chainguard.PackagesChecked || cmp.Chainguard.Vulnerabilities == nil {
		t.Errorf("chainguard side is still checked: %+v", cmp.Chainguard)
	}
	explains := func(n string) bool {
		return strings.Contains(n, note) && strings.Contains(n, "not the same as having none")
	}
	if !slices.ContainsFunc(cmp.Notes, explains) {
		t.Errorf("notes = %q, want one explaining %q", cmp.Notes, note)
	}
}

func TestUC15_1_SBOMWithoutOSPackages(t *testing.T) {
	// Like Docker Hub's busybox: an SBOM, but no distribution packages.
	uncheckedUpstream(t, "static", debianSPDX(t, nil, "semver"), uncheckedNoOSPackage, "lists no distribution packages")
}

func TestUC15_2_UnsupportedDistribution(t *testing.T) {
	// Like Fedora's images: RPM packages, which have no vulnerability data here.
	sbom := mustJSON(t, map[string]any{"packages": []map[string]any{{
		"SPDXID": "SPDXRef-bash", "name": "bash", "versionInfo": "5.2.26-3.fc40",
		"externalRefs": []map[string]string{{"referenceType": "purl", "referenceLocator": "pkg:rpm/fedora/bash@5.2.26-3.fc40?os_name=fedora&os_version=40"}},
	}}})
	uncheckedUpstream(t, "fedora", sbom, uncheckedUnsupported, "rpm/fedora 40")
}

func TestUC2_ChecksPackagesWhenSupported(t *testing.T) {
	svc, _ := newFakeUpstreamService(t)
	cmp, err := svc.CompareUpstream(context.Background(), CompareUpstreamInput{Image: "node:20"})
	if err != nil {
		t.Fatal(err)
	}
	if !cmp.Upstream.PackagesChecked || cmp.Upstream.UncheckedReason != "" || cmp.Upstream.Vulnerabilities == nil {
		t.Errorf("node:20 should be checked: %+v", cmp.Upstream)
	}
	py, err := svc.CompareUpstream(context.Background(), CompareUpstreamInput{Image: "python:3.12-slim"})
	if err != nil {
		t.Fatal(err)
	}
	if py.Upstream.PackagesChecked || py.Upstream.UncheckedReason != uncheckedNoSBOM {
		t.Errorf("python:3.12-slim (no SBOM): %+v", py.Upstream)
	}
}

func TestUC11_2_OSVFailureGivesNoCounts(t *testing.T) {
	svc, f := newFakeUpstreamService(t)
	f.OSV.failName = map[string]bool{"glibc": true}
	cmp, err := svc.CompareUpstream(context.Background(), CompareUpstreamInput{Image: "node:20"})
	var srcErr *SourceError
	if !errors.As(err, &srcErr) || srcErr.Source != "OSV.dev" || cmp != nil {
		t.Fatalf("got %+v, %v; want an OSV.dev error and no counts", cmp, err)
	}
}

func TestUC11_1_DockerHubRateLimit(t *testing.T) {
	svc, f := newFakeUpstreamService(t)
	f.Hub.limited.Store(true)
	_, err := svc.CompareUpstream(context.Background(), CompareUpstreamInput{Image: "node:20"})
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("got %v, want ErrRateLimited", err)
	}
}

func TestUC2_DockerHubImageNotFound(t *testing.T) {
	svc, _ := newFakeUpstreamService(t)
	_, err := svc.CompareUpstream(context.Background(), CompareUpstreamInput{Image: "node:99"})
	if !errors.Is(err, ErrNotFound) || !strings.Contains(err.Error(), "Docker Hub") {
		t.Fatalf("got %v, want a Docker Hub not-found error", err)
	}
}

func TestUC10_1_RepeatComparisonMakesNoOSVOrSBOMRequests(t *testing.T) {
	svc, f := newFakeUpstreamService(t)
	first := compareNode20(t, svc)
	osvBefore, blobsBefore, eolBefore := f.OSV.requests.Load(), f.Hub.blobRequests.Load(), f.EOL.requests.Load()
	if osvBefore == 0 || blobsBefore == 0 {
		t.Fatalf("the first comparison made no requests (osv %d, blobs %d)", osvBefore, blobsBefore)
	}

	again := compareNode20(t, svc)
	if n := f.OSV.requests.Load(); n != osvBefore {
		t.Errorf("repeat made %d OSV requests", n-osvBefore)
	}
	// One blob per comparison is the image configuration; the SBOM is cached by digest.
	if n := f.Hub.blobRequests.Load() - blobsBefore; n != 1 {
		t.Errorf("repeat made %d Docker Hub blob requests, want only the config", n)
	}
	if n := f.EOL.requests.Load(); n != eolBefore {
		t.Errorf("repeat made %d endoflife.date requests", n-eolBefore)
	}
	if *again.Upstream.Vulnerabilities != *first.Upstream.Vulnerabilities || *again.Chainguard.Vulnerabilities != *first.Chainguard.Vulnerabilities {
		t.Error("a repeat comparison gave different counts")
	}
}

func TestUC2_DockerHubRequestBudget(t *testing.T) {
	svc, f := newFakeUpstreamService(t)
	compareNode20(t, svc)
	// Token, index, platform manifest, config, attestation manifest and SBOM.
	if n := f.Hub.requests.Load() + f.Hub.tokenRequests.Load(); n > 10 {
		t.Errorf("a comparison made %d Docker Hub requests, want at most 10", n)
	}
}

func TestCompareUpstreamInputErrors(t *testing.T) {
	svc, _ := newFakeUpstreamService(t)
	for _, in := range []CompareUpstreamInput{{}, {Image: "   "}, {Image: "node:20", Arch: "../x"}, {Image: strings.Repeat("a", 600)}, {Image: "No Good"}} {
		var inputErr *InputError
		if _, err := svc.CompareUpstream(context.Background(), in); !errors.As(err, &inputErr) {
			t.Errorf("%+v: got %v, want an input error", in, err)
		}
	}
}

func TestUC12_1_MCPToolMatchesWebAPI(t *testing.T) {
	svc, _ := newFakeUpstreamService(t)
	srv := httptest.NewServer(newWebHandler(svc, fakeUI))
	t.Cleanup(srv.Close)
	s := mcpClient(t, svc)

	var tool UpstreamComparison
	mustCall(t, s, "compare_with_upstream", map[string]any{"image": "node:20"}, &tool)

	code, body := pollCompare(t, srv, "/api/compare-upstream?image=node:20")
	var web CompareJobStatus
	if err := json.Unmarshal(body, &web); err != nil || code != 200 || web.UpstreamComparison == nil {
		t.Fatalf("web API: %d %s %v", code, body, err)
	}
	if *tool.Upstream.Vulnerabilities != *web.Upstream.Vulnerabilities || *tool.Chainguard.Vulnerabilities != *web.Chainguard.Vulnerabilities {
		t.Errorf("tool %+v / %+v, web %+v / %+v", tool.Upstream.Vulnerabilities, tool.Chainguard.Vulnerabilities, web.Upstream.Vulnerabilities, web.Chainguard.Vulnerabilities)
	}
	if tool.Upstream.OSPackages != web.Upstream.OSPackages || tool.Source == "" || tool.CheckedAt == "" || tool.EndOfLife == nil {
		t.Errorf("tool output incomplete or different: %+v", tool)
	}

	if msg := callTool(t, s, "compare_with_upstream", map[string]any{"image": "ghcr.io/example/app:1"}, nil); !strings.Contains(msg, "only Docker Hub images") {
		t.Errorf("error = %q", msg)
	}
}

func TestUC12_ToolDescriptionStatesSourceAndLimits(t *testing.T) {
	s := mcpClient(t, newFakeService(t))
	tools, err := s.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range tools.Tools {
		if tool.Name != "compare_with_upstream" {
			continue
		}
		for _, want := range []string{"OSV.dev", "known vulnerabilities, not a guarantee", "Docker Hub"} {
			if !strings.Contains(tool.Description, want) {
				t.Errorf("description lacks %q: %s", want, tool.Description)
			}
		}
		return
	}
	t.Error("compare_with_upstream is not registered")
}
