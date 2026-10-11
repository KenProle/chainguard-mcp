package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"sync/atomic"
	"testing"
	"time"
)

func TestUC2_1_ParseDockerHubRef(t *testing.T) {
	for ref, want := range map[string][2]string{
		"node":                           {"library/node", "latest"},
		"node:20":                        {"library/node", "20"},
		"NODE:20-Alpine":                 {"library/node", "20-alpine"},
		"docker.io/library/python:3.12":  {"library/python", "3.12"},
		"index.docker.io/bitnami/redis":  {"bitnami/redis", "latest"},
		"bitnami/redis:7.2":              {"bitnami/redis", "7.2"},
		"docker.io/node":                 {"library/node", "latest"},
		"node@sha256:" + hex64("a"):      {"library/node", "sha256:" + hex64("a")},
		"registry-1.docker.io/grafana/x": {"grafana/x", "latest"},
	} {
		repo, tag, err := parseDockerHubRef(ref)
		if err != nil || repo != want[0] || tag != want[1] {
			t.Errorf("%q: got %q %q %v, want %v", ref, repo, tag, err, want)
		}
	}
}

func TestUC2_3_ParseDockerHubRefOtherRegistry(t *testing.T) {
	for _, ref := range []string{"ghcr.io/example/app:1", "quay.io/prometheus/prometheus", "mcr.microsoft.com/dotnet/aspnet:8.0", "localhost:5000/app", "gcr.io/distroless/static"} {
		if _, _, err := parseDockerHubRef(ref); !errors.Is(err, ErrUnsupportedRegistry) {
			t.Errorf("%q: want ErrUnsupportedRegistry, got %v", ref, err)
		}
	}
	for _, ref := range []string{"", "node:", "Node Js", "../etc", "node:20@sha256:" + hex64("a"), "node@sha256:abc"} {
		var inputErr *InputError
		if _, _, err := parseDockerHubRef(ref); !errors.As(err, &inputErr) {
			t.Errorf("%q: want InputError, got %v", ref, err)
		}
	}
}

func hex64(s string) string {
	out := ""
	for len(out) < 64 {
		out += s
	}
	return out[:64]
}

func TestUC2_1_DockerHubTokenAndRepository(t *testing.T) {
	reg, hub := newFakeDockerHub(t)
	hub.addImage("library/node", "20", dockerImage{created: "2026-04-22T00:00:00Z"})

	m, digest, _, err := reg.PlatformManifest(context.Background(), "library/node", "20", "amd64")
	if err != nil {
		t.Fatal(err)
	}
	if m.Config.Digest == "" || digest == "" {
		t.Fatalf("empty manifest: %+v %q", m, digest)
	}
	if got := hub.lastService.Load(); got != "registry.docker.io" {
		t.Errorf("token service = %v, want registry.docker.io", got)
	}
	// The repository path goes in the scope and URL unchanged: no namespace is added.
	if _, _, _, err := reg.PlatformManifest(context.Background(), "library/node", "20", "ppc64le"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown arch: got %v, want ErrNotFound", err)
	}
}

func TestUC11_1_RateLimited(t *testing.T) {
	reg, hub := newFakeDockerHub(t)
	hub.addImage("library/node", "20", dockerImage{})
	hub.limited.Store(true)
	if _, _, _, err := reg.PlatformManifest(context.Background(), "library/node", "20", "amd64"); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("got %v, want ErrRateLimited", err)
	}
}

func TestUC2_1_ChainguardRegistryUnchanged(t *testing.T) {
	svc := newFakeService(t)
	if got := svc.Registry.repo("python"); got != "chainguard/python" {
		t.Errorf("repo = %q", got)
	}
	if got := svc.Registry.Ref("python"); got != "cgr.dev/chainguard/python" {
		t.Errorf("Ref = %q", got)
	}
}

func TestUC2_1_ReadsDockerAttestation(t *testing.T) {
	reg, hub := newFakeDockerHub(t)
	doc := debianSPDX(t, []fakeDebPkg{{"libssl3", "3.0.19-1~deb12u2", "openssl", "3.0.19-1~deb12u2"}})
	hub.addImage("library/node", "20", dockerImage{sbom: doc})

	got, digest, err := reg.SBOMDocument(context.Background(), "library/node", "20", "amd64")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(doc) || digest == "" {
		t.Errorf("SBOM mismatch or empty digest %q", digest)
	}
}

func TestUC2_2_NoAttestation(t *testing.T) {
	reg, hub := newFakeDockerHub(t)
	hub.addImage("bitnami/redis", "latest", dockerImage{})
	if _, _, err := reg.SBOMDocument(context.Background(), "bitnami/redis", "latest", "amd64"); !errors.Is(err, ErrNoSBOM) {
		t.Fatalf("got %v, want ErrNoSBOM", err)
	}
}

func TestUC3_1_UpstreamPackagesFromRealSBOM(t *testing.T) {
	raw, err := os.ReadFile("testdata/node20-trimmed.spdx.json")
	if err != nil {
		t.Fatal(err)
	}
	got, err := parseUpstreamPackages(raw)
	if err != nil {
		t.Fatal(err)
	}
	// libssl3, libc6, linux-libc-dev, adduser and mercurial are installed;
	// openssl, glibc and linux are source packages the SBOM lists beside them.
	if got.Installed != 5 {
		t.Errorf("installed = %d, want 5", got.Installed)
	}
	var names []string
	for _, l := range got.Lookups {
		if l.Ecosystem != "Debian:12" {
			t.Errorf("%s: ecosystem %q, want Debian:12", l.Name, l.Ecosystem)
		}
		names = append(names, l.Name+"@"+l.Version)
	}
	want := []string{"adduser@3.134", "glibc@2.36-9+deb12u13", "mercurial@6.3.2-1+deb12u1", "openssl@3.0.19-1~deb12u2"}
	if !slices.Equal(names, want) {
		t.Errorf("lookups = %v, want %v (the kernel source package is skipped)", names, want)
	}
	if got.Language["npm"] != 2 || got.Language["pypi"] != 1 || len(got.Language) != 2 {
		t.Errorf("language = %v, want 2 npm and 1 pypi", got.Language)
	}
}

func TestUC4_2_UpstreamLookupsAreDistinctSources(t *testing.T) {
	doc := debianSPDX(t, []fakeDebPkg{
		{"libssl3", "3.0.19-1", "openssl", "3.0.19-1"},
		{"openssl", "3.0.19-1", "openssl", "3.0.19-1"},
		{"libssl-dev", "3.0.19-1", "openssl", "3.0.19-1"},
	})
	got, err := parseUpstreamPackages(doc)
	if err != nil {
		t.Fatal(err)
	}
	if got.Installed != 3 || len(got.Lookups) != 1 {
		t.Errorf("installed=%d lookups=%v, want 3 installed binaries and one openssl lookup", got.Installed, got.Lookups)
	}
}

func TestUC3_1_UpstreamEcosystems(t *testing.T) {
	for purl, want := range map[string]string{
		"pkg:apk/alpine/musl@1.2.5-r10?os_name=alpine&os_version=3.23":       "Alpine:v3.23",
		"pkg:apk/alpine/musl@1.2.5-r10?os_name=alpine&os_version=3.20.3":     "Alpine:v3.20",
		"pkg:deb/ubuntu/libc6@2.39-0ubuntu8?os_name=ubuntu&os_version=24.04": "Ubuntu:24.04:LTS",
		"pkg:deb/ubuntu/libc6@2.40-0ubuntu1?os_name=ubuntu&os_version=24.10": "Ubuntu:24.10",
		"pkg:deb/debian/bash@5.2.15-2%2Bb10?os_name=debian&os_version=12":    "Debian:12",
		"pkg:deb/debian/bash@5.2.15-2?os_name=debian&os_version=13.1":        "Debian:13",
		"pkg:rpm/fedora/bash@5.2?os_name=fedora&os_version=40":               "",
		"pkg:deb/debian/bash@5.2.15-2":                                       "",
		"pkg:deb/raspbian/bash@5.2.15-2?os_name=raspbian&os_version=12":      "",
	} {
		p, ok := parsePurl(purl)
		if !ok {
			t.Fatalf("%s: not parsed", purl)
		}
		if got := osvEcosystemFor(p); got != want {
			t.Errorf("%s: ecosystem %q, want %q", purl, got, want)
		}
	}

	raw := mustJSON(t, map[string]any{"packages": []map[string]any{{
		"SPDXID": "SPDXRef-a", "name": "musl", "versionInfo": "1.2.5-r10",
		"externalRefs": []map[string]string{{"referenceType": "purl", "referenceLocator": "pkg:apk/alpine/musl@1.2.5-r10?os_name=alpine&os_version=3.23"}},
	}}})
	got, err := parseUpstreamPackages(raw)
	if err != nil || got.Installed != 1 || got.Lookups[0] != (OSVPackage{"Alpine:v3.23", "musl", "1.2.5-r10"}) {
		t.Errorf("alpine: %+v %v", got, err)
	}
	unsupported := mustJSON(t, map[string]any{"packages": []map[string]any{{
		"SPDXID": "SPDXRef-a", "name": "bash", "versionInfo": "5",
		"externalRefs": []map[string]string{{"referenceType": "purl", "referenceLocator": "pkg:deb/raspbian/bash@5?os_name=raspbian&os_version=12"}},
	}}})
	// An unsupported distribution is reported, not silently uncounted (UC-15.2).
	got, err = parseUpstreamPackages(unsupported)
	if err != nil || got.Installed != 0 || len(got.Lookups) != 0 || !slices.Equal(got.Unsupported, []string{"deb/raspbian 12"}) {
		t.Errorf("raspbian: %+v %v", got, err)
	}
}

func TestUC4_ChainguardOSVEcosystem(t *testing.T) {
	for distro, want := range map[string]string{"wolfi": "Wolfi", "chainguard": "Chainguard"} {
		if got, ok := osvEcosystemForDistro(distro); !ok || got != want {
			t.Errorf("%s: got %q %v, want %q", distro, got, ok, want)
		}
	}
	if _, ok := osvEcosystemForDistro("debian"); ok {
		t.Error("an unknown distro has no ecosystem")
	}
}

func TestUC10_1_OSVPaginationAndCache(t *testing.T) {
	o, fake := newFakeOSV(t)
	pkg := OSVPackage{"Debian:12", "openssl", "3.0.19-1~deb12u2"}
	for i := range 5 {
		fake.add(pkg, map[string]any{"id": fmt.Sprintf("DEBIAN-CVE-%d", i)})
	}
	fake.pageSize = 2

	got, err := o.Query(context.Background(), pkg)
	if err != nil || len(got) != 5 {
		t.Fatalf("got %d vulns, %v; want all 5 across pages", len(got), err)
	}
	if n := fake.requests.Load(); n != 3 {
		t.Errorf("requests = %d, want 3 pages", n)
	}

	// UC-10.1: a second lookup within the TTL makes no request.
	if _, err := o.Query(context.Background(), pkg); err != nil || fake.requests.Load() != 3 {
		t.Errorf("repeat lookup made requests (%d) or failed: %v", fake.requests.Load(), err)
	}
	// A different version is a different cache key.
	other := pkg
	other.Version = "3.0.20-1"
	if _, err := o.Query(context.Background(), other); err != nil || fake.requests.Load() != 4 {
		t.Errorf("another version should query again; requests=%d err=%v", fake.requests.Load(), err)
	}
	// Expired entries are fetched again.
	o.TTL = time.Nanosecond
	time.Sleep(time.Millisecond)
	if _, err := o.Query(context.Background(), pkg); err != nil || fake.requests.Load() != 7 {
		t.Errorf("expired entry should query again; requests=%d err=%v", fake.requests.Load(), err)
	}
}

func TestUC10_OSVConcurrencyLimit(t *testing.T) {
	o, fake := newFakeOSV(t)
	fake.delay = 20 * time.Millisecond
	var pkgs []OSVPackage
	for i := range 60 {
		pkgs = append(pkgs, OSVPackage{"Debian:12", fmt.Sprintf("pkg%d", i), "1"})
	}
	var lastDone atomic.Int64
	out, err := o.QueryAll(context.Background(), pkgs, func(done, total int) {
		if total != 60 {
			t.Errorf("total = %d", total)
		}
		lastDone.Store(int64(done))
	})
	if err != nil || len(out) != 60 || lastDone.Load() != 60 {
		t.Fatalf("got %d results, progress %d, %v", len(out), lastDone.Load(), err)
	}
	if m := fake.maxInFlight.Load(); m > 10 || m < 2 {
		t.Errorf("max requests in flight = %d, want 2..10", m)
	}
}

func TestUC11_2_OSVFailureIsAnError(t *testing.T) {
	o, fake := newFakeOSV(t)
	fake.failName = map[string]bool{"pkg3": true}
	var pkgs []OSVPackage
	for i := range 8 {
		pkgs = append(pkgs, OSVPackage{"Debian:12", fmt.Sprintf("pkg%d", i), "1"})
	}
	out, err := o.QueryAll(context.Background(), pkgs, nil)
	var srcErr *SourceError
	if !errors.As(err, &srcErr) || srcErr.Source != "OSV.dev" || out != nil {
		t.Fatalf("got %v, %v; want an OSV.dev SourceError and no partial result", out, err)
	}
}

func loadOSVFixture[T any](t *testing.T, name string) T {
	t.Helper()
	raw, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	var v T
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	return v
}

var openssl12 = OSVPackage{"Debian:12", "openssl", "3.0.19-1~deb12u2"}

func TestUC5_ClassifyRealRecords(t *testing.T) {
	// Records captured from OSV.dev on 2026-10-11.
	deb := loadOSVFixture[map[string]OSVVuln](t, "osv-debian-openssl.json")

	// Debian 12 has a fix newer than the installed version.
	if got := classify(deb["fixed"], openssl12); got != FixAvailable {
		t.Errorf("fixed record: %s, want fix_available", got)
	}
	// Debian marks this one "unimportant" and has no Debian 12 fix, though 13 does.
	if got := classify(deb["unimportant"], openssl12); got != WontFix {
		t.Errorf("unimportant record: %s, want wont_fix", got)
	}
	// Chainguard's pending_upstream_fix advisory is no fix yet.
	cg := loadOSVFixture[OSVVuln](t, "osv-chainguard-glibc.json")
	if got := classify(cg, OSVPackage{"Wolfi", "glibc-2.44", "2.44-r8"}); got != NoFixYet {
		t.Errorf("chainguard record: %s, want no_fix_yet", got)
	}
}

func TestUC5_3_FixInAnotherReleaseOnly(t *testing.T) {
	// The real "unimportant" record has a fix for Debian 13 and 14 only; with
	// its urgency changed it is plainly "no fix yet" for a Debian 12 image.
	deb := loadOSVFixture[map[string]OSVVuln](t, "osv-debian-openssl.json")
	v := deb["unimportant"]
	for i := range v.Affected {
		v.Affected[i].EcosystemSpecific.Urgency = "medium"
	}
	if got := classify(v, openssl12); got != NoFixYet {
		t.Errorf("Debian 12: %s, want no_fix_yet", got)
	}
	trixie := OSVPackage{"Debian:13", "openssl", "3.5.0-0"}
	if got := classify(v, trixie); got != FixAvailable {
		t.Errorf("Debian 13: %s, want fix_available", got)
	}
	// No advisory entry for the package at all.
	if got := classify(v, OSVPackage{"Debian:12", "other", "1"}); got != NoFixYet {
		t.Errorf("unrelated package: %s, want no_fix_yet", got)
	}
}

func TestUC4_2_SummarizeCountsEachIDOnce(t *testing.T) {
	deb := loadOSVFixture[map[string]OSVVuln](t, "osv-debian-openssl.json")
	libssl := OSVPackage{"Debian:12", "libssl3", "3.0.19-1~deb12u2"}
	// The same fixed vulnerability on two packages counts once, and the same ID
	// that is "won't fix" for openssl but fixable for another package counts as fix available.
	wontFix := deb["unimportant"]
	fixable := wontFix
	fixable.Affected = deb["fixed"].Affected
	for i := range fixable.Affected {
		fixable.Affected[i].Package.Name = "libssl3"
	}
	got := summarize(map[OSVPackage][]OSVVuln{
		openssl12: {deb["fixed"], wontFix},
		libssl:    {deb["fixed"], fixable},
	})
	want := VulnCounts{Total: 2, FixAvailable: 2}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}

	got = summarize(map[OSVPackage][]OSVVuln{openssl12: {deb["unimportant"]}})
	if want := (VulnCounts{Total: 1, WontFix: 1}); got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
	if got := summarize(nil); got != (VulnCounts{}) {
		t.Errorf("empty: %+v", got)
	}
}
