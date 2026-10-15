package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

// This file reads the catalog history (history.go writes it) and derives
// what the History view, GET /api/history and the catalog_changes tool show.
// See openspec/specs/catalog-history-view (HV-1 to HV-18).

// historyURL is where the daily collector publishes the history file.
const historyURL = "https://raw.githubusercontent.com/KenProle/chainguard-mcp/data/catalog-history.jsonl"

// maxHistoryBytes caps a fetched history file; the real one is ~100 KB and
// grows by ~150 bytes a day.
const maxHistoryBytes = 64 << 20

// stalledAfterDays is how old the last snapshot may be before the daily
// collector is reported as possibly stalled (HV-4).
const stalledAfterDays = 2

// ErrHistoryUnavailable means the server has no valid copy of the history.
var ErrHistoryUnavailable = errors.New("the catalog history couldn't be loaded")

const (
	copyBuild      = "build"
	copyDataBranch = "data_branch"
)

// historyCopy is one parsed history with everything derived from it in a
// single replay (design decision 3).
type historyCopy struct {
	source    string
	fetchedAt time.Time
	lines     []HistoryLine
	// sets[i] is every image name after line i, sorted.
	sets [][]string
	// freeSets[i] is every free image after line i, sorted; nil on archive lines.
	freeSets [][]string
	// newImgs and backImgs split a line's additions into images never listed
	// before and images listed again after a missed capture.
	newImgs, backImgs [][]string
	// firstSeen maps an image to the index of the first line listing it.
	firstSeen map[string]int
}

// buildHistoryCopy parses and replays data. It fails on a damaged file (a
// line whose total doesn't match the rebuilt list) and on an empty one.
func buildHistoryCopy(data []byte, source string, fetchedAt time.Time) (*historyCopy, error) {
	lines, err := parseHistory(data)
	if err != nil {
		return nil, err
	}
	if len(lines) == 0 {
		return nil, errors.New("history has no snapshots")
	}
	if _, err := replayHistory(lines); err != nil {
		return nil, err
	}
	c := &historyCopy{
		source: source, fetchedAt: fetchedAt, lines: lines,
		sets:      make([][]string, len(lines)),
		freeSets:  make([][]string, len(lines)),
		newImgs:   make([][]string, len(lines)),
		backImgs:  make([][]string, len(lines)),
		firstSeen: map[string]int{},
	}
	images, free := map[string]bool{}, map[string]bool{}
	hasLive := false
	for i, l := range lines {
		if i == 0 {
			for _, n := range l.Images {
				images[n] = true
				c.firstSeen[n] = 0
			}
		}
		c.newImgs[i], c.backImgs[i] = []string{}, []string{}
		for _, n := range l.Added {
			if _, seen := c.firstSeen[n]; seen {
				c.backImgs[i] = append(c.backImgs[i], n)
			} else {
				c.firstSeen[n] = i
				c.newImgs[i] = append(c.newImgs[i], n)
			}
			images[n] = true
		}
		for _, n := range l.Removed {
			delete(images, n)
			delete(free, n)
		}
		if l.Source == sourceLive {
			if !hasLive {
				hasLive = true
				for _, n := range l.FreeImages {
					free[n] = true
				}
			}
			for _, n := range l.BecameFree {
				free[n] = true
			}
			for _, n := range l.BecameSubscription {
				delete(free, n)
			}
			c.freeSets[i] = sortedKeys(free)
		}
		c.sets[i] = sortedKeys(images)
	}
	return c, nil
}

// HistorySource supplies the newest valid copy of the history: the one
// embedded at build time, or one fetched from the data branch at most once
// an hour (HV-1, HV-2).
type HistorySource struct {
	URL     string
	HTTP    *http.Client
	TTL     time.Duration // minimum time between fetch attempts
	Timeout time.Duration // longest a request waits for a fetch
	Now     func() time.Time

	mu          sync.Mutex
	cur         *historyCopy
	lastAttempt time.Time
	attempted   bool
	inflight    bool
}

// NewHistorySource starts with the embedded copy, if it is valid.
func NewHistorySource(client *http.Client, embedded []byte) *HistorySource {
	h := &HistorySource{URL: historyURL, HTTP: client, TTL: time.Hour, Timeout: 5 * time.Second, Now: time.Now}
	if len(embedded) > 0 {
		if c, err := buildHistoryCopy(embedded, copyBuild, time.Time{}); err != nil {
			log.Printf("ignoring the embedded catalog history: %v", err)
		} else {
			h.cur = c
		}
	}
	return h
}

// current returns the copy in use, first trying to fetch a newer one if the
// last attempt is older than the TTL. Only one request fetches at a time;
// the others use the copy in use.
func (h *HistorySource) current(ctx context.Context) (*historyCopy, error) {
	h.mu.Lock()
	now := h.Now()
	if !h.inflight && (!h.attempted || now.Sub(h.lastAttempt) >= h.TTL) {
		h.inflight, h.attempted, h.lastAttempt = true, true, now
		h.mu.Unlock()
		c, err := h.fetch(ctx)
		h.mu.Lock()
		h.inflight = false
		switch {
		case err != nil:
			log.Printf("fetching the catalog history: %v", err)
		case h.cur != nil && c.lines[len(c.lines)-1].Date < h.cur.lines[len(h.cur.lines)-1].Date:
			log.Printf("ignoring the fetched catalog history: it ends %s, before the copy in use",
				c.lines[len(c.lines)-1].Date)
		default:
			h.cur = c
		}
	}
	cur := h.cur
	h.mu.Unlock()
	if cur == nil {
		return nil, ErrHistoryUnavailable
	}
	return cur, nil
}

func (h *HistorySource) fetch(ctx context.Context) (*historyCopy, error) {
	// The wait is bounded by Timeout, not by the request that triggered it.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), h.Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, h.URL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := h.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s answered %s", h.URL, resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxHistoryBytes))
	if err != nil {
		return nil, err
	}
	return buildHistoryCopy(data, copyDataBranch, h.Now())
}

// HistoryInput is the input of catalog_changes and GET /api/history.
type HistoryInput struct {
	Since string `json:"since,omitempty" jsonschema:"only changes after this date, YYYY-MM-DD; all changes when omitted"`
	Query string `json:"query,omitempty" jsonschema:"case-insensitive substring to filter image names, e.g. 'fips'"`
}

type HistoryPoint struct {
	Date   string `json:"date"`
	Source string `json:"source" jsonschema:"'archive' (Internet Archive capture) or 'live' (daily snapshot)"`
	Total  int    `json:"total"`
	Free   *int   `json:"free,omitempty" jsonschema:"free-tier images; absent before the first live snapshot"`
}

type HistoryEntry struct {
	Kind string `json:"kind" jsonschema:"'first_capture', 'changes' or 'no_changes'"`
	// Date is set for an exact day; From and To for a range between two
	// snapshots (HV-7), or for a no_changes entry covering several snapshots.
	Date               string   `json:"date,omitempty"`
	From               string   `json:"from,omitempty"`
	To                 string   `json:"to,omitempty"`
	Archive            bool     `json:"archive" jsonschema:"from an Internet Archive capture, so its dates are ranges between captures"`
	Total              int      `json:"total" jsonschema:"matching images after this snapshot"`
	Added              []string `json:"added"`
	Back               []string `json:"back" jsonschema:"archive era: listed again after a capture that missed them"`
	Removed            []string `json:"removed" jsonschema:"live snapshots only"`
	NotInCapture       []string `json:"not_in_capture" jsonschema:"archive era: listed before but missing from this capture, which doesn't mean removed"`
	BecameFree         []string `json:"became_free"`
	BecameSubscription []string `json:"became_subscription"`
	// FreeFirstRecorded is the number of matching free images in the first live snapshot.
	FreeFirstRecorded *int `json:"free_first_recorded,omitempty"`
}

type HistoryTotals struct {
	Date  string `json:"date"`
	Total int    `json:"total"`
	Free  *int   `json:"free,omitempty"`
}

type HistoryReport struct {
	LastSnapshot  string         `json:"last_snapshot"`
	CopySource    string         `json:"copy_source" jsonschema:"'build' (embedded at build time) or 'data_branch' (fetched)"`
	FetchedAt     string         `json:"fetched_at,omitempty"`
	DaysOld       int            `json:"days_old" jsonschema:"age of the last snapshot in days"`
	Stalled       bool           `json:"stalled"`
	Note          string         `json:"note,omitempty" jsonschema:"set when the daily collector may have stalled"`
	DatesNote     string         `json:"dates_note" jsonschema:"how precise the dates are"`
	Query         string         `json:"query,omitempty"`
	Since         string         `json:"since,omitempty"`
	Points        []HistoryPoint `json:"points"`
	Entries       []HistoryEntry `json:"entries" jsonschema:"newest first"`
	TotalsAtSince *HistoryTotals `json:"totals_at_since,omitempty" jsonschema:"totals at the last snapshot on or before since"`
	TotalNow      int            `json:"total_now"`
	FreeNow       *int           `json:"free_now,omitempty"`
}

// CatalogSince says when an image first appeared in the catalog (HV-10).
type CatalogSince struct {
	EarliestCapture bool   `json:"earliest_capture" jsonschema:"the image was already in the earliest capture"`
	Date            string `json:"date,omitempty"`
	From            string `json:"from,omitempty"`
	To              string `json:"to,omitempty"`
	Text            string `json:"text"`
}

func dayBefore(prev, date string) bool {
	p, err1 := time.Parse(time.DateOnly, prev)
	d, err2 := time.Parse(time.DateOnly, date)
	return err1 == nil && err2 == nil && p.AddDate(0, 0, 1).Equal(d)
}

// lineDates dates line i's changes: exact when the previous snapshot is from
// the day before, otherwise a range from the previous snapshot (HV-7).
func (c *historyCopy) lineDates(i int) (date, from, to string) {
	d := c.lines[i].Date
	if i == 0 || dayBefore(c.lines[i-1].Date, d) {
		return d, "", ""
	}
	return "", c.lines[i-1].Date, d
}

func (c *historyCopy) isArchive(i int) bool {
	return i == 0 || c.lines[i].Source == sourceArchive
}

func (c *historyCopy) catalogSince(image string) *CatalogSince {
	i, ok := c.firstSeen[image]
	if !ok {
		return &CatalogSince{Text: "not yet recorded in the catalog history"}
	}
	if i == 0 {
		d := c.lines[0].Date
		return &CatalogSince{EarliestCapture: true, Date: d, Text: "since at least " + d + " (the earliest capture)"}
	}
	date, from, to := c.lineDates(i)
	if date != "" {
		return &CatalogSince{Date: date, Text: "added " + date}
	}
	return &CatalogSince{From: from, To: to, Text: "added between " + from + " and " + to}
}

// matching returns the names containing q (already lowercase), never nil.
func matching(names []string, q string) []string {
	out := []string{}
	for _, n := range names {
		if q == "" || strings.Contains(n, q) {
			out = append(out, n)
		}
	}
	return out
}

func (c *historyCopy) firstLive() int {
	for i, l := range c.lines {
		if l.Source == sourceLive {
			return i
		}
	}
	return -1
}

func (c *historyCopy) report(in HistoryInput, today time.Time) *HistoryReport {
	q := strings.ToLower(strings.TrimSpace(in.Query))
	n := len(c.lines)
	r := &HistoryReport{
		LastSnapshot: c.lines[n-1].Date, CopySource: c.source, Query: q, Since: in.Since,
		Points: make([]HistoryPoint, n), Entries: []HistoryEntry{},
	}
	if !c.fetchedAt.IsZero() {
		r.FetchedAt = c.fetchedAt.UTC().Format(time.RFC3339)
	}
	if last, err := time.Parse(time.DateOnly, r.LastSnapshot); err == nil {
		t := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
		r.DaysOld = int(t.Sub(last).Hours() / 24)
	}
	if r.DaysOld > stalledAfterDays {
		r.Stalled = true
		r.Note = fmt.Sprintf("The last snapshot is %d days old; the daily collector may have stalled, so newer changes may be missing.", r.DaysOld)
	}
	firstLive := c.firstLive()
	r.DatesNote = "Snapshots before the first daily one are Internet Archive captures of Chainguard's image list, so their changes are dated as ranges between two captures, and an image missing from a capture isn't necessarily removed."
	if firstLive >= 0 {
		r.DatesNote = "Snapshots before " + c.lines[firstLive].Date + " are Internet Archive captures of Chainguard's image list, so their changes are dated as ranges between two captures, and an image missing from a capture isn't necessarily removed."
	}

	counts := make([]int, n)
	start := -1
	for i := range c.lines {
		counts[i] = len(matching(c.sets[i], q))
		if start < 0 && counts[i] > 0 {
			start = i
		}
		p := HistoryPoint{Date: c.lines[i].Date, Source: c.lines[i].Source, Total: counts[i]}
		if c.freeSets[i] != nil {
			f := len(matching(c.freeSets[i], q))
			p.Free = &f
		}
		r.Points[i] = p
	}
	r.TotalNow, r.FreeNow = r.Points[n-1].Total, r.Points[n-1].Free
	if in.Since != "" {
		for i := n - 1; i >= 0; i-- {
			if c.lines[i].Date <= in.Since {
				p := r.Points[i]
				r.TotalsAtSince = &HistoryTotals{Date: p.Date, Total: p.Total, Free: p.Free}
				break
			}
		}
	}
	if start < 0 {
		return r
	}

	// Walk newest first from the last line down to the first match (HV-17),
	// merging runs of snapshots without changes (HV-8).
	var entries []HistoryEntry
	for i := n - 1; i >= start; i-- {
		e := c.entry(i, q, counts[i])
		if i == firstLive {
			if f := len(matching(c.freeSets[i], q)); f > 0 {
				e.FreeFirstRecorded = &f
			}
		}
		if k := len(entries) - 1; k >= 0 && e.Kind == "no_changes" && entries[k].Kind == "no_changes" && entries[k].Archive == e.Archive {
			prev := &entries[k]
			if prev.To == "" {
				prev.From, prev.To, prev.Date = prev.Date, prev.Date, ""
			}
			prev.From = e.Date
			if e.FreeFirstRecorded != nil {
				prev.FreeFirstRecorded = e.FreeFirstRecorded
			}
			continue
		}
		entries = append(entries, e)
	}
	for _, e := range entries {
		if in.Since != "" && max(e.Date, e.To) <= in.Since {
			continue
		}
		r.Entries = append(r.Entries, e)
	}
	return r
}

// entry describes line i's changes to the images matching q; total is the
// number of matching images after the line.
func (c *historyCopy) entry(i int, q string, total int) HistoryEntry {
	l := c.lines[i]
	e := HistoryEntry{
		Archive: c.isArchive(i), Total: total,
		Added: []string{}, Back: []string{}, Removed: []string{}, NotInCapture: []string{},
		BecameFree: []string{}, BecameSubscription: []string{},
	}
	e.Date, e.From, e.To = c.lineDates(i)
	if i == 0 {
		e.Kind = "first_capture"
		return e
	}
	if l.Source == sourceArchive {
		e.Added = matching(c.newImgs[i], q)
		e.Back = matching(c.backImgs[i], q)
		e.NotInCapture = matching(l.Removed, q)
	} else {
		e.Added = matching(l.Added, q)
		e.Removed = matching(l.Removed, q)
		e.BecameFree = matching(l.BecameFree, q)
		e.BecameSubscription = matching(l.BecameSubscription, q)
	}
	e.Kind = "no_changes"
	for _, s := range [][]string{e.Added, e.Back, e.Removed, e.NotInCapture, e.BecameFree, e.BecameSubscription} {
		if len(s) > 0 {
			e.Kind = "changes"
		}
	}
	if e.Kind == "no_changes" {
		// Runs of these merge into one entry from the earliest to the latest
		// snapshot, so each is dated by its own snapshot.
		e.Date, e.From, e.To = l.Date, "", ""
	}
	return e
}

// CatalogHistory reports what changed in Chainguard's catalog and when
// (HV-4 to HV-9, HV-11, HV-12).
func (s *Service) CatalogHistory(ctx context.Context, in HistoryInput) (*HistoryReport, error) {
	if in.Since != "" {
		if _, err := time.Parse(time.DateOnly, in.Since); err != nil {
			return nil, inputErrorf("since must be a date like 2026-10-09")
		}
	}
	if s.History == nil {
		return nil, ErrHistoryUnavailable
	}
	c, err := s.History.current(ctx)
	if err != nil {
		return nil, err
	}
	return c.report(in, s.History.Now().UTC()), nil
}

// catalogSince is an image's first appearance, or nil when no history is
// available. It never fails the caller (HV-13.2).
func (s *Service) catalogSince(ctx context.Context, image string) *CatalogSince {
	if s.History == nil {
		return nil
	}
	c, err := s.History.current(ctx)
	if err != nil {
		return nil
	}
	return c.catalogSince(image)
}
