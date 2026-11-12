# Tasks

## 1. Pin the Actions

- [x] 1.1 Replace each major tag in `.github/workflows/{ci,history,pages,release,vulncheck}.yml` with the commit SHA of its latest release and a `# vX.Y.Z` comment, looked up live (DV-8.1). Verify: `grep -rE 'uses: [^ ]+@' .github/workflows` shows only `@<40 hex> # v…` lines, and `go test ./...` passes.
- [x] 1.2 State the convention in CLAUDE.md's "CI and releases" section (DV-8). Verify: the lookup command in CLAUDE.md runs as written.

## 2. Test

- [x] 2.1 Add the subtest "DV-8.1 every action is pinned to a commit SHA with a version comment" to `TestWorkflows`, and "DV-8.2 a floating reference is rejected" as a check of the matcher on sample lines (DV-8). Verify: `go test -run TestWorkflows -v ./...` passes, and fails when one `uses:` line in a workflow is changed to `@v7` by hand, then when its comment is removed.
- [x] 2.2 Update the `TestWorkflows` header comment if needed so it matches the new rule. Verify: `gofmt -l .` and `go vet ./...` are clean.

## 3. Verify end to end

- [ ] 3.1 On PR 14: all required checks are green on the new head commit, and the three reviews are re-run or the re-review noted on the PR for the test change.
