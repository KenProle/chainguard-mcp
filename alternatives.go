package main

import (
	"context"
	"slices"
	"sort"
	"strings"
)

// imageAliases maps common upstream image names to the Chainguard image name
// when they differ. Keys are matched against the last path component, and
// also the last two components (e.g. "dotnet/aspnet").
var imageAliases = map[string]string{
	"golang":                     "go",
	"openjdk":                    "jdk",
	"eclipse-temurin":            "jdk",
	"amazoncorretto":             "jdk",
	"ibm-semeru-runtimes":        "jdk",
	"sapmachine":                 "jdk",
	"ubuntu":                     "wolfi-base",
	"debian":                     "wolfi-base",
	"alpine":                     "wolfi-base",
	"centos":                     "wolfi-base",
	"fedora":                     "wolfi-base",
	"rockylinux":                 "wolfi-base",
	"almalinux":                  "wolfi-base",
	"ubi":                        "wolfi-base",
	"ubi-minimal":                "wolfi-base",
	"ubi8":                       "wolfi-base",
	"ubi9":                       "wolfi-base",
	"scratch":                    "static",
	"distroless/static":          "static",
	"distroless/static-debian12": "static",
	"distroless/base":            "glibc-dynamic",
	"distroless/base-debian12":   "glibc-dynamic",
	"distroless/cc":              "glibc-dynamic",
	"distroless/cc-debian12":     "glibc-dynamic",
	"distroless/java":            "jre",
	"distroless/java17":          "jre",
	"distroless/java21":          "jre",
	"distroless/python3":         "python",
	"distroless/nodejs":          "node",
	"nodejs":                     "node",
	"dotnet/sdk":                 "dotnet-sdk",
	"dotnet/aspnet":              "aspnet-runtime",
	"dotnet/runtime":             "dotnet-runtime",
	"mongo":                      "mongodb",
	"gcc":                        "gcc-glibc",
	"postgresql":                 "postgres",
	"kubernetes-kubectl":         "kubectl",
}

type Alternative struct {
	Image     string `json:"image"`
	Reference string `json:"reference"`
	Free      *bool  `json:"free,omitempty" jsonschema:"whether the image is in the free tier (omitted if unknown)"`
}

type AlternativesResult struct {
	Input        string        `json:"input"`
	Recommended  *Alternative  `json:"recommended,omitempty"`
	Alternatives []Alternative `json:"alternatives" jsonschema:"other related Chainguard images, e.g. FIPS or version-specific variants"`
	DevTag       string        `json:"dev_tag,omitempty" jsonschema:"tag with a shell and package manager, for build stages"`
	Notes        []string      `json:"notes"`
}

// parsedRef is an upstream image reference split into parts.
type parsedRef struct {
	path []string // registry-less path, e.g. ["library", "python"]
	tag  string
}

// parseImageRef splits refs like "docker.io/library/python:3.12-slim",
// "mcr.microsoft.com/dotnet/aspnet:8.0" or "node@sha256:…".
func parseImageRef(ref string) parsedRef {
	ref = strings.TrimSpace(strings.ToLower(ref))
	ref, _, _ = strings.Cut(ref, "@")
	var p parsedRef
	if i := strings.LastIndex(ref, ":"); i > strings.LastIndex(ref, "/") {
		ref, p.tag = ref[:i], ref[i+1:]
	}
	parts := strings.Split(ref, "/")
	// The first component is a registry host if it has a dot or port, or is localhost.
	if len(parts) > 1 && (strings.ContainsAny(parts[0], ".:") || parts[0] == "localhost") {
		parts = parts[1:]
	}
	p.path = parts
	return p
}

// candidateNames returns Chainguard image names to look for, best first.
func (p parsedRef) candidateNames() []string {
	if len(p.path) == 0 {
		return nil
	}
	last := p.path[len(p.path)-1]
	var names []string
	if len(p.path) >= 2 {
		if alias, ok := imageAliases[p.path[len(p.path)-2]+"/"+last]; ok {
			names = append(names, alias)
		}
	}
	if alias, ok := imageAliases[last]; ok {
		names = append(names, alias)
	}
	// JDK images with a JRE tag (e.g. eclipse-temurin:21-jre) map to jre.
	if slices.Contains(names, "jdk") && strings.Contains(p.tag, "jre") {
		names = append([]string{"jre"}, names...)
	}
	names = append(names, last)
	return names
}

// FindAlternatives suggests Chainguard images to replace an upstream image.
func (s *Service) FindAlternatives(ctx context.Context, ref string) (*AlternativesResult, error) {
	images, err := s.Catalog.Images(ctx)
	if err != nil {
		return nil, err
	}
	parsed := parseImageRef(ref)
	result := &AlternativesResult{Input: ref, Alternatives: []Alternative{}}

	var recommended string
	for _, name := range parsed.candidateNames() {
		if ok, _ := s.Catalog.Contains(ctx, name); ok {
			recommended = name
			break
		}
	}

	// Related images are variants of the recommended (or upstream) name,
	// e.g. python → python-fips, jdk → jdk-crac.
	base := recommended
	if base == "" && len(parsed.path) > 0 {
		base = parsed.path[len(parsed.path)-1]
	}
	var related []string
	if base != "" {
		for _, img := range images {
			if img != recommended && isVariant(img, base) {
				related = append(related, img)
			}
		}
		// Prefer shorter, non-FIPS names; cap the list.
		sort.SliceStable(related, func(i, j int) bool {
			fi, fj := strings.Contains(related[i], "fips"), strings.Contains(related[j], "fips")
			if fi != fj {
				return !fi
			}
			return len(related[i]) < len(related[j])
		})
		related = related[:min(len(related), 10)]
	}

	toCheck := append([]string{}, related...)
	if recommended != "" {
		toCheck = append(toCheck, recommended)
	}
	free := s.Registry.PublicStatus(ctx, toCheck)
	alt := func(name string) Alternative {
		a := Alternative{Image: name, Reference: s.Registry.Ref(name)}
		if f, ok := free[name]; ok {
			a.Free = &f
		}
		return a
	}

	if recommended != "" {
		a := alt(recommended)
		result.Recommended = &a
		if a.Free != nil && *a.Free {
			if tags, err := s.Registry.Tags(ctx, recommended); err == nil && slices.Contains(tags, "latest-dev") {
				result.DevTag = "latest-dev"
			}
		}
	}
	for _, name := range related {
		result.Alternatives = append(result.Alternatives, alt(name))
	}
	result.Notes = alternativeNotes(parsed, result)
	return result, nil
}

func alternativeNotes(p parsedRef, r *AlternativesResult) []string {
	var notes []string
	if r.Recommended == nil {
		if len(r.Alternatives) == 0 {
			return []string{"No matching Chainguard image found. Try list_images with a broader query."}
		}
		notes = append(notes, "No exact match; the alternatives share the upstream image's name.")
		return notes
	}
	if r.Recommended.Free != nil && !*r.Recommended.Free {
		notes = append(notes, "The recommended image requires a Chainguard subscription.")
	}
	if r.DevTag != "" {
		notes = append(notes, "Use :"+r.DevTag+" (includes a shell and the apk package manager) for build stages, and the minimal :latest for the final runtime stage.")
	}
	if p.tag != "" && p.tag != "latest" && r.Recommended.Free != nil && *r.Recommended.Free {
		notes = append(notes, "The free tier only provides the latest version. Specific version tags (like "+p.tag+") require a subscription, so pin :latest by digest for reproducible builds.")
	}
	if r.Recommended.Image == "wolfi-base" {
		notes = append(notes, "wolfi-base is a minimal general-purpose base image. Install packages with `apk add` instead of apt/yum/dnf.")
	}
	notes = append(notes, "Chainguard images usually run as a non-root user and may have a different entrypoint, so check the image's user and entrypoint before switching.")
	return notes
}

// variantSuffixes are name parts Chainguard appends to make a variant of an
// image, e.g. node-fips or postgres-iamguarded-fips. Other suffixes usually
// mean a different project (node-local-dns, postgres-operator).
var variantSuffixes = map[string]bool{
	"fips": true, "iamguarded": true, "crac": true, "openssl": true,
	"lts": true, "msft": true, "geomys": true, "slim": true, "glibc": true, "musl": true,
}

// isVariant reports whether img is base plus only variant suffixes.
func isVariant(img, base string) bool {
	rest, ok := strings.CutPrefix(img, base+"-")
	if !ok {
		return false
	}
	for part := range strings.SplitSeq(rest, "-") {
		if !variantSuffixes[part] {
			return false
		}
	}
	return true
}

// ImageFamily is an image and its variants, e.g. nginx with nginx-fips,
// nginx-iamguarded and nginx-iamguarded-fips.
type ImageFamily struct {
	Name   string   `json:"name" jsonschema:"the images' name without variant suffixes; not always an image itself"`
	Images []string `json:"images" jsonschema:"the family's images in name order; the first is its base image when that exists"`
}

// familyName removes trailing variant suffixes from image, never its first
// name part, so every image in a family other than the family name itself
// is a variant of it (isVariant).
func familyName(image string) string {
	name, _ := splitVariant(image)
	return name
}

// splitVariant splits image into its family name and the trailing variant
// suffixes, e.g. "nginx-iamguarded-fips" → "nginx", "iamguarded-fips". The
// suffixes are empty for a base image.
func splitVariant(image string) (name, suffixes string) {
	parts := strings.Split(image, "-")
	n := len(parts)
	for n > 1 && variantSuffixes[parts[n-1]] {
		n--
	}
	return strings.Join(parts[:n], "-"), strings.Join(parts[n:], "-")
}

// groupFamilies groups images into families, ordered by image count
// (descending), then name. Images within a family are in name order, so a
// family's base image, a prefix of the others, comes first when it exists.
func groupFamilies(images []string) []ImageFamily {
	index := make(map[string]int)
	var families []ImageFamily
	for _, img := range images {
		name := familyName(img)
		i, ok := index[name]
		if !ok {
			i = len(families)
			index[name] = i
			families = append(families, ImageFamily{Name: name})
		}
		families[i].Images = append(families[i].Images, img)
	}
	for i := range families {
		slices.Sort(families[i].Images)
	}
	sortFamilies(families)
	return families
}

// sortFamilies orders families by image count (descending), then name.
func sortFamilies(families []ImageFamily) {
	slices.SortFunc(families, func(a, b ImageFamily) int {
		if c := len(b.Images) - len(a.Images); c != 0 {
			return c
		}
		return strings.Compare(a.Name, b.Name)
	})
}
