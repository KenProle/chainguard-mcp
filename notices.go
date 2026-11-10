package main

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"runtime/debug"
	"sort"
	"strings"
)

// goNoticesFS holds notices/go-notices.json: the license files of the Go
// standard library and of every module the binary links. A test generates it
// (TestGoNoticesFile) and fails when it no longer matches go.mod.
//
//go:embed notices
var goNoticesFS embed.FS

const (
	goNoticesFile   = "notices/go-notices.json"
	uiLicensesFile  = "licenses.json"
	noUIMessage     = "The web UI is not included in this build."
	noticeSeparator = "================================================================================"
	// stdlibNoticePath is the Path of the standard library's entry.
	stdlibNoticePath = "Go standard library and runtime"
)

// NoticeFile is one license file and its text.
type NoticeFile struct {
	Name string `json:"name"`
	Text string `json:"text"`
}

// GoNotice is the notice entry of the Go standard library (no Version) or of
// one Go module.
type GoNotice struct {
	Path    string       `json:"path"`
	Version string       `json:"version,omitempty"`
	Files   []NoticeFile `json:"files"`
}

// UINotice is one package bundled into the web UI, as Vite's build.license
// option records it.
type UINotice struct {
	Name       string `json:"name"`
	Version    string `json:"version"`
	Identifier string `json:"identifier"`
	Text       string `json:"text"`
}

// decodeStrict decodes one JSON document and rejects unknown fields and
// trailing data.
func decodeStrict(data []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	if _, err := dec.Token(); err != io.EOF {
		return errors.New("unexpected data after the JSON document")
	}
	return nil
}

// loadGoNotices reads the embedded Go notices.
func loadGoNotices() ([]GoNotice, error) {
	data, err := goNoticesFS.ReadFile(goNoticesFile)
	if err != nil {
		return nil, err
	}
	return parseGoNotices(data)
}

func parseGoNotices(data []byte) ([]GoNotice, error) {
	var notices []GoNotice
	if err := decodeStrict(data, &notices); err != nil {
		return nil, fmt.Errorf("%s: %w", goNoticesFile, err)
	}
	for _, n := range notices {
		if len(n.Files) == 0 {
			return nil, fmt.Errorf("%s: %s has no license files", goNoticesFile, n.Path)
		}
		for _, f := range n.Files {
			if strings.TrimSpace(f.Text) == "" {
				return nil, fmt.Errorf("%s: %s %s is empty", goNoticesFile, n.Path, f.Name)
			}
		}
	}
	return notices, nil
}

// parseUINotices reads Vite's licenses.json. An unknown shape or an entry
// without license text is an error: it isn't the notice the license asks for.
func parseUINotices(data []byte) ([]UINotice, error) {
	var notices []UINotice
	if err := decodeStrict(data, &notices); err != nil {
		return nil, fmt.Errorf("%s: %w", uiLicensesFile, err)
	}
	for _, n := range notices {
		if n.Name == "" {
			return nil, fmt.Errorf("%s: an entry has no name", uiLicensesFile)
		}
		if strings.TrimSpace(n.Text) == "" {
			return nil, fmt.Errorf("%s: %s has no license text", uiLicensesFile, n.Name)
		}
	}
	sort.SliceStable(notices, func(i, j int) bool {
		if notices[i].Name != notices[j].Name {
			return notices[i].Name < notices[j].Name
		}
		return notices[i].Version < notices[j].Version
	})
	return notices, nil
}

// noticesRegenerateCommand regenerates notices/go-notices.json.
const noticesRegenerateCommand = "UPDATE_FIXTURES=1 go test -run TestGoNoticesFile ./..."

// checkBuildInfo compares the modules the binary was built with against the
// Go notices, so a release built from a commit with a stale notices file
// fails. A nil build info skips the check.
func checkBuildInfo(bi *debug.BuildInfo, notices []GoNotice) error {
	if bi == nil {
		return nil
	}
	built := map[string]string{}
	for _, d := range bi.Deps {
		// Key on the original path and version, as go list reports them; a
		// replace directive changes only where the license files come from.
		built[d.Path] = d.Version
	}
	have := map[string]string{}
	for _, n := range notices {
		if n.Version != "" {
			have[n.Path] = n.Version
		}
	}
	var problems []string
	for p, v := range built {
		switch hv, ok := have[p]; {
		case !ok:
			problems = append(problems, fmt.Sprintf("%s %s is in the binary but has no notice", p, v))
		case hv != v:
			problems = append(problems, fmt.Sprintf("%s is %s in the binary but its notice is for %s", p, v, hv))
		}
	}
	for p, v := range have {
		if _, ok := built[p]; !ok {
			problems = append(problems, fmt.Sprintf("%s %s has a notice but is not in the binary", p, v))
		}
	}
	if len(problems) == 0 {
		return nil
	}
	sort.Strings(problems)
	return fmt.Errorf("the license notices don't match the modules in this binary (regenerate with: %s):\n  %s", noticesRegenerateCommand, strings.Join(problems, "\n  "))
}

// renderNotices builds the plain-text notices for ver. mappings holds the dfc
// files (under mappings/) and ui the embedded web UI. The result depends only
// on the inputs, so repeated runs are byte-identical.
func renderNotices(ver string, goNotices []GoNotice, mappings fs.FS, ui fs.FS, bi *debug.BuildInfo) (string, error) {
	if err := checkBuildInfo(bi, goNotices); err != nil {
		return "", err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "chainguard-mcp %s third-party notices\n\n", ver)
	b.WriteString("This program includes the components listed below. Each entry gives the\ncomponent's license files as its authors distribute them.\n")

	entry := func(title string) {
		fmt.Fprintf(&b, "\n%s\n%s\n", noticeSeparator, title)
	}
	file := func(name, text string) {
		fmt.Fprintf(&b, "\n--- %s ---\n%s\n", name, strings.TrimRight(strings.ReplaceAll(text, "\r\n", "\n"), "\n"))
	}

	sorted := append([]GoNotice(nil), goNotices...)
	sort.SliceStable(sorted, func(i, j int) bool {
		si, sj := sorted[i].Path == stdlibNoticePath, sorted[j].Path == stdlibNoticePath
		if si != sj {
			return si
		}
		return sorted[i].Path < sorted[j].Path
	})
	for _, n := range sorted {
		title := n.Path
		if n.Version != "" {
			title += " " + n.Version
		}
		entry(title)
		for _, f := range n.Files {
			file(f.Name, f.Text)
		}
	}

	src, err := fs.ReadFile(mappings, "mappings/"+dfcSourceFile)
	if err != nil {
		return "", err
	}
	var dfc DFCSource
	if err := json.Unmarshal(src, &dfc); err != nil {
		return "", fmt.Errorf("%s: %w", dfcSourceFile, err)
	}
	lic, err := fs.ReadFile(mappings, "mappings/"+dfcLicenseFile)
	if err != nil {
		return "", err
	}
	entry("Chainguard dfc package mappings, commit " + dfc.Commit)
	file(dfcLicenseFile, string(lic))

	if _, err := fs.Stat(ui, "index.html"); err != nil {
		entry("Web UI")
		b.WriteString("\n" + noUIMessage + "\n")
		return b.String(), nil
	}
	data, err := fs.ReadFile(ui, uiLicensesFile)
	if err != nil {
		return "", fmt.Errorf("the web UI is built but its license notices (%s) are missing; run npm --prefix web run build: %w", uiLicensesFile, err)
	}
	uiNotices, err := parseUINotices(data)
	if err != nil {
		return "", err
	}
	for _, n := range uiNotices {
		title := n.Name
		if n.Version != "" {
			title += " " + n.Version
		}
		entry(title + " (web UI)")
		name := "license text"
		if n.Identifier != "" {
			name = "license text (" + n.Identifier + ")"
		}
		file(name, n.Text)
	}
	return b.String(), nil
}

// licensesText is what -licenses prints for this binary.
func licensesText() (string, error) {
	gn, err := loadGoNotices()
	if err != nil {
		return "", err
	}
	bi, _ := debug.ReadBuildInfo()
	return renderNotices(version, gn, dfcFiles, embeddedUI(), bi)
}
