package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func historyPath(t *testing.T) string {
	return filepath.Join(t.TempDir(), "catalog-history.jsonl")
}

func readLines(t *testing.T, path string) []HistoryLine {
	t.Helper()
	_, lines, err := readHistory(path)
	if err != nil {
		t.Fatal(err)
	}
	return lines
}

func TestCH7_1SnapshotMatchesCatalog(t *testing.T) {
	svc := newFakeService(t)
	path := historyPath(t)
	now := time.Date(2026, 10, 12, 6, 17, 0, 0, time.UTC)
	line, skipped, err := recordHistory(context.Background(), svc, path, now)
	if err != nil || skipped {
		t.Fatalf("skipped=%v err=%v", skipped, err)
	}
	all, err := svc.ListImages(context.Background(), ListImagesInput{Limit: 1000})
	if err != nil {
		t.Fatal(err)
	}
	free, err := svc.ListImages(context.Background(), ListImagesInput{FreeOnly: true, Limit: 1000})
	if err != nil {
		t.Fatal(err)
	}
	var freeNames []string
	for _, im := range free.Images {
		freeNames = append(freeNames, im.Name)
	}
	if line.Total != all.Total || *line.Free != free.Total || !slices.Equal(line.FreeImages, freeNames) {
		t.Errorf("snapshot %d/%d %v, catalog %d/%d %v", line.Total, *line.Free, line.FreeImages, all.Total, free.Total, freeNames)
	}
	if got := readLines(t, path); len(got) != 1 || got[0].Date != "2026-10-12" || got[0].Source != sourceLive {
		t.Errorf("file lines = %+v", got)
	}
}

func TestCH5_1SecondRunSameDaySkipped(t *testing.T) {
	svc := newFakeService(t)
	path := historyPath(t)
	ctx := context.Background()
	if _, _, err := recordHistory(ctx, svc, path, time.Date(2026, 10, 12, 1, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)
	_, skipped, err := recordHistory(ctx, svc, path, time.Date(2026, 10, 12, 23, 0, 0, 0, time.UTC))
	if err != nil || !skipped {
		t.Fatalf("skipped=%v err=%v", skipped, err)
	}
	if after, _ := os.ReadFile(path); string(after) != string(before) {
		t.Error("file changed on the second run")
	}
	if _, skipped, _ := recordHistory(ctx, svc, path, time.Date(2026, 10, 13, 1, 0, 0, 0, time.UTC)); skipped {
		t.Error("the next day was skipped")
	}
	if n := len(readLines(t, path)); n != 2 {
		t.Errorf("%d lines, want 2", n)
	}
}

func TestCH6_1RefusedSnapshotLeavesFileUnchanged(t *testing.T) {
	svc, fake := newFakeBackend(t)
	path := historyPath(t)
	ctx := context.Background()
	if _, _, err := recordHistory(ctx, svc, path, time.Date(2026, 10, 12, 1, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)

	fake.images = fake.images[:3] // a truncated sitemap
	svc.Catalog.TTL = 0
	_, _, err := recordHistory(ctx, svc, path, time.Date(2026, 10, 13, 1, 0, 0, 0, time.UTC))
	if err == nil || !strings.Contains(err.Error(), "refused") {
		t.Fatalf("err = %v, want a refusal", err)
	}
	if after, _ := os.ReadFile(path); string(after) != string(before) {
		t.Error("file changed by a refused snapshot")
	}
	// The flag's wrapper reports the same error, which makes the process exit 1 (CH-9.2).
	if err := runHistory(svc, nil, path, ""); err == nil {
		t.Error("runHistory returned nil for a refused snapshot")
	}
}

func TestRecordHistoryRejectsDamagedFile(t *testing.T) {
	path := historyPath(t)
	if err := os.WriteFile(path, []byte("{broken\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := recordHistory(context.Background(), newFakeService(t), path, time.Now()); err == nil || !strings.Contains(err.Error(), "line 1") {
		t.Errorf("err = %v", err)
	}
}

// --- backfill (CH-8) ---

func sitemapXML(names ...string) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0"?><urlset>`)
	for _, n := range names {
		fmt.Fprintf(&b, `<url><loc>https://images.chainguard.dev/directory/image/%s/overview</loc></url>`, n)
	}
	b.WriteString(`</urlset>`)
	return b.String()
}

// fakeArchive serves a CDX listing and captures keyed by timestamp.
func fakeArchive(t *testing.T, captures map[string]string) *Archive {
	t.Helper()
	var stamps []string
	for ts := range captures {
		stamps = append(stamps, ts)
	}
	slices.Sort(stamps)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/cdx" {
			if r.URL.Query().Get("url") != "images.chainguard.dev/sitemap.xml" {
				http.Error(w, "wrong url", http.StatusBadRequest)
				return
			}
			rows := `[["timestamp","mimetype"]`
			for _, ts := range stamps {
				rows += fmt.Sprintf(`,["%s","text/plain"]`, ts)
			}
			fmt.Fprint(w, rows+"]")
			return
		}
		ts, ok := strings.CutPrefix(r.URL.Path, "/web/")
		ts, ok2 := strings.CutSuffix(ts, "if_/https://images.chainguard.dev/sitemap.xml")
		body, found := captures[ts]
		if !ok || !ok2 || !found {
			http.NotFound(w, r)
			return
		}
		fmt.Fprint(w, body)
	}))
	t.Cleanup(srv.Close)
	return &Archive{HTTP: srv.Client(), CDXURL: srv.URL + "/cdx", WaybackURL: srv.URL}
}

func TestCH8_2BackfillSkipsHTMLCapture(t *testing.T) {
	a := fakeArchive(t, map[string]string{
		"20240412101500": sitemapXML("go", "node"),
		"20240601000000": "<html><body>Not found</body></html>",
		"20240620000000": sitemapXML("go", "node", "python"),
		"20240702000000": "<html><body>Not found</body></html>", // July has no usable capture
		"20240801000000": sitemapXML("go", "python", "static"),
	})
	path := historyPath(t)
	if _, err := backfillHistory(context.Background(), a, path); err != nil {
		t.Fatal(err)
	}
	lines := readLines(t, path)
	if len(lines) != 3 {
		t.Fatalf("%d lines, want 3 (April, June, August)", len(lines))
	}
	var dates []string
	for _, l := range lines {
		dates = append(dates, l.Date)
		if l.Source != sourceArchive || l.Free != nil {
			t.Errorf("%s: source %s free %v", l.Date, l.Source, l.Free)
		}
	}
	if want := []string{"2024-04-12", "2024-06-20", "2024-08-01"}; !slices.Equal(dates, want) {
		t.Errorf("dates = %v, want %v", dates, want)
	}
	if !slices.Equal(lines[0].Images, []string{"go", "node"}) || len(lines[0].Added) != 0 {
		t.Errorf("first line = %+v", lines[0])
	}
	if !slices.Equal(lines[1].Added, []string{"python"}) || len(lines[1].Removed) != 0 {
		t.Errorf("second line diffs from the first: %+v", lines[1])
	}
	if !slices.Equal(lines[2].Added, []string{"static"}) || !slices.Equal(lines[2].Removed, []string{"node"}) {
		t.Errorf("third line = %+v", lines[2])
	}
	if _, err := replayHistory(lines); err != nil {
		t.Error(err)
	}
}

func TestCH8_3BackfillRefusesExistingHistory(t *testing.T) {
	a := fakeArchive(t, map[string]string{"20240412101500": sitemapXML("go")})
	path := historyPath(t)
	existing := `{"date":"2026-10-01","source":"live","total":0}` + "\n"
	if err := os.WriteFile(path, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := backfillHistory(context.Background(), a, path); err == nil {
		t.Fatal("backfill into a non-empty file succeeded")
	}
	if got, _ := os.ReadFile(path); string(got) != existing {
		t.Error("file changed")
	}
}

func TestBackfillPausesBetweenRequests(t *testing.T) {
	a := fakeArchive(t, map[string]string{
		"20240412101500": sitemapXML("go"),
		"20240512101500": sitemapXML("go", "node"),
	})
	a.Pause = 30 * time.Millisecond
	start := time.Now()
	if _, err := backfillHistory(context.Background(), a, historyPath(t)); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(start); elapsed < 60*time.Millisecond { // listing + 2 captures: 2 pauses
		t.Errorf("3 requests took %v, expected pauses between them", elapsed)
	}
}

func TestCH8_RedirectedCaptureIsRefused(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/web/20261008040136if_/") {
			http.Redirect(w, r, "/web/20260510033055if_/https://images.chainguard.dev/sitemap.xml", http.StatusFound)
			return
		}
		fmt.Fprint(w, sitemapXML("go"))
	}))
	defer srv.Close()
	a := &Archive{HTTP: srv.Client(), WaybackURL: srv.URL}
	if _, err := a.Sitemap(context.Background(), "20261008040136"); err == nil || !strings.Contains(err.Error(), "redirected") {
		t.Errorf("err = %v, want a redirect error", err)
	}
	if names, err := a.Sitemap(context.Background(), "20260510033055"); err != nil || len(names) != 1 {
		t.Errorf("names = %v, err = %v", names, err)
	}
}

func TestArchiveRetriesTransientErrors(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls < 3 {
			http.Error(w, "busy", http.StatusServiceUnavailable)
			return
		}
		fmt.Fprint(w, `[["timestamp","mimetype"],["20240419222115","application/xml"]]`)
	}))
	defer srv.Close()
	a := &Archive{HTTP: srv.Client(), CDXURL: srv.URL}
	got, err := a.Captures(context.Background())
	if err != nil || len(got) != 1 || calls != 3 {
		t.Errorf("captures = %v, err = %v, calls = %d", got, err, calls)
	}
}
