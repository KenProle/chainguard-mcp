//go:build ignore

// securityscan is the decision logic behind .github/workflows/security-scan.yml.
// It is CI tooling, not part of the server: it carries the build tag "ignore",
// so `go build ./...` skips it and the workflow runs it with `go run`.
//
//	securityscan decide [-paths FILE] -repo OWNER/NAME -head-repo OWNER/NAME -author LOGIN -actor LOGIN < NUL-separated changed files
//	securityscan verdict [-head SHA] [-mode MODE] [-effort TIER] DIR_OR_STAMP
//
// Everything it reads comes from a pull request, so nothing a pull request
// controls (file names, the scan's own text) is ever printed raw: the output
// goes into $GITHUB_OUTPUT and the job summary, where a stray newline or
// Markdown would be an injection. See openspec/changes/add-security-scan-gate.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	defaultPaths    = ".github/security-scan-paths.txt"
	maxListBytes    = 4 << 20 // changed-file list on stdin
	maxStampBytes   = 1 << 20 // the scan's revision stamp
	maxPatternBytes = 200
	dependabot      = "dependabot[bot]"
	workflowsPrefix = ".github/workflows/"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: securityscan decide|verdict ...")
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "decide":
		err = runDecide(os.Args[2:], os.Stdin, os.Stdout)
	case "verdict":
		var ok bool
		ok, err = runVerdict(os.Args[2:], os.Stdout)
		if err == nil && !ok {
			os.Exit(1)
		}
	default:
		err = fmt.Errorf("unknown subcommand %q", os.Args[1])
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "securityscan:", err)
		os.Exit(2)
	}
}

// ---- decide ---------------------------------------------------------------

type rule struct {
	text string
	skip bool // a leading "!" in the file
	re   *regexp.Regexp
}

// loadRules reads the path list: one glob per line, "#" starts a comment, a
// leading "!" names files that do not need a scan. For each file the last
// matching line wins, and a file that no line matches is scanned, so a new
// kind of file errs towards scanning.
func loadRules(file string) ([]rule, error) {
	b, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	var rules []rule
	for i, line := range strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n") {
		if j := strings.Index(line, "#"); j >= 0 {
			line = line[:j]
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		skip := strings.HasPrefix(line, "!")
		glob := strings.TrimPrefix(line, "!")
		if glob == "" || len(glob) > maxPatternBytes || strings.ContainsAny(glob, "\x00\r\n") {
			return nil, fmt.Errorf("%s:%d: unusable pattern", file, i+1)
		}
		re, err := globToRegexp(glob)
		if err != nil {
			return nil, fmt.Errorf("%s:%d: %v", file, i+1, err)
		}
		// The text comes from our own reviewed file and was checked above to be
		// one line, so it is safe to echo.
		rules = append(rules, rule{text: glob, skip: skip, re: re})
	}
	if len(rules) == 0 {
		return nil, fmt.Errorf("%s has no patterns", file)
	}
	return rules, nil
}

// globToRegexp turns a glob into a full-match regular expression. "**" matches
// across directories, "*" and "?" within one name. A pattern with no "/" is
// matched against the file's base name, like a .gitignore line.
func globToRegexp(glob string) (*regexp.Regexp, error) {
	var sb strings.Builder
	sb.WriteString("^")
	if !strings.Contains(glob, "/") {
		sb.WriteString("(?:.*/)?")
	}
	for i := 0; i < len(glob); i++ {
		switch c := glob[i]; c {
		case '*':
			if i+1 < len(glob) && glob[i+1] == '*' {
				sb.WriteString(".*")
				i++
			} else {
				sb.WriteString("[^/]*")
			}
		case '?':
			sb.WriteString("[^/]")
		default:
			sb.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	sb.WriteString("$")
	return regexp.Compile(sb.String())
}

// needsScan reports whether file must be scanned and which pattern said so
// ("" when no pattern matched and the default applied).
func needsScan(rules []rule, file string) (bool, string) {
	scan, by := true, ""
	for _, r := range rules {
		if r.re.MatchString(file) {
			scan, by = !r.skip, r.text
		}
	}
	return scan, by
}

// splitFiles reads a NUL-separated list, the form `git diff --name-only -z`
// prints, so a file name with a newline in it stays one name.
func splitFiles(b []byte) []string {
	var files []string
	for _, f := range bytes.Split(b, []byte{0}) {
		if len(f) > 0 {
			files = append(files, string(f))
		}
	}
	return files
}

func runDecide(args []string, stdin io.Reader, out io.Writer) error {
	fs := flag.NewFlagSet("decide", flag.ContinueOnError)
	paths := fs.String("paths", defaultPaths, "path list file")
	repo := fs.String("repo", "", "base repository, OWNER/NAME")
	headRepo := fs.String("head-repo", "", "head repository, OWNER/NAME (empty if it was deleted)")
	author := fs.String("author", "", "login of the pull request's author")
	actor := fs.String("actor", "", "login of the user whose action started this run")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *repo == "" {
		return errors.New("-repo is required")
	}
	rules, err := loadRules(*paths)
	if err != nil {
		return err
	}
	raw, err := io.ReadAll(io.LimitReader(stdin, maxListBytes+1))
	if err != nil {
		return err
	}
	var files []string
	if len(raw) <= maxListBytes {
		files = splitFiles(raw)
	}

	decision, reason := decide(rules, files, len(raw) > maxListBytes,
		!strings.EqualFold(*headRepo, *repo), *author, *actor)
	// Both values are fixed strings or text from our own path list.
	fmt.Fprintf(out, "decision=%s\nreason=%s\n", decision, reason)
	return nil
}

// decide applies SG-2, SG-6 and SG-11.
func decide(rules []rule, files []string, tooBig, fromFork bool, author, actor string) (decision, reason string) {
	if tooBig || len(files) == 0 {
		// SG-2.4: a list that cannot be read is never a reason to skip.
		decision, reason = "scan", "the changed files could not be listed"
	} else {
		decision, reason = "skip", "no security-sensitive file changed"
		for _, f := range files {
			if scan, by := needsScan(rules, f); scan {
				decision = "scan"
				if by == "" {
					reason = "a changed file matches no pattern, so it is scanned"
				} else {
					reason = "a changed file matches " + by
				}
				break
			}
		}
	}
	if decision == "skip" {
		return decision, reason
	}
	// The scan needs the token, which Dependabot runs and fork runs never get.
	if author == dependabot && actor == dependabot {
		if onlyWorkflows(files) {
			return "skip-dependabot", "Dependabot workflow update, covered by the pull request review"
		}
		return "fail-no-token", "Dependabot runs cannot use the scan token"
	}
	if fromFork {
		return "fail-no-token", "pull requests from forks cannot use the scan token"
	}
	return decision, reason
}

func onlyWorkflows(files []string) bool {
	if len(files) == 0 {
		return false
	}
	for _, f := range files {
		if !strings.HasPrefix(f, workflowsPrefix) || strings.Contains(f, "..") {
			return false
		}
	}
	return true
}

// ---- verdict --------------------------------------------------------------

type stamp struct {
	Mode     *string `json:"mode"`
	Effort   *string `json:"effort"`
	RunShape *struct {
		DiffFiles *int  `json:"diff_files"`
		EmptyDiff *bool `json:"empty_diff"`
	} `json:"run_shape"`
	Revision *struct {
		Commit *string `json:"commit"`
	} `json:"revision"`
	Findings *struct {
		Total    *int `json:"total"`
		Critical *int `json:"critical"`
		High     *int `json:"high"`
		Medium   *int `json:"medium"`
		Low      *int `json:"low"`
	} `json:"findings"`
	Verification *struct {
		Status     *string `json:"status"`
		Reason     *string `json:"reason"`
		Dispatched *int    `json:"researchers_dispatched"`
		Returned   *int    `json:"researchers_returned"`
	} `json:"verification"`
}

var hex40 = regexp.MustCompile(`^[0-9a-f]{40}$|^[0-9a-f]{64}$`)

// printable keeps a string safe to put in a Markdown line: ASCII letters,
// digits and plain punctuation only, shortened.
func printable(s string) string {
	var sb strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9',
			strings.ContainsRune(" .,:;_-/()'", r):
			sb.WriteRune(r)
		default:
			sb.WriteRune('?')
		}
		if sb.Len() >= 160 {
			sb.WriteString("...")
			break
		}
	}
	return sb.String()
}

// findStamp accepts the stamp file itself, or a directory holding exactly one
// CLAUDE-SECURITY-*/CLAUDE-SECURITY-REVISION-*.json (a fresh checkout has no
// other, so two means something is wrong).
func findStamp(arg string) (string, error) {
	if arg == "" {
		return "", errors.New("verdict needs the stamp file or the directory holding the report")
	}
	fi, err := os.Stat(arg)
	if err != nil {
		return "", fmt.Errorf("no report: %v", err)
	}
	if !fi.IsDir() {
		return arg, nil
	}
	matches, err := filepath.Glob(filepath.Join(arg, "CLAUDE-SECURITY-*", "CLAUDE-SECURITY-REVISION-*.json"))
	if err != nil {
		return "", err
	}
	switch len(matches) {
	case 0:
		return "", errors.New("the scan wrote no revision stamp")
	case 1:
		return matches[0], nil
	}
	return "", fmt.Errorf("found %d revision stamps, expected one", len(matches))
}

// loadStamp reads and checks the stamp. Every field the verdict relies on must
// be present: a missing count is a failure, never a zero.
func loadStamp(file string) (*stamp, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, maxStampBytes+1))
	if err != nil {
		return nil, err
	}
	if len(b) > maxStampBytes {
		return nil, errors.New("the revision stamp is too large to trust")
	}
	var s stamp
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, errors.New("the revision stamp is not valid JSON")
	}
	if s.Findings == nil || s.Findings.Total == nil || s.Findings.Critical == nil || s.Findings.High == nil ||
		s.Findings.Medium == nil || s.Findings.Low == nil {
		return nil, errors.New("the revision stamp has no finding counts")
	}
	v := s.Verification
	if v == nil || v.Status == nil || v.Dispatched == nil || v.Returned == nil {
		return nil, errors.New("the revision stamp has no verification record")
	}
	for _, n := range []int{*s.Findings.Total, *s.Findings.Critical, *s.Findings.High,
		*s.Findings.Medium, *s.Findings.Low, *v.Dispatched, *v.Returned} {
		if n < 0 {
			return nil, errors.New("the revision stamp has a negative count")
		}
	}
	return &s, nil
}

// expectation is what the workflow knows about the scan it asked for. A stamp
// that does not match is for some other scan, and some other scan's result says
// nothing about this pull request (SG-12).
type expectation struct{ head, mode, effort string }

func (e expectation) check(s *stamp) string {
	if e.head != "" {
		if s.Revision == nil || s.Revision.Commit == nil || !strings.EqualFold(*s.Revision.Commit, e.head) {
			return "the stamp is for a different commit than the pull request head"
		}
	}
	if e.mode != "" && (s.Mode == nil || *s.Mode != e.mode) {
		return "the stamp is not from a " + printable(e.mode) + " scan"
	}
	if e.effort != "" && (s.Effort == nil || *s.Effort != e.effort) {
		return "the stamp is not from a scan at " + printable(e.effort) + " effort"
	}
	if e.head != "" {
		if rs := s.RunShape; rs == nil || rs.DiffFiles == nil || *rs.DiffFiles < 1 || (rs.EmptyDiff != nil && *rs.EmptyDiff) {
			return "the scan covered no changed file"
		}
	}
	return ""
}

// judge applies SG-4. It returns whether the job passes and, when it does not,
// why. Anything the scan cannot vouch for is a failure.
func judge(s *stamp, want expectation) (bool, string) {
	if why := want.check(s); why != "" {
		return false, why
	}
	fc := s.Findings
	if sum := *fc.Critical + *fc.High + *fc.Medium + *fc.Low; sum != *fc.Total {
		return false, "the finding counts do not add up"
	}
	v := s.Verification
	if *v.Status != "verified" {
		why := "verification status is " + printable(*v.Status)
		if v.Reason != nil && *v.Reason != "" {
			why += " (" + printable(*v.Reason) + ")"
		}
		return false, why
	}
	if *v.Dispatched < 1 {
		return false, "no researcher was dispatched"
	}
	if *v.Returned < *v.Dispatched {
		return false, fmt.Sprintf("%d of %d researchers returned", *v.Returned, *v.Dispatched)
	}
	if n := *fc.Critical + *fc.High; n > 0 {
		return false, fmt.Sprintf("%d verified critical or high finding(s)", n)
	}
	return true, ""
}

func runVerdict(args []string, out io.Writer) (bool, error) {
	w := bufio.NewWriter(out)
	defer w.Flush()
	fs := flag.NewFlagSet("verdict", flag.ContinueOnError)
	wantHead := fs.String("head", "", "the pull request head commit the stamp must be for")
	wantMode := fs.String("mode", "", "the scan mode the stamp must record, e.g. changes")
	wantEffort := fs.String("effort", "", "the effort tier the stamp must record, e.g. medium")
	if err := fs.Parse(args); err != nil {
		return false, err
	}
	if fs.NArg() != 1 {
		return false, errors.New("usage: securityscan verdict [-head SHA] [-mode MODE] [-effort TIER] DIR_OR_STAMP")
	}
	if *wantHead != "" && !hex40.MatchString(strings.ToLower(*wantHead)) {
		return false, errors.New("-head is not a full commit id")
	}
	args = fs.Args()
	fail := func(why string) (bool, error) {
		fmt.Fprintf(w, "## Security scan\n\nResult: **failed** - %s.\n\nThe scan could not vouch for this change, so the check fails rather than passes.\n", printable(why))
		return false, nil
	}
	file, err := findStamp(args[0])
	if err != nil {
		return fail(err.Error())
	}
	s, err := loadStamp(file)
	if err != nil {
		return fail(err.Error())
	}
	ok, why := judge(s, expectation{head: *wantHead, mode: *wantMode, effort: *wantEffort})

	commit := "unknown"
	if s.Revision != nil && s.Revision.Commit != nil && hex40.MatchString(*s.Revision.Commit) {
		commit = (*s.Revision.Commit)[:12]
	}
	effort := "unknown"
	if s.Effort != nil {
		effort = printable(*s.Effort)
	}
	fc, v := s.Findings, s.Verification
	if ok {
		fmt.Fprintf(w, "## Security scan\n\nResult: **passed** - the scan reported no critical or high findings that survived verification.\n\n")
	} else {
		fmt.Fprintf(w, "## Security scan\n\nResult: **failed** - %s.\n\n", printable(why))
	}
	fmt.Fprintf(w, "| Critical | High | Medium | Low |\n| ---: | ---: | ---: | ---: |\n| %d | %d | %d | %d |\n\n",
		*fc.Critical, *fc.High, *fc.Medium, *fc.Low)
	fmt.Fprintf(w, "Scanned commit `%s` at %s effort. Verification: %s, %d of %d researchers returned.\n\n",
		commit, effort, printable(*v.Status), *v.Returned, *v.Dispatched)
	fmt.Fprintln(w, "Medium and low findings are listed here and as annotations but do not block the merge. "+
		"A clean result is a model's reading of the change, not proof that it is safe, and it does not replace the code and security reviews.")
	return ok, nil
}
