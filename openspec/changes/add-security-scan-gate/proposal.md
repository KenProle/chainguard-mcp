# Proposal

## Why

Every pull request already gets three reviews before it merges, but they run by hand from a developer's session, so they depend on someone remembering and on one person's usage limits. The Claude Security change scan reads a diff the way a security researcher would and has every finding challenged by a verification panel; a spike on 2026-11-10 showed it runs headless (84 seconds for an 18-file, 1,040-line commit at low effort). Running it automatically on pull requests that touch security-sensitive code, and refusing to merge on a verified high or critical finding, makes that check part of the merge rather than a habit.

## What Changes

- A new workflow, `.github/workflows/security-scan.yml`, with one job named **Security scan**. It runs on pull requests.
- The job first decides whether the pull request changes security-sensitive files (a path list in the repo, applied by a small tested program). If not, it passes at once. If so, it runs the Claude Security change scan on the pull request's base-to-head range at medium effort.
- The job **fails** on any verified critical or high finding in the change, and on any scan that did not finish, was not verified, or lost a researcher. Medium and low findings are reported but do not block.
- Findings in the change are uploaded as SARIF to GitHub code scanning, so they show as annotations on the pull request. Findings the scan marks as already present before the change are not uploaded; only their count is reported.
- Authentication uses a Claude subscription OAuth token stored as the `CLAUDE_CODE_OAUTH_TOKEN` repository secret, passed to `anthropics/claude-code-action`, pinned to a commit.
- Only pull requests from branches in this repository get a scan. A pull request from a fork that changes security-sensitive files fails the job with an explanation, because GitHub withholds secrets from fork runs.
- `CLAUDE.md` records the process and one new exception to its rule about not posting exploit details before a fix: findings in an unmerged pull request's own changes may appear on that pull request.
- The `Protect main` ruleset is updated to require **Security scan**.

Decisions made with the user, and what was rejected:

- **Blocking and automatic**, not advisory and not manual. Rejected: running it only by hand, which is what the existing reviews already are.
- **Only when the change looks security-sensitive**, not on every pull request, to save the subscription's usage allowance. Rejected: every pull request (cost, and most changes are docs or tests); a weekly full-repository scan (too late to block).
- **Pro-plan OAuth token**, because there is no API key. Accepted costs: scans draw on the owner's personal usage limits, and the token lasts one year.
- **Full SARIF with public annotations is acceptable** because findings in a pull request are fixed before it merges. Rejected: a trimmed SARIF, an encrypted artifact, or re-running locally to read details.

## Capabilities

### New Capabilities
- `security-scan-gate`: when a pull request is scanned, what the scan's result must do to the pull request (block, annotate, report), and the limits on who and what may run with the subscription token.

### Modified Capabilities

None. The `dependency-vulnerability-checks` workflow is unchanged.

## Impact

- New files: `.github/workflows/security-scan.yml`, `scripts/securityscan.go` (a `go:build ignore` program run with `go run`), `.github/security-scan-paths.txt`, a Go test `security_scan_test.go` with fixtures in `testdata/`.
- Edited: `CLAUDE.md` (CI section and the exception), `README.md` only if it documents CI.
- Repository settings the owner must change: add the `CLAUDE_CODE_OAUTH_TOKEN` secret and add **Security scan** to the `Protect main` ruleset. Neither can be done from the repository's files.
- One new third-party GitHub Action (`anthropics/claude-code-action`) and one plugin installed at run time (`claude-security` from `anthropics/claude-plugins-official`). No Go or npm dependency is added. The Go server, web API and UI do not change.
- Out of scope: scanning the whole repository on a schedule; scanning pull requests from forks; automatic patch suggestions (the plugin's "suggest patches" job); posting review comments other than code-scanning annotations; changing the existing three-review merge process, which still applies.
