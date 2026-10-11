package main

import (
	"context"
	"slices"
	"sort"
	"strings"
	"time"
)

// AdvisoryStatus is where Chainguard stands on one vulnerability in an image
// (OA-3, OA-4).
type AdvisoryStatus string

const (
	// AdvFixAvailable: Chainguard's record names a fixed version.
	AdvFixAvailable AdvisoryStatus = "fix_available"
	// AdvPendingUpstream: waiting on an upstream fix (pending_upstream_fix).
	AdvPendingUpstream AdvisoryStatus = "pending_upstream_fix"
	// AdvConfirmed: Chainguard confirmed it applies (any other state).
	AdvConfirmed AdvisoryStatus = "confirmed"
	// AdvFixNotPlanned: Chainguard does not plan a fix (fix_not_planned).
	AdvFixNotPlanned AdvisoryStatus = "fix_not_planned"
	// AdvNotAssessed: detected but not yet assessed (detection).
	AdvNotAssessed AdvisoryStatus = "not_yet_assessed"
)

// advisoryRank orders statuses when one vulnerability has different statuses
// for different packages, and orders the rows: the actionable state first,
// confirmed states above unassessed ones (OA-4).
var advisoryRank = map[AdvisoryStatus]int{
	AdvFixAvailable: 0, AdvPendingUpstream: 1, AdvConfirmed: 2, AdvFixNotPlanned: 3, AdvNotAssessed: 4,
}

const advisorySource = "Known open advisories per Chainguard's published advisories, found via OSV.dev with their status from Chainguard's advisory feed"

// AdvisoryCounts counts vulnerabilities by status. The counts add up to the
// total, and statuses with none are 0 (OA-5).
type AdvisoryCounts struct {
	FixAvailable       int `json:"fix_available" jsonschema:"a fixed version exists for at least one package"`
	PendingUpstreamFix int `json:"pending_upstream_fix" jsonschema:"known, waiting on a fix from the upstream project"`
	Confirmed          int `json:"confirmed" jsonschema:"Chainguard confirmed it applies"`
	FixNotPlanned      int `json:"fix_not_planned" jsonschema:"Chainguard does not plan a fix"`
	NotYetAssessed     int `json:"not_yet_assessed" jsonschema:"detected but not yet assessed by Chainguard; some may turn out not to apply"`
}

// AdvisoryComponent is software compiled or bundled into a package that an
// advisory is about (OA-7).
type AdvisoryComponent struct {
	Name string `json:"name"`
	Kind string `json:"kind" jsonschema:"for example 'Rust crate' or 'Python package'"`
}

// AdvisoryPackage is one installed package a vulnerability affects.
type AdvisoryPackage struct {
	Name         string              `json:"name"`
	Version      string              `json:"version"`
	Status       AdvisoryStatus      `json:"status" jsonschema:"this package's own status"`
	Advisories   []string            `json:"advisories" jsonschema:"Chainguard advisory IDs (CGA-…) for this package"`
	FixedVersion string              `json:"fixed_version,omitempty"`
	Components   []AdvisoryComponent `json:"components,omitempty" jsonschema:"bundled software the advisory is about, when it is not the package itself"`
}

// OpenVulnerability is one vulnerability, counted once however many
// advisories and packages it has (OA-2).
type OpenVulnerability struct {
	ID       string            `json:"id" jsonschema:"the CVE (or GHSA, or advisory) ID the vulnerability is counted under"`
	Status   AdvisoryStatus    `json:"status"`
	Aliases  []string          `json:"aliases,omitempty" jsonschema:"other IDs for the same vulnerability"`
	Packages []AdvisoryPackage `json:"packages"`
}

// AdvisoryReport is an image's open advisories.
type AdvisoryReport struct {
	Image           string              `json:"image"`
	Tag             string              `json:"tag"`
	Digest          string              `json:"digest" jsonschema:"linux/amd64 image digest that was analyzed"`
	CheckedAt       string              `json:"checked_at" jsonschema:"when the oldest data used was fetched from OSV.dev (RFC 3339)"`
	Source          string              `json:"source"`
	Note            string              `json:"note,omitempty"`
	Total           int                 `json:"total" jsonschema:"distinct vulnerabilities with an open advisory"`
	Counts          AdvisoryCounts      `json:"counts"`
	Vulnerabilities []OpenVulnerability `json:"vulnerabilities"`
	NotCovered      []string            `json:"not_covered,omitempty" jsonschema:"packages from other sources that were not looked up"`
	StatusFromOSV   []string            `json:"status_from_osv,omitempty" jsonschema:"advisories that Chainguard's advisory feed has no record of; their status is OSV.dev's copy and couldn't be confirmed"`
	ID              string              `json:"id,omitempty" jsonschema:"the vulnerability ID that was looked up, if any"`
	Match           *OpenVulnerability  `json:"match,omitempty" jsonschema:"the open advisory for the looked-up ID, when there is one"`
	LookupNote      string              `json:"lookup_note,omitempty"`
}

// componentKinds gives readable labels for Chainguard's component types.
var componentKinds = map[string]string{
	"rust-crate": "Rust crate",
	"python":     "Python package",
	"go-module":  "Go module",
	"npm":        "npm package",
}

func componentKind(t string) string {
	if k, ok := componentKinds[t]; ok {
		return k
	}
	return t
}

// advisoryLookups returns what to look up on OSV.dev for each installed
// package (OA-1) and the names of packages from other sources, which are not
// looked up.
func advisoryLookups(pkgs []Package) (lookups []OSVPackage, notCovered []string) {
	seen := make(map[OSVPackage]bool)
	for _, p := range pkgs {
		eco, ok := osvEcosystemForDistro(p.Distro)
		if !ok {
			notCovered = append(notCovered, p.Name)
			continue
		}
		l := OSVPackage{Ecosystem: eco, Name: p.Name, Version: p.Version}
		if !seen[l] {
			seen[l] = true
			lookups = append(lookups, l)
		}
	}
	return lookups, notCovered
}

// openState says whether a record is open for the installed version of a
// package, reading the affected entry with the package's own name and
// ecosystem, and the fixed version that would close it. A record whose ranges
// end at a fixed version at or below the installed version is fixed (OA-16).
// A record without ranges, or without an entry for the package, is open: the
// latter may be a real advisory, and leaving it out could hide it (OA-16.4).
func openState(v OSVVuln, pkg OSVPackage) (open bool, fixed string) {
	for _, a := range v.Affected {
		if a.Package.Name != pkg.Name || a.Package.Ecosystem != pkg.Ecosystem {
			continue
		}
		if len(a.Ranges) == 0 {
			return true, ""
		}
		for _, r := range a.Ranges {
			// Events come in order: each "introduced" opens an interval that
			// the next "fixed" closes.
			introduced, inside := "", false
			for _, e := range r.Events {
				switch {
				case e.Introduced != "":
					introduced, inside = e.Introduced, true
				case e.Fixed != "" && inside:
					if (introduced == "0" || compareAPKVersions(pkg.Version, introduced) >= 0) && compareAPKVersions(pkg.Version, e.Fixed) < 0 {
						return true, e.Fixed
					}
					inside = false
				}
			}
			if inside && (introduced == "0" || compareAPKVersions(pkg.Version, introduced) >= 0) {
				return true, ""
			}
		}
		return false, ""
	}
	return true, ""
}

// hasEntry says whether a record has an affected entry for the package's own
// name and ecosystem.
func hasEntry(v OSVVuln, pkg OSVPackage) bool {
	return slices.ContainsFunc(v.Affected, func(a OSVAffected) bool {
		return a.Package.Name == pkg.Name && a.Package.Ecosystem == pkg.Ecosystem
	})
}

// advisoryStatus reads one open record's status for a package: fix available
// when it names a fixed version newer than the installed one, otherwise the
// apk component's state, else the first component's. States this code doesn't
// know count as confirmed, which is never shown as fixed or harmless.
func advisoryStatus(v OSVVuln, pkg OSVPackage) AdvisoryStatus {
	if _, fixed := openState(v, pkg); fixed != "" {
		return AdvFixAvailable
	}
	for _, a := range v.Affected {
		if a.Package.Name != pkg.Name || a.Package.Ecosystem != pkg.Ecosystem {
			continue
		}
		comps := a.EcosystemSpecific.Components
		if len(comps) == 0 {
			break
		}
		c := comps[0]
		for _, x := range comps {
			if x.ComponentType == "apk" {
				c = x
				break
			}
		}
		switch c.LatestEventStatus {
		case "pending_upstream_fix":
			return AdvPendingUpstream
		case "fix_not_planned":
			return AdvFixNotPlanned
		case "detection":
			return AdvNotAssessed
		}
		break
	}
	return AdvConfirmed
}

// nonAPKComponents lists the bundled software a record is about for a package.
func nonAPKComponents(v OSVVuln, pkg OSVPackage) []AdvisoryComponent {
	var out []AdvisoryComponent
	for _, a := range v.Affected {
		if a.Package.Name != pkg.Name || a.Package.Ecosystem != pkg.Ecosystem {
			continue
		}
		for _, c := range a.EcosystemSpecific.Components {
			if c.ComponentType == "apk" || c.ComponentName == "" {
				continue
			}
			ac := AdvisoryComponent{Name: c.ComponentName, Kind: componentKind(c.ComponentType)}
			if !slices.Contains(out, ac) {
				out = append(out, ac)
			}
		}
	}
	return out
}

// vulnAccumulator collects one vulnerability's packages and IDs.
type vulnAccumulator struct {
	ids  map[string]bool // upper-case record IDs, aliases and upstream IDs, for lookups
	also map[string]bool // IDs shown as aliases
	pkgs map[string]*AdvisoryPackage
}

// buildAdvisoryReport groups OSV results by vulnerability (vulnKeys, so it
// counts as the upstream comparison does, OA-2), statuses them (OA-3, OA-4),
// and totals them (OA-5). If id is set, the matching vulnerability is
// returned as Match (OA-9).
func buildAdvisoryReport(pkgs []Package, results map[OSVPackage][]OSVVuln, checked time.Time, id string) AdvisoryReport {
	lookups, notCovered := advisoryLookups(pkgs)
	groups := make(map[string]*vulnAccumulator)
	for _, lk := range lookups {
		for _, v := range results[lk] {
			open, fixed := openState(v, lk)
			if !open {
				continue
			}
			status := advisoryStatus(v, lk)
			comps := nonAPKComponents(v, lk)
			for _, key := range vulnKeys(v) {
				g := groups[key]
				if g == nil {
					g = &vulnAccumulator{ids: map[string]bool{}, also: map[string]bool{}, pkgs: map[string]*AdvisoryPackage{}}
					groups[key] = g
				}
				for _, x := range slices.Concat([]string{v.ID, key}, v.Upstream, v.Aliases) {
					g.ids[strings.ToUpper(x)] = true
				}
				for _, x := range slices.Concat(v.Upstream, v.Aliases) {
					if x != key && !strings.HasPrefix(x, "CGA-") {
						g.also[x] = true
					}
				}
				p := g.pkgs[lk.Name]
				if p == nil {
					p = &AdvisoryPackage{Name: lk.Name, Version: lk.Version, Status: status, Advisories: []string{}}
					g.pkgs[lk.Name] = p
				} else if advisoryRank[status] < advisoryRank[p.Status] {
					p.Status = status
				}
				if !slices.Contains(p.Advisories, v.ID) {
					p.Advisories = append(p.Advisories, v.ID)
				}
				if fixed != "" && p.FixedVersion == "" {
					p.FixedVersion = fixed
				}
				for _, c := range comps {
					if !slices.Contains(p.Components, c) {
						p.Components = append(p.Components, c)
					}
				}
			}
		}
	}

	report := AdvisoryReport{
		CheckedAt:       checked.UTC().Format(time.RFC3339),
		Source:          advisorySource + ", checked " + checked.UTC().Format("2006-01-02 15:04 UTC") + ".",
		Vulnerabilities: []OpenVulnerability{},
		NotCovered:      notCovered,
	}
	idUpper := strings.ToUpper(strings.TrimSpace(id))
	report.ID = idUpper
	ids := make(map[string]map[string]bool, len(groups))
	for key, g := range groups {
		ov := OpenVulnerability{ID: key, Status: AdvNotAssessed}
		for a := range g.also {
			ov.Aliases = append(ov.Aliases, a)
		}
		sort.Strings(ov.Aliases)
		names := make([]string, 0, len(g.pkgs))
		for n := range g.pkgs {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, n := range names {
			p := g.pkgs[n]
			sort.Strings(p.Advisories)
			ov.Packages = append(ov.Packages, *p)
			if advisoryRank[p.Status] < advisoryRank[ov.Status] {
				ov.Status = p.Status
			}
		}
		report.Vulnerabilities = append(report.Vulnerabilities, ov)
		ids[key] = g.ids
	}
	sort.Slice(report.Vulnerabilities, func(i, j int) bool {
		a, b := report.Vulnerabilities[i], report.Vulnerabilities[j]
		if advisoryRank[a.Status] != advisoryRank[b.Status] {
			return advisoryRank[a.Status] < advisoryRank[b.Status]
		}
		return a.ID < b.ID
	})
	for _, v := range report.Vulnerabilities {
		report.Total++
		switch v.Status {
		case AdvFixAvailable:
			report.Counts.FixAvailable++
		case AdvPendingUpstream:
			report.Counts.PendingUpstreamFix++
		case AdvFixNotPlanned:
			report.Counts.FixNotPlanned++
		case AdvNotAssessed:
			report.Counts.NotYetAssessed++
		default:
			report.Counts.Confirmed++
		}
	}

	if report.Total == 0 {
		report.Note = "No open advisories are published for this image's packages as of " + checked.UTC().Format("2006-01-02 15:04 UTC") + ". This covers Chainguard's published advisories only; it is not a full scan."
	} else if report.Counts.NotYetAssessed > 0 {
		report.Note = "Not yet assessed means detected but not yet assessed by Chainguard; some may turn out not to apply."
	}

	if idUpper != "" {
		for i := range report.Vulnerabilities {
			if ids[report.Vulnerabilities[i].ID][idUpper] {
				m := report.Vulnerabilities[i]
				report.Match = &m
				break
			}
		}
		if report.Match == nil {
			report.LookupNote = "No open advisory for " + idUpper + " is published for this image's packages as of " + checked.UTC().Format("2006-01-02 15:04 UTC") + ". This covers Chainguard's published advisories only; it is not a full scan."
		}
	}
	return report
}

type OpenAdvisoriesInput struct {
	Image string `json:"image" jsonschema:"image name as returned by list_images, e.g. 'python'"`
	Tag   string `json:"tag,omitempty" jsonschema:"tag to inspect (default 'latest')"`
	ID    string `json:"id,omitempty" jsonschema:"optional vulnerability ID (CVE, GHSA or CGA) to look up among the open advisories"`
}

// OpenAdvisories looks up the open advisories Chainguard has published for an
// image's linux/amd64 packages, through OSV.dev.
func (s *Service) OpenAdvisories(ctx context.Context, in OpenAdvisoriesInput) (*AdvisoryReport, error) {
	image, tag, err := imageAndTag(in.Image, in.Tag)
	if err != nil {
		return nil, err
	}
	pkgs, digest, err := s.Registry.Packages(ctx, image, tag, "amd64")
	if err != nil {
		return nil, notPublicHint(image, err)
	}
	return s.openAdvisories(ctx, image, tag, digest, pkgs, in.ID)
}

// openAdvisories is OpenAdvisories for packages that are already read, so a
// caller that also needs the SBOM for something else reads it once.
func (s *Service) openAdvisories(ctx context.Context, image, tag, digest string, pkgs []Package, id string) (*AdvisoryReport, error) {
	lookups, _ := advisoryLookups(pkgs)
	results, checked, err := s.OSV.QueryAllAt(ctx, lookups, nil)
	if err != nil {
		return nil, err
	}
	results, fromOSV, feedChecked, err := s.currentAdvisories(ctx, results)
	if err != nil {
		return nil, err
	}
	if !feedChecked.IsZero() && (checked.IsZero() || feedChecked.Before(checked)) {
		checked = feedChecked
	}
	if checked.IsZero() {
		checked = time.Now()
	}

	report := buildAdvisoryReport(pkgs, results, checked, id)
	report.Image, report.Tag, report.Digest = image, tag, digest
	if len(fromOSV) > 0 {
		report.StatusFromOSV = fromOSV
		report.Note = strings.TrimSpace(report.Note + " " + unconfirmedStatusNote(fromOSV))
	}
	return &report, nil
}

// currentAdvisories replaces the Chainguard advisories in OSV results by their
// current records from Chainguard's feed, since OSV.dev can serve older copies
// (OA-17, UC-17). It returns the IDs the feed has no record of, whose OSV.dev
// copy was kept, and the oldest time a record was fetched. Run it after the
// OSV step, so no more than osvMaxInFlight requests run at once.
func (s *Service) currentAdvisories(ctx context.Context, results map[OSVPackage][]OSVVuln) (map[OSVPackage][]OSVVuln, []string, time.Time, error) {
	var ids []string
	seen := make(map[string]bool)
	for _, vulns := range results {
		for _, v := range vulns {
			if advisoryID.MatchString(v.ID) && !seen[v.ID] {
				seen[v.ID] = true
				ids = append(ids, v.ID)
			}
		}
	}
	sort.Strings(ids)
	current, checked, err := s.Feed.FetchAll(ctx, ids)
	if err != nil {
		return nil, nil, time.Time{}, err
	}
	results, fromOSV := withCurrentRecords(results, current)
	return results, fromOSV, checked, nil
}

// unconfirmedStatusNote says which advisories' status came from OSV.dev
// because Chainguard's feed has no record of them.
func unconfirmedStatusNote(ids []string) string {
	return "Chainguard's advisory feed has no record of " + strings.Join(ids, ", ") + ", so OSV.dev's copy was used for the status of " + map[bool]string{true: "this advisory", false: "these advisories"}[len(ids) == 1] + ", which couldn't be confirmed."
}

// withCurrentRecords replaces each OSV.dev record by the feed's current one.
// A nil entry means the feed has no record (a 404): OSV.dev's copy stays, and
// the advisory's ID is returned, sorted, as one whose status is unconfirmed
// (OA-17.3). A current record with no entry for the package is not used for
// it: OSV.dev's copy stays, so its status, if any, is kept (OA-16.4). Records
// that aren't Chainguard advisories are kept as they are.
func withCurrentRecords(results map[OSVPackage][]OSVVuln, current map[string]*OSVVuln) (map[OSVPackage][]OSVVuln, []string) {
	out := make(map[OSVPackage][]OSVVuln, len(results))
	missing := make(map[string]bool)
	for pkg, vulns := range results {
		for _, v := range vulns {
			if cur, ok := current[v.ID]; ok {
				if cur == nil {
					missing[v.ID] = true
				} else if hasEntry(*cur, pkg) {
					v = *cur
				}
			}
			out[pkg] = append(out[pkg], v)
		}
	}
	ids := make([]string, 0, len(missing))
	for id := range missing {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return out, ids
}
