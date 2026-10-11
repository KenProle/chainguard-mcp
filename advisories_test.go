package main

import (
	"encoding/json"
	"os"
	"slices"
	"strings"
	"testing"
	"time"
)

var checkedAt = time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)

// pythonOSV loads the OSV.dev records captured for python's SBOM packages on
// 2026-10-11 (trimmed to the Wolfi entries) and the packages themselves. OSV.dev
// served older copies of some advisories then: the four glibc ones below were
// already fixed in the installed version in Chainguard's feed.
func pythonOSV(t *testing.T, tag string) ([]Package, map[OSVPackage][]OSVVuln) {
	t.Helper()
	raw, err := os.ReadFile("testdata/osv-python-2026-10-11.json")
	if err != nil {
		t.Fatal(err)
	}
	var all map[string]struct {
		Packages []Package            `json:"packages"`
		Records  map[string][]OSVVuln `json:"records"`
	}
	if err := json.Unmarshal(raw, &all); err != nil {
		t.Fatal(err)
	}
	d := all[tag]
	results := map[OSVPackage][]OSVVuln{}
	for _, p := range d.Packages {
		results[OSVPackage{"Wolfi", p.Name, p.Version}] = d.Records[p.Name]
	}
	return d.Packages, results
}

// pythonFeed loads the current records of those advisories from Chainguard's
// advisory feed, captured on 2026-10-11 (trimmed to the Wolfi entries).
func pythonFeed(t *testing.T) map[string]*OSVVuln {
	t.Helper()
	raw, err := os.ReadFile("testdata/feed-python-2026-10-11.json")
	if err != nil {
		t.Fatal(err)
	}
	var all map[string]*OSVVuln
	if err := json.Unmarshal(raw, &all); err != nil {
		t.Fatal(err)
	}
	return all
}

// pythonReport builds the report the way openAdvisories does: OSV.dev's
// records, replaced by the feed's current ones.
func pythonReport(t *testing.T, tag, id string) AdvisoryReport {
	t.Helper()
	pkgs, results := pythonOSV(t, tag)
	results, missing := withCurrentRecords(results, pythonFeed(t))
	if len(missing) != 0 {
		t.Fatalf("advisories missing from the feed fixture: %v", missing)
	}
	return buildAdvisoryReport(pkgs, results, checkedAt, id)
}

// openRecords counts the (package, advisory) pairs that are open, and all.
func openRecords(results map[OSVPackage][]OSVVuln) (open, all int) {
	for pkg, vulns := range results {
		for _, v := range vulns {
			all++
			if ok, _ := openState(v, pkg); ok {
				open++
			}
		}
	}
	return open, all
}

func findVuln(t *testing.T, r AdvisoryReport, id string) OpenVulnerability {
	t.Helper()
	for _, v := range r.Vulnerabilities {
		if v.ID == id {
			return v
		}
	}
	t.Fatalf("%s not in the report", id)
	return OpenVulnerability{}
}

func hasVuln(r AdvisoryReport, id string) bool {
	return slices.ContainsFunc(r.Vulnerabilities, func(v OpenVulnerability) bool { return v.ID == id })
}

func pkgNames(v OpenVulnerability) []string {
	var out []string
	for _, p := range v.Packages {
		out = append(out, p.Name)
	}
	return out
}

// record builds a Chainguard-shaped OSV record for one Wolfi package. comps
// are (name, type) pairs; the default is the package itself as an apk.
func record(id string, upstream []string, pkg, status, fixed string, comps ...[2]string) OSVVuln {
	raw := map[string]any{"package": map[string]any{"name": pkg, "ecosystem": "Wolfi"}}
	if fixed != "" {
		raw["ranges"] = []any{map[string]any{"events": []any{map[string]any{"introduced": "0"}, map[string]any{"fixed": fixed}}}}
	}
	if len(comps) == 0 {
		comps = [][2]string{{pkg, "apk"}}
	}
	var cs []any
	for _, c := range comps {
		cs = append(cs, map[string]any{"component_name": c[0], "component_type": c[1], "latest_event_status": status})
	}
	raw["ecosystem_specific"] = map[string]any{"components": cs}
	b, err := json.Marshal(map[string]any{"id": id, "upstream": upstream, "affected": []any{raw}})
	if err != nil {
		panic(err)
	}
	var v OSVVuln
	if err := json.Unmarshal(b, &v); err != nil {
		panic(err)
	}
	return v
}

func wolfi(name, version string) (Package, OSVPackage) {
	return Package{Name: name, Version: version, Distro: "wolfi"}, OSVPackage{"Wolfi", name, version}
}

func TestOA1_3_PackageFromAnotherSourceIsNotLookedUp(t *testing.T) {
	pkgs := []Package{
		{Name: "a", Version: "1", Distro: "wolfi"},
		{Name: "b", Version: "2", Distro: "chainguard"},
		{Name: "c", Version: "3", Distro: "alpine"},
	}
	lookups, notCovered := advisoryLookups(pkgs)
	if len(lookups) != 2 || lookups[0].Ecosystem != "Wolfi" || lookups[1].Ecosystem != "Chainguard" {
		t.Errorf("lookups = %v", lookups)
	}
	if !slices.Equal(notCovered, []string{"c"}) {
		t.Errorf("not covered = %v", notCovered)
	}
	r := buildAdvisoryReport(pkgs, nil, checkedAt, "")
	if !slices.Equal(r.NotCovered, []string{"c"}) {
		t.Errorf("report not covered = %v", r.NotCovered)
	}
}

func TestOA1_1_PythonLatestLooksUpEveryPackage(t *testing.T) {
	pkgs, _ := pythonOSV(t, "latest")
	lookups, notCovered := advisoryLookups(pkgs)
	if len(lookups) != 29 || len(notCovered) != 0 {
		t.Errorf("%d lookups, %d not covered; want 29 and 0", len(lookups), len(notCovered))
	}
	for _, l := range lookups {
		if l.Ecosystem != "Wolfi" {
			t.Errorf("%v not in Wolfi", l)
		}
	}
}

func TestOA1_2_FoundForEachPackageNotEachSourcePackage(t *testing.T) {
	v := findVuln(t, pythonReport(t, "latest", ""), "CVE-2025-15367")
	// python-3.15-base is built from the same source and has no advisory.
	if !slices.Equal(pkgNames(v), []string{"python-3.15"}) {
		t.Errorf("packages = %v", pkgNames(v))
	}
}

func TestOA2_1_PythonLatestDevSixtyFourAdvisoriesAreTwentyOneVulnerabilities(t *testing.T) {
	_, results := pythonOSV(t, "latest-dev")
	results, _ = withCurrentRecords(results, pythonFeed(t))
	open, all := openRecords(results)
	r := pythonReport(t, "latest-dev", "")
	if all != 64 || open != 39 || r.Total != 21 || len(r.Vulnerabilities) != 21 {
		t.Errorf("%d advisories, %d open → %d vulnerabilities, want 64, 39 → 21", all, open, r.Total)
	}
}

func TestOA2_2_TwoAdvisoriesOnOnePackageCountOnce(t *testing.T) {
	v := findVuln(t, pythonReport(t, "latest", ""), "CVE-2026-12345")
	if len(v.Packages) != 1 || v.Packages[0].Name != "python-3.15" ||
		!slices.Equal(v.Packages[0].Advisories, []string{"CGA-2w26-qvh7-3jhg", "CGA-mp3h-w2rr-vq2f"}) {
		t.Errorf("packages = %+v, want python-3.15 with both advisory IDs", v.Packages)
	}
}

func TestOA2_3_AdvisoryWithoutCVECountsUnderGHSA(t *testing.T) {
	a, ak := wolfi("a", "1")
	b, bk := wolfi("b", "1")
	va := record("CGA-aaaa", []string{"GHSA-1111-2222-3333"}, "a", "detection", "")
	vb := record("CGA-bbbb", []string{"GHSA-1111-2222-3333"}, "b", "detection", "")
	r := buildAdvisoryReport([]Package{a, b}, map[OSVPackage][]OSVVuln{ak: {va}, bk: {vb}}, checkedAt, "")
	if r.Total != 1 || r.Vulnerabilities[0].ID != "GHSA-1111-2222-3333" || len(r.Vulnerabilities[0].Packages) != 2 {
		t.Errorf("report = %+v", r.Vulnerabilities)
	}
}

func TestOA2_4_AdvisoryNamingSeveralCVEsCountsUnderEach(t *testing.T) {
	a, ak := wolfi("a", "1")
	v := record("CGA-aaaa", []string{"CVE-2026-0001", "CVE-2026-0002"}, "a", "pending_upstream_fix", "")
	r := buildAdvisoryReport([]Package{a}, map[OSVPackage][]OSVVuln{ak: {v}}, checkedAt, "")
	if r.Total != 2 {
		t.Fatalf("total = %d, want 2", r.Total)
	}
	for _, id := range []string{"CVE-2026-0001", "CVE-2026-0002"} {
		if p := findVuln(t, r, id).Packages; len(p) != 1 || p[0].Advisories[0] != "CGA-aaaa" {
			t.Errorf("%s packages = %+v", id, p)
		}
	}
}

func TestOA3_1_PythonStatuses(t *testing.T) {
	latest := pythonReport(t, "latest", "")
	for _, id := range []string{"CVE-2025-15367", "CVE-2026-12345"} {
		if s := findVuln(t, latest, id).Status; s != AdvNotAssessed {
			t.Errorf("latest %s = %s, want not_yet_assessed", id, s)
		}
	}
	dev := pythonReport(t, "latest-dev", "")
	for _, id := range []string{"CVE-2026-6844", "CVE-2026-58470"} {
		if s := findVuln(t, dev, id).Status; s != AdvPendingUpstream {
			t.Errorf("latest-dev %s = %s, want pending_upstream_fix", id, s)
		}
	}
}

func TestOA3_2_FixedVersion(t *testing.T) {
	a, ak := wolfi("cilium-fips-1.20-operator-aws", "1.20.1-r0")
	v := record("CGA-rc72-c57w-7cmr", []string{"CVE-2026-0003"}, a.Name, "fixed", "1.20.2-r3")
	r := buildAdvisoryReport([]Package{a}, map[OSVPackage][]OSVVuln{ak: {v}}, checkedAt, "")
	got := r.Vulnerabilities[0]
	if got.Status != AdvFixAvailable || got.Packages[0].FixedVersion != "1.20.2-r3" {
		t.Errorf("got %+v", got)
	}
}

func TestOA3_3_FixNotPlanned(t *testing.T) {
	a, ak := wolfi("argo-cd-fips-3.2-compat", "3.2.0-r0")
	v := record("CGA-vjm6-5xm3-7w39", []string{"CVE-2026-0004"}, a.Name, "fix_not_planned", "")
	r := buildAdvisoryReport([]Package{a}, map[OSVPackage][]OSVVuln{ak: {v}}, checkedAt, "")
	if r.Vulnerabilities[0].Status != AdvFixNotPlanned || r.Counts.FixNotPlanned != 1 {
		t.Errorf("got %+v, counts %+v", r.Vulnerabilities[0], r.Counts)
	}
}

func TestOA3_UnknownStateIsConfirmed(t *testing.T) {
	a, ak := wolfi("a", "1")
	for _, state := range []string{"true_positive_determination", "something_new", ""} {
		v := record("CGA-aaaa", []string{"CVE-2026-0005"}, "a", state, "")
		r := buildAdvisoryReport([]Package{a}, map[OSVPackage][]OSVVuln{ak: {v}}, checkedAt, "")
		if r.Vulnerabilities[0].Status != AdvConfirmed {
			t.Errorf("state %q → %s, want confirmed", state, r.Vulnerabilities[0].Status)
		}
	}
}

func TestOA4_1_MixedStatusesTakeTheFirstThatApplies(t *testing.T) {
	a, ak := wolfi("a", "1")
	b, bk := wolfi("b", "1")
	va := record("CGA-aaaa", []string{"CVE-2026-0006"}, "a", "detection", "")
	vb := record("CGA-bbbb", []string{"CVE-2026-0006"}, "b", "pending_upstream_fix", "")
	r := buildAdvisoryReport([]Package{a, b}, map[OSVPackage][]OSVVuln{ak: {va}, bk: {vb}}, checkedAt, "")
	v := r.Vulnerabilities[0]
	if v.Status != AdvPendingUpstream || v.Packages[0].Status != AdvNotAssessed || v.Packages[1].Status != AdvPendingUpstream {
		t.Errorf("got %+v", v)
	}
}

func TestOA4_2_FixForOnePackageWins(t *testing.T) {
	a, ak := wolfi("a", "1")
	b, bk := wolfi("b", "1")
	va := record("CGA-aaaa", []string{"CVE-2026-0007"}, "a", "pending_upstream_fix", "")
	vb := record("CGA-bbbb", []string{"CVE-2026-0007"}, "b", "fixed", "2")
	r := buildAdvisoryReport([]Package{a, b}, map[OSVPackage][]OSVVuln{ak: {va}, bk: {vb}}, checkedAt, "")
	if r.Vulnerabilities[0].Status != AdvFixAvailable || r.Counts.FixAvailable != 1 {
		t.Errorf("got %+v", r.Vulnerabilities[0])
	}
}

func TestOA5_1_PythonLatestDevTotals(t *testing.T) {
	r := pythonReport(t, "latest-dev", "")
	want := AdvisoryCounts{PendingUpstreamFix: 17, NotYetAssessed: 4}
	if r.Total != 21 || r.Counts != want {
		t.Errorf("total %d, counts %+v; want 21, %+v", r.Total, r.Counts, want)
	}
}

func TestOA5_2_CountsAddUp(t *testing.T) {
	for _, tag := range []string{"latest", "latest-dev"} {
		r := pythonReport(t, tag, "")
		c := r.Counts
		if sum := c.FixAvailable + c.PendingUpstreamFix + c.Confirmed + c.FixNotPlanned + c.NotYetAssessed; sum != r.Total {
			t.Errorf("%s: counts sum to %d, total %d", tag, sum, r.Total)
		}
	}
}

func TestOA6_1_PythonLatestRow(t *testing.T) {
	v := findVuln(t, pythonReport(t, "latest", ""), "CVE-2026-12345")
	if v.Status != AdvNotAssessed || len(v.Packages) != 1 {
		t.Fatalf("got %+v", v)
	}
	if p := v.Packages[0]; p.Name != "python-3.15" || p.Version != "3.15.0_git20261010-r0" || p.Status != AdvNotAssessed {
		t.Errorf("package = %+v", p)
	}
}

func TestOA6_2_RowOrder(t *testing.T) {
	r := pythonReport(t, "latest-dev", "")
	var pending, assessed []string
	seenAssessed := false
	for _, v := range r.Vulnerabilities {
		switch v.Status {
		case AdvPendingUpstream:
			if seenAssessed {
				t.Errorf("%s (waiting) comes after a not-yet-assessed row", v.ID)
			}
			pending = append(pending, v.ID)
		case AdvNotAssessed:
			seenAssessed = true
			assessed = append(assessed, v.ID)
		default:
			t.Errorf("%s has status %s", v.ID, v.Status)
		}
	}
	// Each group is in ID order (a plain string comparison, as the UI shows it).
	for _, ids := range [][]string{pending, assessed} {
		if !slices.IsSorted(ids) {
			t.Errorf("not in ID order: %v", ids)
		}
	}
	if len(pending) != 17 || !slices.Equal(assessed, []string{"CVE-2025-15367", "CVE-2025-47273", "CVE-2026-12345", "CVE-2026-21895"}) {
		t.Errorf("pending %d, not yet assessed %v", len(pending), assessed)
	}
}

func TestOA7_1_RustCrateInUv(t *testing.T) {
	v := findVuln(t, pythonReport(t, "latest-dev", ""), "CVE-2026-21895")
	if len(v.Packages) != 1 || v.Packages[0].Name != "uv" || !slices.Equal(v.Packages[0].Components, []AdvisoryComponent{{"rsa", "Rust crate"}}) {
		t.Errorf("got %+v", v.Packages)
	}
}

func TestOA7_2_BundledPythonPackage(t *testing.T) {
	v := findVuln(t, pythonReport(t, "latest-dev", ""), "CVE-2025-47273")
	if len(v.Packages) != 1 || v.Packages[0].Name != "py3.15-pip-base" || !slices.Equal(v.Packages[0].Components, []AdvisoryComponent{{"setuptools", "Python package"}}) {
		t.Errorf("got %+v", v.Packages)
	}
}

func TestOA7_ApkAdvisoryHasNoComponents(t *testing.T) {
	v := findVuln(t, pythonReport(t, "latest-dev", ""), "CVE-2026-6844")
	for _, p := range v.Packages {
		if len(p.Components) != 0 {
			t.Errorf("%s components = %v", p.Name, p.Components)
		}
	}
}

func TestOA9_1_LookupByCVE(t *testing.T) {
	r := pythonReport(t, "latest", "cve-2026-12345")
	if r.ID != "CVE-2026-12345" || r.Match == nil || r.Match.Status != AdvNotAssessed || !slices.Equal(pkgNames(*r.Match), []string{"python-3.15"}) {
		t.Fatalf("match = %+v", r.Match)
	}
	if r.LookupNote != "" {
		t.Errorf("lookup note = %q", r.LookupNote)
	}
}

func TestOA9_2_LookupByAdvisoryID(t *testing.T) {
	r := pythonReport(t, "latest", "CGA-2w26-qvh7-3jhg")
	if r.Match == nil || r.Match.ID != "CVE-2026-12345" {
		t.Errorf("match = %+v", r.Match)
	}
}

func TestOA9_LookupByGHSA(t *testing.T) {
	r := pythonReport(t, "latest", "ghsa-g82h-mgfp-jx8g")
	if r.Match == nil || r.Match.ID != "CVE-2025-15367" {
		t.Errorf("match = %+v", r.Match)
	}
}

func TestOA9_3_NoOpenAdvisory(t *testing.T) {
	r := pythonReport(t, "latest", "CVE-2020-0001")
	if r.Match != nil || !strings.Contains(r.LookupNote, "No open advisory for CVE-2020-0001") {
		t.Fatalf("match %+v, note %q", r.Match, r.LookupNote)
	}
	if !strings.Contains(r.LookupNote, "not a full scan") {
		t.Errorf("note should disclaim: %q", r.LookupNote)
	}
}

func TestOA9_4_AdvisoryAlreadyFixedInTheInstalledVersion(t *testing.T) {
	// OSV.dev still lists these as pending; the feed has them fixed in 2.44-r8.
	for _, id := range []string{"CVE-2026-89092", "CGA-2727-9j94-3x3p", "GHSA-gwqp-9qgw-c5pv"} {
		r := pythonReport(t, "latest", id)
		if r.Match != nil || !strings.Contains(r.LookupNote, "No open advisory for") {
			t.Errorf("%s: match %+v, note %q", id, r.Match, r.LookupNote)
		}
	}
}

func TestOA8_2_ZeroTotalWording(t *testing.T) {
	r := buildAdvisoryReport([]Package{{Name: "a", Version: "1", Distro: "wolfi"}}, map[OSVPackage][]OSVVuln{}, checkedAt, "")
	if r.Total != 0 || !strings.Contains(r.Note, "No open advisories are published") {
		t.Errorf("total %d, note %q", r.Total, r.Note)
	}
	for _, bad := range []string{"vulnerability-free", "no vulnerabilities", "secure"} {
		if strings.Contains(strings.ToLower(r.Note+r.Source), bad) {
			t.Errorf("wording contains %q", bad)
		}
	}
}

func TestOA8_1_SourceNamesBothSources(t *testing.T) {
	r := pythonReport(t, "latest", "")
	for _, want := range []string{"Chainguard's published advisories", "found via OSV.dev", "status from Chainguard's advisory feed", "2026-10-11 12:00 UTC"} {
		if !strings.Contains(r.Source, want) {
			t.Errorf("source %q lacks %q", r.Source, want)
		}
	}
}

func TestOA16_1_GlibcFixedIn244r8(t *testing.T) {
	// The OSV.dev copy of CGA-2727-9j94-3x3p is pending; the feed's is fixed.
	_, osvResults := pythonOSV(t, "latest")
	var stale OSVVuln
	for _, v := range osvResults[OSVPackage{"Wolfi", "glibc-2.44", "2.44-r8"}] {
		if v.ID == "CGA-2727-9j94-3x3p" {
			stale = v
		}
	}
	glibc := OSVPackage{"Wolfi", "glibc-2.44", "2.44-r8"}
	if open, fixed := openState(stale, glibc); !open || fixed != "" {
		t.Fatalf("the OSV.dev copy should be open without a fix, got %v %q", open, fixed)
	}
	if open, _ := openState(*pythonFeed(t)["CGA-2727-9j94-3x3p"], glibc); open {
		t.Error("the feed's record is fixed in 2.44-r8 and should not be open")
	}

	r := pythonReport(t, "latest", "")
	for _, id := range []string{"CVE-2026-89092", "CVE-2026-8674", "CVE-2026-86805", "CVE-2026-95818", "CVE-2026-97399"} {
		if hasVuln(r, id) {
			t.Errorf("%s is reported, but its advisories are fixed in the installed version", id)
		}
	}
	if r.Total != 2 {
		t.Errorf("total = %d, want 2", r.Total)
	}
}

func TestOA16_2_FixInANewerVersionStaysOpen(t *testing.T) {
	a, ak := wolfi("a", "1.0-r1")
	v := record("CGA-aaaa", []string{"CVE-2026-0010"}, "a", "fixed", "1.0-r2")
	r := buildAdvisoryReport([]Package{a}, map[OSVPackage][]OSVVuln{ak: {v}}, checkedAt, "")
	if r.Total != 1 || r.Vulnerabilities[0].Status != AdvFixAvailable || r.Vulnerabilities[0].Packages[0].FixedVersion != "1.0-r2" {
		t.Errorf("got %+v", r.Vulnerabilities)
	}
	// Fixed in exactly the installed version, and in an older one: not open.
	for _, fixed := range []string{"1.0-r1", "0.9-r5"} {
		v := record("CGA-aaaa", []string{"CVE-2026-0010"}, "a", "fixed", fixed)
		if r := buildAdvisoryReport([]Package{a}, map[OSVPackage][]OSVVuln{ak: {v}}, checkedAt, ""); r.Total != 0 {
			t.Errorf("fixed in %s: %+v", fixed, r.Vulnerabilities)
		}
	}
}

func TestOA16_3_PartlyFixedVulnerabilityKeepsOnlyTheOpenPackage(t *testing.T) {
	a, ak := wolfi("a", "1-r1")
	b, bk := wolfi("b", "1-r1")
	va := record("CGA-aaaa", []string{"CVE-2026-0011"}, "a", "fixed", "1-r1")
	vb := record("CGA-bbbb", []string{"CVE-2026-0011"}, "b", "pending_upstream_fix", "")
	r := buildAdvisoryReport([]Package{a, b}, map[OSVPackage][]OSVVuln{ak: {va}, bk: {vb}}, checkedAt, "")
	if r.Total != 1 || !slices.Equal(pkgNames(r.Vulnerabilities[0]), []string{"b"}) || r.Vulnerabilities[0].Status != AdvPendingUpstream {
		t.Errorf("got %+v", r.Vulnerabilities)
	}
}

func TestOA16_4_RecordWithoutThePackageStaysOpen(t *testing.T) {
	a, ak := wolfi("a", "1")
	// The current record has no entry for the package: open, confirmed.
	cur := record("CGA-aaaa-bbbb-cccc", []string{"CVE-2026-0012"}, "other-package", "fix_not_planned", "")
	if r := buildAdvisoryReport([]Package{a}, map[OSVPackage][]OSVVuln{ak: {cur}}, checkedAt, ""); r.Total != 1 || r.Vulnerabilities[0].Status != AdvConfirmed {
		t.Errorf("got %+v", r.Vulnerabilities)
	}

	// With OSV.dev's copy for the package, its status is kept.
	osvCopy := record("CGA-aaaa-bbbb-cccc", []string{"CVE-2026-0012"}, "a", "pending_upstream_fix", "")
	results, missing := withCurrentRecords(map[OSVPackage][]OSVVuln{ak: {osvCopy}}, map[string]*OSVVuln{"CGA-aaaa-bbbb-cccc": &cur})
	r := buildAdvisoryReport([]Package{a}, results, checkedAt, "")
	if len(missing) != 0 || r.Total != 1 || r.Vulnerabilities[0].Status != AdvPendingUpstream {
		t.Errorf("missing %v, got %+v", missing, r.Vulnerabilities)
	}

	// A current record that has the entry wins over OSV.dev's copy.
	fixed := record("CGA-aaaa-bbbb-cccc", []string{"CVE-2026-0012"}, "a", "fixed", "1")
	results, _ = withCurrentRecords(map[OSVPackage][]OSVVuln{ak: {osvCopy}}, map[string]*OSVVuln{"CGA-aaaa-bbbb-cccc": &fixed})
	if r := buildAdvisoryReport([]Package{a}, results, checkedAt, ""); r.Total != 0 {
		t.Errorf("fixed in the installed version should not be open: %+v", r.Vulnerabilities)
	}
}

func TestOA16_RangesWithSeveralIntervals(t *testing.T) {
	var v OSVVuln
	if err := json.Unmarshal([]byte(`{"id":"CGA-aaaa","affected":[{"package":{"name":"a","ecosystem":"Wolfi"},
		"ranges":[{"events":[{"introduced":"0"},{"fixed":"1.0-r1"},{"introduced":"2.0-r0"},{"fixed":"2.5-r0"}]}]}]}`), &v); err != nil {
		t.Fatal(err)
	}
	for version, want := range map[string]struct {
		open  bool
		fixed string
	}{
		"0.5-r0": {true, "1.0-r1"},
		"1.0-r1": {false, ""}, // fixed, and before the second introduction
		"1.5-r0": {false, ""},
		"2.2-r0": {true, "2.5-r0"},
		"2.5-r0": {false, ""},
	} {
		open, fixed := openState(v, OSVPackage{"Wolfi", "a", version})
		if open != want.open || fixed != want.fixed {
			t.Errorf("%s: open %v fixed %q, want %v %q", version, open, fixed, want.open, want.fixed)
		}
	}
}
