package main

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// dfcFiles holds Chainguard's dfc (Dockerfile Converter) package mappings,
// unchanged, with the upstream commit they came from and dfc's license. See
// mappings/dfc-source.json and README's "Third-party data".
//
//go:embed mappings
var dfcFiles embed.FS

const (
	dfcMappingsFile = "dfc-builtin-mappings.yaml"
	dfcSourceFile   = "dfc-source.json"
	dfcLicenseFile  = "LICENSE-dfc"
)

// DFCSource says which upstream commit the embedded mapping file came from.
type DFCSource struct {
	Commit  string `json:"commit"`
	Fetched string `json:"fetched"`
}

// DFCMappings is the Debian section of dfc's mapping file.
type DFCMappings struct {
	Source DFCSource
	// Debian maps an apt package name to the Wolfi names dfc gives for it. An
	// empty list means dfc says no replacement is needed.
	Debian map[string][]string
}

// loadDFCMappings reads the embedded snapshot.
func loadDFCMappings() (*DFCMappings, error) {
	src, err := dfcFiles.ReadFile("mappings/" + dfcSourceFile)
	if err != nil {
		return nil, err
	}
	var m DFCMappings
	if err := json.Unmarshal(src, &m.Source); err != nil {
		return nil, fmt.Errorf("%s: %w", dfcSourceFile, err)
	}
	data, err := dfcFiles.ReadFile("mappings/" + dfcMappingsFile)
	if err != nil {
		return nil, err
	}
	if m.Debian, err = parseDFCMappings(data); err != nil {
		return nil, fmt.Errorf("%s: %w", dfcMappingsFile, err)
	}
	return &m, nil
}

// parseDFCMappings is a strict reader for the shape of dfc's
// builtin-mappings.yaml, which has no quoting, anchors, tabs or multi-line
// values. It accepts only:
//
//	blank lines and # comments
//	images:                       4-space "key: value" lines (read, not used)
//	packages:                     4-space "<distro>:" or "<distro>: {}",
//	                              8-space "<name>:" or "<name>: []",
//	                              12-space "- <name>"
//
// Any other line is an error naming its line number. It returns the Debian
// section, and fails if there is none.
func parseDFCMappings(data []byte) (map[string][]string, error) {
	const (
		topNone = iota
		topImages
		topPackages
	)
	var (
		top     = topNone
		distro  string
		name    string
		haveKey bool
		debian  = map[string][]string{}
		sawDeb  bool
	)
	bad := func(n int, format string, args ...any) error {
		return fmt.Errorf("line %d: %s", n, fmt.Sprintf(format, args...))
	}
	for i, line := range strings.Split(string(data), "\n") {
		n := i + 1
		line = strings.TrimSuffix(line, "\r")
		if strings.TrimSpace(line) == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.ContainsAny(line, "\t\"'") {
			return nil, bad(n, "tabs and quotes aren't supported")
		}
		indent := len(line) - len(strings.TrimLeft(line, " "))
		text := line[indent:]
		if strings.HasPrefix(text, "#") {
			return nil, bad(n, "comments must start the line")
		}
		switch indent {
		case 0:
			switch text {
			case "images:":
				top = topImages
			case "packages:":
				top = topPackages
			default:
				return nil, bad(n, "unknown top-level key %q", text)
			}
			distro, haveKey = "", false
		case 4:
			switch top {
			case topImages:
				k, v, ok := strings.Cut(text, ": ")
				if !ok || k == "" || strings.TrimSpace(v) == "" || strings.ContainsAny(v, "[]{}&*!|>") {
					return nil, bad(n, "want \"<image>: <name>\"")
				}
			case topPackages:
				d, rest, ok := strings.Cut(text, ":")
				if !ok || d == "" || strings.ContainsAny(d, " ") || (rest != "" && rest != " {}") {
					return nil, bad(n, "want \"<distro>:\" or \"<distro>: {}\"")
				}
				distro, haveKey = d, false
				if d == "debian" {
					sawDeb = true
				}
			default:
				return nil, bad(n, "line outside images: and packages:")
			}
		case 8:
			if top != topPackages || distro == "" {
				return nil, bad(n, "unexpected 8-space line")
			}
			k, rest, ok := strings.Cut(text, ":")
			if !ok || k == "" || strings.ContainsAny(k, " ") || (rest != "" && rest != " []") {
				return nil, bad(n, "want \"<name>:\" or \"<name>: []\"")
			}
			name, haveKey = k, true
			if distro == "debian" {
				if _, dup := debian[name]; dup {
					return nil, bad(n, "duplicate package %q", name)
				}
				debian[name] = []string{}
			}
		case 12:
			if top != topPackages || !haveKey || !strings.HasPrefix(text, "- ") {
				return nil, bad(n, "want \"- <name>\" under a package")
			}
			target := strings.TrimPrefix(text, "- ")
			if target == "" || strings.ContainsAny(target, " :[]{}") {
				return nil, bad(n, "invalid package name %q", target)
			}
			if distro == "debian" {
				debian[name] = append(debian[name], target)
			}
		default:
			return nil, bad(n, "unexpected indentation of %d spaces", indent)
		}
	}
	if !sawDeb || len(debian) == 0 {
		return nil, errors.New("no Debian package mappings found")
	}
	return debian, nil
}

// Upstream locations used by -refresh-mappings.
const (
	dfcCommitsURL = "https://api.github.com/repos/chainguard-dev/dfc/commits?path=pkg/dfc/builtin-mappings.yaml&per_page=1"
	dfcRawURL     = "https://raw.githubusercontent.com/chainguard-dev/dfc/%s/%s"
	dfcMappingsAt = "pkg/dfc/builtin-mappings.yaml"
)

// DFCRefresher downloads the latest dfc mapping file. The URLs are fields so
// tests can point them at a server.
type DFCRefresher struct {
	HTTP       *http.Client
	CommitsURL string
	RawURL     string // format with the commit and the file path
	Now        func() string
}

func NewDFCRefresher(client *http.Client) *DFCRefresher {
	return &DFCRefresher{HTTP: client, CommitsURL: dfcCommitsURL, RawURL: dfcRawURL}
}

func (r *DFCRefresher) get(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := r.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: unexpected status %s", url, resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 8<<20))
}

// LatestCommit returns the newest upstream commit that changed the mapping file.
func (r *DFCRefresher) LatestCommit(ctx context.Context) (string, error) {
	body, err := r.get(ctx, r.CommitsURL)
	if err != nil {
		return "", err
	}
	var commits []struct {
		SHA string `json:"sha"`
	}
	if err := json.Unmarshal(body, &commits); err != nil || len(commits) == 0 || len(commits[0].SHA) != 40 {
		return "", fmt.Errorf("unexpected commit list from %s", r.CommitsURL)
	}
	return commits[0].SHA, nil
}

// Refresh downloads the latest mapping file, checks it with the strict reader
// and writes it, its commit and dfc's license next to file (which names the
// YAML file). Nothing is written if any step fails. It returns a report of the
// Debian entries added, removed and changed.
func (r *DFCRefresher) Refresh(ctx context.Context, file string) (string, error) {
	sha, err := r.LatestCommit(ctx)
	if err != nil {
		return "", err
	}
	yamlData, err := r.get(ctx, fmt.Sprintf(r.RawURL, sha, dfcMappingsAt))
	if err != nil {
		return "", err
	}
	licenseData, err := r.get(ctx, fmt.Sprintf(r.RawURL, sha, "LICENSE"))
	if err != nil {
		return "", err
	}
	next, err := parseDFCMappings(yamlData)
	if err != nil {
		return "", fmt.Errorf("refusing the downloaded mapping file: %w", err)
	}

	var prev map[string][]string
	if old, err := os.ReadFile(file); err == nil {
		prev, _ = parseDFCMappings(old)
	}
	dir := filepath.Dir(file)
	fetched := ""
	if r.Now != nil {
		fetched = r.Now()
	}
	source, _ := json.Marshal(DFCSource{Commit: sha, Fetched: fetched})
	for _, f := range []struct {
		path string
		data []byte
	}{
		{file, yamlData},
		{filepath.Join(dir, dfcSourceFile), append(source, '\n')},
		{filepath.Join(dir, dfcLicenseFile), licenseData},
	} {
		if err := os.WriteFile(f.path, f.data, 0o644); err != nil {
			return "", err
		}
	}
	return diffDFC(prev, next, sha), nil
}

func diffDFC(prev, next map[string][]string, sha string) string {
	var added, removed, changed []string
	for k, v := range next {
		old, ok := prev[k]
		switch {
		case !ok:
			added = append(added, fmt.Sprintf("%s -> %v", k, v))
		case strings.Join(old, " ") != strings.Join(v, " "):
			changed = append(changed, fmt.Sprintf("%s: %v -> %v", k, old, v))
		}
	}
	for k := range prev {
		if _, ok := next[k]; !ok {
			removed = append(removed, k)
		}
	}
	sort.Strings(added)
	sort.Strings(removed)
	sort.Strings(changed)
	var b strings.Builder
	fmt.Fprintf(&b, "dfc mappings at %s: %d Debian entries\n", sha, len(next))
	if len(added)+len(removed)+len(changed) == 0 {
		b.WriteString("no changes\n")
	}
	for _, group := range []struct {
		label string
		items []string
	}{{"added", added}, {"removed", removed}, {"changed", changed}} {
		for _, it := range group.items {
			fmt.Fprintf(&b, "%s: %s\n", group.label, it)
		}
	}
	return b.String()
}

// mustLoadDFCMappings reads the embedded snapshot. A malformed file can't
// reach a release because TestPM15_2 fails first.
func mustLoadDFCMappings() *DFCMappings {
	m, err := loadDFCMappings()
	if err != nil {
		panic("embedded dfc mappings: " + err.Error())
	}
	return m
}
