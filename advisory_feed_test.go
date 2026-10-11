package main

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"
)

func feedID(i int) string { return fmt.Sprintf("CGA-aaaa-bbbb-%04d", i) }

func TestOA12_1_FeedRepeatWithinAnHourIsCached(t *testing.T) {
	feed, fake := newFakeFeed(t)
	clk := &clock{t: time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)}
	feed.Now = clk.now
	fake.add(advisoryVuln("CGA-aaaa-0000-0001", "a", "detection", "", []string{"CVE-2026-0001"}))
	ids := []string{"CGA-aaaa-0000-0001", "CGA-aaaa-0000-0002"} // the second is not in the feed

	out, first, err := feed.FetchAll(context.Background(), ids)
	if err != nil || !first.Equal(clk.t) || out["CGA-aaaa-0000-0001"] == nil || out["CGA-aaaa-0000-0002"] != nil {
		t.Fatalf("first = %v, %v, %v", out, first, err)
	}
	clk.t = clk.t.Add(59 * time.Minute)
	before := fake.requests.Load()
	_, second, err := feed.FetchAll(context.Background(), ids)
	if err != nil || fake.requests.Load() != before {
		t.Fatalf("repeat made %d requests (a 404 is cached too), err %v", fake.requests.Load()-before, err)
	}
	if !second.Equal(first) {
		t.Errorf("checked time = %v, want the first fetch %v", second, first)
	}
}

func TestOA12_2_FeedAfterAnHourReadsAgain(t *testing.T) {
	feed, fake := newFakeFeed(t)
	clk := &clock{t: time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)}
	feed.Now = clk.now
	fake.add(advisoryVuln("CGA-aaaa-0000-0001", "a", "detection", "", []string{"CVE-2026-0001"}))
	ids := []string{"CGA-aaaa-0000-0001"}
	if _, _, err := feed.FetchAll(context.Background(), ids); err != nil {
		t.Fatal(err)
	}
	clk.t = clk.t.Add(61 * time.Minute)
	before := fake.requests.Load()
	_, at, err := feed.FetchAll(context.Background(), ids)
	if err != nil || fake.requests.Load() != before+1 || !at.Equal(clk.t) {
		t.Errorf("requests +%d, at %v, err %v; want one request at %v", fake.requests.Load()-before, at, err, clk.t)
	}
}

func TestOA12_4_FeedConcurrencyLimit(t *testing.T) {
	feed, fake := newFakeFeed(t)
	fake.delay = 20 * time.Millisecond
	var ids []string
	for i := range 60 {
		fake.add(advisoryVuln(feedID(i), "a", "detection", "", []string{"CVE-2026-0001"}))
		ids = append(ids, feedID(i))
	}
	out, _, err := feed.FetchAll(context.Background(), ids)
	if err != nil || len(out) != 60 {
		t.Fatalf("got %d records, %v", len(out), err)
	}
	if m := fake.maxInFlight.Load(); m > 10 || m < 2 {
		t.Errorf("max requests in flight = %d, want 2..10", m)
	}
}

func TestOA17_3_FeedMissingAdvisoryIsNilNotAnError(t *testing.T) {
	feed, _ := newFakeFeed(t)
	out, _, err := feed.FetchAll(context.Background(), []string{"CGA-zzzz-0000-0000"})
	if v, ok := out["CGA-zzzz-0000-0000"]; err != nil || !ok || v != nil {
		t.Errorf("got %v, %v", out, err)
	}
}

func TestOA17_4_FeedServerErrorFailsTheWholeSet(t *testing.T) {
	feed, fake := newFakeFeed(t)
	var ids []string
	for i := range 8 {
		fake.add(advisoryVuln(feedID(i), "a", "detection", "", []string{"CVE-2026-0001"}))
		ids = append(ids, feedID(i))
	}
	fake.failID = map[string]bool{feedID(3): true}
	out, _, err := feed.FetchAll(context.Background(), ids)
	var srcErr *SourceError
	if !errors.As(err, &srcErr) || srcErr.Source != "Chainguard advisories" || out != nil {
		t.Fatalf("got %v, %v; want a feed SourceError and no partial result", out, err)
	}
}

func TestOA17_4_FeedInvalidBodyFails(t *testing.T) {
	feed, fake := newFakeFeed(t)
	fake.records = map[string][]byte{"CGA-aaaa-0000-0001": []byte(`<html>not json`), "CGA-aaaa-0000-0002": []byte(`{"id":"CGA-other-0000-0000"}`)}
	for _, id := range []string{"CGA-aaaa-0000-0001", "CGA-aaaa-0000-0002"} {
		if _, _, err := feed.FetchAll(context.Background(), []string{id}); err == nil {
			t.Errorf("%s: want an error", id)
		}
	}
}

func TestFeedOnlyFetchesAdvisoryIDs(t *testing.T) {
	feed, fake := newFakeFeed(t)
	for _, id := range []string{"../../etc/passwd", "CVE-2026-0001", "cga-aaaa-bbbb-cccc"} {
		if _, _, err := feed.FetchAll(context.Background(), []string{id}); err == nil {
			t.Errorf("%q: want an error", id)
		}
	}
	if n := fake.requests.Load(); n != 0 {
		t.Errorf("%d requests for IDs that aren't advisories", n)
	}
}

func openAdvisoriesFor(t *testing.T, svc *Service, tag, id string) *AdvisoryReport {
	t.Helper()
	r, err := svc.OpenAdvisories(context.Background(), OpenAdvisoriesInput{Image: "python", Tag: tag, ID: id})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestOA17_1_StatusComesFromTheFeedNotOSV(t *testing.T) {
	svc, _, _, _ := newFakeAdvisoryService(t)
	r := openAdvisoriesFor(t, svc, "latest", "")
	// CVE-2026-0106 is pending in OSV.dev's copy and fixed in the feed.
	if hasVuln(*r, "CVE-2026-0106") || r.Total != 4 {
		t.Errorf("total %d, CVE-2026-0106 reported: %v", r.Total, hasVuln(*r, "CVE-2026-0106"))
	}
	// An ID lookup agrees (OA-9.4).
	l := openAdvisoriesFor(t, svc, "latest", "CGA-bbbb-0000-0005")
	if l.Match != nil || !strings.Contains(l.LookupNote, "No open advisory for CGA-BBBB-0000-0005") {
		t.Errorf("match %+v, note %q", l.Match, l.LookupNote)
	}
}

func TestOA17_2_StatusAndFixedVersionFromTheFeedRecord(t *testing.T) {
	// The feed's record differs from OSV.dev's: it now has a fixed version.
	svc, _, _, feed := newFakeAdvisoryService(t)
	feed.add(advisoryVuln("CGA-aaaa-0000-0001", "python-3.13", "fixed", "3.13.9-r0", []string{"CVE-2026-0101"}))
	r := openAdvisoriesFor(t, svc, "latest", "")
	v := findVuln(t, *r, "CVE-2026-0101")
	if v.Status != AdvFixAvailable || v.Packages[0].FixedVersion != "3.13.9-r0" {
		t.Errorf("got %+v", v)
	}
}

func TestOA17_3_MissingFromTheFeedUsesOSVAndSaysSo(t *testing.T) {
	svc, _, _, _ := newFakeAdvisoryService(t)
	r := openAdvisoriesFor(t, svc, "latest-dev", "")
	v := findVuln(t, *r, "CVE-2026-0107")
	if v.Status != AdvPendingUpstream {
		t.Errorf("status = %s, want the OSV.dev copy's pending_upstream_fix", v.Status)
	}
	if !slices.Equal(r.StatusFromOSV, []string{"CGA-cccc-0000-0002"}) || !strings.Contains(r.Note, "CGA-cccc-0000-0002") ||
		!strings.Contains(r.Note, "couldn't be confirmed") {
		t.Errorf("status_from_osv %v, note %q", r.StatusFromOSV, r.Note)
	}
	// An image whose advisories are all in the feed has none.
	if l := openAdvisoriesFor(t, svc, "latest", ""); len(l.StatusFromOSV) != 0 {
		t.Errorf("latest: %v", l.StatusFromOSV)
	}
}

func TestOA17_4_FeedFailureFailsOpenAdvisories(t *testing.T) {
	svc, _, _, feed := newFakeAdvisoryService(t)
	feed.failID = map[string]bool{"CGA-bbbb-0000-0002": true}
	_, err := svc.OpenAdvisories(context.Background(), OpenAdvisoriesInput{Image: "python"})
	var srcErr *SourceError
	if !errors.As(err, &srcErr) || srcErr.Source != "Chainguard advisories" {
		t.Errorf("got %v, want a feed SourceError", err)
	}
}

func TestOA12_3_FeedRecordsAreSharedBetweenTags(t *testing.T) {
	svc, _, _, feed := newFakeAdvisoryService(t)
	openAdvisoriesFor(t, svc, "latest", "")
	if n := feed.requestsFor("CGA-aaaa-0000-0001"); n != 1 {
		t.Fatalf("latest read it %d times", n)
	}
	openAdvisoriesFor(t, svc, "latest-dev", "")
	for _, id := range []string{"CGA-aaaa-0000-0001", "CGA-aaaa-0000-0004"} {
		if n := feed.requestsFor(id); n != 1 {
			t.Errorf("%s was read %d times across the two tags", id, n)
		}
	}
	if n := feed.requestsFor("CGA-cccc-0000-0001"); n != 1 {
		t.Errorf("the latest-dev-only advisory was read %d times", n)
	}
}

func TestOA12_1_RepeatedRequestMakesNoRequestsToEitherSource(t *testing.T) {
	svc, osv, _, feed := newFakeAdvisoryService(t)
	first := openAdvisoriesFor(t, svc, "latest", "")
	o, f := osv.requests.Load(), feed.requests.Load()
	second := openAdvisoriesFor(t, svc, "latest", "")
	if osv.requests.Load() != o || feed.requests.Load() != f {
		t.Errorf("repeat made %d OSV and %d feed requests", osv.requests.Load()-o, feed.requests.Load()-f)
	}
	if second.CheckedAt != first.CheckedAt {
		t.Errorf("checked_at %s, want the first check's %s", second.CheckedAt, first.CheckedAt)
	}
}

func TestOA12_CheckedAtIsTheOlderOfTheTwoSources(t *testing.T) {
	svc, _, _, _ := newFakeAdvisoryService(t)
	osvClock := &clock{t: time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)}
	svc.OSV.Now = osvClock.now
	feedClock := &clock{t: time.Date(2026, 10, 11, 11, 30, 0, 0, time.UTC)}
	svc.Feed.Now = feedClock.now
	r := openAdvisoriesFor(t, svc, "latest", "")
	if r.CheckedAt != "2026-10-11T11:30:00Z" {
		t.Errorf("checked_at = %s, want the feed's earlier time", r.CheckedAt)
	}
}
