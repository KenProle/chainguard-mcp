package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// Service bundles the data sources behind both the MCP tools (tools.go) and
// the web API (web.go). Its methods validate input, apply defaults and
// return the same output types to both.
type Service struct {
	Catalog  *Catalog
	Registry *Registry
	SecDB    *SecDB
	// SBOMDir is where save_sbom writes files. The tool is only offered
	// when it's set.
	SBOMDir string
}

func NewService() *Service {
	client := &http.Client{Timeout: 30 * time.Second}
	return &Service{
		Catalog:  NewCatalog(client),
		Registry: NewRegistry(client),
		SecDB:    NewSecDB(client),
	}
}

// InputError reports invalid caller input, as opposed to an upstream failure.
type InputError struct{ Msg string }

func (e *InputError) Error() string { return e.Msg }

func inputErrorf(format string, args ...any) error {
	return &InputError{Msg: fmt.Sprintf(format, args...)}
}

var (
	validImage = regexp.MustCompile(`^[a-z0-9]+(?:[._-][a-z0-9]+)*$`)
	validTag   = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9._-]{0,127}$`)
	validArch  = regexp.MustCompile(`^[a-z0-9]{1,16}$`)
)

// imageAndTag validates an image name and tag and applies the default tag.
func imageAndTag(image, tag string) (string, string, error) {
	image = strings.TrimSpace(image)
	if !validImage.MatchString(image) {
		return "", "", inputErrorf("invalid image name %q: use a name from list_images, e.g. \"python\"", image)
	}
	tag = strings.TrimSpace(tag)
	if tag == "" {
		tag = "latest"
	}
	if !validTag.MatchString(tag) {
		return "", "", inputErrorf("invalid tag %q", tag)
	}
	return image, tag, nil
}

// normalizeArch validates a CPU architecture and defaults it to amd64.
func normalizeArch(arch string) (string, error) {
	arch = strings.TrimSpace(arch)
	if arch == "" {
		return "amd64", nil
	}
	if !validArch.MatchString(arch) {
		return "", inputErrorf("invalid arch %q: use amd64 or arm64", arch)
	}
	return arch, nil
}

// notPublicHint adds context to registry errors for an image.
func notPublicHint(image string, err error) error {
	if errors.Is(err, ErrNotPublic) {
		return fmt.Errorf("%s: %w. Only free-tier images can be inspected anonymously", image, err)
	}
	return fmt.Errorf("%s: %w", image, err)
}

type ListImagesInput struct {
	Query    string `json:"query,omitempty" jsonschema:"case-insensitive substring to filter image names, e.g. 'python' or 'fips'"`
	FreeOnly bool   `json:"free_only,omitempty" jsonschema:"only return free-tier images (the first unfiltered call checks every image and can take ~30s)"`
	Limit    int    `json:"limit,omitempty" jsonschema:"maximum number of images to return (default 100, max 1000)"`
	Offset   int    `json:"offset,omitempty" jsonschema:"number of matching images to skip, for pagination"`
}

type ImageSummary struct {
	Name      string `json:"name"`
	Reference string `json:"reference"`
	Free      *bool  `json:"free,omitempty" jsonschema:"true if pullable without a subscription (omitted if the check failed)"`
}

type ListImagesOutput struct {
	Total  int            `json:"total" jsonschema:"number of images matching the query"`
	Count  int            `json:"count" jsonschema:"number of images in this page"`
	Offset int            `json:"offset"`
	Images []ImageSummary `json:"images"`
}

// ListImages filters the catalog by name and free-tier status and returns
// one page, with free-tier status for each image on it.
func (s *Service) ListImages(ctx context.Context, in ListImagesInput) (ListImagesOutput, error) {
	all, err := s.Catalog.Images(ctx)
	if err != nil {
		return ListImagesOutput{}, err
	}
	q := strings.ToLower(strings.TrimSpace(in.Query))
	var matches []string
	for _, name := range all {
		if q == "" || strings.Contains(name, q) {
			matches = append(matches, name)
		}
	}

	var free map[string]bool
	if in.FreeOnly {
		free = s.Registry.PublicStatus(ctx, matches)
		var freeMatches []string
		for _, name := range matches {
			if free[name] {
				freeMatches = append(freeMatches, name)
			}
		}
		matches = freeMatches
	}

	limit := in.Limit
	if limit <= 0 {
		limit = 100
	}
	limit = min(limit, 1000)
	offset := min(max(in.Offset, 0), len(matches))
	page := matches[offset:min(offset+limit, len(matches))]
	if free == nil {
		free = s.Registry.PublicStatus(ctx, page)
	}

	images := make([]ImageSummary, 0, len(page))
	for _, name := range page {
		sum := ImageSummary{Name: name, Reference: s.Registry.Ref(name)}
		if f, ok := free[name]; ok {
			sum.Free = &f
		}
		images = append(images, sum)
	}
	return ListImagesOutput{Total: len(matches), Count: len(images), Offset: offset, Images: images}, nil
}

type ImageInput struct {
	Image string `json:"image" jsonschema:"image name as returned by list_images, e.g. 'python'"`
	Tag   string `json:"tag,omitempty" jsonschema:"tag to inspect (default 'latest')"`
}

type GetImageTagsInput struct {
	Image string `json:"image" jsonschema:"image name as returned by list_images, e.g. 'python'"`
}

type GetImageTagsOutput struct {
	Image     string   `json:"image"`
	Reference string   `json:"reference" jsonschema:"full registry reference for the image"`
	Tags      []string `json:"tags"`
}

func (s *Service) ImageTags(ctx context.Context, image string) (GetImageTagsOutput, error) {
	image, _, err := imageAndTag(image, "")
	if err != nil {
		return GetImageTagsOutput{}, err
	}
	tags, err := s.Registry.Tags(ctx, image)
	if err != nil {
		return GetImageTagsOutput{}, notPublicHint(image, err)
	}
	return GetImageTagsOutput{Image: image, Reference: s.Registry.Ref(image), Tags: tags}, nil
}

func (s *Service) ImageDetails(ctx context.Context, image, tag string) (*ImageDetails, error) {
	image, tag, err := imageAndTag(image, tag)
	if err != nil {
		return nil, err
	}
	d, err := s.Registry.Details(ctx, image, tag)
	if err != nil {
		return nil, notPublicHint(image, err)
	}
	return d, nil
}

func (s *Service) PinImage(ctx context.Context, image, tag string) (*PinResult, error) {
	image, tag, err := imageAndTag(image, tag)
	if err != nil {
		return nil, err
	}
	p, err := s.Registry.Pin(ctx, image, tag)
	if err != nil {
		return nil, notPublicHint(image, err)
	}
	return p, nil
}

type GetImagePackagesInput struct {
	Image string `json:"image" jsonschema:"image name as returned by list_images, e.g. 'python'"`
	Tag   string `json:"tag,omitempty" jsonschema:"tag to inspect (default 'latest')"`
	Arch  string `json:"arch,omitempty" jsonschema:"CPU architecture: 'amd64' (default) or 'arm64'"`
	Query string `json:"query,omitempty" jsonschema:"case-insensitive substring to filter package names, e.g. 'ssl'"`
}

type GetImagePackagesOutput struct {
	Image    string    `json:"image"`
	Tag      string    `json:"tag"`
	Arch     string    `json:"arch"`
	Digest   string    `json:"digest" jsonschema:"platform image digest the SBOM describes"`
	HasShell bool      `json:"has_shell" jsonschema:"whether a shell (busybox, bash or dash) is installed"`
	HasAPK   bool      `json:"has_apk" jsonschema:"whether the apk package manager is installed"`
	Total    int       `json:"total" jsonschema:"number of packages in the image"`
	Packages []Package `json:"packages" jsonschema:"packages matching the query (all if no query)"`
}

// ImagePackages lists the packages in an image's SBOM, optionally filtered
// by name or origin, and detects a shell and the apk package manager.
func (s *Service) ImagePackages(ctx context.Context, in GetImagePackagesInput) (GetImagePackagesOutput, error) {
	image, tag, err := imageAndTag(in.Image, in.Tag)
	if err != nil {
		return GetImagePackagesOutput{}, err
	}
	arch, err := normalizeArch(in.Arch)
	if err != nil {
		return GetImagePackagesOutput{}, err
	}
	pkgs, digest, err := s.Registry.Packages(ctx, image, tag, arch)
	if err != nil {
		return GetImagePackagesOutput{}, notPublicHint(image, err)
	}
	out := GetImagePackagesOutput{Image: image, Tag: tag, Arch: arch, Digest: digest, Total: len(pkgs), Packages: []Package{}}
	q := strings.ToLower(strings.TrimSpace(in.Query))
	for _, p := range pkgs {
		switch p.Name {
		case "busybox", "bash", "dash":
			out.HasShell = true
		case "apk-tools":
			out.HasAPK = true
		}
		if q == "" || strings.Contains(p.Name, q) || strings.Contains(p.Origin, q) {
			out.Packages = append(out.Packages, p)
		}
	}
	return out, nil
}

type SaveSBOMInput struct {
	Image     string `json:"image" jsonschema:"image name as returned by list_images, e.g. 'python'"`
	Tag       string `json:"tag,omitempty" jsonschema:"tag to export (default 'latest')"`
	Arch      string `json:"arch,omitempty" jsonschema:"CPU architecture: 'amd64' (default) or 'arm64'"`
	Filename  string `json:"filename,omitempty" jsonschema:"plain file name without folders (default '<image>-<tag>-<arch>.spdx.json')"`
	Overwrite bool   `json:"overwrite,omitempty" jsonschema:"replace the file if it already exists"`
}

// SaveSBOM writes an image's SPDX SBOM to SBOMDir.
func (s *Service) SaveSBOM(ctx context.Context, in SaveSBOMInput) (*SavedSBOM, error) {
	if s.SBOMDir == "" {
		return nil, errors.New("saving SBOMs is disabled")
	}
	image, tag, err := imageAndTag(in.Image, in.Tag)
	if err != nil {
		return nil, err
	}
	arch, err := normalizeArch(in.Arch)
	if err != nil {
		return nil, err
	}
	saved, err := s.Registry.SaveSBOM(ctx, s.SBOMDir, image, tag, arch, in.Filename, in.Overwrite)
	if err != nil {
		return nil, notPublicHint(image, err)
	}
	return saved, nil
}

// SBOM returns an image's SPDX SBOM document with a suggested file name,
// for streaming to a client rather than saving on the server.
func (s *Service) SBOM(ctx context.Context, image, tag, arch string) (json.RawMessage, string, error) {
	image, tag, err := imageAndTag(image, tag)
	if err != nil {
		return nil, "", err
	}
	arch, err = normalizeArch(arch)
	if err != nil {
		return nil, "", err
	}
	doc, _, err := s.Registry.SBOMDocument(ctx, image, tag, arch)
	if err != nil {
		return nil, "", notPublicHint(image, err)
	}
	name, err := sbomFilename("", image, tag, arch)
	if err != nil {
		return nil, "", err
	}
	return doc, name, nil
}

type CheckVulnerabilitiesInput struct {
	Image string `json:"image" jsonschema:"image name as returned by list_images, e.g. 'python'"`
	Tag   string `json:"tag,omitempty" jsonschema:"tag to inspect (default 'latest')"`
	ID    string `json:"id,omitempty" jsonschema:"optional vulnerability ID to look up, e.g. 'CVE-2024-12797' or a GHSA ID; omit for a per-package summary"`
}

// CheckVulnerabilities analyzes an image's linux/amd64 packages against the
// Wolfi security database.
func (s *Service) CheckVulnerabilities(ctx context.Context, in CheckVulnerabilitiesInput) (VulnReport, error) {
	image, tag, err := imageAndTag(in.Image, in.Tag)
	if err != nil {
		return VulnReport{}, err
	}
	pkgs, digest, err := s.Registry.Packages(ctx, image, tag, "amd64")
	if err != nil {
		return VulnReport{}, notPublicHint(image, err)
	}
	fixes, err := s.SecDB.Fixes(ctx)
	if err != nil {
		return VulnReport{}, err
	}
	report := analyzeVulns(pkgs, fixes, in.ID)
	report.Image, report.Tag, report.Digest = image, tag, digest
	return report, nil
}

type FindAlternativeInput struct {
	Image string `json:"image" jsonschema:"upstream image reference, e.g. 'node:20-alpine', 'docker.io/library/python:3.12-slim' or 'mcr.microsoft.com/dotnet/aspnet:8.0'"`
}

// FindAlternative validates an upstream reference and suggests replacements.
func (s *Service) FindAlternative(ctx context.Context, ref string) (*AlternativesResult, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, inputErrorf("image is required")
	}
	if len(ref) > 512 {
		return nil, inputErrorf("image reference is too long")
	}
	return s.FindAlternatives(ctx, ref)
}
