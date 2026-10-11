package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
)

// OSVPackage names a package to look up on OSV.dev: the name and version are
// the ones the ecosystem's advisories are keyed by (for Debian, the source
// package).
type OSVPackage struct {
	Ecosystem string `json:"ecosystem"`
	Name      string `json:"name"`
	Version   string `json:"version"`
}

// UpstreamPackages is what an upstream image's SBOM says is installed.
type UpstreamPackages struct {
	// Installed is the number of installed OS (binary) packages. The SBOM
	// also lists source packages, which are not installed and not counted.
	Installed int
	// Lookups are the distinct source packages to check for vulnerabilities,
	// without the Linux kernel.
	Lookups []OSVPackage
	// Language counts language packages (npm, pypi, …) by ecosystem; they are
	// not compared.
	Language map[string]int
}

// kernelSource is Debian's source package for the Linux kernel. Containers
// ship only its headers (linux-libc-dev) and run on the host's kernel, so its
// advisories don't apply and are left out (UC-4).
const kernelSource = "linux"

// languagePurlTypes are the purl types counted as language packages.
var languagePurlTypes = map[string]bool{
	"npm": true, "pypi": true, "gem": true, "golang": true, "maven": true,
	"cargo": true, "nuget": true, "composer": true, "cocoapods": true, "hex": true, "pub": true,
}

// purlParts is a package URL split into the parts used here.
type purlParts struct {
	typ, namespace, name, version string
	qualifiers                    url.Values
}

// parsePurl splits "pkg:deb/debian/libssl3@3.0.19-1~deb12u2?os_version=12".
func parsePurl(purl string) (purlParts, bool) {
	rest, ok := strings.CutPrefix(purl, "pkg:")
	if !ok {
		return purlParts{}, false
	}
	rest, query, _ := strings.Cut(rest, "?")
	rest, _, _ = strings.Cut(rest, "#")
	var p purlParts
	p.qualifiers, _ = url.ParseQuery(query)
	rest, version, _ := strings.Cut(rest, "@")
	segments := strings.Split(rest, "/")
	p.typ = segments[0]
	if len(segments) < 2 {
		return purlParts{}, false
	}
	p.name = unescapePurl(segments[len(segments)-1])
	if len(segments) > 2 {
		p.namespace = strings.Join(segments[1:len(segments)-1], "/")
	}
	p.version = unescapePurl(version)
	return p, true
}

func unescapePurl(s string) string {
	if u, err := url.PathUnescape(s); err == nil {
		return u
	}
	return s
}

// osvEcosystemFor returns the OSV ecosystem for an OS package purl, such as
// Debian:12, Ubuntu:24.04:LTS or Alpine:v3.23, or "" for other distributions.
func osvEcosystemFor(p purlParts) string {
	version := p.qualifiers.Get("os_version")
	switch {
	case p.typ == "deb" && p.namespace == "debian" && version != "":
		return "Debian:" + majorVersion(version)
	case p.typ == "deb" && p.namespace == "ubuntu" && version != "":
		// OSV names long-term-support releases (an even year's .04) with a suffix.
		year, minor, _ := strings.Cut(version, ".")
		if minor == "04" && len(year) == 2 && (year[1]-'0')%2 == 0 {
			return "Ubuntu:" + version + ":LTS"
		}
		return "Ubuntu:" + version
	case p.typ == "apk" && p.namespace == "alpine" && version != "":
		return "Alpine:v" + majorMinor(version)
	}
	return ""
}

func majorVersion(v string) string { return strings.SplitN(v, ".", 2)[0] }

func majorMinor(v string) string {
	parts := strings.SplitN(v, ".", 3)
	return strings.Join(parts[:min(len(parts), 2)], ".")
}

type spdxPurlPackage struct {
	id, typ  string
	purl     purlParts
	isOS     bool
	language bool
}

// parseUpstreamPackages reads the OS packages of a Debian, Ubuntu or Alpine
// image from its SPDX SBOM. Each installed binary package is linked by a
// GENERATED_FROM relationship to the source package it was built from, which
// is what the distribution's advisories name (UC-3, UC-4).
func parseUpstreamPackages(raw json.RawMessage) (*UpstreamPackages, error) {
	var doc spdxDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("decoding SPDX document: %w", err)
	}
	byID := make(map[string]spdxPurlPackage, len(doc.Packages))
	for _, p := range doc.Packages {
		for _, ref := range p.ExternalRefs {
			if ref.ReferenceType != "purl" {
				continue
			}
			parts, ok := parsePurl(ref.ReferenceLocator)
			if !ok {
				continue
			}
			byID[p.SPDXID] = spdxPurlPackage{
				id: p.SPDXID, typ: parts.typ, purl: parts,
				isOS:     parts.typ == "deb" || parts.typ == "apk",
				language: languagePurlTypes[parts.typ],
			}
			break
		}
	}

	sourceOf := make(map[string]string) // binary SPDXID → source SPDXID
	isSource := make(map[string]bool)
	for _, rel := range doc.Relationships {
		if rel.Type == "GENERATED_FROM" {
			if _, ok := byID[rel.Related]; ok {
				sourceOf[rel.Element] = rel.Related
				isSource[rel.Related] = true
			}
		}
	}

	out := &UpstreamPackages{Language: map[string]int{}}
	seen := make(map[OSVPackage]bool)
	for _, p := range doc.Packages {
		pkg, ok := byID[p.SPDXID]
		switch {
		case !ok:
			continue
		case pkg.language:
			out.Language[pkg.typ]++
			continue
		case !pkg.isOS || isSource[p.SPDXID]:
			continue
		}
		ecosystem := osvEcosystemFor(pkg.purl)
		if ecosystem == "" {
			return nil, fmt.Errorf("%s: unsupported distribution %s/%s %s", p.Name, pkg.typ, pkg.purl.namespace, pkg.purl.qualifiers.Get("os_version"))
		}
		out.Installed++
		src := pkg.purl
		if s, ok := byID[sourceOf[p.SPDXID]]; ok {
			src = s.purl
		}
		if src.name == kernelSource {
			continue
		}
		lookup := OSVPackage{Ecosystem: ecosystem, Name: src.name, Version: src.version}
		if !seen[lookup] {
			seen[lookup] = true
			out.Lookups = append(out.Lookups, lookup)
		}
	}
	sort.Slice(out.Lookups, func(i, j int) bool {
		a, b := out.Lookups[i], out.Lookups[j]
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		return a.Version < b.Version
	})
	return out, nil
}

// osvEcosystemForDistro returns the OSV ecosystem for a Chainguard image
// package from its purl namespace, e.g. "wolfi" → "Wolfi". Other
// distributions have no known ecosystem.
func osvEcosystemForDistro(distro string) (string, bool) {
	switch distro {
	case "wolfi":
		return "Wolfi", true
	case "chainguard":
		return "Chainguard", true
	}
	return "", false
}
