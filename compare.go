package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"
)

// comparisonSource is the wording that accompanies every vulnerability count
// (UC-6): the counts are known vulnerabilities per each side's own
// maintainers' advisories, not a verdict on either image. Chainguard's statuses
// come from its own feed, since OSV.dev can serve older copies (UC-17).
const comparisonSource = "Known vulnerabilities per each distribution's published advisories, via OSV.dev, with the status of Chainguard's advisories from Chainguard's advisory feed. " +
	"Each image is graded by its own maintainers' advisories, and the counts are known vulnerabilities only, not a guarantee about either image."

type CompareUpstreamInput struct {
	Image string `json:"image" jsonschema:"upstream image reference on Docker Hub exactly as you run it, e.g. 'node:20', 'python:3.12-slim' or 'docker.io/library/nginx:1.27'"`
	Arch  string `json:"arch,omitempty" jsonschema:"CPU architecture: 'amd64' (default) or 'arm64'"`
}

// LanguageCount is a number of language packages (npm, pypi, …) in an image.
type LanguageCount struct {
	Ecosystem string `json:"ecosystem" jsonschema:"package type, e.g. npm or pypi"`
	Count     int    `json:"count"`
}

// UpstreamSide is one image in a comparison.
type UpstreamSide struct {
	Reference string `json:"reference" jsonschema:"the image as compared, e.g. docker.io/library/node:20"`
	Digest    string `json:"digest" jsonschema:"digest of the platform image that was analyzed"`
	Platform  string `json:"platform" jsonschema:"os/architecture, e.g. linux/amd64"`
	Created   string `json:"created,omitempty" jsonschema:"build date from the image configuration"`
	SizeBytes int64  `json:"size_bytes" jsonschema:"compressed download size in bytes"`
	// SBOMPublished is false for upstream images that publish no SBOM
	// attestation; their package and vulnerability figures are then absent.
	SBOMPublished bool `json:"sbom_published"`
	// PackagesChecked is true only when the image's OS packages could be
	// looked up. Otherwise UncheckedReason says why, and there are no
	// vulnerability counts: an image with nothing checked must never read as
	// having 0 known vulnerabilities (UC-15).
	PackagesChecked  bool            `json:"packages_checked" jsonschema:"whether the image's OS packages could be checked for known vulnerabilities"`
	UncheckedReason  string          `json:"unchecked_reason,omitempty" jsonschema:"why the packages weren't checked: 'no_sbom', 'no_os_packages' or 'unsupported_distribution'"`
	OSPackages       int             `json:"os_packages" jsonschema:"installed OS packages; for Debian images, binary packages rather than the source packages the SBOM also lists"`
	LanguagePackages []LanguageCount `json:"language_packages,omitempty" jsonschema:"language packages in the image; counted but not compared"`
	Vulnerabilities  *VulnCounts     `json:"vulnerabilities,omitempty" jsonschema:"known vulnerabilities in the OS packages, absent when the packages weren't checked"`
}

// Why an image's packages weren't checked (UC-2).
const (
	uncheckedNoSBOM      = "no_sbom"
	uncheckedNoOSPackage = "no_os_packages"
	uncheckedUnsupported = "unsupported_distribution"
)

// UpstreamComparison compares an upstream image with its Chainguard
// replacement.
type UpstreamComparison struct {
	Input      string         `json:"input" jsonschema:"the reference as entered"`
	Upstream   UpstreamSide   `json:"upstream"`
	Chainguard UpstreamSide   `json:"chainguard"`
	EndOfLife  *EndOfLifeNote `json:"end_of_life,omitempty" jsonschema:"set when the upstream release has passed its end-of-life date"`
	Source     string         `json:"source" jsonschema:"where the vulnerability counts come from and how to read them"`
	CheckedAt  string         `json:"checked_at" jsonschema:"when the comparison was computed, RFC 3339"`
	Notes      []string       `json:"notes"`
}

// comparisonCache holds package lists by platform image digest (UC-10): the
// same digest always has the same packages.
type comparisonCache struct {
	mu         sync.Mutex
	upstream   map[string]*UpstreamPackages // nil value: no SBOM published
	chainguard map[string][]Package
}

const maxCachedPackageLists = 64

func (c *comparisonCache) upstreamPackages(digest string) (*UpstreamPackages, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	p, ok := c.upstream[digest]
	return p, ok
}

func (c *comparisonCache) putUpstream(digest string, p *UpstreamPackages) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.upstream == nil || len(c.upstream) >= maxCachedPackageLists {
		c.upstream = make(map[string]*UpstreamPackages)
	}
	c.upstream[digest] = p
}

func (c *comparisonCache) chainguardPackages(digest string) ([]Package, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	p, ok := c.chainguard[digest]
	return p, ok
}

func (c *comparisonCache) putChainguard(digest string, p []Package) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.chainguard == nil || len(c.chainguard) >= maxCachedPackageLists {
		c.chainguard = make(map[string][]Package)
	}
	c.chainguard[digest] = p
}

// CompareUpstream compares an upstream Docker Hub image, exactly as entered,
// with the latest tag of its recommended Chainguard replacement on one
// architecture. It blocks until done; the web API runs it as a background job.
func (s *Service) CompareUpstream(ctx context.Context, in CompareUpstreamInput) (*UpstreamComparison, error) {
	return s.compareUpstream(ctx, in, nil)
}

// Steps a running comparison reports (UC-9).
const (
	// compareStepPackages: looking up each package on OSV.dev, done of total.
	compareStepPackages = "packages"
	// compareStepAdvisories: every package is looked up, and the Chainguard
	// advisories' current status is being read from Chainguard's feed.
	compareStepAdvisories = "advisories"
)

// compareUpstream is CompareUpstream that reports progress: the step, and
// done of total packages through the vulnerability lookups, the slow part.
func (s *Service) compareUpstream(ctx context.Context, in CompareUpstreamInput, progress func(step string, done, total int)) (*UpstreamComparison, error) {
	ref := strings.TrimSpace(in.Image)
	repo, tag, arch, err := validateCompareInput(in)
	if err != nil {
		return nil, err
	}
	alt, err := s.FindAlternatives(ctx, ref)
	if err != nil {
		return nil, err
	}
	if alt.Recommended == nil {
		return nil, fmt.Errorf("%s: %w", ref, ErrNoAlternative)
	}

	up, upPkgs, err := s.upstreamSide(ctx, repo, tag, arch)
	if err != nil {
		return nil, err
	}
	cg, cgPkgs, err := s.chainguardSide(ctx, alt.Recommended.Image, arch)
	if err != nil {
		return nil, err
	}

	var upLookups, cgLookups []OSVPackage
	if up.PackagesChecked {
		upLookups = upPkgs.Lookups
	}
	seen := make(map[OSVPackage]bool)
	for _, p := range cgPkgs {
		ecosystem, ok := osvEcosystemForDistro(p.Distro)
		if !ok {
			return nil, fmt.Errorf("%s: package %s is from %q, which has no vulnerability data here", alt.Recommended.Image, p.Name, p.Distro)
		}
		lookup := OSVPackage{Ecosystem: ecosystem, Name: p.Name, Version: p.Version}
		if !seen[lookup] {
			seen[lookup] = true
			cgLookups = append(cgLookups, lookup)
		}
	}
	lookups := slices.Concat(upLookups, cgLookups)
	var packagesDone func(done, total int)
	if progress != nil {
		packagesDone = func(done, total int) { progress(compareStepPackages, done, total) }
	}
	results, err := s.OSV.QueryAll(ctx, lookups, packagesDone)
	if err != nil {
		return nil, err
	}
	if progress != nil {
		progress(compareStepAdvisories, len(lookups), len(lookups))
	}
	if up.PackagesChecked {
		c := summarize(subset(results, upLookups))
		up.Vulnerabilities = &c
	}
	cgResults, fromOSV, err := s.chainguardOpenAdvisories(ctx, subset(results, cgLookups))
	if err != nil {
		return nil, err
	}
	c := summarize(cgResults)
	cg.Vulnerabilities = &c

	cmp := &UpstreamComparison{
		Input:      ref,
		Upstream:   up,
		Chainguard: cg,
		EndOfLife:  s.EOL.Note(ctx, repo, tag),
		Source:     comparisonSource,
		CheckedAt:  time.Now().UTC().Format(time.RFC3339),
		Notes: []string{
			"Only installed OS packages are compared; language packages (npm, PyPI, …) are counted but not compared.",
			"Advisories for the Linux kernel are excluded: containers ship only its headers and run on the host's kernel.",
		},
	}
	if len(fromOSV) > 0 {
		cmp.Notes = append(cmp.Notes, unconfirmedStatusNote(fromOSV))
	}
	switch up.UncheckedReason {
	case uncheckedNoSBOM:
		cmp.Notes = append(cmp.Notes, "The upstream image publishes no SBOM, so its packages and vulnerabilities can't be listed.")
	case uncheckedNoOSPackage:
		cmp.Notes = append(cmp.Notes, "The upstream image's SBOM lists no distribution packages (it may be statically built), so its known vulnerabilities can't be checked. That is not the same as having none.")
	case uncheckedUnsupported:
		cmp.Notes = append(cmp.Notes, "The upstream image's packages come from a distribution with no vulnerability data here ("+strings.Join(upPkgs.Unsupported, ", ")+"), so its known vulnerabilities can't be checked. That is not the same as having none.")
	}
	return cmp, nil
}

// validateCompareInput checks the request without any network access.
func validateCompareInput(in CompareUpstreamInput) (repo, tag, arch string, err error) {
	ref := strings.TrimSpace(in.Image)
	if ref == "" {
		return "", "", "", inputErrorf("image is required")
	}
	if len(ref) > 512 {
		return "", "", "", inputErrorf("image reference is too long")
	}
	if repo, tag, err = parseDockerHubRef(ref); err != nil {
		return "", "", "", err
	}
	if arch, err = normalizeArch(in.Arch); err != nil {
		return "", "", "", err
	}
	return repo, tag, arch, nil
}

// chainguardOpenAdvisories reads the Chainguard image's advisories from
// Chainguard's feed (UC-17) and keeps only those still open for the installed
// version (UC-18). It also returns the advisories the feed has no record of,
// whose OSV.dev copy was used. The upstream side never goes through it.
func (s *Service) chainguardOpenAdvisories(ctx context.Context, results map[OSVPackage][]OSVVuln) (map[OSVPackage][]OSVVuln, []string, error) {
	current, fromOSV, _, err := s.currentAdvisories(ctx, results)
	if err != nil {
		return nil, nil, err
	}
	open := make(map[OSVPackage][]OSVVuln, len(current))
	for pkg, vulns := range current {
		open[pkg] = nil
		for _, v := range vulns {
			if ok, _ := openState(v, pkg); ok {
				open[pkg] = append(open[pkg], v)
			}
		}
	}
	return open, fromOSV, nil
}

func subset(results map[OSVPackage][]OSVVuln, pkgs []OSVPackage) map[OSVPackage][]OSVVuln {
	out := make(map[OSVPackage][]OSVVuln, len(pkgs))
	for _, p := range pkgs {
		out[p] = results[p]
	}
	return out
}

// dockerError labels a Docker Hub failure. A rate limit stays recognizable;
// a missing or private repository reads as not found.
func dockerError(ref string, err error) error {
	switch {
	case errors.Is(err, ErrRateLimited):
		return fmt.Errorf("Docker Hub: %w", err)
	case errors.Is(err, ErrNotFound), errors.Is(err, ErrNotPublic):
		return fmt.Errorf("%s was not found on Docker Hub (or is private): %w", ref, ErrNotFound)
	}
	return &SourceError{Source: "Docker Hub", Err: err}
}

// buildInfo returns an image's build date and compressed size from its
// platform manifest and configuration.
func (r *Registry) buildInfo(ctx context.Context, image string, m *Manifest) (created string, size int64, err error) {
	size = m.Config.Size
	for _, l := range m.Layers {
		size += l.Size
	}
	raw, err := r.Blob(ctx, image, m.Config.Digest, maxConfigBytes)
	if err != nil {
		return "", 0, err
	}
	var cfg struct {
		Created string `json:"created"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return "", 0, fmt.Errorf("decoding image config: %w", err)
	}
	return cfg.Created, size, nil
}

func languageCounts(m map[string]int) []LanguageCount {
	var out []LanguageCount
	for eco, n := range m {
		out = append(out, LanguageCount{Ecosystem: eco, Count: n})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Ecosystem < out[j].Ecosystem })
	return out
}

// upstreamSide describes the Docker Hub image. The returned packages are nil
// when the image publishes no SBOM attestation.
func (s *Service) upstreamSide(ctx context.Context, repo, tag, arch string) (UpstreamSide, *UpstreamPackages, error) {
	display := "docker.io/" + repo + ":" + tag
	if strings.HasPrefix(tag, "sha256:") {
		display = "docker.io/" + repo + "@" + tag
	}
	m, digest, _, top, err := s.Upstream.resolvePlatform(ctx, repo, tag, arch)
	if err != nil {
		return UpstreamSide{}, nil, dockerError(display, err)
	}
	created, size, err := s.Upstream.buildInfo(ctx, repo, m)
	if err != nil {
		return UpstreamSide{}, nil, dockerError(display, err)
	}
	side := UpstreamSide{Reference: display, Digest: digest, Platform: "linux/" + arch, Created: created, SizeBytes: size}

	pkgs, cached := s.cache.upstreamPackages(digest)
	if !cached {
		doc, err := s.Upstream.sbomForPlatform(ctx, repo, tag, arch, top, digest)
		switch {
		case errors.Is(err, ErrNoSBOM):
			pkgs = nil
		case err != nil:
			return UpstreamSide{}, nil, dockerError(display, err)
		default:
			if pkgs, err = parseUpstreamPackages(doc); err != nil {
				return UpstreamSide{}, nil, fmt.Errorf("%s: %w", display, err)
			}
		}
		s.cache.putUpstream(digest, pkgs)
	}
	switch {
	case pkgs == nil:
		side.UncheckedReason = uncheckedNoSBOM
	default:
		side.SBOMPublished = true
		side.LanguagePackages = languageCounts(pkgs.Language)
		switch {
		case len(pkgs.Unsupported) > 0:
			// A partial check would undercount, so check nothing.
			side.UncheckedReason = uncheckedUnsupported
		case pkgs.Installed == 0:
			side.UncheckedReason = uncheckedNoOSPackage
		default:
			side.PackagesChecked = true
			side.OSPackages = pkgs.Installed
		}
	}
	return side, pkgs, nil
}

// chainguardSide describes the Chainguard image's latest tag and returns its
// installed packages.
func (s *Service) chainguardSide(ctx context.Context, image, arch string) (UpstreamSide, []Package, error) {
	const tag = "latest"
	d, err := s.Registry.Details(ctx, image, tag)
	if err != nil {
		return UpstreamSide{}, nil, notPublicHint(image, err)
	}
	platform := "linux/" + arch
	side := UpstreamSide{Reference: s.Registry.Ref(image) + ":" + tag, Platform: platform, Created: d.Created, SBOMPublished: true, PackagesChecked: true}
	for _, p := range d.Platforms {
		if p.Platform == platform {
			side.Digest, side.SizeBytes = p.Digest, p.SizeBytes
		}
	}
	if side.Digest == "" {
		return UpstreamSide{}, nil, notPublicHint(image, fmt.Errorf("%s:%s has no %s variant: %w", image, tag, arch, ErrNotFound))
	}

	pkgs, cached := s.cache.chainguardPackages(side.Digest)
	if !cached {
		_, digest, _, top, err := s.Registry.resolvePlatform(ctx, image, tag, arch)
		if err != nil {
			return UpstreamSide{}, nil, notPublicHint(image, err)
		}
		doc, err := s.Registry.sbomForPlatform(ctx, image, tag, arch, top, digest)
		if err != nil {
			return UpstreamSide{}, nil, notPublicHint(image, err)
		}
		if pkgs, err = parseSBOMPackages(doc); err != nil {
			return UpstreamSide{}, nil, err
		}
		s.cache.putChainguard(side.Digest, pkgs)
	}
	side.OSPackages = len(pkgs)
	return side, pkgs, nil
}
