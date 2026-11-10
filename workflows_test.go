package main

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// The vulnerability check and Dependabot configuration are plain YAML that
// GitHub reads; these checks pin the parts of openspec/specs/
// dependency-vulnerability-checks that a stray edit could quietly undo. The
// files are small and hand-written, so line-level text checks are enough.

func readRepoFile(t *testing.T, parts ...string) string {
	t.Helper()
	p := filepath.Join(parts...)
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("%s: %v", p, err)
	}
	return strings.ReplaceAll(string(b), "\r\n", "\n")
}

// yamlLines returns the file's lines with comments and blank lines removed.
func yamlLines(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if i := strings.Index(l, "#"); i >= 0 && !strings.Contains(l[:i], `"`) {
			l = l[:i]
		}
		if strings.TrimSpace(l) != "" {
			out = append(out, strings.TrimRight(l, " "))
		}
	}
	return out
}

func TestWorkflows(t *testing.T) {
	vuln := readRepoFile(t, ".github", "workflows", "vulncheck.yml")
	lines := yamlLines(vuln)
	body := strings.Join(lines, "\n")

	t.Run("DV-5.1 govulncheck version is exact", func(t *testing.T) {
		re := regexp.MustCompile(`go run golang\.org/x/vuln/cmd/govulncheck@(\S+) \./\.\.\.`)
		m := re.FindAllStringSubmatch(body, -1)
		if len(m) != 1 {
			t.Fatalf("want one `go run golang.org/x/vuln/cmd/govulncheck@<version> ./...` step, found %d", len(m))
		}
		if !regexp.MustCompile(`^v\d+\.\d+\.\d+$`).MatchString(m[0][1]) {
			t.Errorf("govulncheck version %q is not an exact vX.Y.Z", m[0][1])
		}
		// CLAUDE.md's commands block runs the same pin locally.
		for _, d := range re.FindAllStringSubmatch(readRepoFile(t, "CLAUDE.md"), -1) {
			if d[1] != m[0][1] {
				t.Errorf("CLAUDE.md runs govulncheck@%s, the workflow @%s", d[1], m[0][1])
			}
		}
	})

	t.Run("DV-3.2 vulncheck workflow has one job and the four triggers", func(t *testing.T) {
		// Top-level keys under `on:` and `jobs:` are indented two spaces.
		section := func(name string) []string {
			var keys []string
			in := false
			for _, l := range lines {
				if !strings.HasPrefix(l, " ") {
					in = l == name+":"
					continue
				}
				if in && strings.HasPrefix(l, "  ") && !strings.HasPrefix(l, "   ") {
					keys = append(keys, strings.TrimSuffix(strings.TrimSpace(l), ":"))
				}
			}
			return keys
		}
		on := section("on")
		sort.Strings(on)
		if triggers := strings.Join(on, ","); triggers != "pull_request,push,schedule,workflow_dispatch" {
			t.Errorf("triggers = %q, want pull_request,push,schedule,workflow_dispatch", triggers)
		}
		if !strings.Contains(body, "    branches: [main]") {
			t.Error("push trigger is not limited to main")
		}
		if !regexp.MustCompile(`- cron: "[^"]+"`).MatchString(body) {
			t.Error("schedule has no cron line")
		}
		jobs := section("jobs")
		if len(jobs) != 1 {
			t.Fatalf("jobs = %v, want exactly one", jobs)
		}
		// The job's own keys are indented four spaces; steps sit deeper.
		named := false
		for _, l := range lines {
			named = named || l == "    name: Vulnerability check"
		}
		if !named {
			t.Error(`the job is not named "Vulnerability check", the name the ruleset requires`)
		}
	})

	t.Run("DV-2.1 npm audit fails on high and includes dev dependencies", func(t *testing.T) {
		re := regexp.MustCompile(`(?m)^\s+run: npm audit(.*)$`)
		m := re.FindAllStringSubmatch(body, -1)
		if len(m) != 1 {
			t.Fatalf("want one `run: npm audit ...` step, found %d", len(m))
		}
		args := strings.Fields(m[0][1])
		if strings.Join(args, " ") != "--audit-level=high" {
			t.Errorf("npm audit arguments = %q, want only --audit-level=high (no --omit, no --production)", args)
		}
		if !strings.Contains(body, "working-directory: web") {
			t.Error("npm audit doesn't run in web/")
		}
	})

	t.Run("DV-7.2 Dependabot covers only github-actions", func(t *testing.T) {
		dep := strings.Join(yamlLines(readRepoFile(t, ".github", "dependabot.yml")), "\n")
		eco := regexp.MustCompile(`package-ecosystem:\s*"?([\w-]+)"?`).FindAllStringSubmatch(dep, -1)
		if len(eco) != 1 || eco[0][1] != "github-actions" {
			t.Errorf("ecosystems = %v, want only github-actions", eco)
		}
		for _, want := range []string{"interval: monthly", `patterns: ["*"]`} {
			if !strings.Contains(dep, want) {
				t.Errorf("dependabot.yml lacks %q", want)
			}
		}
	})

	t.Run("DV-6.2 go.mod has a toolchain that the Go workflows read", func(t *testing.T) {
		mod := readRepoFile(t, "go.mod")
		if !regexp.MustCompile(`(?m)^toolchain go\d+\.\d+\.\d+$`).MatchString(mod) {
			t.Error("go.mod has no `toolchain goX.Y.Z` line")
		}
		files, err := filepath.Glob(filepath.Join(".github", "workflows", "*.yml"))
		if err != nil || len(files) == 0 {
			t.Fatalf("no workflows found: %v", err)
		}
		for _, f := range files {
			wf := strings.Join(yamlLines(readRepoFile(t, f)), "\n")
			setups := strings.Count(wf, "uses: actions/setup-go@")
			reads := strings.Count(wf, "go-version-file: go.mod")
			if setups != reads {
				t.Errorf("%s: %d setup-go steps but %d read go-version-file: go.mod", f, setups, reads)
			}
			if strings.Contains(wf, "go-version:") {
				t.Errorf("%s sets go-version directly instead of reading go.mod", f)
			}
		}
	})
}
