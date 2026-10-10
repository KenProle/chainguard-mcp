package main

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestCompareAPKVersions(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"3.6.5-r1", "3.6.5-r1", 0},
		{"3.6.5-r1", "3.0.7-r0", 1},
		{"3.0.7-r0", "3.0.7-r1", -1},
		{"3.0.10-r0", "3.0.9-r0", 1}, // numeric, not lexical
		{"1.0", "1.0.1", -1},
		{"1.0a", "1.0", 1},
		{"1.0_rc1", "1.0", -1},
		{"1.0_alpha2", "1.0_beta1", -1},
		{"1.0_p1", "1.0", 1},
		{"0_git20240101", "0_git20240201", -1},
		{"20230201-r30", "20230201-r4", 1},
		{"007-r0", "7-r0", 0},
		{"99999999999999999999.0", "1.0", 1}, // no integer overflow
		{"weird", "weird", 0},                // unparseable falls back to string compare
	}
	for _, tt := range tests {
		if got := compareAPKVersions(tt.a, tt.b); got != tt.want {
			t.Errorf("compareAPKVersions(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
		if got := compareAPKVersions(tt.b, tt.a); got != -tt.want {
			t.Errorf("compareAPKVersions(%q, %q) = %d, want %d", tt.b, tt.a, got, -tt.want)
		}
	}
}

func TestParseImageRef(t *testing.T) {
	tests := []struct {
		ref  string
		path []string
		tag  string
	}{
		{"python", []string{"python"}, ""},
		{"Node:20-Alpine", []string{"node"}, "20-alpine"},
		{"docker.io/library/python:3.12-slim", []string{"library", "python"}, "3.12-slim"},
		{"localhost:5000/team/app:v1", []string{"team", "app"}, "v1"},
		{"mcr.microsoft.com/dotnet/aspnet:8.0", []string{"dotnet", "aspnet"}, "8.0"},
		{"redis@sha256:abc", []string{"redis"}, ""},
		{"bitnami/redis", []string{"bitnami", "redis"}, ""},
	}
	for _, tt := range tests {
		got := parseImageRef(tt.ref)
		if !slices.Equal(got.path, tt.path) || got.tag != tt.tag {
			t.Errorf("parseImageRef(%q) = %v %q, want %v %q", tt.ref, got.path, got.tag, tt.path, tt.tag)
		}
	}
}

func TestCandidateNames(t *testing.T) {
	tests := map[string][]string{
		"golang:1.23":                         {"go", "golang"},
		"eclipse-temurin:21-jre":              {"jre", "jdk", "eclipse-temurin"},
		"mcr.microsoft.com/dotnet/aspnet:8.0": {"aspnet-runtime", "aspnet"},
		"redis":                               {"redis"},
	}
	for ref, want := range tests {
		if got := parseImageRef(ref).candidateNames(); !slices.Equal(got, want) {
			t.Errorf("candidateNames(%q) = %v, want %v", ref, got, want)
		}
	}
}

func TestIsRootUser(t *testing.T) {
	for user, want := range map[string]bool{
		"": true, "root": true, "0": true, "0:0": true, "root:root": true,
		"65532": false, "nonroot": false, "65532:65532": false,
	} {
		if got := isRootUser(user); got != want {
			t.Errorf("isRootUser(%q) = %v, want %v", user, got, want)
		}
	}
}

func TestImageAndTag(t *testing.T) {
	if img, tag, err := imageAndTag(" python ", ""); err != nil || img != "python" || tag != "latest" {
		t.Errorf("got %q %q %v", img, tag, err)
	}
	for _, bad := range []string{"", "Python", "a/b", "../x", "py thon", "-x"} {
		if _, _, err := imageAndTag(bad, ""); err == nil {
			t.Errorf("imageAndTag(%q) should fail", bad)
		}
	}
	if _, _, err := imageAndTag("python", "bad/tag"); err == nil {
		t.Error("tag with slash should fail")
	}
}

func TestAnalyzeVulnsUncovered(t *testing.T) {
	pkgs := []Package{
		{Name: "libssl3", Version: "3.6.5-r1", Origin: "openssl", Distro: "wolfi"},
		{Name: "private-tool", Version: "1.0-r0", Origin: "private-tool", Distro: "chainguard"},
	}
	r := analyzeVulns(pkgs, map[string]map[string][]string{}, "")
	if !slices.Equal(r.Uncovered, []string{"private-tool"}) || len(r.Packages) != 1 {
		t.Fatalf("unexpected report: %+v", r)
	}
}

func TestIsVariant(t *testing.T) {
	// Real catalog names: variants vs. unrelated projects sharing a prefix.
	tests := []struct {
		img, base string
		want      bool
	}{
		{"node-fips", "node", true},
		{"postgres-iamguarded-fips", "postgres", true},
		{"jdk-crac", "jdk", true},
		{"go-msft-fips", "go", true},
		{"node-local-dns", "node", false},
		{"node-problem-detector-fips", "node", false},
		{"postgres-operator", "postgres", false},
		{"go-ipfs", "go", false},
		{"nodejs", "node", false}, // no separator
		{"python", "python", false},
	}
	for _, tt := range tests {
		if got := isVariant(tt.img, tt.base); got != tt.want {
			t.Errorf("isVariant(%q, %q) = %v, want %v", tt.img, tt.base, got, tt.want)
		}
	}
}

func TestAnalyzeVulnsVersionStreamFallback(t *testing.T) {
	// Wolfi keeps OpenSSL's history under "openssl" while images install the
	// versioned "openssl-4.0" package.
	fixes := map[string]map[string][]string{
		"openssl": {
			"0":        {"CVE-2023-0466"},
			"3.0.7-r0": {"CVE-2022-3602"},
			"4.1.0-r0": {"CVE-2099-0002"}, // a newer stream's fix
		},
		"python-3.13": {"3.13.9-r0": {"CVE-2099-0003"}},
	}
	pkgs := []Package{
		{Name: "openssl-4.0-libssl", Version: "4.0.3-r5", Origin: "openssl-4.0", Distro: "wolfi"},
		{Name: "python-3.13", Version: "3.13.7-r0", Origin: "python-3.13", Distro: "wolfi"},
	}

	r := analyzeVulns(pkgs, fixes, "CVE-2022-3602")
	if len(r.Matches) != 1 || r.Matches[0].Status != StatusFixed || r.Matches[0].RecordsFrom != "openssl" {
		t.Fatalf("want CVE-2022-3602 fixed via openssl records, got %+v", r.Matches)
	}

	// A newer-stream fix found via the fallback is skipped, not reported as vulnerable.
	if r := analyzeVulns(pkgs, fixes, "CVE-2099-0002"); len(r.Matches) != 0 {
		t.Errorf("fallback should skip newer-stream fixes, got %+v", r.Matches)
	}

	// A package with its own records still reports pending fixes.
	summary := analyzeVulns(pkgs, fixes, "")
	if summary.TotalFixed != 1 || summary.TotalNotAffected != 1 {
		t.Errorf("unexpected totals: fixed=%d not_affected=%d", summary.TotalFixed, summary.TotalNotAffected)
	}
	py := summary.Packages[1]
	if py.RecordsFrom != "" || !slices.Equal(py.PendingFixes, []string{"CVE-2099-0003"}) {
		t.Errorf("python-3.13 should use its own records: %+v", py)
	}
}

// readNames reads one image name per line from a testdata file.
func readNames(t *testing.T, file string) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", file))
	if err != nil {
		t.Fatal(err)
	}
	return strings.Fields(string(data))
}

// realCatalog is the image list from the live sitemap on 2026-10-09.
func realCatalog(t *testing.T) []string { return readNames(t, "catalog-2026-10-09.txt") }

func findFamily(families []ImageFamily, name string) *ImageFamily {
	for i := range families {
		if families[i].Name == name {
			return &families[i]
		}
	}
	return nil
}

func TestImageFamilies(t *testing.T) {
	images := realCatalog(t)
	families := groupFamilies(images)
	familyImages := func(name string) []string {
		if f := findFamily(families, name); f != nil {
			return f.Images
		}
		return nil
	}
	wantFamily := func(t *testing.T, name string, want ...string) {
		t.Helper()
		if got := familyImages(name); !slices.Equal(got, want) {
			t.Errorf("family %s = %v, want %v", name, got, want)
		}
	}

	t.Run("CM-1.1 nginx and its variants", func(t *testing.T) {
		wantFamily(t, "nginx", "nginx", "nginx-fips", "nginx-iamguarded", "nginx-iamguarded-fips")
	})
	t.Run("CM-1.2 shared prefixes stay separate", func(t *testing.T) {
		wantFamily(t, "node", "node", "node-fips")
		wantFamily(t, "node-local-dns", "node-local-dns")
		wantFamily(t, "node-problem-detector", "node-problem-detector", "node-problem-detector-fips")
		wantFamily(t, "postgres-operator", "postgres-operator", "postgres-operator-fips")
	})
	t.Run("CM-1.3 several suffixes", func(t *testing.T) {
		wantFamily(t, "go", "go", "go-fips", "go-geomys-fips", "go-msft-fips", "go-openssl", "go-openssl-fips")
	})
	t.Run("CM-1.4 same variants as find_alternative", func(t *testing.T) {
		for _, f := range families {
			members := make(map[string]bool, len(f.Images))
			for _, img := range f.Images {
				members[img] = true
			}
			for _, img := range images {
				want := img == f.Name || isVariant(img, f.Name)
				if members[img] != want {
					t.Errorf("image %s in family %s: got %v, want %v", img, f.Name, members[img], want)
				}
			}
		}
	})
	t.Run("CM-2.1 os-shell", func(t *testing.T) {
		wantFamily(t, "os-shell", "os-shell-iamguarded", "os-shell-iamguarded-fips")
	})
	t.Run("CM-2.2 gcc-glibc", func(t *testing.T) {
		wantFamily(t, "gcc", "gcc-glibc")
	})
	t.Run("CM-3.1 whole catalog", func(t *testing.T) {
		if len(images) != 3166 || len(families) != 1751 {
			t.Fatalf("got %d images in %d families, want 3166 in 1751", len(images), len(families))
		}
		sizes := map[int]int{}
		for i, f := range families {
			sizes[len(f.Images)]++
			if i > 0 && len(f.Images) > len(families[i-1].Images) {
				t.Errorf("%s (%d images) comes after %s (%d)", f.Name, len(f.Images), families[i-1].Name, len(families[i-1].Images))
			}
		}
		if want := map[int]int{1: 521, 2: 1129, 3: 19, 4: 81, 6: 1}; !maps.Equal(sizes, want) {
			t.Errorf("family sizes = %v, want %v", sizes, want)
		}
		if families[0].Name != "go" || len(families[0].Images) != 6 {
			t.Errorf("first family = %s (%d images), want go (6)", families[0].Name, len(families[0].Images))
		}
	})
	t.Run("CM-3.2 images in name order", func(t *testing.T) {
		for _, f := range families {
			if !slices.IsSorted(f.Images) {
				t.Errorf("%s images not sorted: %v", f.Name, f.Images)
			}
		}
	})
}

func TestImageFamiliesSearch(t *testing.T) {
	images := realCatalog(t)
	svc := &Service{Catalog: &Catalog{TTL: time.Hour, images: images, fetchedAt: time.Now()}}
	search := func(t *testing.T, query string) ImageFamiliesOutput {
		t.Helper()
		out, err := svc.ImageFamilies(context.Background(), ImageFamiliesInput{Query: query})
		if err != nil {
			t.Fatal(err)
		}
		return out
	}

	t.Run("CM-4.1 nginx", func(t *testing.T) {
		out := search(t, "nginx")
		var got []string
		for _, f := range out.Families {
			got = append(got, f.Images...)
		}
		slices.Sort(got)
		if want := matchingImages(images, "nginx"); out.Total != 31 || len(out.Families) != 13 || !slices.Equal(got, want) {
			t.Errorf("got %d images in %d families, want 31 in 13 matching %v", out.Total, len(out.Families), want)
		}
		for _, name := range []string{"nginx-prometheus-exporter", "ingress-nginx"} {
			if findFamily(out.Families, name) == nil {
				t.Errorf("%s should be a family of its own", name)
			}
		}
		if f := findFamily(out.Families, "nginx"); f == nil || len(f.Images) != 4 {
			t.Errorf("nginx family: %+v", f)
		}
	})
	t.Run("CM-4.2 partial family", func(t *testing.T) {
		out := search(t, "FIPS")
		if f := findFamily(out.Families, "nginx"); f == nil || !slices.Equal(f.Images, []string{"nginx-fips", "nginx-iamguarded-fips"}) {
			t.Errorf("nginx family for fips: %+v", f)
		}
		for i := 1; i < len(out.Families); i++ {
			a, b := out.Families[i-1], out.Families[i]
			if len(a.Images) < len(b.Images) || len(a.Images) == len(b.Images) && a.Name > b.Name {
				t.Fatalf("%s before %s breaks the count-then-name order", a.Name, b.Name)
			}
		}
	})
}

// TestCatalogFamiliesFixture keeps the web UI's catalog fixtures in step with
// the Go grouping. With UPDATE_FIXTURES=1 it writes them; otherwise it checks
// them. Comparing decoded values keeps it independent of line endings.
func TestCatalogFamiliesFixture(t *testing.T) {
	svc := &Service{Catalog: &Catalog{TTL: time.Hour, images: realCatalog(t), fetchedAt: time.Now()}}
	families, err := svc.ImageFamilies(context.Background(), ImageFamiliesInput{})
	if err != nil {
		t.Fatal(err)
	}
	// One family or name per line keeps the files small and their diffs readable.
	var fam strings.Builder
	fmt.Fprintf(&fam, "{\"total\": %d, \"families\": [\n", families.Total)
	for i, f := range families.Families {
		b, err := json.Marshal(f)
		if err != nil {
			t.Fatal(err)
		}
		fam.Write(b)
		if i < len(families.Families)-1 {
			fam.WriteByte(',')
		}
		fam.WriteByte('\n')
	}
	fam.WriteString("]}\n")
	free, err := json.MarshalIndent(readNames(t, "free-2026-10-09.txt"), "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	// The map's groups for the whole catalog and a search, one group per line.
	var grp strings.Builder
	grp.WriteString("{\n")
	keys := []string{"variant", "prefix", "variant:nginx", "prefix:nginx"}
	for i, key := range keys {
		by, query, _ := strings.Cut(key, ":")
		out, err := svc.ImageGroups(context.Background(), ImageGroupsInput{Query: query, GroupBy: by})
		if err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(&grp, "%q: {\"group_by\": %q, \"total\": %d, \"groups\": [\n", key, out.GroupBy, out.Total)
		for j, g := range out.Groups {
			b, err := json.Marshal(g)
			if err != nil {
				t.Fatal(err)
			}
			grp.Write(b)
			if j < len(out.Groups)-1 {
				grp.WriteByte(',')
			}
			grp.WriteByte('\n')
		}
		grp.WriteString("]}")
		if i < len(keys)-1 {
			grp.WriteByte(',')
		}
		grp.WriteByte('\n')
	}
	grp.WriteString("}\n")
	fixtures := map[string][]byte{
		"catalogFamilies.json": []byte(fam.String()),
		"catalogFree.json":     append(free, '\n'),
		"catalogGroups.json":   []byte(grp.String()),
	}
	for file, want := range fixtures {
		path := filepath.Join("web", "src", "test", "fixtures", file)
		if os.Getenv("UPDATE_FIXTURES") == "1" {
			if err := os.WriteFile(path, want, 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%v (run with UPDATE_FIXTURES=1 to create it)", err)
		}
		var gotV, wantV any
		if err := json.Unmarshal(got, &gotV); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		json.Unmarshal(want, &wantV)
		if !reflect.DeepEqual(gotV, wantV) {
			t.Errorf("%s is out of date; run UPDATE_FIXTURES=1 go test -run TestCatalogFamiliesFixture ./...", path)
		}
	}
}

func TestImageGroups(t *testing.T) {
	images := realCatalog(t)
	free := readNames(t, "free-2026-10-09.txt")
	type summary struct {
		label        string
		count, freeN int
		folded       int
	}
	summarize := func(groups []ImageGroup) []summary {
		var out []summary
		for _, g := range groups {
			n := 0
			for _, img := range g.Images {
				if slices.Contains(free, img) {
					n++
				}
			}
			out = append(out, summary{g.Label, len(g.Images), n, g.Folded})
		}
		return out
	}
	nginx := matchingImages(images, "nginx")

	t.Run("CM-14.1 whole catalog by variant", func(t *testing.T) {
		groups := groupImages(images, GroupByVariant)
		want := []summary{
			{"Base images", 1730, 58, 0},
			{"FIPS", 1210, 0, 0},
			{"IAM-guarded", 117, 0, 0},
			{"IAM-guarded FIPS", 95, 0, 0},
			{"Other variants", 14, 1, 7},
		}
		if got := summarize(groups); !slices.Equal(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
		other := groups[4].Images
		if !slices.Contains(other, "gcc-glibc") || !slices.ContainsFunc(other, func(s string) bool { return strings.HasSuffix(s, "-openssl-fips") }) {
			t.Errorf("Other variants: %v", other)
		}
	})
	t.Run("CM-14.2 image suffixes", func(t *testing.T) {
		for img, want := range map[string]string{"nginx": "", "node-local-dns": "", "nginx-fips": "fips", "nginx-iamguarded-fips": "iamguarded-fips"} {
			if got := variantKind(img); got != want {
				t.Errorf("variantKind(%q) = %q, want %q", img, got, want)
			}
		}
		if got := variantLabel("iamguarded-fips"); got != "IAM-guarded FIPS" {
			t.Errorf("label = %q", got)
		}
	})
	t.Run("CM-14.3 nginx search by variant", func(t *testing.T) {
		want := []summary{{"Base images", 13, 1, 0}, {"FIPS", 10, 0, 0}, {"IAM-guarded", 4, 0, 0}, {"IAM-guarded FIPS", 4, 0, 0}}
		if got := summarize(groupImages(nginx, GroupByVariant)); !slices.Equal(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("CM-15.1 whole catalog by prefix", func(t *testing.T) {
		got := summarize(groupImages(images, GroupByPrefix))
		want := []summary{
			{"Other", 2335, 59, 690}, {"crossplane", 414, 0, 0}, {"prometheus", 72, 0, 0}, {"kubeflow", 61, 0, 0},
			{"kubernetes", 51, 0, 0}, {"knative", 45, 0, 0}, {"aws", 43, 0, 0}, {"cert", 37, 0, 0},
			{"harbor", 37, 0, 0}, {"gitlab", 36, 0, 0}, {"kube", 35, 0, 0},
		}
		if !slices.Equal(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("CM-15.2 nginx search by prefix", func(t *testing.T) {
		want := []summary{{"nginx", 14, 1, 0}, {"ingress", 9, 0, 0}, {"commercial", 4, 0, 0}, {"privatebin", 2, 0, 0}, {"zabbix", 2, 0, 0}}
		if got := summarize(groupImages(nginx, GroupByPrefix)); !slices.Equal(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("groups split into valid blocks and cover every image once", func(t *testing.T) {
		for _, by := range []string{GroupByVariant, GroupByPrefix} {
			var all []string
			for _, g := range groupImages(images, by) {
				checkBlocks(t, g)
				all = append(all, g.Images...)
			}
			slices.Sort(all)
			if !slices.Equal(all, slices.Sorted(slices.Values(images))) {
				t.Errorf("%s: groups don't partition the catalog", by)
			}
		}
	})
}

// checkBlocks asserts a group's blocks are consecutive runs that cover its
// images, one per prefix, each in name order.
func checkBlocks(t *testing.T, g ImageGroup) {
	t.Helper()
	i, seen := 0, map[string]bool{}
	for _, b := range g.Blocks {
		if seen[b.Prefix] {
			t.Errorf("%s: prefix %q has two blocks", g.Label, b.Prefix)
		}
		seen[b.Prefix] = true
		run := g.Images[i : i+b.Count]
		i += b.Count
		if !slices.IsSorted(run) {
			t.Errorf("%s block %q not in name order", g.Label, b.Prefix)
		}
		for _, img := range run {
			p := namePrefix(img)
			if p != b.Prefix {
				t.Errorf("%s: %s in block %q", g.Label, img, b.Prefix)
			}
		}
	}
	if i != len(g.Images) {
		t.Errorf("%s: blocks cover %d of %d images", g.Label, i, len(g.Images))
	}
}

func TestImageBlocks(t *testing.T) {
	images := realCatalog(t)
	type run struct {
		prefix string
		count  int
	}
	head := func(g ImageGroup, n int) []run {
		var out []run
		for _, b := range g.Blocks[:min(n, len(g.Blocks))] {
			out = append(out, run{b.Prefix, b.Count})
		}
		return out
	}
	byLabel := func(groups []ImageGroup, label string) ImageGroup {
		for _, g := range groups {
			if g.Label == label {
				return g
			}
		}
		t.Fatalf("no group %q", label)
		return ImageGroup{}
	}

	t.Run("CM-18.1 variant groups", func(t *testing.T) {
		groups := groupImages(images, GroupByVariant)
		base, fips := byLabel(groups, "Base images"), byLabel(groups, "FIPS")
		if len(base.Blocks) != 696 || !slices.Equal(head(base, 3), []run{{"crossplane", 220}, {"kubeflow", 39}, {"kubernetes", 25}}) {
			t.Errorf("Base images: %d blocks, %v", len(base.Blocks), head(base, 3))
		}
		if len(fips.Blocks) != 460 || !slices.Equal(head(fips, 1), []run{{"crossplane", 194}}) {
			t.Errorf("FIPS: %d blocks, %v", len(fips.Blocks), head(fips, 2))
		}
	})
	t.Run("CM-18.2 other in the prefix view", func(t *testing.T) {
		groups := groupImages(images, GroupByPrefix)
		other, cross := byLabel(groups, "Other"), byLabel(groups, "crossplane")
		if len(other.Blocks) != 690 || !slices.Equal(head(other, 3), []run{{"flux", 30}, {"sigstore", 30}, {"kserve", 28}}) {
			t.Errorf("Other: %d blocks, %v", len(other.Blocks), head(other, 3))
		}
		if !slices.Equal(cross.Blocks, []ImageBlock{{Prefix: "crossplane", Count: 414}}) {
			t.Errorf("crossplane: %v", cross.Blocks)
		}
	})
	t.Run("CM-18.3 nginx search", func(t *testing.T) {
		base := byLabel(groupImages(matchingImages(images, "nginx"), GroupByVariant), "Base images")
		if want := []run{{"nginx", 5}, {"commercial", 3}, {"ingress", 3}, {"privatebin", 1}, {"zabbix", 1}}; !slices.Equal(head(base, 9), want) {
			t.Errorf("got %v, want %v", head(base, 9), want)
		}
	})
}
