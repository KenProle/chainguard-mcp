package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	predicateSPDX = "https://spdx.dev/Document"
	// maxSBOMBytes caps the SPDX attestation; typical images are well under 1 MB.
	maxSBOMBytes = 32 << 20
)

// Package is an OS package installed in an image, from its SBOM.
type Package struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	// Origin is the source package that produced this one, e.g. libssl3 comes
	// from openssl. Security advisories are tracked per origin.
	Origin  string `json:"origin" jsonschema:"source package this was built from; advisories are tracked per origin"`
	License string `json:"license,omitempty"`
	// Distro is the purl namespace, e.g. "wolfi" or "chainguard".
	Distro string `json:"distro"`
}

// Packages returns the OS packages in an image's SBOM, sorted by name, and
// the platform image digest the SBOM describes.
func (r *Registry) Packages(ctx context.Context, image, tag, arch string) ([]Package, string, error) {
	doc, digest, err := r.SBOMDocument(ctx, image, tag, arch)
	if err != nil {
		return nil, "", err
	}
	pkgs, err := parseSBOMPackages(doc)
	if err != nil {
		return nil, "", err
	}
	return pkgs, digest, nil
}

// SBOMDocument fetches the SPDX SBOM that Chainguard attaches (as a signed
// in-toto attestation) to each platform image. It returns the raw SPDX JSON
// document and the platform image digest it describes.
func (r *Registry) SBOMDocument(ctx context.Context, image, tag, arch string) (json.RawMessage, string, error) {
	_, digest, _, err := r.PlatformManifest(ctx, image, tag, arch)
	if err != nil {
		return nil, "", err
	}

	// Cosign stores attestations under the tag "sha256-<hex>.att".
	attTag := strings.Replace(digest, ":", "-", 1) + ".att"
	att, _, err := r.Manifest(ctx, image, attTag, acceptManifestOnly)
	if err != nil {
		return nil, "", fmt.Errorf("fetching attestations: %w", err)
	}
	var sbomLayer *Descriptor
	for i, l := range att.Layers {
		if l.Annotations["predicateType"] == predicateSPDX {
			sbomLayer = &att.Layers[i]
			break
		}
	}
	if sbomLayer == nil {
		return nil, "", fmt.Errorf("%s:%s (%s) has no SPDX SBOM attestation", image, tag, arch)
	}

	raw, err := r.Blob(ctx, image, sbomLayer.Digest, maxSBOMBytes)
	if err != nil {
		return nil, "", err
	}
	doc, err := unwrapSBOMAttestation(raw)
	if err != nil {
		return nil, "", err
	}
	return doc, digest, nil
}

// unwrapSBOMAttestation decodes a DSSE envelope wrapping an in-toto statement
// and returns its predicate, which must be an SPDX document.
func unwrapSBOMAttestation(raw []byte) (json.RawMessage, error) {
	var envelope struct {
		Payload string `json:"payload"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("decoding attestation envelope: %w", err)
	}
	payload, err := base64.StdEncoding.DecodeString(envelope.Payload)
	if err != nil {
		return nil, fmt.Errorf("decoding attestation payload: %w", err)
	}
	var statement struct {
		PredicateType string          `json:"predicateType"`
		Predicate     json.RawMessage `json:"predicate"`
	}
	if err := json.Unmarshal(payload, &statement); err != nil {
		return nil, fmt.Errorf("decoding in-toto statement: %w", err)
	}
	if statement.PredicateType != predicateSPDX {
		return nil, fmt.Errorf("unexpected predicate type %q", statement.PredicateType)
	}
	return statement.Predicate, nil
}

// spdxDoc holds the parts of an SPDX 2.x document that we use.
type spdxDoc struct {
	Packages []struct {
		SPDXID          string `json:"SPDXID"`
		Name            string `json:"name"`
		VersionInfo     string `json:"versionInfo"`
		LicenseDeclared string `json:"licenseDeclared"`
		ExternalRefs    []struct {
			ReferenceType    string `json:"referenceType"`
			ReferenceLocator string `json:"referenceLocator"`
		} `json:"externalRefs"`
	} `json:"packages"`
	Relationships []struct {
		Element string `json:"spdxElementId"`
		Type    string `json:"relationshipType"`
		Related string `json:"relatedSpdxElement"`
	} `json:"relationships"`
}

// parseSBOMPackages extracts the apk packages from an SPDX document.
func parseSBOMPackages(raw json.RawMessage) ([]Package, error) {
	var doc spdxDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("decoding SPDX document: %w", err)
	}

	// Each apk package is DESCRIBED_BY the melange build file of its origin,
	// e.g. SPDXRef-Package-apk-libssl3-… → a package named "openssl.yaml".
	names := make(map[string]string, len(doc.Packages))
	for _, p := range doc.Packages {
		names[p.SPDXID] = p.Name
	}
	origins := make(map[string]string)
	for _, rel := range doc.Relationships {
		if rel.Type != "DESCRIBED_BY" {
			continue
		}
		if name, ok := strings.CutSuffix(names[rel.Related], ".yaml"); ok {
			origins[rel.Element] = name
		}
	}

	// apko lists each installed package twice (once with a distro purl, once
	// with an origin purl), so merge entries by name and version.
	byKey := make(map[string]*Package)
	for _, p := range doc.Packages {
		for _, ref := range p.ExternalRefs {
			if ref.ReferenceType != "purl" || !strings.HasPrefix(ref.ReferenceLocator, "pkg:apk/") {
				continue
			}
			distro, qualifiers := parseAPKPurl(ref.ReferenceLocator)
			key := p.Name + "@" + p.VersionInfo
			pkg := byKey[key]
			if pkg == nil {
				pkg = &Package{Name: p.Name, Version: p.VersionInfo, Distro: distro}
				byKey[key] = pkg
			}
			// The melange relationship is the most reliable origin, then the
			// purl qualifier; fall back to the package's own name below.
			if o := origins[p.SPDXID]; o != "" {
				pkg.Origin = o
			} else if o := qualifiers.Get("origin"); o != "" && pkg.Origin == "" {
				pkg.Origin = o
			}
			if pkg.License == "" && p.LicenseDeclared != "NOASSERTION" {
				pkg.License = p.LicenseDeclared
			}
			break
		}
	}
	pkgs := make([]Package, 0, len(byKey))
	for _, pkg := range byKey {
		if pkg.Origin == "" {
			pkg.Origin = pkg.Name
		}
		pkgs = append(pkgs, *pkg)
	}
	sort.Slice(pkgs, func(i, j int) bool { return pkgs[i].Name < pkgs[j].Name })
	return pkgs, nil
}

// parseAPKPurl splits "pkg:apk/wolfi/name@ver?arch=x86_64&origin=foo" into
// its namespace ("wolfi") and qualifiers.
func parseAPKPurl(purl string) (string, url.Values) {
	rest := strings.TrimPrefix(purl, "pkg:apk/")
	rest, query, _ := strings.Cut(rest, "?")
	namespace, _, _ := strings.Cut(rest, "/")
	qualifiers, _ := url.ParseQuery(query)
	return namespace, qualifiers
}

// validSBOMFilename allows plain file names only: no paths or leading dots.
var validSBOMFilename = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,200}$`)

type SavedSBOM struct {
	Path         string `json:"path" jsonschema:"absolute path of the saved SPDX JSON file"`
	SizeBytes    int    `json:"size_bytes"`
	Image        string `json:"image"`
	Tag          string `json:"tag"`
	Arch         string `json:"arch"`
	Digest       string `json:"digest" jsonschema:"platform image digest the SBOM describes"`
	SPDXVersion  string `json:"spdx_version"`
	PackageCount int    `json:"package_count" jsonschema:"number of OS packages in the SBOM"`
}

// sbomFilename validates a requested file name, or builds the default
// "<image>-<tag>-<arch>.spdx.json". A ".json" extension is added if missing.
func sbomFilename(requested, image, tag, arch string) (string, error) {
	name := strings.TrimSpace(requested)
	if name == "" {
		name = image + "-" + tag + "-" + arch + ".spdx.json"
	}
	if !validSBOMFilename.MatchString(name) {
		return "", fmt.Errorf("invalid filename %q: use a plain file name such as python.spdx.json, without folders", requested)
	}
	if !strings.HasSuffix(strings.ToLower(name), ".json") {
		name += ".json"
	}
	return name, nil
}

// writeSBOM pretty-prints an SPDX document into dir/name. It refuses to
// replace an existing file unless overwrite is set. os.Root confines the
// write to dir even if name were to contain path elements.
func writeSBOM(dir, name string, doc json.RawMessage, overwrite bool) (string, int, error) {
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, doc, "", "  "); err != nil {
		return "", 0, fmt.Errorf("formatting SBOM: %w", err)
	}
	pretty.WriteByte('\n')

	root, err := os.OpenRoot(dir)
	if err != nil {
		return "", 0, fmt.Errorf("opening SBOM folder: %w", err)
	}
	defer root.Close()

	flags := os.O_WRONLY | os.O_CREATE | os.O_EXCL
	if overwrite {
		flags = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	}
	f, err := root.OpenFile(name, flags, 0o644)
	if errors.Is(err, fs.ErrExist) {
		return "", 0, fmt.Errorf("%s already exists; set overwrite to replace it or choose another filename", name)
	}
	if err != nil {
		return "", 0, fmt.Errorf("creating SBOM file: %w", err)
	}
	if _, err := f.Write(pretty.Bytes()); err != nil {
		f.Close()
		return "", 0, fmt.Errorf("writing SBOM file: %w", err)
	}
	if err := f.Close(); err != nil {
		return "", 0, fmt.Errorf("writing SBOM file: %w", err)
	}

	abs, err := filepath.Abs(filepath.Join(dir, name))
	if err != nil {
		abs = filepath.Join(dir, name)
	}
	return abs, pretty.Len(), nil
}

// SaveSBOM fetches an image's SPDX SBOM and saves it to dir.
func (r *Registry) SaveSBOM(ctx context.Context, dir, image, tag, arch, filename string, overwrite bool) (*SavedSBOM, error) {
	name, err := sbomFilename(filename, image, tag, arch)
	if err != nil {
		return nil, err
	}
	doc, digest, err := r.SBOMDocument(ctx, image, tag, arch)
	if err != nil {
		return nil, err
	}
	pkgs, err := parseSBOMPackages(doc)
	if err != nil {
		return nil, err
	}
	var meta struct {
		SPDXVersion string `json:"spdxVersion"`
	}
	_ = json.Unmarshal(doc, &meta) // best effort; the version is informational

	path, size, err := writeSBOM(dir, name, doc, overwrite)
	if err != nil {
		return nil, err
	}
	return &SavedSBOM{
		Path: path, SizeBytes: size, Image: image, Tag: tag, Arch: arch,
		Digest: digest, SPDXVersion: meta.SPDXVersion, PackageCount: len(pkgs),
	}, nil
}
