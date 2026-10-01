package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

const defaultSecDBURL = "https://packages.wolfi.dev/os/security.json"

// SecDB is the Wolfi security database (Alpine "secdb" format). For each
// source package it maps a version to the vulnerability IDs fixed in that
// version. The special version "0" lists IDs that never affected the
// package. It records fixes, not open vulnerabilities.
type SecDB struct {
	HTTP *http.Client
	URL  string
	TTL  time.Duration

	mu        sync.Mutex
	fixes     map[string]map[string][]string
	fetchedAt time.Time
}

func NewSecDB(client *http.Client) *SecDB {
	return &SecDB{HTTP: client, URL: defaultSecDBURL, TTL: 6 * time.Hour}
}

// Fixes returns origin package → version → vulnerability IDs.
func (s *SecDB) Fixes(ctx context.Context) (map[string]map[string][]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fixes != nil && time.Since(s.fetchedAt) < s.TTL {
		return s.fixes, nil
	}
	fixes, err := s.fetch(ctx)
	if err != nil {
		if s.fixes != nil {
			return s.fixes, nil
		}
		return nil, err
	}
	s.fixes, s.fetchedAt = fixes, time.Now()
	return fixes, nil
}

func (s *SecDB) fetch(ctx context.Context) (map[string]map[string][]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.URL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching security database: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching security database: unexpected status %s", resp.Status)
	}
	var db struct {
		Packages []struct {
			Pkg struct {
				Name     string              `json:"name"`
				Secfixes map[string][]string `json:"secfixes"`
			} `json:"pkg"`
		} `json:"packages"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&db); err != nil {
		return nil, fmt.Errorf("decoding security database: %w", err)
	}
	fixes := make(map[string]map[string][]string, len(db.Packages))
	for _, p := range db.Packages {
		fixes[p.Pkg.Name] = p.Pkg.Secfixes
	}
	return fixes, nil
}

// Vulnerability statuses reported for a specific ID.
const (
	StatusNotAffected = "not_affected" // never affected this package
	StatusFixed       = "fixed"        // installed version includes the fix
	StatusVulnerable  = "vulnerable"   // a fix exists but isn't installed
)

type CVEMatch struct {
	Package          string   `json:"package" jsonschema:"origin (source) package"`
	InstalledVersion string   `json:"installed_version"`
	Status           string   `json:"status" jsonschema:"not_affected, fixed, or vulnerable"`
	FixedVersion     string   `json:"fixed_version,omitempty"`
	Subpackages      []string `json:"subpackages" jsonschema:"installed packages built from this origin"`
}

type PackageFixes struct {
	Package          string   `json:"package" jsonschema:"origin (source) package"`
	InstalledVersion string   `json:"installed_version"`
	FixedCount       int      `json:"fixed_count" jsonschema:"vulnerabilities fixed at or before the installed version"`
	NotAffectedCount int      `json:"not_affected_count" jsonschema:"vulnerabilities recorded as never affecting this package"`
	PendingFixes     []string `json:"pending_fixes,omitempty" jsonschema:"IDs fixed only in newer versions than the one installed"`
}

type VulnReport struct {
	Image            string         `json:"image"`
	Tag              string         `json:"tag"`
	Digest           string         `json:"digest" jsonschema:"linux/amd64 image digest that was analyzed"`
	ID               string         `json:"id,omitempty" jsonschema:"the vulnerability ID that was looked up, if any"`
	Matches          []CVEMatch     `json:"matches,omitempty"`
	Packages         []PackageFixes `json:"packages,omitempty"`
	TotalFixed       int            `json:"total_fixed"`
	TotalNotAffected int            `json:"total_not_affected"`
	Uncovered        []string       `json:"uncovered,omitempty" jsonschema:"packages outside Wolfi that this data doesn't cover"`
	Note             string         `json:"note"`
}

const vulnNote = "Based on the Wolfi security database, which records fixed and non-applicable vulnerabilities per package. " +
	"It does not list unfixed vulnerabilities, so absence of a record does not prove an image is unaffected. " +
	"Use a scanner such as grype for a full report."

// analyzeVulns reports on an image's packages against the security database.
// If id is set, only that vulnerability is looked up.
func analyzeVulns(pkgs []Package, fixes map[string]map[string][]string, id string) VulnReport {
	type origin struct {
		version string
		subs    []string
	}
	origins := make(map[string]*origin)
	var uncovered []string
	for _, p := range pkgs {
		if p.Distro != "wolfi" {
			uncovered = append(uncovered, p.Name)
			continue
		}
		o := origins[p.Origin]
		if o == nil {
			o = &origin{version: p.Version}
			origins[p.Origin] = o
		}
		o.subs = append(o.subs, p.Name)
	}
	names := make([]string, 0, len(origins))
	for name := range origins {
		names = append(names, name)
	}
	sort.Strings(names)

	report := VulnReport{Uncovered: uncovered, Note: vulnNote}
	id = strings.ToUpper(strings.TrimSpace(id))
	report.ID = id

	for _, name := range names {
		o := origins[name]
		pf := PackageFixes{Package: name, InstalledVersion: o.version}
		for ver, ids := range fixes[name] {
			for _, vid := range ids {
				var status string
				switch {
				case ver == "0":
					status = StatusNotAffected
					pf.NotAffectedCount++
				case compareAPKVersions(o.version, ver) >= 0:
					status = StatusFixed
					pf.FixedCount++
				default:
					status = StatusVulnerable
					pf.PendingFixes = append(pf.PendingFixes, vid)
				}
				if id != "" && strings.EqualFold(vid, id) {
					m := CVEMatch{Package: name, InstalledVersion: o.version, Status: status, Subpackages: o.subs}
					if ver != "0" {
						m.FixedVersion = ver
					}
					report.Matches = append(report.Matches, m)
				}
			}
		}
		sort.Strings(pf.PendingFixes)
		report.TotalFixed += pf.FixedCount
		report.TotalNotAffected += pf.NotAffectedCount
		if id == "" {
			report.Packages = append(report.Packages, pf)
		}
	}
	if id != "" && len(report.Matches) == 0 {
		report.Note = "No record of " + id + " for any package in this image. " + vulnNote
	}
	return report
}

// compareAPKVersions compares two Alpine/Wolfi package versions such as
// "3.6.5-r1" or "1.2_rc1-r0". It returns -1, 0 or 1.
func compareAPKVersions(a, b string) int {
	va, okA := parseAPKVersion(a)
	vb, okB := parseAPKVersion(b)
	if !okA || !okB {
		return strings.Compare(a, b)
	}
	for i := range max(len(va.nums), len(vb.nums)) {
		if i >= len(va.nums) {
			return -1
		}
		if i >= len(vb.nums) {
			return 1
		}
		if c := compareNumeric(va.nums[i], vb.nums[i]); c != 0 {
			return c
		}
	}
	if va.letter != vb.letter {
		if va.letter < vb.letter {
			return -1
		}
		return 1
	}
	for i := range max(len(va.suffixes), len(vb.suffixes)) {
		sa, sb := apkSuffix{}, apkSuffix{}
		if i < len(va.suffixes) {
			sa = va.suffixes[i]
		}
		if i < len(vb.suffixes) {
			sb = vb.suffixes[i]
		}
		if sa.rank != sb.rank {
			if sa.rank < sb.rank {
				return -1
			}
			return 1
		}
		if c := compareNumeric(sa.num, sb.num); c != 0 {
			return c
		}
	}
	return compareNumeric(va.release, vb.release)
}

type apkVersion struct {
	nums     []string
	letter   byte
	suffixes []apkSuffix
	release  string
}

// apkSuffix ranks pre-release suffixes below "no suffix" (rank 0) and
// post-release suffixes above it.
type apkSuffix struct {
	rank int
	num  string
}

var apkSuffixRanks = map[string]int{
	"alpha": -4, "beta": -3, "pre": -2, "rc": -1,
	"cvs": 1, "svn": 2, "git": 3, "hg": 4, "p": 5,
}

func parseAPKVersion(s string) (apkVersion, bool) {
	var v apkVersion
	if i := strings.LastIndex(s, "-r"); i >= 0 && isDigits(s[i+2:]) {
		s, v.release = s[:i], s[i+2:]
	}
	parts := strings.Split(s, "_")
	base := parts[0]
	if n := len(base); n > 0 && base[n-1] >= 'a' && base[n-1] <= 'z' {
		v.letter, base = base[n-1], base[:n-1]
	}
	v.nums = strings.Split(base, ".")
	for _, n := range v.nums {
		if !isDigits(n) {
			return v, false
		}
	}
	for _, suf := range parts[1:] {
		name := strings.TrimRight(suf, "0123456789")
		rank, ok := apkSuffixRanks[name]
		if !ok {
			return v, false
		}
		v.suffixes = append(v.suffixes, apkSuffix{rank: rank, num: suf[len(name):]})
	}
	return v, true
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := range len(s) {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// compareNumeric compares digit strings of any length; empty counts as 0.
func compareNumeric(a, b string) int {
	a, b = strings.TrimLeft(a, "0"), strings.TrimLeft(b, "0")
	if len(a) != len(b) {
		if len(a) < len(b) {
			return -1
		}
		return 1
	}
	return strings.Compare(a, b)
}
