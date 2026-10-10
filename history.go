package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
)

// The catalog history is a JSON-lines file, one line per snapshot. See
// openspec/specs/catalog-history for the format (CH-1 to CH-4).

const (
	sourceArchive = "archive"
	sourceLive    = "live"

	// maxRemovedPercent is the share of the previous snapshot's images a
	// snapshot may remove before it is refused as a broken sitemap (CH-6).
	maxRemovedPercent = 5
)

// HistoryLine is one snapshot. Lists are never nil, so they marshal as [].
type HistoryLine struct {
	Date               string   `json:"date"`
	Source             string   `json:"source"`
	Total              int      `json:"total"`
	Free               *int     `json:"free"`
	Added              []string `json:"added"`
	Removed            []string `json:"removed"`
	BecameFree         []string `json:"became_free"`
	BecameSubscription []string `json:"became_subscription"`
	Unchecked          []string `json:"unchecked"`
	// Images is every image name, on the first line only (CH-2).
	Images []string `json:"images,omitempty"`
	// FreeImages is every free image, on the first live line only (CH-2).
	FreeImages []string `json:"free_images,omitempty"`
}

// historyState is the catalog as of the last line of a history.
type historyState struct {
	lines    int
	images   map[string]bool
	free     map[string]bool
	hasLive  bool
	lastDate string
}

// parseHistory reads a history file's lines. A missing file is an empty history.
func parseHistory(data []byte) ([]HistoryLine, error) {
	var lines []HistoryLine
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 0, 1<<20), 64<<20)
	for n := 1; sc.Scan(); n++ {
		text := strings.TrimSpace(sc.Text())
		if text == "" {
			continue
		}
		var l HistoryLine
		if err := json.Unmarshal([]byte(text), &l); err != nil {
			return nil, fmt.Errorf("history line %d: %w", n, err)
		}
		lines = append(lines, l)
	}
	return lines, sc.Err()
}

func readHistory(path string) (data []byte, lines []HistoryLine, err error) {
	data, err = os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	lines, err = parseHistory(data)
	return data, lines, err
}

// replayHistory rebuilds the latest state from the first line's images and
// every line's added and removed lists (CH-2.1). It fails if a line's total
// doesn't match the rebuilt list, since that means the file is damaged.
func replayHistory(lines []HistoryLine) (historyState, error) {
	st := historyState{images: map[string]bool{}, free: map[string]bool{}}
	for i, l := range lines {
		if i == 0 {
			for _, n := range l.Images {
				st.images[n] = true
			}
		}
		for _, n := range l.Added {
			st.images[n] = true
		}
		for _, n := range l.Removed {
			delete(st.images, n)
			delete(st.free, n)
		}
		if l.Source == sourceLive {
			if !st.hasLive {
				st.hasLive = true
				for _, n := range l.FreeImages {
					st.free[n] = true
				}
			}
			for _, n := range l.BecameFree {
				st.free[n] = true
			}
			for _, n := range l.BecameSubscription {
				delete(st.free, n)
			}
		}
		if len(st.images) != l.Total {
			return st, fmt.Errorf("history line %d (%s): rebuilt %d images but total is %d", i+1, l.Date, len(st.images), l.Total)
		}
		st.lines, st.lastDate = i+1, l.Date
	}
	return st, nil
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// diffSnapshot builds the line for a snapshot of names taken on date. For a
// live snapshot, status holds the free-tier status of each image whose check
// succeeded; images missing from it keep their previous status (CH-4). An
// archive snapshot has no status at all (CH-1.2).
func diffSnapshot(prev historyState, date, source string, names []string, status map[string]bool) HistoryLine {
	cur := make(map[string]bool, len(names))
	for _, n := range names {
		cur[n] = true
	}
	l := HistoryLine{
		Date: date, Source: source, Total: len(cur),
		Added: []string{}, Removed: []string{}, BecameFree: []string{},
		BecameSubscription: []string{}, Unchecked: []string{},
	}
	if prev.lines == 0 {
		l.Images = sortedKeys(cur)
	} else {
		for n := range cur {
			if !prev.images[n] {
				l.Added = append(l.Added, n)
			}
		}
		for n := range prev.images {
			if !cur[n] {
				l.Removed = append(l.Removed, n)
			}
		}
	}
	if source == sourceLive {
		free := map[string]bool{}
		for n := range cur {
			if s, ok := status[n]; ok {
				if s {
					free[n] = true
				}
			} else {
				l.Unchecked = append(l.Unchecked, n)
				if prev.free[n] {
					free[n] = true
				}
			}
		}
		if prev.hasLive {
			for n := range free {
				if !prev.free[n] {
					l.BecameFree = append(l.BecameFree, n)
				}
			}
			for n := range prev.free {
				if cur[n] && !free[n] {
					l.BecameSubscription = append(l.BecameSubscription, n)
				}
			}
		} else {
			l.FreeImages = sortedKeys(free)
		}
		count := len(free)
		l.Free = &count
	}
	for _, s := range [][]string{l.Added, l.Removed, l.BecameFree, l.BecameSubscription, l.Unchecked} {
		sort.Strings(s)
	}
	return l
}

// checkPlausible refuses a snapshot that looks like a broken or partial
// sitemap rather than real retirements (CH-6).
func checkPlausible(prevTotal int, l HistoryLine) error {
	if l.Total == 0 {
		return errors.New("snapshot refused: the sitemap lists no images")
	}
	if prevTotal > 0 && len(l.Removed)*100 > prevTotal*maxRemovedPercent {
		return fmt.Errorf("snapshot refused: the sitemap lists %s images, down from %s (%d removed, more than %d%%); it may be truncated",
			commas(l.Total), commas(prevTotal), len(l.Removed), maxRemovedPercent)
	}
	return nil
}

func commas(n int) string {
	s := fmt.Sprint(n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}

// describeLine is the one-line report printed after recording.
func describeLine(l HistoryLine) string {
	s := fmt.Sprintf("%s: %s images, +%d −%d", l.Date, commas(l.Total), len(l.Added), len(l.Removed))
	if l.Free != nil {
		s += fmt.Sprintf(", %d free", *l.Free)
	}
	return s
}

// appendHistory writes the lines to path in one Write to an O_APPEND file, so
// a crash can't leave a half line followed by a good one.
func appendHistory(path string, existing []byte, lines ...HistoryLine) error {
	var buf bytes.Buffer
	if len(existing) > 0 && existing[len(existing)-1] != '\n' {
		buf.WriteByte('\n')
	}
	for _, l := range lines {
		b, err := json.Marshal(l)
		if err != nil {
			return err
		}
		buf.Write(b)
		buf.WriteByte('\n')
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.Write(buf.Bytes()); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// recordHistory takes a live snapshot and appends it to the history file at
// path. It reports skipped when the file already has a line for today's UTC
// date (CH-5) and refuses implausible snapshots (CH-6), leaving the file
// unchanged in both cases.
func recordHistory(ctx context.Context, svc *Service, path string, now time.Time) (line HistoryLine, skipped bool, err error) {
	data, lines, err := readHistory(path)
	if err != nil {
		return line, false, err
	}
	prev, err := replayHistory(lines)
	if err != nil {
		return line, false, err
	}
	date := now.UTC().Format("2006-01-02")
	switch {
	case prev.lastDate == date:
		return lines[len(lines)-1], true, nil
	case prev.lastDate > date:
		return line, false, fmt.Errorf("history already has a line dated %s, after %s", prev.lastDate, date)
	}

	names, err := svc.Catalog.Images(ctx)
	if err != nil {
		return line, false, err
	}
	status := svc.Registry.PublicStatus(ctx, names)
	line = diffSnapshot(prev, date, sourceLive, names, status)
	if err := checkPlausible(len(prev.images), line); err != nil {
		return line, false, err
	}
	return line, false, appendHistory(path, data, line)
}

// Archive reads the Internet Archive's captures of Chainguard's sitemap.
type Archive struct {
	HTTP       *http.Client
	CDXURL     string        // capture listing endpoint
	WaybackURL string        // base of capture URLs
	Pause      time.Duration // between requests (CH-8)

	requests int
}

// archiveAttempts is how many times a request to the Archive is tried.
const archiveAttempts = 3

const archivedSitemap = "images.chainguard.dev/sitemap.xml"

func NewArchive(client *http.Client) *Archive {
	return &Archive{
		HTTP:       client,
		CDXURL:     "https://web.archive.org/cdx/search/cdx",
		WaybackURL: "https://web.archive.org",
		Pause:      2 * time.Second,
	}
}

func (a *Archive) get(ctx context.Context, url string) (*http.Response, error) {
	var lastErr error
	for attempt := range archiveAttempts {
		if a.requests > 0 && a.Pause > 0 {
			// Wait longer after each failure: the Archive rate-limits and has brief outages.
			select {
			case <-time.After(a.Pause * time.Duration(1+attempt*4)):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		a.requests++
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", "chainguard-mcp catalog history backfill (+https://github.com/KenProle/chainguard-mcp)")
		resp, err := a.HTTP.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		if resp.StatusCode == http.StatusOK {
			return resp, nil
		}
		resp.Body.Close()
		lastErr = fmt.Errorf("%s: unexpected status %s", url, resp.Status)
		if resp.StatusCode < 500 && resp.StatusCode != http.StatusTooManyRequests && resp.StatusCode != 498 {
			break
		}
	}
	return nil, lastErr
}

// Captures lists the timestamps (YYYYMMDDhhmmss) of the sitemap's successful
// captures, oldest first.
func (a *Archive) Captures(ctx context.Context) ([]string, error) {
	resp, err := a.get(ctx, a.CDXURL+"?url="+archivedSitemap+"&output=json&fl=timestamp,mimetype&filter=statuscode:200")
	if err != nil {
		return nil, fmt.Errorf("listing archive captures: %w", err)
	}
	defer resp.Body.Close()
	body := resp.Body
	var rows [][]string
	if err := json.NewDecoder(io.LimitReader(body, 8<<20)).Decode(&rows); err != nil {
		return nil, fmt.Errorf("decoding archive captures: %w", err)
	}
	var out []string
	for _, r := range rows {
		if len(r) > 0 && len(r[0]) >= 8 && r[0] != "timestamp" {
			out = append(out, r[0])
		}
	}
	sort.Strings(out)
	return out, nil
}

// Sitemap fetches one capture and parses it. It uses the Archive's if_ form,
// which serves a sitemap's original XML: the plain form wraps it in an HTML
// page, and the id_ form redirected the 2026-10-08 capture to an older one. A
// redirect to another capture is an error, so a capture is never read under
// the wrong date. A capture that isn't a sitemap with images returns
// errNotSitemap.
func (a *Archive) Sitemap(ctx context.Context, timestamp string) ([]string, error) {
	resp, err := a.get(ctx, a.WaybackURL+"/web/"+timestamp+"if_/https://"+archivedSitemap)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if !strings.Contains(resp.Request.URL.Path, "/web/"+timestamp+"if_/") {
		return nil, fmt.Errorf("capture %s redirected to %s", timestamp, resp.Request.URL.Path)
	}
	names, err := parseSitemap(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return nil, fmt.Errorf("%w: capture %s: %v", errNotSitemap, timestamp, err)
	}
	return names, nil
}

var errNotSitemap = errors.New("not a sitemap")

// backfillHistory writes one archive line for each month that has a usable
// capture (CH-8), into an empty or missing history file.
func backfillHistory(ctx context.Context, a *Archive, path string) ([]HistoryLine, error) {
	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if len(bytes.TrimSpace(data)) > 0 {
		return nil, fmt.Errorf("%s already has history; backfill only writes to an empty file", path)
	}
	captures, err := a.Captures(ctx)
	if err != nil {
		return nil, err
	}
	byMonth := map[string][]string{}
	var months []string
	for _, ts := range captures {
		m := ts[:6]
		if _, ok := byMonth[m]; !ok {
			months = append(months, m)
		}
		byMonth[m] = append(byMonth[m], ts)
	}

	var lines []HistoryLine
	var prev historyState
	for _, m := range months {
		for _, ts := range byMonth[m] {
			names, err := a.Sitemap(ctx, ts)
			if errors.Is(err, errNotSitemap) {
				continue
			}
			if err != nil {
				return nil, err
			}
			date := ts[:4] + "-" + ts[4:6] + "-" + ts[6:8]
			l := diffSnapshot(prev, date, sourceArchive, names, nil)
			lines = append(lines, l)
			if prev, err = replayHistory(lines); err != nil {
				return nil, err
			}
			break
		}
	}
	if len(lines) == 0 {
		return nil, errors.New("no usable sitemap captures found")
	}
	if err := appendHistory(path, nil, lines...); err != nil {
		return nil, err
	}
	return lines, nil
}

// runHistory handles the -record-history and -backfill-history flags.
func runHistory(svc *Service, archive *Archive, recordFile, backfillFile string) error {
	if recordFile != "" && backfillFile != "" {
		return errors.New("use only one of -record-history and -backfill-history")
	}
	ctx := context.Background()
	if backfillFile != "" {
		lines, err := backfillHistory(ctx, archive, backfillFile)
		if err != nil {
			return err
		}
		fmt.Printf("wrote %d archive lines: %s to %s\n", len(lines), lines[0].Date, lines[len(lines)-1].Date)
		return nil
	}
	line, skipped, err := recordHistory(ctx, svc, recordFile, time.Now())
	if err != nil {
		return err
	}
	if skipped {
		fmt.Printf("skipped: already recorded for %s\n", line.Date)
		return nil
	}
	fmt.Println("added line for " + describeLine(line))
	return nil
}
