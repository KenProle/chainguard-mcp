package main

import (
	"context"
	"regexp"
	"sort"
	"strings"
	"time"
)

const maxMapNames = 100

// Statuses of a mapped package name.
const (
	mapExact      = "exact"
	mapMapped     = "mapped"
	mapCandidates = "candidates"
	mapUnknown    = "unknown"
)

type MapPackagesInput struct {
	Names []string `json:"names" jsonschema:"apt (Debian/Ubuntu) package names, e.g. ['build-essential','libpq-dev']; at most 100"`
}

type PackageMappingResult struct {
	Results       []PackageMapping `json:"results"`
	IndexFetched  time.Time        `json:"index_fetched" jsonschema:"when Wolfi's package index was fetched; older than a few hours means a refresh failed"`
	MappingCommit string           `json:"mapping_commit" jsonschema:"upstream commit of the embedded dfc mapping file"`
}

type PackageMapping struct {
	Name         string   `json:"name" jsonschema:"the apt package name asked for"`
	Status       string   `json:"status" jsonschema:"exact (same name in Wolfi), mapped (from dfc), candidates (found by a rule; check them) or unknown"`
	Packages     []string `json:"packages" jsonschema:"Wolfi packages to apk add; empty for unknown, or when dfc lists no replacement"`
	Alternatives []string `json:"alternatives,omitempty" jsonschema:"other matching Wolfi packages, such as other version streams"`
	Source       string   `json:"source,omitempty" jsonschema:"where the answer came from"`
	Note         string   `json:"note,omitempty" jsonschema:"set when a choice was made, such as a version stream"`
}

var validAptName = regexp.MustCompile(`^[a-z0-9][a-z0-9+.-]+$`)

// validateMapNames checks the request and returns the distinct names in order.
func validateMapNames(names []string) ([]string, error) {
	if len(names) == 0 {
		return nil, inputErrorf("names is required: give at least one apt package name")
	}
	if len(names) > maxMapNames {
		return nil, inputErrorf("too many names: %d given, at most %d", len(names), maxMapNames)
	}
	seen := map[string]bool{}
	var out []string
	for _, n := range names {
		n = strings.TrimSpace(n)
		if !validAptName.MatchString(n) {
			return nil, inputErrorf("invalid package name %q: use a Debian package name such as \"libpq-dev\"", n)
		}
		if !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	return out, nil
}

// MapPackages maps apt package names to the Wolfi packages that replace them.
// Each answer is checked against Wolfi's live package index.
func (s *Service) MapPackages(ctx context.Context, in MapPackagesInput) (*PackageMappingResult, error) {
	names, err := validateMapNames(in.Names)
	if err != nil {
		return nil, err
	}
	idx, fetched, err := s.Packages.Get(ctx)
	if err != nil {
		return nil, err
	}
	out := &PackageMappingResult{Results: make([]PackageMapping, 0, len(names)), IndexFetched: fetched, MappingCommit: s.DFC.Source.Commit}
	for _, n := range names {
		out.Results = append(out.Results, mapPackage(idx, s.DFC.Debian, n))
	}
	return out, nil
}

// mapPackage resolves one name: the name itself, its dfc mapping, then the rules.
func mapPackage(idx *IndexData, dfc map[string][]string, name string) PackageMapping {
	if _, ok := idx.Pkgs[name]; ok {
		return PackageMapping{Name: name, Status: mapExact, Packages: []string{name}, Source: packageIndexSource}
	}
	if targets, ok := dfc[name]; ok {
		if len(targets) == 0 {
			return PackageMapping{Name: name, Status: mapMapped, Packages: []string{}, Source: "dfc", Note: "dfc lists no replacement; this package may not be needed"}
		}
		if pkgs, corrections := dfcTargets(idx, name, targets); len(pkgs) > 0 {
			source := "dfc"
			if len(corrections) > 0 {
				source = "dfc, corrected to the -dev package (" + strings.Join(corrections, ", ") + ")"
			}
			return PackageMapping{Name: name, Status: mapMapped, Packages: pkgs, Source: source}
		}
	}
	if m, ok := applyRules(idx, name); ok {
		return m
	}
	return PackageMapping{Name: name, Status: mapUnknown, Packages: []string{}}
}

// dfcTargets keeps dfc's targets that are in the index (PM-2). For a -dev
// name, a target that isn't a -dev package becomes its -dev package, or its
// origin's, when the index has one (PM-5): libssl-dev → libssl3 → openssl-dev.
func dfcTargets(idx *IndexData, name string, targets []string) (pkgs, corrections []string) {
	seen := map[string]bool{}
	add := func(p string) {
		if !seen[p] {
			seen[p] = true
			pkgs = append(pkgs, p)
		}
	}
	for _, t := range targets {
		if strings.HasSuffix(name, "-dev") && !strings.HasSuffix(t, "-dev") {
			cands := []string{t + "-dev"}
			if info, ok := idx.Pkgs[t]; ok && info.Origin != "" {
				cands = append(cands, info.Origin+"-dev")
			}
			corrected := false
			for _, c := range cands {
				if _, ok := idx.Pkgs[c]; ok {
					add(c)
					corrections = append(corrections, t+" → "+c)
					corrected = true
					break
				}
			}
			if corrected {
				continue
			}
		}
		if _, ok := idx.Pkgs[t]; ok {
			add(t)
		}
	}
	return pkgs, corrections
}

// match is a package found by a rule.
type match struct {
	pkg    string
	source string
}

var (
	libNumRe    = regexp.MustCompile(`^lib(.*[^0-9])(\d+)$`)
	trailDigits = regexp.MustCompile(`\d+$`)
	streamRe    = regexp.MustCompile(`^(.*?)-(\d+(?:\.\d+)*)(-.+)?$`)
	prereleases = []string{"_alpha", "_beta", "_pre", "_rc"}
	verSegRe    = regexp.MustCompile(`^\d+(?:\.\d+)*$`)
)

// applyRules looks for packages that provide what the apt package is named
// for. The first rule with a hit decides; a name starting with default- is
// tried again without it.
func applyRules(idx *IndexData, name string) (PackageMapping, bool) {
	forms := []struct {
		name, suffix string
	}{{name, ""}}
	if rest := strings.TrimPrefix(name, "default-"); rest != name && len(rest) > 1 {
		forms = append(forms, struct{ name, suffix string }{rest, " (without default-)"})
	}
	for _, f := range forms {
		for _, rule := range ruleFuncs {
			if hits := rule(idx, f.name); len(hits) > 0 {
				return chooseCandidates(idx, name, hits, f.suffix), true
			}
		}
	}
	return PackageMapping{}, false
}

var ruleFuncs = []func(*IndexData, string) []match{
	ruleVirtual, rulePython, ruleDev, ruleLibrary, ruleCommand, ruleStreams,
}

func fromProviders(names []string, source string) []match {
	var out []match
	for _, n := range names {
		out = append(out, match{n, source})
	}
	return out
}

func ruleVirtual(idx *IndexData, name string) []match {
	if hits := fromProviders(idx.Providers(name), "virtual package "+name); len(hits) > 0 {
		return hits
	}
	// golang-go: the virtual package of the last word (go), not a package
	// that merely has a command of that name.
	if i := strings.LastIndex(name, "-"); i > 0 && i < len(name)-1 {
		last := name[i+1:]
		return fromProviders(idx.Providers(last), "virtual package "+last)
	}
	return nil
}

func rulePython(idx *IndexData, name string) []match {
	if x, ok := strings.CutPrefix(name, "python3-"); ok {
		if _, found := idx.Pkgs["py3-"+x]; found {
			return []match{{"py3-" + x, "python3-" + x + " → py3-" + x + " naming"}}
		}
	}
	return nil
}

// ruleDev handles lib<x>-dev: packages named <x>-dev or lib<x>-dev, then
// packages that provide the pkg-config module <x> or lib<x>.
func ruleDev(idx *IndexData, name string) []match {
	base, ok := strings.CutSuffix(name, "-dev")
	if !ok || base == "" {
		return nil
	}
	x := strings.TrimPrefix(base, "lib")
	xs := []string{x}
	if trimmed := trailDigits.ReplaceAllString(x, ""); trimmed != x && trimmed != "" {
		xs = append(xs, trimmed)
	}
	var out []match
	for _, v := range xs {
		for _, n := range []string{v + "-dev", "lib" + v + "-dev"} {
			if _, found := idx.Pkgs[n]; found && n != name {
				out = append(out, match{n, "package " + n})
			}
		}
	}
	if len(out) > 0 {
		return out
	}
	for _, v := range xs {
		for _, mod := range []string{v, "lib" + v} {
			out = append(out, fromProviders(idx.Providers("pc:"+mod), "pkg-config module "+mod)...)
		}
		if len(out) > 0 {
			return out
		}
	}
	return nil
}

// ruleLibrary handles lib<x><N>: the shared library lib<x>.so.<N>, then
// packages named lib<x> or <x>.
func ruleLibrary(idx *IndexData, name string) []match {
	m := libNumRe.FindStringSubmatch(name)
	if m == nil {
		return nil
	}
	lib := "lib" + m[1] + ".so." + m[2]
	if hits := fromProviders(idx.ProvidersFold("so:"+lib), "library "+lib); len(hits) > 0 {
		return hits
	}
	for _, n := range []string{"lib" + m[1], m[1]} {
		if _, found := idx.Pkgs[n]; found {
			return []match{{n, "package " + n}}
		}
	}
	return nil
}

// ruleCommand looks for a package providing a command named like the
// package, or like its last hyphen-separated word.
func ruleCommand(idx *IndexData, name string) []match {
	cmds := []string{name}
	if i := strings.LastIndex(name, "-"); i > 0 {
		cmds = append(cmds, name[i+1:])
	}
	for _, c := range cmds {
		if hits := fromProviders(idx.Providers("cmd:"+c), "command "+c); len(hits) > 0 {
			return hits
		}
	}
	return nil
}

// ruleStreams finds version streams: <base>-<version>[-<suffix>], where the
// name is <base>[-<suffix>] (nodejs → nodejs-24, postgresql-client →
// postgresql-18-client).
func ruleStreams(idx *IndexData, name string) []match {
	segs := strings.Split(name, "-")
	var out []match
	for k := len(segs); k >= 1; k-- {
		base, suffix := strings.Join(segs[:k], "-"), strings.Join(segs[k:], "-")
		for pkg := range idx.Pkgs {
			rest, ok := strings.CutPrefix(pkg, base+"-")
			if !ok {
				continue
			}
			ver, tail, _ := strings.Cut(rest, "-")
			if !verSegRe.MatchString(ver) || tail != suffix {
				continue
			}
			out = append(out, match{pkg, "version streams of " + name})
		}
		if len(out) > 0 {
			break
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].pkg < out[j].pkg })
	return out
}

type streamKey struct{ prefix, suffix string }

// streamOf splits a package name of the form <prefix>-<version>[-<suffix>].
func streamOf(pkg string) (key streamKey, ver string, ok bool) {
	m := streamRe.FindStringSubmatch(pkg)
	if m == nil {
		return streamKey{}, "", false
	}
	return streamKey{m[1], m[3]}, m[2], true
}

// isPrerelease reports whether version is a prerelease: _alpha, _beta, _pre or
// _rc, or a _git snapshot toward a first release (3.15.0_git20261017). A _git
// snapshot of a later patch (3.14.8_git20261014) is a build of a stable release
// stream, as Wolfi packages python-3.13 and python-3.14 this way.
func isPrerelease(version string) bool {
	for _, p := range prereleases {
		if strings.Contains(version, p) {
			return true
		}
	}
	if v, _, ok := strings.Cut(version, "_git"); ok {
		return strings.HasSuffix(v, ".0") || !strings.Contains(v, ".")
	}
	return false
}

const activeBuildWindow = 180 * 24 * time.Hour

// chooseCandidates picks the package to install among a rule's matches and
// lists the rest as alternatives (design decision 4).
func chooseCandidates(idx *IndexData, name string, hits []match, sourceSuffix string) PackageMapping {
	source := map[string]string{}
	var pkgs []string
	for _, h := range hits {
		if _, dup := source[h.pkg]; !dup {
			source[h.pkg] = h.source
			pkgs = append(pkgs, h.pkg)
		}
	}
	// 1. The right kind of package.
	pool := pkgs
	if strings.HasSuffix(name, "-dev") {
		pool = filterOr(pool, func(p string) bool { return strings.HasSuffix(p, "-dev") })
	} else {
		pool = filterOr(pool, func(p string) bool { return !strings.HasSuffix(p, "-minimal") })
	}
	// 2. Streams that can't be chosen: prereleases and ones no longer built.
	var newest int64
	for _, p := range pool {
		newest = max(newest, idx.Pkgs[p].Built)
	}
	cutoff := newest - int64(activeBuildWindow/time.Second)
	eligible := filterOr(pool, func(p string) bool {
		info := idx.Pkgs[p]
		return !isPrerelease(info.Version) && (newest == 0 || info.Built >= cutoff)
	})

	// 3 and 4. Rank: streams of one family by version, otherwise newest build.
	rank := func(list []string) { rankPackages(idx, list) }
	sort.Strings(eligible)
	rank(eligible)
	chosen := eligible[0]

	rest := make([]string, 0, len(pkgs))
	for _, p := range pkgs {
		if p != chosen {
			rest = append(rest, p)
		}
	}
	sort.Strings(rest)
	rank(rest)
	if len(rest) > 5 {
		rest = rest[:5]
	}

	m := PackageMapping{
		Name: name, Status: mapCandidates, Packages: []string{chosen}, Alternatives: rest,
		Source: source[chosen] + sourceSuffix,
	}
	if key, _, ok := streamOf(chosen); ok {
		for _, p := range pkgs {
			if k, _, ok := streamOf(p); ok && p != chosen && k == key {
				m.Note = "Chose " + chosen + " from several version streams; it should match the version your Dockerfile used (see alternatives)."
				break
			}
		}
	}
	return m
}

// filterOr returns the items satisfying keep, or all of them if none do.
func filterOr(list []string, keep func(string) bool) []string {
	var out []string
	for _, s := range list {
		if keep(s) {
			out = append(out, s)
		}
	}
	if len(out) == 0 {
		return list
	}
	return out
}

// rankPackages orders matches best first. If they are all streams of one
// family (python-3.13, python-3.14) they go by version, highest first;
// otherwise by build time, then version, then name.
func rankPackages(idx *IndexData, list []string) {
	sort.Strings(list)
	var key streamKey
	oneFamily := len(list) > 1
	for i, p := range list {
		k, _, ok := streamOf(p)
		if !ok || (i > 0 && k != key) {
			oneFamily = false
			break
		}
		key = k
	}
	if oneFamily {
		sort.SliceStable(list, func(i, j int) bool {
			_, vi, _ := streamOf(list[i])
			_, vj, _ := streamOf(list[j])
			return compareAPKVersions(vi, vj) > 0
		})
		return
	}
	sort.SliceStable(list, func(i, j int) bool {
		a, b := idx.Pkgs[list[i]], idx.Pkgs[list[j]]
		if a.Built != b.Built {
			return a.Built > b.Built
		}
		return compareAPKVersions(a.Version, b.Version) > 0
	})
}
