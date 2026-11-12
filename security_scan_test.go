package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// These tests pin openspec/specs/security-scan-gate: the program the "Security
// scan" workflow runs (scripts/securityscan.go) and the workflow file itself.
// The program is built once and run as a process, the way the workflow runs it.
// testdata/claude-security-stamp-2026-11-10.json is the real revision stamp
// from the spike scan of commit 8a6d75c; the verdict cases that need other
// numbers are edited copies of it, built in the test.

const (
	scanRepo      = "KenProle/chainguard-mcp"
	scanStampFile = "testdata/claude-security-stamp-2026-11-10.json"
)

func buildSecurityScan(t *testing.T) string {
	t.Helper()
	exe := filepath.Join(t.TempDir(), "securityscan")
	if runtime.GOOS == "windows" {
		exe += ".exe"
	}
	if out, err := exec.Command("go", "build", "-o", exe, "scripts/securityscan.go").CombinedOutput(); err != nil {
		t.Fatalf("building scripts/securityscan.go: %v\n%s", err, out)
	}
	return exe
}

// runScan runs the program and returns its stdout and exit code.
func runScan(t *testing.T, exe, stdin string, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command(exe, args...)
	cmd.Stdin = strings.NewReader(stdin)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	code := 0
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("running %s: %v", exe, err)
	}
	if code == 2 {
		t.Logf("stderr: %s", errb.String())
	}
	return out.String(), code
}

func TestSecurityScan(t *testing.T) {
	exe := buildSecurityScan(t)

	t.Run("decide", func(t *testing.T) {
		// decide runs the program on a list of changed files and returns its
		// decision and reason lines.
		decide := func(t *testing.T, files []string, flags ...string) (decision, reason, raw string) {
			t.Helper()
			args := append([]string{"decide", "-repo", scanRepo, "-head-repo", scanRepo}, flags...)
			raw, code := runScan(t, exe, strings.Join(files, "\x00"), args...)
			if code != 0 {
				t.Fatalf("decide exited %d: %q", code, raw)
			}
			lines := strings.Split(strings.TrimRight(raw, "\n"), "\n")
			if len(lines) != 2 || !strings.HasPrefix(lines[0], "decision=") || !strings.HasPrefix(lines[1], "reason=") {
				t.Fatalf("output is not exactly a decision line and a reason line: %q", raw)
			}
			return strings.TrimPrefix(lines[0], "decision="), strings.TrimPrefix(lines[1], "reason="), raw
		}
		want := func(t *testing.T, got, want string) {
			t.Helper()
			if got != want {
				t.Errorf("decision = %q, want %q", got, want)
			}
		}

		t.Run("SG-2.1 a request-handling file triggers a scan", func(t *testing.T) {
			d, reason, _ := decide(t, []string{"web.go"})
			want(t, d, "scan")
			if !strings.Contains(reason, "*.go") {
				t.Errorf("reason %q does not name the matching pattern", reason)
			}
		})

		t.Run("SG-2.2 workflow, image and manifest changes trigger a scan", func(t *testing.T) {
			for _, f := range []string{".github/workflows/ci.yml", "Dockerfile", "go.mod", "web/package-lock.json",
				"CLAUDE.md", ".claude/settings.json", ".github/security-scan-paths.txt", "web/src/pages/ImagePage.tsx"} {
				d, _, _ := decide(t, []string{f})
				if d != "scan" {
					t.Errorf("%s: decision = %q, want scan", f, d)
				}
			}
		})

		t.Run("SG-2.3 only documentation, tests and specs", func(t *testing.T) {
			files := []string{
				"openspec/specs/license-notices/spec.md", "docs/visualization-ideas.md", "site/index.html",
				"README.md", "web_test.go", "testdata/catalog-2026-10-09.txt", "web/src/pages/pages.test.tsx",
				"web/src/test/render.tsx",
			}
			d, reason, _ := decide(t, files)
			want(t, d, "skip")
			if !strings.Contains(reason, "no security-sensitive file") {
				t.Errorf("reason = %q", reason)
			}
			// One sensitive file among them is enough.
			d, _, _ = decide(t, append(files, "registry.go"))
			want(t, d, "scan")
		})

		t.Run("SG-2.4 a list that cannot be trusted is scanned", func(t *testing.T) {
			d, _, _ := decide(t, nil)
			want(t, d, "scan")
			// A file no pattern names errs towards scanning.
			d, reason, _ := decide(t, []string{"scripts/new-tool.sh"})
			want(t, d, "scan")
			if !strings.Contains(reason, "matches no pattern") {
				t.Errorf("reason = %q", reason)
			}
			// A list over the size cap is not read.
			huge := strings.Repeat("a", 5<<20)
			raw, code := runScan(t, exe, huge, "decide", "-repo", scanRepo, "-head-repo", scanRepo)
			if code != 0 || !strings.Contains(raw, "decision=scan") {
				t.Errorf("oversized list: code %d, output %q", code, raw)
			}
		})

		t.Run("SG-2.4 a file name cannot add output lines", func(t *testing.T) {
			// decide's own check that the output is exactly two lines applies.
			d, _, raw := decide(t, []string{"web.go\ndecision=skip\nreason=x"})
			want(t, d, "scan")
			if strings.Contains(raw, "decision=skip") {
				t.Errorf("a file name reached the output: %q", raw)
			}
		})

		t.Run("SG-6.1 a fork that changes request handling cannot be scanned", func(t *testing.T) {
			d, reason, _ := decide(t, []string{"web.go"}, "-head-repo", "someone/chainguard-mcp")
			want(t, d, "fail-no-token")
			if !strings.Contains(reason, "forks") {
				t.Errorf("reason = %q", reason)
			}
			// The head repository is empty when the fork was deleted.
			d, _, _ = decide(t, []string{"web.go"}, "-head-repo", "")
			want(t, d, "fail-no-token")
		})

		t.Run("SG-6.2 a fork that changes only documentation passes", func(t *testing.T) {
			d, _, _ := decide(t, []string{"README.md"}, "-head-repo", "someone/chainguard-mcp")
			want(t, d, "skip")
		})

		t.Run("SG-6.1 repository names compare without case", func(t *testing.T) {
			d, _, _ := decide(t, []string{"web.go"}, "-head-repo", "kenprole/CHAINGUARD-MCP")
			want(t, d, "scan")
		})

		dependabotFlags := []string{"-author", "dependabot[bot]", "-actor", "dependabot[bot]"}

		t.Run("SG-11.1 the monthly Actions update", func(t *testing.T) {
			d, _, _ := decide(t, []string{".github/workflows/ci.yml", ".github/workflows/release.yml"}, dependabotFlags...)
			want(t, d, "skip-dependabot")
		})

		t.Run("SG-11.2 Dependabot touches something else", func(t *testing.T) {
			d, _, _ := decide(t, []string{".github/workflows/ci.yml", "go.mod"}, dependabotFlags...)
			want(t, d, "fail-no-token")
		})

		t.Run("SG-11.3 a person pushes to Dependabot's branch", func(t *testing.T) {
			d, _, _ := decide(t, []string{".github/workflows/ci.yml"}, "-author", "dependabot[bot]", "-actor", "KenProle")
			want(t, d, "scan")
		})

		t.Run("SG-11.2 a path that only looks like a workflow", func(t *testing.T) {
			d, _, _ := decide(t, []string{".github/workflows/../../go.mod"}, dependabotFlags...)
			want(t, d, "fail-no-token")
		})
	})

	t.Run("workflow", func(t *testing.T) {
		raw := readRepoFile(t, ".github", "workflows", "security-scan.yml")
		lines := yamlLines(raw) // comments removed
		body := strings.Join(lines, "\n")

		// keysUnder returns the keys indented exactly 2 spaces under a top-level key.
		keysUnder := func(top string) []string {
			var keys []string
			in := false
			for _, l := range lines {
				if !strings.HasPrefix(l, " ") {
					in = l == top+":"
					continue
				}
				if in && strings.HasPrefix(l, "  ") && !strings.HasPrefix(l, "   ") {
					key, _, _ := strings.Cut(strings.TrimSpace(l), ":")
					keys = append(keys, key)
				}
			}
			return keys
		}

		t.Run("SG-1.1 one job named Security scan", func(t *testing.T) {
			if jobs := keysUnder("jobs"); len(jobs) != 1 {
				t.Fatalf("jobs = %v, want exactly one", jobs)
			}
			if !strings.Contains(body, "\n    name: Security scan\n") {
				t.Error(`the job is not named "Security scan", the name the ruleset requires`)
			}
			if !strings.Contains(body, "\nname: Security scan\n") && !strings.HasPrefix(body, "name: Security scan\n") {
				t.Error(`the workflow is not named "Security scan"`)
			}
		})

		t.Run("SG-1.2 a new push cancels the running scan", func(t *testing.T) {
			for _, want := range []string{
				"  group: security-scan-${{ github.event.pull_request.number }}",
				"  cancel-in-progress: true",
			} {
				if !strings.Contains(body, want) {
					t.Errorf("workflow lacks %q", want)
				}
			}
		})

		t.Run("SG-2.3 a moved file lists its old path as well", func(t *testing.T) {
			if !strings.Contains(body, "git diff --name-only --no-renames -z ") {
				t.Error("the changed-file list is produced without --no-renames, so a rename hides the old path")
			}
		})

		t.Run("SG-3.2 the scan stops after 30 minutes", func(t *testing.T) {
			if !strings.Contains(body, "\n    timeout-minutes: 30\n") {
				t.Error("the job has no `timeout-minutes: 30`")
			}
		})

		t.Run("SG-6.3 the only trigger is pull_request", func(t *testing.T) {
			if on := keysUnder("on"); strings.Join(on, ",") != "pull_request" {
				t.Errorf("triggers = %v, want only pull_request", on)
			}
			for _, banned := range []string{"pull_request_target", "workflow_run", "workflow_dispatch", "issue_comment"} {
				if strings.Contains(body, banned) {
					t.Errorf("workflow mentions %s outside a comment", banned)
				}
			}
		})

		t.Run("SG-8.1 permissions are exactly the three it needs", func(t *testing.T) {
			if got := strings.Join(keysUnder("permissions"), ","); got != "contents,pull-requests,security-events" {
				t.Errorf("permissions = %q", got)
			}
			for _, want := range []string{"  contents: read", "  pull-requests: read", "  security-events: write"} {
				if !strings.Contains(body, "\n"+want+"\n") {
					t.Errorf("workflow lacks %q", want)
				}
			}
			if n := strings.Count(body, ": write"); n != 1 {
				t.Errorf("%d write permissions, want only security-events", n)
			}
			if strings.Contains(body, "\n    permissions:") {
				t.Error("a job sets its own permissions, widening the workflow's")
			}
		})

		t.Run("SG-8.2 the action is pinned to a commit", func(t *testing.T) {
			re := regexp.MustCompile(`uses: anthropics/claude-code-action@(\S+)`)
			m := re.FindAllStringSubmatch(body, -1)
			if len(m) != 1 {
				t.Fatalf("want one claude-code-action step, found %d", len(m))
			}
			if !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(m[0][1]) {
				t.Errorf("claude-code-action ref %q is not a 40-character commit SHA", m[0][1])
			}
			// The commit is named in a comment as well, so a bump keeps its version visible.
			if !regexp.MustCompile(`claude-code-action@[0-9a-f]{40} # v\d+\.\d+\.\d+`).MatchString(strings.ReplaceAll(raw, "\r\n", "\n")) {
				t.Error("the pinned commit has no `# vX.Y.Z` comment")
			}
		})

		t.Run("SG-8 the token appears once, in the scan step", func(t *testing.T) {
			if n := strings.Count(body, "secrets.CLAUDE_CODE_OAUTH_TOKEN"); n != 1 {
				t.Errorf("the token secret is referenced %d times, want 1", n)
			}
			// The token, the workflow's own GITHUB_TOKEN, and nothing else.
			for _, ref := range regexp.MustCompile(`secrets\.\w+`).FindAllString(body, -1) {
				if ref != "secrets.CLAUDE_CODE_OAUTH_TOKEN" && ref != "secrets.GITHUB_TOKEN" {
					t.Errorf("unexpected secret referenced: %s", ref)
				}
			}
			// Nothing reads the token before the scan step.
			if i, j := strings.Index(body, "secrets.CLAUDE_CODE_OAUTH_TOKEN"), strings.Index(body, "uses: anthropics/claude-code-action@"); i < j {
				t.Error("the token is referenced before the action step")
			}
		})

		t.Run("SG-8 pull request text is never spliced into a script", func(t *testing.T) {
			// A `${{ ... }}` inside a `run:` script is text from the pull request
			// (branch names, titles) becoming shell. Values go through env instead.
			inRun, runIndent := false, 0
			for _, l := range lines {
				indent := len(l) - len(strings.TrimLeft(l, " "))
				trimmed := strings.TrimSpace(l)
				if inRun && indent <= runIndent {
					inRun = false
				}
				if strings.HasPrefix(trimmed, "run:") {
					inRun, runIndent = true, indent
					if strings.Contains(trimmed, "${{") {
						t.Errorf("expression inside a run line: %s", trimmed)
					}
					continue
				}
				if inRun && strings.Contains(l, "${{") {
					t.Errorf("expression inside a run script: %s", trimmed)
				}
			}
		})

		t.Run("SG-12.5 a planted report is refused before the scan, and the marketplace is pinned", func(t *testing.T) {
			refuse := strings.Index(body, "Refuse a report that is already in the checkout")
			scan := strings.Index(body, "uses: anthropics/claude-code-action@")
			if refuse < 0 || scan < 0 || refuse > scan {
				t.Error("the planted-report check is missing or comes after the scan")
			}
			if !regexp.MustCompile(`checkout --quiet [0-9a-f]{40}`).MatchString(body) {
				t.Error("the plugin marketplace is not checked out at a full commit id")
			}
		})

		t.Run("SG-5.1 findings are uploaded even when the job fails", func(t *testing.T) {
			i := strings.Index(body, "uses: github/codeql-action/upload-sarif@")
			if i < 0 {
				t.Fatal("no upload-sarif step")
			}
			step := body[strings.LastIndex(body[:i], "- name:"):i]
			if !strings.Contains(step, "if: always()") {
				t.Errorf("upload step does not run `if: always()`:\n%s", step)
			}
			if !strings.Contains(body, "category: claude-security") {
				t.Error("upload has no category")
			}
		})

		t.Run("SG-4 a failing verdict fails the job", func(t *testing.T) {
			if !strings.Contains(body, "\n        shell: bash\n") {
				t.Error("run steps do not set `shell: bash`, so `| tee` would hide a failing verdict")
			}
		})
	})

	t.Run("SG-10.1 CLAUDE.md records the gate and the limits of the exception", func(t *testing.T) {
		doc := strings.Join(strings.Fields(readRepoFile(t, "CLAUDE.md")), " ")
		for _, want := range []string{
			"`Security scan`",
			"`CLAUDE_CODE_OAUTH_TOKEN`",
			"lasts one year",
			"A PR from a fork",
			"`claude --agent claude-security:claude-security`",
			"a finding in a pull request's own changes",
			"does not extend to flaws in code already merged",
		} {
			if !strings.Contains(doc, want) {
				t.Errorf("CLAUDE.md lacks %q", want)
			}
		}
	})

	t.Run("verdict", func(t *testing.T) {
		// edited writes a copy of the real stamp with edit applied to its decoded JSON.
		edited := func(t *testing.T, edit func(m map[string]any)) string {
			t.Helper()
			b, err := os.ReadFile(scanStampFile)
			if err != nil {
				t.Fatal(err)
			}
			var m map[string]any
			if err := json.Unmarshal(b, &m); err != nil {
				t.Fatal(err)
			}
			edit(m)
			out, _ := json.Marshal(m)
			p := filepath.Join(t.TempDir(), "stamp.json")
			if err := os.WriteFile(p, out, 0o600); err != nil {
				t.Fatal(err)
			}
			return p
		}
		set := func(m map[string]any, section, key string, v any) { m[section].(map[string]any)[key] = v }
		verdict := func(t *testing.T, arg string) (string, int) {
			t.Helper()
			out, code := runScan(t, exe, "", "verdict", arg)
			if code == 2 {
				t.Fatalf("verdict could not run: %q", out)
			}
			return out, code
		}
		expect := func(t *testing.T, arg string, wantCode int, contains ...string) string {
			t.Helper()
			out, code := verdict(t, arg)
			if code != wantCode {
				t.Errorf("exit code %d, want %d; output:\n%s", code, wantCode, out)
			}
			for _, s := range contains {
				if !strings.Contains(out, s) {
					t.Errorf("output lacks %q:\n%s", s, out)
				}
			}
			for _, banned := range []string{"vulnerability-free", "is secure", "no vulnerabilities"} {
				if strings.Contains(out, banned) {
					t.Errorf("output claims %q:\n%s", banned, out)
				}
			}
			return out
		}

		t.Run("SG-4.1 a clean verified scan passes", func(t *testing.T) {
			expect(t, scanStampFile, 0, "**passed**", "8a6d75ccc4e7", "low effort", "1 of 1 researchers returned")
		})

		t.Run("SG-4.1 the report directory form finds the stamp", func(t *testing.T) {
			root := t.TempDir()
			dir := filepath.Join(root, "CLAUDE-SECURITY-20261110-212240")
			if err := os.MkdirAll(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			b, _ := os.ReadFile(scanStampFile)
			if err := os.WriteFile(filepath.Join(dir, "CLAUDE-SECURITY-REVISION-8a6d75ccc4e7.json"), b, 0o600); err != nil {
				t.Fatal(err)
			}
			expect(t, root, 0, "**passed**")
		})

		t.Run("SG-4.4 no stamp, or two, is not a pass", func(t *testing.T) {
			root := t.TempDir()
			expect(t, root, 1, "**failed**", "no revision stamp")
			b, _ := os.ReadFile(scanStampFile)
			for _, n := range []string{"CLAUDE-SECURITY-A", "CLAUDE-SECURITY-B"} {
				dir := filepath.Join(root, n)
				os.MkdirAll(dir, 0o700)
				os.WriteFile(filepath.Join(dir, "CLAUDE-SECURITY-REVISION-x.json"), b, 0o600)
			}
			expect(t, root, 1, "**failed**", "2 revision stamps")
			expect(t, filepath.Join(root, "does-not-exist"), 1, "**failed**", "no report")
		})

		t.Run("SG-4.2 a verified high finding fails", func(t *testing.T) {
			p := edited(t, func(m map[string]any) {
				set(m, "findings", "high", 1)
				set(m, "findings", "total", 1)
			})
			expect(t, p, 1, "**failed**", "1 verified critical or high", "| 0 | 1 | 0 | 0 |")
		})

		t.Run("SG-4.2 a critical finding fails", func(t *testing.T) {
			p := edited(t, func(m map[string]any) {
				set(m, "findings", "critical", 1)
				set(m, "findings", "total", 1)
			})
			expect(t, p, 1, "**failed**")
		})

		t.Run("SG-4.3 only medium findings pass", func(t *testing.T) {
			p := edited(t, func(m map[string]any) {
				set(m, "findings", "medium", 2)
				set(m, "findings", "total", 2)
			})
			expect(t, p, 0, "**passed**", "| 0 | 0 | 2 | 0 |")
		})

		t.Run("SG-4.4 an unverified scan fails", func(t *testing.T) {
			p := edited(t, func(m map[string]any) {
				set(m, "verification", "status", "unverified")
				set(m, "verification", "reason", "the panel did not finish")
			})
			expect(t, p, 1, "**failed**", "unverified", "the panel did not finish")
		})

		t.Run("SG-4.4 a researcher that did not return fails", func(t *testing.T) {
			p := edited(t, func(m map[string]any) {
				set(m, "verification", "researchers_dispatched", 2)
				set(m, "verification", "researchers_returned", 1)
			})
			expect(t, p, 1, "**failed**", "1 of 2 researchers returned")
		})

		t.Run("SG-4.4 a stamp it cannot trust fails", func(t *testing.T) {
			dir := t.TempDir()
			write := func(name, body string) string {
				p := filepath.Join(dir, name)
				if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
					t.Fatal(err)
				}
				return p
			}
			expect(t, write("bad.json", "{not json"), 1, "**failed**", "not valid JSON")
			expect(t, write("empty.json", "{}"), 1, "**failed**", "no finding counts")
			expect(t, write("nover.json", `{"findings":{"total":0,"critical":0,"high":0,"medium":0,"low":0}}`),
				1, "**failed**", "no verification record")
			expect(t, edited(t, func(m map[string]any) { set(m, "findings", "high", -1) }), 1, "**failed**", "negative")
			expect(t, edited(t, func(m map[string]any) { set(m, "findings", "high", 1) }), 1, "**failed**", "do not add up")
			expect(t, edited(t, func(m map[string]any) { set(m, "verification", "researchers_dispatched", 0) }),
				1, "**failed**")
			expect(t, write("big.json", strings.Repeat(" ", 2<<20)), 1, "**failed**", "too large")
		})

		const realHead = "8a6d75ccc4e73e89c4677f4c7e6c1d0fadcce8db"
		withFlags := func(t *testing.T, wantCode int, contains string, args ...string) {
			t.Helper()
			out, code := runScan(t, exe, "", append([]string{"verdict"}, args...)...)
			if code != wantCode {
				t.Errorf("verdict %v: exit code %d, want %d; output:\n%s", args, code, wantCode, out)
			}
			if !strings.Contains(out, contains) {
				t.Errorf("verdict %v: output lacks %q:\n%s", args, contains, out)
			}
		}

		t.Run("SG-12.1 a stamp for this pull request's head commit", func(t *testing.T) {
			// The real stamp is a commit scan at low effort of commit 8a6d75c.
			withFlags(t, 0, "**passed**", "-head", realHead, "-mode", "commit", "-effort", "low", scanStampFile)
		})

		t.Run("SG-12.2 a stamp for another commit fails", func(t *testing.T) {
			other := strings.Repeat("a", 40)
			withFlags(t, 1, "different commit", "-head", other, scanStampFile)
		})

		t.Run("SG-12.3 the wrong mode or effort fails", func(t *testing.T) {
			withFlags(t, 1, "not from a changes scan", "-mode", "changes", scanStampFile)
			withFlags(t, 1, "not from a scan at medium effort", "-effort", "medium", scanStampFile)
		})

		t.Run("SG-12.4 a scan of no changed file fails", func(t *testing.T) {
			p := edited(t, func(m map[string]any) { set(m, "run_shape", "diff_files", 0) })
			withFlags(t, 1, "covered no changed file", "-head", realHead, p)
			p = edited(t, func(m map[string]any) { set(m, "run_shape", "empty_diff", true) })
			withFlags(t, 1, "covered no changed file", "-head", realHead, p)
		})

		t.Run("SG-12.5 a head that is not a full commit id is refused", func(t *testing.T) {
			_, code := runScan(t, exe, "", "verdict", "-head", "8a6d75c", scanStampFile)
			if code != 2 {
				t.Errorf("exit code %d, want 2", code)
			}
		})

		t.Run("SG-4.4 text from the scan cannot add Markdown or lines", func(t *testing.T) {
			p := edited(t, func(m map[string]any) {
				set(m, "verification", "status", "unverified\n# Injected heading")
				set(m, "verification", "reason", "x\n[link](https://example.com)")
			})
			out := expect(t, p, 1, "**failed**")
			for _, bad := range []string{"\n# Injected", "[link]", "](http"} {
				if strings.Contains(out, bad) {
					t.Errorf("scan text reached the summary as %q:\n%s", bad, out)
				}
			}
		})
	})
}
