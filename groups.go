package main

import (
	"cmp"
	"slices"
	"strings"
)

// Groupings for the web UI's catalog map. Unlike families, groups are formed
// from the images being shown, so a search still gets named groups.
const (
	GroupByVariant = "variant"
	GroupByPrefix  = "prefix"
)

// ImageGroup is a set of images that share a variant kind (e.g. FIPS) or a
// name prefix (e.g. crossplane), or the Other group that collects the rest.
type ImageGroup struct {
	Label  string       `json:"label"`
	Folded int          `json:"folded,omitempty" jsonschema:"for the Other group, how many smaller groups it merges"`
	Images []string     `json:"images" jsonschema:"the group's images, block by block"`
	Blocks []ImageBlock `json:"blocks" jsonschema:"consecutive runs of images sharing a name prefix"`
}

// ImageBlock is a run of a group's images that share a name prefix.
type ImageBlock struct {
	Prefix string `json:"prefix"`
	Count  int    `json:"count"`
}

// variantKind returns the variant suffixes of image, e.g. "iamguarded-fips",
// or "" for a base image. It agrees with familyName by construction.
func variantKind(image string) string {
	_, suffixes := splitVariant(image)
	return suffixes
}

// namePrefix returns the part of image before the first hyphen.
func namePrefix(image string) string {
	prefix, _, _ := strings.Cut(image, "-")
	return prefix
}

var suffixLabels = map[string]string{"fips": "FIPS", "iamguarded": "IAM-guarded"}

// variantLabel names a variant kind: "" → "Base images", "iamguarded-fips"
// → "IAM-guarded FIPS", "openssl" → "openssl".
func variantLabel(kind string) string {
	if kind == "" {
		return "Base images"
	}
	words := strings.Split(kind, "-")
	for i, w := range words {
		if label, ok := suffixLabels[w]; ok {
			words[i] = label
		}
	}
	return strings.Join(words, " ")
}

// groupImages groups images by variant kind or name prefix. A group is named
// only if it has at least 1% of the images (rounded up) and at least 2; the
// rest merge into one Other group. Groups are ordered by image count
// (descending), then label, and each group's images are ordered into
// name-prefix blocks (splitBlocks).
func groupImages(images []string, by string) []ImageGroup {
	key, label, otherLabel := variantKind, variantLabel, "Other variants"
	if by == GroupByPrefix {
		key, label, otherLabel = namePrefix, func(p string) string { return p }, "Other"
	}
	buckets := make(map[string][]string)
	var keys []string
	for _, img := range images {
		k := key(img)
		if _, ok := buckets[k]; !ok {
			keys = append(keys, k)
		}
		buckets[k] = append(buckets[k], img)
	}
	minSize := max(2, (len(images)+99)/100)
	groups := []ImageGroup{}
	other := ImageGroup{Label: otherLabel}
	for _, k := range keys {
		if imgs := buckets[k]; len(imgs) >= minSize {
			groups = append(groups, ImageGroup{Label: label(k), Images: imgs})
		} else {
			other.Folded++
			other.Images = append(other.Images, imgs...)
		}
	}
	if other.Folded > 0 {
		groups = append(groups, other)
	}
	for i := range groups {
		groups[i].Images, groups[i].Blocks = splitBlocks(groups[i].Images)
	}
	slices.SortFunc(groups, func(a, b ImageGroup) int {
		if c := len(b.Images) - len(a.Images); c != 0 {
			return c
		}
		return cmp.Compare(a.Label, b.Label)
	})
	return groups
}

// splitBlocks orders a group's images into name-prefix blocks, one per prefix,
// ordered by size (descending), then prefix, with images in name order
// within a block.
func splitBlocks(images []string) ([]string, []ImageBlock) {
	byPrefix := make(map[string][]string)
	var prefixes []string
	for _, img := range images {
		p := namePrefix(img)
		if _, ok := byPrefix[p]; !ok {
			prefixes = append(prefixes, p)
		}
		byPrefix[p] = append(byPrefix[p], img)
	}
	slices.SortFunc(prefixes, func(a, b string) int {
		if c := len(byPrefix[b]) - len(byPrefix[a]); c != 0 {
			return c
		}
		return cmp.Compare(a, b)
	})
	ordered := make([]string, 0, len(images))
	runs := make([]ImageBlock, 0, len(prefixes))
	for _, p := range prefixes {
		imgs := byPrefix[p]
		slices.Sort(imgs)
		ordered = append(ordered, imgs...)
		runs = append(runs, ImageBlock{Prefix: p, Count: len(imgs)})
	}
	return ordered, runs
}
