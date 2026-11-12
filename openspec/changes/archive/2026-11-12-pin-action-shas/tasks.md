# Tasks

## 1. Pin the Actions

- [x] 1.1 Replace each major tag in `.github/workflows/{ci,history,pages,release,vulncheck}.yml` with the commit SHA of its latest release and a `# vX.Y.Z` comment, looked up live (DV-8.1). Verify: `grep -rE 'uses: [^ ]+@' .github/workflows` shows only `@<40 hex> # v…` lines, and `go test ./...` passes.
- [x] 1.2 State the convention in CLAUDE.md's "CI and releases" section (DV-8). Verify: the lookup command in CLAUDE.md runs as written.

## 2. Test

- [x] 2.1 Add `TestWorkflowPins` to `workflows_test.go` with the subtests "DV-8.1 every action is pinned to a commit SHA with a version comment" (all `.yml`/`.yaml` workflows; a `uses:` it cannot classify also fails) and "DV-8.2 a floating reference is rejected" (the matcher on sample lines) (DV-8). Verify: `go test -run TestWorkflowPins -v ./...` passes, and fails when one `uses:` line in a workflow is changed to `@v7` by hand, then when its comment is removed.
- [x] 2.2 Name the test in CLAUDE.md. Verify: `gofmt -l .` (line endings stripped) and `go vet ./...` are clean.

## 3. Verify end to end

- [x] 3.1 On PR 14: all required checks are green on the new head commit, and the three reviews are re-run or the re-review noted on the PR for the test change.
