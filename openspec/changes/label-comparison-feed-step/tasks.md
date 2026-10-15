# Tasks

Commands are for Git Bash on Windows as in CLAUDE.md. Test names start with the scenario ID they cover. No design.md: the change is small and has no open technical decisions.

## 1. Server

- [x] 1.1 Pass the step through the comparison's progress callback ("packages" during the OSV.dev lookups, "advisories" before Chainguard's advisory feed is read), store it in the job, and return it as `step` in the running status (UC-9). Verify with a test in `web_compare_test.go`, `TestUC9_2_…`: a fake feed slowed by a delay reports `step` "advisories" with `checked` equal to `total`, then `done`. The existing UC-9 and job tests still pass.

## 2. Web UI

- [x] 2.1 Add the optional `step` to the running status in `web/src/api.ts`, and show "Checking the current status of Chainguard's advisories…" with a full bar when `step` is "advisories" (UC-9). Verify with a component test "UC-9.2 …" in `UpstreamComparison.test.tsx`, and check that the existing UC-9.1 test still passes.

## 3. Verification

- [x] 3.1 Run `gofmt` on the changed files (`tr -d '\r' < file | gofmt -l`), `go vet ./...`, `go test ./...`, the web lint, typecheck and tests, and `openspec validate --all --strict`. Verify: all pass.
- [x] 3.2 In the running app, compare `node:20` on the Alternatives page with Chainguard's advisory feed uncached (a fresh server), and watch the label change after the package count completes. Verify: the label appears before the result.
