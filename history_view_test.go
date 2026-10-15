package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const historyFixture = "testdata/catalog-history-2026-10-14.jsonl"

func fixtureBytes(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile(historyFixture)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// fixtureHead is the fixture without its last n lines.
func fixtureHead(t *testing.T, n int) []byte {
	t.Helper()
	lines := bytes.Split(bytes.TrimRight(fixtureBytes(t), "\r\n"), []byte("\n"))
	return append(bytes.Join(lines[:len(lines)-n], []byte("\n")), '\n')
}

func fixtureCopy(t *testing.T) *historyCopy {
	t.Helper()
	c, err := buildHistoryCopy(fixtureBytes(t), copyBuild, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func day(s string) time.Time {
	d, err := time.Parse(time.DateOnly, s)
	if err != nil {
		panic(err)
	}
	return d.Add(12 * time.Hour)
}

func fixtureReport(t *testing.T, in HistoryInput, today string) *HistoryReport {
	t.Helper()
	return fixtureCopy(t).report(in, day(today))
}

// fakeDataBranch serves a history file and counts requests.
type fakeDataBranch struct {
	*httptest.Server
	body     atomic.Value // []byte
	status   atomic.Int64
	requests atomic.Int64
	delay    atomic.Int64 // nanoseconds
}

func newFakeDataBranch(t *testing.T, body []byte) *fakeDataBranch {
	f := &fakeDataBranch{}
	f.body.Store(body)
	f.status.Store(200)
	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.requests.Add(1)
		time.Sleep(time.Duration(f.delay.Load()))
		if s := int(f.status.Load()); s != 200 {
			http.Error(w, "no", s)
			return
		}
		w.Write(f.body.Load().([]byte))
	}))
	t.Cleanup(f.Close)
	return f
}

// newTestSource is a HistorySource with an embedded copy, a fake clock and
// the fake data branch.
func newTestSource(embedded []byte, f *fakeDataBranch, now *time.Time) *HistorySource {
	h := NewHistorySource(http.DefaultClient, embedded)
	h.URL = f.URL
	h.Now = func() time.Time { return *now }
	return h
}

func TestHV3_FixtureReplays(t *testing.T) {
	lines, err := parseHistory(fixtureBytes(t))
	if err != nil || len(lines) != 22 {
		t.Fatalf("%d lines, err %v", len(lines), err)
	}
	st, err := replayHistory(lines)
	if err != nil || st.lastDate != "2026-10-14" || len(st.images) != 3171 {
		t.Fatalf("state %+v, err %v", st, err)
	}
}

func TestHV3_2NoEmbeddedCopy(t *testing.T) {
	// Only .gitkeep is committed, so a plain checkout embeds no history.
	if b := embeddedHistory(); b != nil {
		t.Skipf("historydata has a %d-byte history file", len(b))
	}
	f := newFakeDataBranch(t, nil)
	f.status.Store(500)
	now := day("2026-10-14")
	h := newTestSource(nil, f, &now)
	if _, err := h.current(context.Background()); !errors.Is(err, ErrHistoryUnavailable) {
		t.Fatalf("err = %v, want ErrHistoryUnavailable", err)
	}
	svc := newFakeService(t)
	svc.History = h
	if _, err := svc.CatalogHistory(context.Background(), HistoryInput{}); !errors.Is(err, ErrHistoryUnavailable) {
		t.Fatalf("CatalogHistory err = %v", err)
	}
}

func TestHV1_1NewerCopyOnline(t *testing.T) {
	f := newFakeDataBranch(t, fixtureBytes(t))
	now := day("2026-10-14")
	h := newTestSource(fixtureHead(t, 2), f, &now)
	c, err := h.current(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := c.lines[len(c.lines)-1].Date; got != "2026-10-14" || c.source != copyDataBranch {
		t.Errorf("last %s source %s, want 2026-10-14 from the data branch", got, c.source)
	}
	r := c.report(HistoryInput{}, now)
	if r.CopySource != "data_branch" || r.FetchedAt == "" {
		t.Errorf("report source %q fetched %q", r.CopySource, r.FetchedAt)
	}
}

func TestHV1_2Offline(t *testing.T) {
	f := newFakeDataBranch(t, fixtureBytes(t))
	f.status.Store(503)
	now := day("2026-10-14")
	h := newTestSource(fixtureHead(t, 2), f, &now)
	c, err := h.current(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if c.source != copyBuild || c.lines[len(c.lines)-1].Date != "2026-10-12" {
		t.Errorf("source %s last %s, want the embedded copy ending 2026-10-12", c.source, c.lines[len(c.lines)-1].Date)
	}
	if r := c.report(HistoryInput{}, now); r.CopySource != "build" {
		t.Errorf("report says %q", r.CopySource)
	}
}

func TestHV1_3DamagedCopy(t *testing.T) {
	data := fixtureBytes(t)
	damaged := bytes.Replace(data, []byte(`"total":3171`), []byte(`"total":3170`), 1)
	if bytes.Equal(damaged, data) {
		t.Fatal("fixture has no total to damage")
	}
	f := newFakeDataBranch(t, damaged)
	now := day("2026-10-14")
	h := newTestSource(fixtureHead(t, 2), f, &now)
	c, err := h.current(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if c.source != copyBuild {
		t.Errorf("the damaged copy was used (source %s)", c.source)
	}
	// An older fetched copy is ignored too.
	f.body.Store(fixtureHead(t, 4))
	now = now.Add(2 * time.Hour)
	if c, _ = h.current(context.Background()); c.lines[len(c.lines)-1].Date != "2026-10-12" {
		t.Errorf("an older copy replaced the one in use: %s", c.lines[len(c.lines)-1].Date)
	}
}

func TestHV2_1RepeatRequest(t *testing.T) {
	f := newFakeDataBranch(t, fixtureBytes(t))
	now := day("2026-10-14")
	h := newTestSource(fixtureHead(t, 2), f, &now)
	for range 3 {
		if _, err := h.current(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	now = now.Add(59 * time.Minute)
	h.current(context.Background())
	if n := f.requests.Load(); n != 1 {
		t.Fatalf("%d fetches within the hour, want 1", n)
	}
	now = now.Add(2 * time.Minute)
	h.current(context.Background())
	if n := f.requests.Load(); n != 2 {
		t.Fatalf("%d fetches after the hour, want 2", n)
	}
}

func TestHV2_1FailedFetchesAlsoWaitAnHour(t *testing.T) {
	f := newFakeDataBranch(t, nil)
	f.status.Store(500)
	now := day("2026-10-14")
	h := newTestSource(fixtureBytes(t), f, &now)
	for range 3 {
		h.current(context.Background())
	}
	if n := f.requests.Load(); n != 1 {
		t.Fatalf("%d fetches, want 1", n)
	}
}

func TestHV2_2SlowSource(t *testing.T) {
	f := newFakeDataBranch(t, fixtureBytes(t))
	f.delay.Store(int64(2 * time.Second))
	now := day("2026-10-14")
	h := newTestSource(fixtureHead(t, 2), f, &now)
	h.Timeout = 100 * time.Millisecond
	start := time.Now()
	c, err := h.current(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if took := time.Since(start); took > time.Second {
		t.Errorf("took %v with a 100ms timeout", took)
	}
	if c.source != copyBuild {
		t.Errorf("source %s, want the embedded copy", c.source)
	}
}

func TestHV5_1GrowthOverTime(t *testing.T) {
	r := fixtureReport(t, HistoryInput{}, "2026-10-14")
	if len(r.Points) != 22 {
		t.Fatalf("%d points, want 22", len(r.Points))
	}
	if p := r.Points[0]; p.Date != "2024-04-19" || p.Total != 675 || p.Free != nil {
		t.Errorf("first point %+v", p)
	}
	for i, p := range r.Points {
		if i < 17 && p.Free != nil {
			t.Errorf("%s has a free count %d before the first live snapshot", p.Date, *p.Free)
		}
		if i >= 17 && (p.Free == nil || *p.Free != 59) {
			t.Errorf("%s free = %v, want 59", p.Date, p.Free)
		}
	}
	if p := r.Points[21]; p.Date != "2026-10-14" || p.Total != 3171 {
		t.Errorf("last point %+v", p)
	}
	if r.Points[17].Date != "2026-10-10" || r.Points[17].Source != "live" || r.Points[16].Source != "archive" {
		t.Errorf("sources: %+v %+v", r.Points[16], r.Points[17])
	}
	if r.TotalNow != 3171 || r.FreeNow == nil || *r.FreeNow != 59 {
		t.Errorf("now: %d %v", r.TotalNow, r.FreeNow)
	}
}

func TestHV4_1Current(t *testing.T) {
	r := fixtureReport(t, HistoryInput{}, "2026-10-14")
	if r.LastSnapshot != "2026-10-14" || r.DaysOld != 0 || r.Stalled || r.Note != "" {
		t.Errorf("%+v", r)
	}
}

func TestHV4_2Stalled(t *testing.T) {
	r := fixtureReport(t, HistoryInput{}, "2026-10-17")
	if r.DaysOld != 3 || !r.Stalled || !strings.Contains(r.Note, "3 days old") || !strings.Contains(r.Note, "stalled") {
		t.Errorf("%+v", r)
	}
	if r := fixtureReport(t, HistoryInput{}, "2026-10-16"); r.Stalled {
		t.Errorf("2 days old reported as stalled")
	}
}

func TestHV6_1FipsSearch(t *testing.T) {
	r := fixtureReport(t, HistoryInput{Query: "FIPS"}, "2026-10-14")
	if r.Points[0].Total != 204 || r.Points[21].Total != 1315 {
		t.Errorf("fips: %d to %d, want 204 to 1315", r.Points[0].Total, r.Points[21].Total)
	}
}

func TestHV6_2OneImage(t *testing.T) {
	r := fixtureReport(t, HistoryInput{Query: "verdaccio"}, "2026-10-14")
	for i, p := range r.Points {
		want := 0
		if i >= 17 {
			want = 1
		}
		if p.Total != want {
			t.Errorf("%s: %d, want %d", p.Date, p.Total, want)
		}
	}
	var changes []HistoryEntry
	for _, e := range r.Entries {
		if e.Kind == "changes" {
			changes = append(changes, e)
		}
	}
	if len(changes) != 1 || !slices.Equal(changes[0].Added, []string{"verdaccio"}) ||
		changes[0].From != "2026-10-08" || changes[0].To != "2026-10-10" {
		t.Errorf("changes = %+v", changes)
	}
}

func TestHV7_1FirstLiveSnapshotIsARange(t *testing.T) {
	r := fixtureReport(t, HistoryInput{}, "2026-10-14")
	e := r.Entries[1]
	if e.Date != "" || e.From != "2026-10-08" || e.To != "2026-10-10" || len(e.Added) != 5 {
		t.Errorf("%+v", e)
	}
}

// liveHistory builds a history of one archive snapshot and live snapshots on
// the given dates, each adding one image.
func liveViewHistory(t *testing.T, dates ...string) *historyCopy {
	t.Helper()
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	empty := []string{}
	enc.Encode(HistoryLine{Date: "2026-10-01", Source: sourceArchive, Total: 1, Images: []string{"a"},
		Added: empty, Removed: empty, BecameFree: empty, BecameSubscription: empty, Unchecked: empty})
	var names []string
	for i, d := range dates {
		name := string(rune('b' + i))
		names = append(names, name)
		zero := 0
		l := HistoryLine{Date: d, Source: sourceLive, Total: 2 + i, Free: &zero, Added: []string{name},
			Removed: empty, BecameFree: empty, BecameSubscription: empty, Unchecked: empty}
		if i == 0 {
			l.FreeImages = empty
		}
		enc.Encode(l)
	}
	c, err := buildHistoryCopy(buf.Bytes(), copyBuild, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestHV7_2ConsecutiveDays(t *testing.T) {
	c := liveViewHistory(t, "2026-10-02", "2026-10-03")
	r := c.report(HistoryInput{}, day("2026-10-03"))
	// Newest first: 10-03 follows 10-02, so it is an exact day; 10-02 follows
	// the 10-01 capture, so it is too.
	if e := r.Entries[0]; e.Date != "2026-10-03" || e.From != "" {
		t.Errorf("%+v", e)
	}
	if e := r.Entries[1]; e.Date != "2026-10-02" || e.From != "" {
		t.Errorf("%+v", e)
	}
}

func TestHV7_3GapInTheCollector(t *testing.T) {
	c := liveViewHistory(t, "2026-10-02", "2026-10-05")
	r := c.report(HistoryInput{}, day("2026-10-05"))
	if e := r.Entries[0]; e.Date != "" || e.From != "2026-10-02" || e.To != "2026-10-05" {
		t.Errorf("%+v", e)
	}
	if cs := c.catalogSince("c"); cs.Text != "added between 2026-10-02 and 2026-10-05" {
		t.Errorf("%+v", cs)
	}
}

func TestHV8_1TimelineOfTheWeek(t *testing.T) {
	r := fixtureReport(t, HistoryInput{}, "2026-10-14")
	e := r.Entries[0]
	if e.Kind != "no_changes" || e.From != "2026-10-11" || e.To != "2026-10-14" || e.Date != "" {
		t.Errorf("newest entry %+v", e)
	}
	e = r.Entries[1]
	want := []string{"openstack-barbican-oshelm", "openstack-keystone-oshelm-fips", "openstack-manila-oshelm", "verdaccio", "whereabouts-fips"}
	if e.Kind != "changes" || !slices.Equal(e.Added, want) {
		t.Errorf("second entry %+v", e)
	}
	if e.FreeFirstRecorded == nil || *e.FreeFirstRecorded != 59 {
		t.Errorf("free first recorded = %v", e.FreeFirstRecorded)
	}
	if len(r.Entries) != 19 {
		t.Errorf("%d entries, want 2 live + 17 archive", len(r.Entries))
	}
}

func TestHV8_2FirstCapture(t *testing.T) {
	r := fixtureReport(t, HistoryInput{}, "2026-10-14")
	e := r.Entries[len(r.Entries)-1]
	if e.Kind != "first_capture" || e.Date != "2024-04-19" || e.Total != 675 {
		t.Errorf("%+v", e)
	}
}

func TestHV8_3LongLists(t *testing.T) {
	r := fixtureReport(t, HistoryInput{}, "2026-10-14")
	var e HistoryEntry
	for _, x := range r.Entries {
		if x.To == "2026-10-08" {
			e = x
		}
	}
	if e.From != "2026-05-10" || len(e.Added) != 781 || len(e.Back) != 3 || len(e.NotInCapture) != 15 {
		t.Errorf("%s to %s: %d added, %d back, %d not in capture", e.From, e.To, len(e.Added), len(e.Back), len(e.NotInCapture))
	}
}

func TestHV9_1CaptureGap(t *testing.T) {
	r := fixtureReport(t, HistoryInput{}, "2026-10-14")
	for _, e := range r.Entries {
		if e.To == "2026-10-08" && (len(e.NotInCapture) != 15 || len(e.Removed) != 0) {
			t.Errorf("not in capture %d, removed %d", len(e.NotInCapture), len(e.Removed))
		}
	}
}

func TestHV9_2ImageThatReappeared(t *testing.T) {
	r := fixtureReport(t, HistoryInput{Query: "argocd-fips-repo-server"}, "2026-10-14")
	for _, e := range r.Entries {
		switch e.To {
		case "2024-07-17":
			if !slices.Equal(e.NotInCapture, []string{"argocd-fips-repo-server"}) {
				t.Errorf("2024-07-17: %+v", e)
			}
		case "2024-11-17":
			if !slices.Equal(e.Back, []string{"argocd-fips-repo-server"}) || len(e.Added) != 0 {
				t.Errorf("2024-11-17: %+v", e)
			}
		}
	}
}

func TestHV10_FirstAppearance(t *testing.T) {
	c := fixtureCopy(t)
	for _, tc := range []struct{ image, want string }{
		{"python", "since at least 2024-04-19 (the earliest capture)"},
		{"verdaccio", "added between 2026-10-08 and 2026-10-10"},
		{"argocd-fips-repo-server", "since at least 2024-04-19 (the earliest capture)"},
		{"not-in-the-history", "not yet recorded in the catalog history"},
	} {
		if got := c.catalogSince(tc.image).Text; got != tc.want {
			t.Errorf("%s: %q, want %q", tc.image, got, tc.want)
		}
	}
	if !c.catalogSince("python").EarliestCapture || c.catalogSince("verdaccio").EarliestCapture {
		t.Error("earliest_capture is wrong")
	}
}

func TestHV17_1SearchStartsAtFirstMatch(t *testing.T) {
	r := fixtureReport(t, HistoryInput{Query: "verdaccio"}, "2026-10-14")
	if len(r.Entries) != 2 {
		t.Fatalf("%d entries: %+v", len(r.Entries), r.Entries)
	}
	if e := r.Entries[0]; e.Kind != "no_changes" || e.From != "2026-10-11" || e.To != "2026-10-14" {
		t.Errorf("%+v", e)
	}
	if e := r.Entries[1]; e.Kind != "changes" || e.From != "2026-10-08" || e.To != "2026-10-10" || len(e.Added) != 1 {
		t.Errorf("%+v", e)
	}
}

func TestHV17_2SearchMatchingFirstCapture(t *testing.T) {
	r := fixtureReport(t, HistoryInput{Query: "fips"}, "2026-10-14")
	e := r.Entries[len(r.Entries)-1]
	if e.Kind != "first_capture" || e.Total != 204 {
		t.Errorf("%+v", e)
	}
}

func TestHV17_NoMatch(t *testing.T) {
	r := fixtureReport(t, HistoryInput{Query: "zzz-nothing"}, "2026-10-14")
	if len(r.Entries) != 0 || r.TotalNow != 0 {
		t.Errorf("%+v", r.Entries)
	}
}

func TestHV18_1ArchiveEntriesAreMarked(t *testing.T) {
	r := fixtureReport(t, HistoryInput{}, "2026-10-14")
	archive := 0
	for _, e := range r.Entries {
		date := max(e.Date, e.To)
		if e.Archive != (date < "2026-10-10") {
			t.Errorf("%+v: archive = %v", e, e.Archive)
		}
		if e.Archive {
			archive++
		}
	}
	if archive != 17 {
		t.Errorf("%d archive entries, want 17", archive)
	}
}

func TestHV12_1Since(t *testing.T) {
	r := fixtureReport(t, HistoryInput{Since: "2026-10-09"}, "2026-10-14")
	if r.TotalsAtSince == nil || r.TotalsAtSince.Total != 3166 || r.TotalsAtSince.Date != "2026-10-08" || r.TotalNow != 3171 {
		t.Errorf("totals at since %+v, now %d", r.TotalsAtSince, r.TotalNow)
	}
	var added []string
	for _, e := range r.Entries {
		added = append(added, e.Added...)
	}
	if len(added) != 5 {
		t.Errorf("%d added: %v", len(added), added)
	}
}

func TestHV12_1EntrySpanningSinceIsKept(t *testing.T) {
	// 2026-10-09 is inside the range 2026-10-08 to 2026-10-10.
	r := fixtureReport(t, HistoryInput{Since: "2026-10-09"}, "2026-10-14")
	found := false
	for _, e := range r.Entries {
		found = found || (e.From == "2026-10-08" && e.To == "2026-10-10")
	}
	if !found {
		t.Error("the range spanning since was dropped")
	}
	r = fixtureReport(t, HistoryInput{Since: "2026-10-10"}, "2026-10-14")
	for _, e := range r.Entries {
		if e.Kind == "changes" {
			t.Errorf("a change on or before since was kept: %+v", e)
		}
	}
}

func TestHV11_SinceValidation(t *testing.T) {
	svc := newFakeService(t)
	_, err := svc.CatalogHistory(context.Background(), HistoryInput{Since: "yesterday"})
	var ie *InputError
	if !errors.As(err, &ie) {
		t.Fatalf("err = %v, want *InputError", err)
	}
}

func TestHV11_1IndexEntry(t *testing.T) {
	srv := newWebServer(t, fakeUI)
	var index apiIndex
	getJSON(t, srv, "/api/", &index)
	for _, e := range index.Endpoints {
		if e.Path != "/api/history" {
			continue
		}
		got := map[string]bool{}
		for _, p := range e.Parameters {
			if p.In != "query" || p.Required {
				t.Errorf("parameter %+v should be an optional query parameter", p)
			}
			got[p.Name] = true
		}
		if len(got) != 2 || !got["query"] || !got["since"] {
			t.Errorf("parameters = %v", got)
		}
		return
	}
	t.Error("/api/history is not in the index")
}

func TestHV11_2InvalidDate(t *testing.T) {
	srv := newWebServer(t, fakeUI)
	var e apiError
	if resp := getJSON(t, srv, "/api/history?since=yesterday", &e); resp.StatusCode != 400 || e.Code != "invalid_input" {
		t.Errorf("%d %+v", resp.StatusCode, e)
	}
	var r HistoryReport
	if resp := getJSON(t, srv, "/api/history?query=verdaccio", &r); resp.StatusCode != 200 || r.TotalNow != 1 || len(r.Points) != 22 {
		t.Errorf("%d total %d points %d", resp.StatusCode, r.TotalNow, len(r.Points))
	}
}

func TestHV3_2HistoryUnavailableAnswers503(t *testing.T) {
	svc := newFakeService(t)
	f := newFakeDataBranch(t, nil)
	f.status.Store(500)
	now := day("2026-10-14")
	svc.History = newTestSource(nil, f, &now)
	srv := httptest.NewServer(newWebHandler(svc, fakeUI))
	t.Cleanup(srv.Close)
	var e apiError
	if resp := getJSON(t, srv, "/api/history", &e); resp.StatusCode != 503 || e.Code != "history_unavailable" {
		t.Errorf("%d %+v", resp.StatusCode, e)
	}
	// HV-13.2: everything else still works, without catalog_since.
	var d ImageDetails
	if resp := getJSON(t, srv, "/api/images/python/details", &d); resp.StatusCode != 200 || d.CatalogSince != nil {
		t.Errorf("details: %d %+v", resp.StatusCode, d.CatalogSince)
	}
}

func TestHV12_1ToolSince(t *testing.T) {
	s := mcpClient(t, newFakeService(t))
	var r HistoryReport
	mustCall(t, s, "catalog_changes", map[string]any{"since": "2026-10-09"}, &r)
	if r.TotalsAtSince == nil || r.TotalsAtSince.Total != 3166 || r.TotalNow != 3171 {
		t.Errorf("totals %+v now %d", r.TotalsAtSince, r.TotalNow)
	}
	added := 0
	for _, e := range r.Entries {
		added += len(e.Added)
	}
	if added != 5 {
		t.Errorf("%d added, want 5", added)
	}
	if msg := callTool(t, s, "catalog_changes", map[string]any{"since": "yesterday"}, nil); !strings.Contains(msg, "since") {
		t.Errorf("error = %q", msg)
	}
	res, err := s.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range res.Tools {
		if tool.Name == "catalog_changes" {
			if !strings.Contains(tool.Description, "ranges between") || !strings.Contains(tool.Description, "age of the data") {
				t.Errorf("description = %q", tool.Description)
			}
			return
		}
	}
	t.Error("catalog_changes is not registered")
}

func TestHV12_2StalledCollector(t *testing.T) {
	svc := newFakeService(t)
	now := day("2026-10-20")
	svc.History.Now = func() time.Time { return now }
	s := mcpClient(t, svc)
	var r HistoryReport
	mustCall(t, s, "catalog_changes", nil, &r)
	if !r.Stalled || !strings.Contains(r.Note, "6 days old") || !strings.Contains(r.Note, "stalled") {
		t.Errorf("%+v", r)
	}
}

func TestHV13_1CatalogSinceInDetails(t *testing.T) {
	s := mcpClient(t, newFakeService(t))
	var d ImageDetails
	mustCall(t, s, "get_image_details", map[string]any{"image": "python"}, &d)
	if d.CatalogSince == nil || d.CatalogSince.Text != "since at least 2024-04-19 (the earliest capture)" {
		t.Errorf("catalog_since = %+v", d.CatalogSince)
	}
}

// TestHistoryReportFixture keeps the web UI's history fixture in step with
// the Go report. With UPDATE_FIXTURES=1 it writes it; otherwise it checks it.
func TestHistoryReportFixture(t *testing.T) {
	// A fixed "today" keeps the report stable.
	report := fixtureReport(t, HistoryInput{}, "2026-10-14")
	want, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	want = append(want, '\n')
	path := filepath.Join("web", "src", "test", "fixtures", "historyReport.json")
	if os.Getenv("UPDATE_FIXTURES") == "1" {
		if err := os.WriteFile(path, want, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run with UPDATE_FIXTURES=1 to create it)", err)
	}
	var gotV, wantV any
	if err := json.Unmarshal(got, &gotV); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	json.Unmarshal(want, &wantV)
	if !reflect.DeepEqual(gotV, wantV) {
		t.Errorf("%s is out of date; run UPDATE_FIXTURES=1 go test -run TestHistoryReportFixture ./...", path)
	}
}
