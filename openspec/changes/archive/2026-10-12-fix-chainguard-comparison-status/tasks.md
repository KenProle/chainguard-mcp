# Tasks

Commands are for Git Bash on Windows, as in CLAUDE.md. Test names start with the scenario ID they cover (e.g. `UC-18.1 …`). Decision numbers refer to `design.md`. Check gofmt with `gofmt -l . | tr -d '\r'`, since the checkout uses CRLF. Before starting, confirm the `fix_not_planned` → won't fix decision with the user. If they reject it, drop UC-5.4 from the delta spec first.

## 1. Shared feed step

- [x] 1.1 Extract `Service.currentAdvisories` from `openAdvisories` (decision 1). It collects the distinct `CGA-` IDs, calls `Feed.FetchAll` and `withCurrentRecords`, and returns the current results, the IDs that came from OSV.dev, and the feed's oldest fetch time. Make `openAdvisories` call it. Verify: the OA tests (`go test -run 'OA' ./...`) pass unchanged.

## 2. Chainguard-side counting

- [x] 2.1 Make `classify` branch on the `Wolfi` and `Chainguard` ecosystems (decision 4: `advisoryStatus` fix available → fix available, `fix_not_planned` → won't fix, else no fix yet). Keep the Debian and Alpine path as it is. Verify with unit tests: `UC-5.4`, `UC-5.5`, a Chainguard record whose only `fixed` event closes an earlier range counts as no fix yet, and the existing `UC-5.3` and `UC-16` tests still pass.
- [x] 2.2 Give `newFakeUpstreamService` a fake feed with the current records of `CGA-0000-0000-0001` and `CGA-0000-0000-0002` (decision 7). Verify: the existing comparison tests (`go test -run 'UC' ./...`) pass with unchanged counts.
- [x] 2.3 In `compareUpstream`, pass the Chainguard side's results through `currentAdvisories`, drop records that `openState` reports fixed, and add the note for IDs from OSV.dev (decisions 2, 3, 5; UC-17, UC-18). Verify with tests on the fake upstream service:
  - `UC-17.1` and `UC-18.1`: a stale OSV copy that the feed has fixed in the installed version isn't counted.
  - `UC-18.2`: partly fixed.
  - `UC-18.3`: fix in a newer version → fix available.
  - `UC-17.2`: a 404 → OSV.dev's copy is counted, and a note names the ID.
  - `UC-17.3`: a record without the package's entry still counts.
  - `UC-17.4`: no feed request for a Debian record, and upstream counts unchanged.
  - `UC-11.3`: a feed 500 → error with source "Chainguard advisories".
  - `UC-10.1`: a repeat within an hour makes no OSV or feed requests.
  - `UC-10.2`: after an hour, only Chainguard lookups and feed reads repeat; use the OSV and feed `Now` clocks.
  - `UC-10.3`: an advisory read by `OpenAdvisories` isn't read again.
- [x] 2.4 Check the web job path for `UC-11.3`: in `web_compare_test.go`, a feed failure gives 502 `source_failed` with `source` "Chainguard advisories", and a retry after the feed recovers succeeds. Verify: `go test -run 'UC-11' ./...` passes.

## 3. Wording and docs

- [x] 3.1 Update `comparisonSource`, the `compare_with_upstream` description in `tools.go`, the README's comparison section, tool table and data-source table, and CLAUDE.md's comparison and open-advisories notes (remove "The upstream comparison does not use the feed yet") (decision 6; UC-6). Verify: a `UC-6.3` test asserts that the source mentions Chainguard's advisory feed, that the existing `UC-6` tests and the web suite pass, and that a grep finds no "secure" or "vulnerability-free" in the new text.
- [x] 3.2 Update the Purpose paragraph of `openspec/specs/upstream-comparison/spec.md` to list UC-17 and UC-18 (it can't be changed through a delta). Verify: `openspec validate --all --strict` passes.

## 4. Live checks

- [x] 4.1 Add a live subtest: for Chainguard's `node` and `python` (amd64), the comparison's Chainguard total equals `OpenAdvisories(image, "latest").Total`, and no feed failure occurs. Verify: `CHAINGUARD_LIVE=1 go test -run 'TestLive/upstream' -v ./...` passes.
- [x] 4.2 Compare `node:20` and `python:3` live through the built server (`go build -o chainguard-mcp.exe . && ./chainguard-mcp.exe -http 127.0.0.1:8080`, then the Alternatives page and `GET /api/compare-upstream`). Record both sides' figures in this change and check them against UC-4.3, UC-5.2 and UC-15.1 (`busybox`). If they've moved, update the scenarios with the new date. Verify: the UI shows the new source text and counts, and the console has no CSP errors.

## 5. Full verification

- [x] 5.1 Run `gofmt -l . | tr -d '\r'` (empty), `go vet ./...`, `go test ./...`, `npm --prefix web run lint && npm --prefix web run typecheck && npm --prefix web test`, and `openspec validate --all --strict`. Verify: all pass.

## Workflow follow-up

- Ask the user before every commit and push.
- Archive after review, once CI passes on the implementing commit.
