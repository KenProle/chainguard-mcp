package main

import (
	"context"
	"html"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// The project site (site/index.html) is hand-written. These checks enforce
// the wording and structure rules in openspec/specs/project-site: the page is
// flat, controlled markup, so the standard library is enough.

func readSite(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("site", "index.html"))
	if err != nil {
		t.Fatalf("site/index.html: %v", err)
	}
	return string(b)
}

// siteBlock returns the inner HTML of the first <tag ...id="id"> element.
// Sections and the hero are never nested, so the first closing tag ends them.
func siteBlock(t *testing.T, page, tag, id string) string {
	t.Helper()
	re := regexp.MustCompile(`(?s)<` + tag + `\b[^>]*\bid="` + id + `"[^>]*>(.*?)</` + tag + `>`)
	m := re.FindStringSubmatch(page)
	if m == nil {
		t.Fatalf("no <%s id=%q> in site/index.html", tag, id)
	}
	return m[1]
}

func siteFooter(t *testing.T, page string) string {
	t.Helper()
	m := regexp.MustCompile(`(?s)<footer\b[^>]*>(.*?)</footer>`).FindStringSubmatch(page)
	if m == nil {
		t.Fatal("no <footer> in site/index.html")
	}
	return m[1]
}

func plainText(s string) string {
	s = regexp.MustCompile(`(?s)<[^>]*>`).ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	return strings.Join(strings.Fields(s), " ")
}

var imgRe = regexp.MustCompile(`(?s)<img\b[^>]*>`)

func attr(tag, name string) (string, bool) {
	m := regexp.MustCompile(`\b` + name + `="([^"]*)"`).FindStringSubmatch(tag)
	if m == nil {
		return "", false
	}
	return html.UnescapeString(m[1]), true
}

func isRelative(u string) bool {
	l := strings.ToLower(u)
	return u != "" && !strings.HasPrefix(l, "//") && !strings.Contains(l, "://") &&
		!strings.HasPrefix(l, "data:") && !strings.HasPrefix(l, "javascript:")
}

func TestSite(t *testing.T) {
	page := readSite(t)
	const notice = "not affiliated with Chainguard"
	dateRe := regexp.MustCompile(`<time\b[^>]*datetime="\d{4}-\d{2}-\d{2}"`)

	t.Run("PS-1.1 notice in hero and footer", func(t *testing.T) {
		for name, block := range map[string]string{
			"hero":   siteBlock(t, page, "header", "hero"),
			"footer": siteFooter(t, page),
		} {
			if !strings.Contains(plainText(block), notice) {
				t.Errorf("%s doesn't say %q", name, notice)
			}
		}
	})

	t.Run("PS-1.2 only the project's own images", func(t *testing.T) {
		imgs := imgRe.FindAllString(page, -1)
		if len(imgs) == 0 {
			t.Fatal("no images")
		}
		ownScreenshot := regexp.MustCompile(`^screenshots/[a-z0-9-]+\.png$`)
		for _, img := range imgs {
			src, _ := attr(img, "src")
			if src == "icon.svg" {
				continue
			}
			if !ownScreenshot.MatchString(src) {
				t.Errorf("image %q is neither icon.svg nor screenshots/<name>.png", src)
				continue
			}
			if _, err := os.Stat(filepath.Join("docs", "screenshots", path.Base(src))); err != nil {
				t.Errorf("image %q has no file in docs/screenshots: %v", src, err)
			}
		}
		if strings.Contains(strings.ToLower(page), "chainguard.dev/images") {
			t.Error("page references Chainguard artwork")
		}
	})

	t.Run("PS-2.1 no banned wording", func(t *testing.T) {
		lower := strings.ToLower(page)
		if regexp.MustCompile(`\bsecure\b`).MatchString(lower) {
			t.Error(`contains the word "secure"`)
		}
		for _, phrase := range []string{
			"vulnerability-free", "vulnerability free", "cve-free", "cve free",
			"zero vulnerabilities", "zero cves", "no vulnerabilities", "no cves",
		} {
			if strings.Contains(lower, phrase) {
				t.Errorf("contains %q", phrase)
			}
		}
	})

	t.Run("PS-2.2 a zero count is labelled known vulnerabilities", func(t *testing.T) {
		compare := siteBlock(t, page, "section", "compare")
		if !regexp.MustCompile(`(?is)<th\b[^>]*>\s*Known vulnerabilities\s*</th>`).MatchString(compare) {
			t.Error(`the comparison table has no "Known vulnerabilities" column header`)
		}
		for _, want := range []string{"python:3.12-slim", "69", "cgr.dev/chainguard/python"} {
			if !strings.Contains(compare, want) {
				t.Errorf("comparison doesn't show %q", want)
			}
		}
	})

	t.Run("PS-3.1 caveat in the comparison section", func(t *testing.T) {
		text := strings.ToLower(plainText(siteBlock(t, page, "section", "compare")))
		for _, want := range []string{"graded by its own maintainers", "known vulnerabilities"} {
			if !strings.Contains(text, want) {
				t.Errorf("comparison doesn't say %q", want)
			}
		}
	})

	t.Run("PS-4.1 comparison date", func(t *testing.T) {
		compare := siteBlock(t, page, "section", "compare")
		if !strings.Contains(compare, `datetime="2026-10-20"`) {
			t.Error("comparison isn't labelled 2026-10-20")
		}
		for _, want := range []string{"node:20", "875", "393", "398.4 MB", "66.0 MB"} {
			if !strings.Contains(compare, want) {
				t.Errorf("comparison doesn't show %q", want)
			}
		}
	})

	t.Run("PS-4.2 screenshot dates", func(t *testing.T) {
		figs := regexp.MustCompile(`(?s)<figure\b.*?</figure>`).FindAllString(page, -1)
		if len(figs) != 3 {
			t.Fatalf("%d figures, want 3", len(figs))
		}
		for _, f := range figs {
			if !dateRe.MatchString(f) {
				t.Errorf("figure has no <time datetime>: %.80s", plainText(f))
			}
		}
	})

	t.Run("PS-5.1 each client", func(t *testing.T) {
		install := siteBlock(t, page, "section", "install")
		for _, c := range []string{"Claude Code", "Cursor", "Claude Desktop", "VS Code"} {
			if !strings.Contains(install, c) {
				t.Errorf("install section doesn't mention %s", c)
			}
		}
		if n := strings.Count(install, "<details"); n != 4 {
			t.Errorf("%d <details>, want one per client (4)", n)
		}
	})

	t.Run("PS-5.2 latest release", func(t *testing.T) {
		if !strings.Contains(siteBlock(t, page, "section", "install"), "releases/latest") {
			t.Error("install section doesn't link releases/latest")
		}
		if strings.Contains(page, "releases/tag/v") || strings.Contains(page, "releases/download/v") {
			t.Error("page links a numbered release")
		}
	})

	t.Run("PS-5.3 go install has no web UI", func(t *testing.T) {
		install := siteBlock(t, page, "section", "install")
		cmd := "go install github.com/KenProle/chainguard-mcp@latest"
		i := strings.Index(install, cmd)
		if i < 0 {
			t.Fatalf("install section doesn't show %q", cmd)
		}
		rest := install[i+len(cmd):]
		if len(rest) > 400 {
			rest = rest[:400]
		}
		if !strings.Contains(strings.ToLower(plainText(rest)), "no web ui") {
			t.Error(`"go install" isn't followed by a "no web UI" note`)
		}
	})

	toolNames := func(t *testing.T) map[string]bool {
		t.Helper()
		svc := newFakeService(t)
		svc.SBOMDir = t.TempDir() // registers save_sbom
		s := mcpClient(t, svc)
		ctx := context.Background()
		names := map[string]bool{}
		tools, err := s.ListTools(ctx, &mcp.ListToolsParams{})
		if err != nil {
			t.Fatal(err)
		}
		for _, tool := range tools.Tools {
			names[tool.Name] = true
		}
		prompts, err := s.ListPrompts(ctx, &mcp.ListPromptsParams{})
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range prompts.Prompts {
			names[p.Name] = true
		}
		return names
	}
	toolRe := regexp.MustCompile(`<code class="tool">([^<]*)</code>`)

	t.Run("PS-6.1 workflow steps", func(t *testing.T) {
		workflow := siteBlock(t, page, "section", "workflow")
		for _, n := range []string{"find_alternative", "compare_with_upstream", "pin_image", "migrate_dockerfile"} {
			if !strings.Contains(workflow, `<code class="tool">`+n+`</code>`) {
				t.Errorf("workflow doesn't name %s", n)
			}
		}
	})

	t.Run("PS-6.2 only real tool names", func(t *testing.T) {
		real := toolNames(t)
		found := toolRe.FindAllStringSubmatch(page, -1)
		if len(found) == 0 {
			t.Fatal("no <code class=\"tool\"> names on the page")
		}
		for _, m := range found {
			if !real[m[1]] {
				t.Errorf("%q isn't a tool or prompt the server provides", m[1])
			}
		}
	})

	t.Run("PS-7.1 limits section", func(t *testing.T) {
		text := strings.ToLower(plainText(siteBlock(t, page, "section", "limits")))
		for _, want := range []string{"free tier", "docker hub", "os packages"} {
			if !strings.Contains(text, want) {
				t.Errorf("limits don't mention %q", want)
			}
		}
	})

	t.Run("PS-8.1 docs MCP server", func(t *testing.T) {
		related := siteBlock(t, page, "section", "related")
		if !strings.Contains(related, "mcp.edu.chainguard.dev") {
			t.Error("related section doesn't link Chainguard's docs MCP server")
		}
		if !strings.Contains(strings.ToLower(related), "complement") {
			t.Error(`related section doesn't say this tool complements it`)
		}
	})

	t.Run("PS-9.1 no external resources", func(t *testing.T) {
		for _, m := range regexp.MustCompile(`\bsrc="([^"]*)"`).FindAllStringSubmatch(page, -1) {
			if !isRelative(m[1]) {
				t.Errorf("src %q isn't relative", m[1])
			}
		}
		links := regexp.MustCompile(`<link\b[^>]*>`).FindAllString(page, -1)
		sawStyle := false
		for _, l := range links {
			rel, _ := attr(l, "rel")
			href, _ := attr(l, "href")
			if strings.Contains(rel, "stylesheet") {
				sawStyle = true
			}
			if (strings.Contains(rel, "stylesheet") || strings.Contains(rel, "icon")) && !isRelative(href) {
				t.Errorf("<link rel=%q href=%q> isn't relative", rel, href)
			}
		}
		if !sawStyle {
			t.Error("no stylesheet link")
		}
		css, err := os.ReadFile(filepath.Join("site", "style.css"))
		if err != nil {
			t.Fatal(err)
		}
		if regexp.MustCompile(`(?i)@import|://|url\(\s*['"]?(https?:|//|data:)`).Match(css) {
			t.Error("style.css loads or names an external resource")
		}
	})

	t.Run("PS-9.2 Content Security Policy", func(t *testing.T) {
		m := regexp.MustCompile(`(?s)<meta\b[^>]*http-equiv="Content-Security-Policy"[^>]*>`).FindString(page)
		if m == "" {
			t.Fatal("no CSP meta tag")
		}
		content, _ := attr(m, "content")
		if !strings.Contains(content, "default-src 'self'") {
			t.Errorf("CSP %q lacks default-src 'self'", content)
		}
	})

	t.Run("PS-10.1 no scripts", func(t *testing.T) {
		if strings.Contains(strings.ToLower(page), "<script") {
			t.Error("page contains <script")
		}
	})

	t.Run("PS-10.5 image text", func(t *testing.T) {
		for _, img := range imgRe.FindAllString(page, -1) {
			if alt, ok := attr(img, "alt"); !ok || strings.TrimSpace(alt) == "" {
				t.Errorf("image without alt text: %s", img)
			}
		}
	})

	t.Run("PS-11.1 repository links", func(t *testing.T) {
		const repo = "https://github.com/KenProle/chainguard-mcp"
		for _, want := range []string{
			`href="` + repo + `"`,
			repo + `#readme`,
			repo + `/releases/latest`,
			repo + `/blob/main/LICENSE`,
		} {
			if !strings.Contains(page, want) {
				t.Errorf("page doesn't contain %q", want)
			}
		}
	})

	t.Run("PS-12.3 screenshots exist", func(t *testing.T) {
		for _, m := range regexp.MustCompile(`screenshots/[a-z0-9-]+\.png`).FindAllString(page, -1) {
			if _, err := os.Stat(filepath.Join("docs", "screenshots", path.Base(m))); err != nil {
				t.Errorf("%s: %v", m, err)
			}
		}
	})
}
