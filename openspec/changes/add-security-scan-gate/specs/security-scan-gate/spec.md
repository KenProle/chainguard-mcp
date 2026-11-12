# Spec Delta

## Purpose

Scans pull requests that change security-sensitive code with the Claude Security change scan, blocks the merge on verified high or critical findings, and limits what may run with the owner's subscription token. Examples come from the spike run on commit 8a6d75c (18 files, 1,040 lines, low effort, 84 seconds) on 2026-11-10.

## ADDED Requirements

### Requirement: SG-1 One check that always reports
A workflow SHALL run a job named "Security scan" on every pull request opened against `main`. The job SHALL finish with a pass or a fail on every run, including when no scan is needed, so that it can be a required check.

#### Scenario: SG-1.1 A docs-only pull request still gets a result
- **WHEN** a pull request changes only `README.md`
- **THEN** the "Security scan" job runs, starts no scan, and passes

#### Scenario: SG-1.2 A new push replaces the running scan
- **WHEN** a commit is pushed to a pull request while its scan is still running
- **THEN** the earlier run is cancelled and a new run starts for the new head commit

### Requirement: SG-2 Deciding that a change is security-sensitive
The job SHALL compare the pull request's head with its base and SHALL scan only when a changed file matches the path list in `.github/security-scan-paths.txt`. The list SHALL cover the files that handle untrusted input or the network, the build and release setup, the dependency manifests, and the files that configure Claude (`.claude/`, `CLAUDE.md`). If the changed files cannot be listed, the job SHALL scan.

#### Scenario: SG-2.1 A request-handling file triggers a scan
- **WHEN** a pull request changes `web.go`
- **THEN** the job runs the scan

#### Scenario: SG-2.2 A workflow or manifest change triggers a scan
- **WHEN** a pull request changes `.github/workflows/ci.yml`, `Dockerfile`, `go.mod` or `web/package-lock.json`
- **THEN** the job runs the scan

#### Scenario: SG-2.3 Only documentation, tests and specs
- **WHEN** a pull request changes only files under `openspec/`, `docs/`, `site/` and `*_test.go`
- **THEN** the job passes without a scan and says in its summary that no security-sensitive file changed

#### Scenario: SG-2.4 A claim that cannot be checked
- **WHEN** the list of changed files cannot be produced
- **THEN** the job runs the scan rather than passing

### Requirement: SG-3 Scan scope and effort
When it scans, the job SHALL run the Claude Security change scan over exactly the commits between the pull request's base and head, at medium effort, with no scope limit. It SHALL stop and fail if the scan runs longer than 30 minutes.

#### Scenario: SG-3.1 The range is the pull request
- **WHEN** a pull request's base is commit 3e3e306 and its head is commit 8a6d75c
- **THEN** the scan reviews the diff of those two commits (18 files, 1,040 lines) and its stamp records that range

#### Scenario: SG-3.2 A scan that never ends
- **WHEN** the scan is still running after 30 minutes
- **THEN** the job fails and its summary says the scan timed out

### Requirement: SG-4 What blocks the merge
The job SHALL fail when the scan's stamp reports one or more critical or high findings and a verification status of `verified`. The job SHALL also fail, rather than pass, when the stamp is missing or unreadable, the status is anything other than `verified`, fewer researchers returned than were dispatched, or the scan itself errors. Medium and low findings SHALL be listed in the summary without failing the job.

#### Scenario: SG-4.1 A clean verified scan
- **WHEN** the stamp reports zero findings, status `verified`, and 1 researcher dispatched and 1 returned
- **THEN** the job passes

#### Scenario: SG-4.2 A verified high finding
- **WHEN** the stamp reports one high finding with status `verified`
- **THEN** the job fails and its summary gives the counts by severity

#### Scenario: SG-4.3 Only medium findings
- **WHEN** the stamp reports two medium findings and no high or critical ones
- **THEN** the job passes and the summary lists the two findings

#### Scenario: SG-4.4 An incomplete scan is not a pass
- **WHEN** the stamp is missing, or its status is `unverified`, or 1 of 2 researchers returned
- **THEN** the job fails and says why

### Requirement: SG-5 Findings appear on the pull request
The job SHALL upload the scan's SARIF to GitHub code scanning, with a category naming this scan, so findings in the change show as annotations on the pull request. The upload SHALL happen even when the job fails because of those findings. It SHALL NOT upload findings that the scan reports as present before the change or outside the requested scope; the job summary SHALL give their number only.

#### Scenario: SG-5.1 A finding in the change
- **WHEN** the scan reports a high finding in a line the pull request added
- **THEN** the SARIF is uploaded and the finding shows as an annotation on that line

#### Scenario: SG-5.2 A flaw that predates the change
- **WHEN** the scan lists one candidate as real but present before the change
- **THEN** it is not in the uploaded SARIF and the summary says "1 finding predates this change" without naming it

#### Scenario: SG-5.3 No findings
- **WHEN** the scan finds nothing
- **THEN** the SARIF with an empty results list is uploaded, which clears earlier alerts for the same category

### Requirement: SG-6 Only same-repository pull requests get the token
The job SHALL use only the `pull_request` event and SHALL NOT use `pull_request_target` or `workflow_run`. It SHALL scan only pull requests whose head branch is in this repository. A pull request from a fork that needs a scan SHALL fail the job with a message that the owner must run the scan locally, and SHALL NOT receive the token.

#### Scenario: SG-6.1 A fork changes request handling
- **WHEN** a pull request from a fork changes `web.go`
- **THEN** the job fails and says the scan cannot run for forks

#### Scenario: SG-6.2 A fork changes only documentation
- **WHEN** a pull request from a fork changes only `README.md`
- **THEN** the job passes without a scan

#### Scenario: SG-6.3 The trigger is checked
- **WHEN** the workflow file is read
- **THEN** its only pull-request trigger is `pull_request`, and the strings `pull_request_target` and `workflow_run` do not appear

### Requirement: SG-7 Claude configuration comes from the base branch
The scan SHALL run with Claude's configuration (`.claude/`, `.mcp.json`, `CLAUDE.md`) as it is on the pull request's base branch, not as the pull request changes it. A pull request that changes those files SHALL itself be scanned (SG-2) and the change SHALL NOT affect how its own scan runs.

#### Scenario: SG-7.1 A pull request adds an MCP server
- **WHEN** a pull request adds a `.mcp.json` that starts a program
- **THEN** that program does not run during the scan, and the scan reviews the file as part of the diff

### Requirement: SG-8 Least privilege for the token and the workflow
The subscription token SHALL be available only as the repository secret `CLAUDE_CODE_OAUTH_TOKEN` and only to the step that runs the scan. The workflow SHALL request only `contents: read`, `pull-requests: read` and `security-events: write`, and the action SHALL be pinned to a full commit SHA with its version in a comment. No step SHALL write to the repository or comment on the pull request.

#### Scenario: SG-8.1 Permissions
- **WHEN** the workflow file is read
- **THEN** its permissions are exactly those three, and no step uses `contents: write` or `pull-requests: write`

#### Scenario: SG-8.2 The action version is exact
- **WHEN** the workflow file is read
- **THEN** `anthropics/claude-code-action` is referenced as a 40-character commit SHA, not `@v1` or a branch

### Requirement: SG-9 The check is required on main
The `Protect main` ruleset SHALL require "Security scan" to pass before a pull request can merge, in addition to the checks it already requires.

#### Scenario: SG-9.1 A failing scan
- **WHEN** a pull request's "Security scan" run fails
- **THEN** GitHub does not allow it to merge without the owner's admin bypass

### Requirement: SG-12 The result must be for this pull request
The job SHALL accept a scan result only if its revision stamp records the pull request's head commit, a change scan, medium effort, and at least one changed file. A report directory that is already present in the checkout before the scan SHALL fail the job, so a pull request cannot supply its own result.

#### Scenario: SG-12.1 A matching stamp
- **WHEN** the stamp records the head commit, mode and effort the job asked for, and 18 changed files
- **THEN** the verdict is decided by the findings and verification as in SG-4

#### Scenario: SG-12.2 A stamp for another commit
- **WHEN** the stamp records a commit other than the pull request head
- **THEN** the job fails and says the stamp is for a different commit

#### Scenario: SG-12.3 The wrong kind of scan
- **WHEN** the stamp records a commit scan at low effort where a change scan at medium effort was asked for
- **THEN** the job fails and says which of the two differs

#### Scenario: SG-12.4 A scan of nothing
- **WHEN** the stamp records zero changed files, or an empty diff
- **THEN** the job fails and says the scan covered no changed file

#### Scenario: SG-12.5 A planted report
- **WHEN** a pull request adds a directory named `CLAUDE-SECURITY-planted` containing a clean stamp
- **THEN** the job fails before the scan starts

### Requirement: SG-11 Dependabot pull requests
A pull request opened by Dependabot, in a run started by Dependabot, that changes only files under `.github/workflows/` SHALL pass without a scan, because GitHub gives Dependabot runs a separate set of secrets and the token is deliberately not shared with them. Such pull requests keep the existing human review. Any other Dependabot-run pull request that needs a scan SHALL fail as one that cannot use the token; a run started by a person SHALL follow SG-2 and SG-6.

#### Scenario: SG-11.1 The monthly action update
- **WHEN** Dependabot opens its grouped GitHub Actions update, which changes `.github/workflows/ci.yml` and `release.yml`
- **THEN** "Security scan" passes without a scan and its summary says it was skipped for a Dependabot workflow update

#### Scenario: SG-11.2 Dependabot touches something else
- **WHEN** a Dependabot pull request also changes `go.mod`
- **THEN** the job fails as a fork-like case, since it cannot use the token, and the owner runs the scan locally

#### Scenario: SG-11.3 A person pushes to Dependabot's branch
- **WHEN** the owner pushes a commit to a Dependabot branch that changes only `.github/workflows/ci.yml`
- **THEN** the run is started by a person, so the job scans it like any other pull request

### Requirement: SG-10 The process is written down
`CLAUDE.md` SHALL describe the gate: when it runs, what blocks, that fork pull requests cannot be scanned, how to run the scan locally, and when the token expires. It SHALL state that findings in an unmerged pull request's own changes may appear on that pull request, and that this does not extend to flaws in code already merged.

#### Scenario: SG-10.1 The exception has limits
- **WHEN** `CLAUDE.md` is read
- **THEN** it says findings in a pull request's own changes may be public before the fix, and that details of an exploitable finding in merged code stay private until fixed
