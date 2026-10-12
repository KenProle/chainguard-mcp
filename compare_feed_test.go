package main

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// Tests for the Chainguard side of the upstream comparison reading
// advisories' status from Chainguard's feed (UC-17, UC-18). The fake node
// image has glibc-2.44 and ld-linux at 2.44-r8, busybox and c-ares.

var (
	fakeGlibc   = OSVPackage{"Wolfi", "glibc-2.44", "2.44-r8"}
	fakeLdLinux = OSVPackage{"Wolfi", "ld-linux", "2.44-r8"}
)

// addAdvisory lists an advisory on the fake OSV.dev as osvStatus with no fix,
// and publishes current as the feed's record unless it is nil.
func addAdvisory(f *fakeUpstream, pkg OSVPackage, id, cve, osvStatus string, current map[string]any) {
	f.OSV.add(pkg, advisoryVuln(id, pkg.Name, osvStatus, "", []string{cve}))
	if current != nil {
		f.Feed.add(current)
	}
}

func chainguardCounts(t *testing.T, svc *Service) (VulnCounts, []string) {
	t.Helper()
	cmp := compareNode20(t, svc)
	return *cmp.Chainguard.Vulnerabilities, cmp.Notes
}

func TestUC17_1_UC18_1_FixedInInstalledVersionNotCounted(t *testing.T) {
	svc, f := newFakeUpstreamService(t)
	// Like CGA-2727-9j94-3x3p on 2026-10-11: OSV.dev's copy is pending an
	// upstream fix, the feed's record is fixed in the installed 2.44-r8.
	addAdvisory(f, fakeGlibc, "CGA-2727-9j94-3x3p", "CVE-2026-89092", "pending_upstream_fix",
		advisoryVuln("CGA-2727-9j94-3x3p", fakeGlibc.Name, "fixed", "2.44-r8", []string{"CVE-2026-89092"}))
	// Fixed in an older version than the installed one: not counted either.
	addAdvisory(f, fakeLdLinux, "CGA-84qm-pxw8-f58f", "CVE-2026-8674", "pending_upstream_fix",
		advisoryVuln("CGA-84qm-pxw8-f58f", fakeLdLinux.Name, "fixed", "2.44-r3", []string{"CVE-2026-8674"}))

	got, notes := chainguardCounts(t, svc)
	if want := (VulnCounts{Total: 1, NoFixYet: 1}); got != want {
		t.Errorf("chainguard = %+v, want only the fixture's open CVE-2026-0001: %+v", got, want)
	}
	if slices.ContainsFunc(notes, func(n string) bool { return strings.Contains(n, "advisory feed") }) {
		t.Errorf("no advisory is missing from the feed, but notes = %q", notes)
	}
}

func TestUC18_2_PartlyFixedVulnerability(t *testing.T) {
	svc, f := newFakeUpstreamService(t)
	// The same CVE: fixed in the installed version for glibc-2.44, not
	// planned for ld-linux. Only the open advisory decides its status: kept,
	// the fixed one would count as no fix yet and outrank won't fix.
	addAdvisory(f, fakeGlibc, "CGA-0000-0000-0011", "CVE-2026-0011", "pending_upstream_fix",
		advisoryVuln("CGA-0000-0000-0011", fakeGlibc.Name, "fixed", "2.44-r8", []string{"CVE-2026-0011"}))
	addAdvisory(f, fakeLdLinux, "CGA-0000-0000-0012", "CVE-2026-0011", "pending_upstream_fix",
		advisoryVuln("CGA-0000-0000-0012", fakeLdLinux.Name, "fix_not_planned", "", []string{"CVE-2026-0011"}))

	got, _ := chainguardCounts(t, svc)
	if want := (VulnCounts{Total: 2, NoFixYet: 1, WontFix: 1}); got != want {
		t.Errorf("chainguard = %+v, want CVE-2026-0011 once as won't fix, plus CVE-2026-0001: %+v", got, want)
	}
}

func TestUC18_3_FixInNewerVersionAndUC5_4_FixNotPlanned(t *testing.T) {
	svc, f := newFakeUpstreamService(t)
	addAdvisory(f, fakeGlibc, "CGA-0000-0000-0021", "CVE-2026-0021", "pending_upstream_fix",
		advisoryVuln("CGA-0000-0000-0021", fakeGlibc.Name, "pending_upstream_fix", "2.44-r9", []string{"CVE-2026-0021"}))
	addAdvisory(f, fakeLdLinux, "CGA-0000-0000-0022", "CVE-2026-0022", "pending_upstream_fix",
		advisoryVuln("CGA-0000-0000-0022", fakeLdLinux.Name, "fix_not_planned", "", []string{"CVE-2026-0022"}))

	got, _ := chainguardCounts(t, svc)
	if want := (VulnCounts{Total: 3, FixAvailable: 1, NoFixYet: 1, WontFix: 1}); got != want {
		t.Errorf("chainguard = %+v, want %+v", got, want)
	}
}

func TestUC17_2_AdvisoryMissingFromFeed(t *testing.T) {
	svc, f := newFakeUpstreamService(t)
	addAdvisory(f, fakeGlibc, "CGA-0000-0000-0031", "CVE-2026-0031", "pending_upstream_fix", nil)

	got, notes := chainguardCounts(t, svc)
	if want := (VulnCounts{Total: 2, NoFixYet: 2}); got != want {
		t.Errorf("chainguard = %+v, want OSV.dev's copy counted: %+v", got, want)
	}
	if !slices.ContainsFunc(notes, func(n string) bool {
		return strings.Contains(n, "CGA-0000-0000-0031") && strings.Contains(n, "couldn't be confirmed")
	}) {
		t.Errorf("notes = %q, want one naming CGA-0000-0000-0031", notes)
	}
}

func TestUC17_3_RecordWithoutThePackage(t *testing.T) {
	svc, f := newFakeUpstreamService(t)
	// The feed's current record covers another package only.
	addAdvisory(f, fakeGlibc, "CGA-0000-0000-0041", "CVE-2026-0041", "pending_upstream_fix",
		advisoryVuln("CGA-0000-0000-0041", "glibc-2.43", "fixed", "2.43-r1", []string{"CVE-2026-0041"}))

	got, _ := chainguardCounts(t, svc)
	if want := (VulnCounts{Total: 2, NoFixYet: 2}); got != want {
		t.Errorf("chainguard = %+v, want the advisory still counted from OSV.dev's copy: %+v", got, want)
	}
}

func TestUC17_4_UpstreamSideNeverReadsTheFeed(t *testing.T) {
	svc, f := newFakeUpstreamService(t)
	cmp := compareNode20(t, svc)
	if got, want := *cmp.Upstream.Vulnerabilities, (VulnCounts{Total: 3, FixAvailable: 1, NoFixYet: 1, WontFix: 1}); got != want {
		t.Errorf("upstream = %+v, want %+v", got, want)
	}
	f.Feed.mu.Lock()
	defer f.Feed.mu.Unlock()
	for id := range f.Feed.requested {
		if !strings.HasPrefix(id, "CGA-") {
			t.Errorf("the feed was asked for %s", id)
		}
	}
	if n := f.Feed.requests.Load(); n != 2 {
		t.Errorf("feed requests = %d, want 2 (the two Chainguard advisories)", n)
	}
}

func TestUC11_3_FeedFailure(t *testing.T) {
	svc, f := newFakeUpstreamService(t)
	f.Feed.failID = map[string]bool{"CGA-0000-0000-0001": true}
	cmp, err := svc.CompareUpstream(context.Background(), CompareUpstreamInput{Image: "node:20"})
	var srcErr *SourceError
	if !errors.As(err, &srcErr) || srcErr.Source != "Chainguard advisories" || cmp != nil {
		t.Fatalf("got %+v, %v; want a Chainguard advisories source error and no counts", cmp, err)
	}
}

// fakeClock is a controllable clock for the OSV and feed caches.
type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

func TestUC10_1_UC10_2_RepeatComparison(t *testing.T) {
	svc, f := newFakeUpstreamService(t)
	clock := &fakeClock{now: time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)}
	svc.OSV.Now, svc.Feed.Now = clock.Now, clock.Now

	compareNode20(t, svc)
	osv, feed := f.OSV.requests.Load(), f.Feed.requests.Load()

	// UC-10.1: within an hour, nothing is asked again.
	clock.advance(59 * time.Minute)
	compareNode20(t, svc)
	if f.OSV.requests.Load() != osv || f.Feed.requests.Load() != feed {
		t.Errorf("repeat within an hour: OSV %d→%d, feed %d→%d requests", osv, f.OSV.requests.Load(), feed, f.Feed.requests.Load())
	}

	// UC-10.2: after an hour, only the 4 Chainguard packages and their 2
	// advisories are asked again; the 2 upstream lookups stay cached.
	clock.advance(2 * time.Hour)
	compareNode20(t, svc)
	if d := f.OSV.requests.Load() - osv; d != 4 {
		t.Errorf("OSV requests after an hour = %d, want 4", d)
	}
	if d := f.Feed.requests.Load() - feed; d != 2 {
		t.Errorf("feed requests after an hour = %d, want 2", d)
	}
}

func TestUC10_3_FeedRecordSharedWithOpenAdvisories(t *testing.T) {
	svc, f := newFakeUpstreamService(t)
	if _, err := svc.OpenAdvisories(context.Background(), OpenAdvisoriesInput{Image: "node"}); err != nil {
		t.Fatal(err)
	}
	compareNode20(t, svc)
	for _, id := range []string{"CGA-0000-0000-0001", "CGA-0000-0000-0002"} {
		if n := f.Feed.requestsFor(id); n != 1 {
			t.Errorf("%s read %d times from the feed, want 1", id, n)
		}
	}
}

func TestUC6_3_SourceNamesTheFeed(t *testing.T) {
	svc, _ := newFakeUpstreamService(t)
	cmp := compareNode20(t, svc)
	if !strings.Contains(cmp.Source, "Chainguard's advisory feed") || !strings.Contains(cmp.Source, "known vulnerabilities only") {
		t.Errorf("source = %q", cmp.Source)
	}
	cs := mcpClient(t, svc)
	tools, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range tools.Tools {
		if tool.Name != "compare_with_upstream" {
			continue
		}
		if !strings.Contains(tool.Description, "advisory feed") {
			t.Errorf("description = %q, want it to mention Chainguard's advisory feed", tool.Description)
		}
		for _, bad := range []string{"vulnerability-free", "is secure"} {
			if strings.Contains(strings.ToLower(tool.Description), bad) {
				t.Errorf("description must not say %q", bad)
			}
		}
	}
}
