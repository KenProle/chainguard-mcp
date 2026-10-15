# Tasks

Commands are for Git Bash on Windows as in CLAUDE.md. Test names start with the scenario ID they cover (e.g. `TestHV7_1_…`, page tests "HV-14.2 …"). Design decisions are numbered as in `design.md`. The spec's figures come from the real history on 2026-10-14, kept as `testdata/catalog-history-2026-10-14.jsonl`.

## 1. Fixture and embedding

- [x] 1.1 Copy the `data` branch's `catalog-history.jsonl` as of 2026-10-14 (22 lines, ending 2026-10-14) into `testdata/catalog-history-2026-10-14.jsonl` (`git show origin/data:catalog-history.jsonl`, or `b39829a` if `data` has moved on). Verify: it has 22 lines and `replayHistory` accepts it (a unit test).
- [x] 1.2 Add `history_embed.go` with `//go:embed all:historydata`, a committed `historydata/.gitkeep`, and `historydata/catalog-history.jsonl` in `.gitignore` (design decision 1; HV-3). Verify: `go build` and `go test ./...` pass with only `.gitkeep` present, and a unit test shows the embed yields no copy then.

## 2. Reading the history

- [x] 2.1 Add `HistorySource` with the embedded copy, the hourly fetch from the `data` branch, the 5-second timeout, validation and `ErrHistoryUnavailable` (design decision 2; HV-1, HV-2, HV-3.2). Verify with tests against a fake `data`-branch server, using a controllable clock:
  - `TestHV1_1` (a newer copy is used and reported as from the data branch)
  - `TestHV1_2` (offline keeps the embedded copy)
  - `TestHV1_3` (a damaged copy, made by changing one line's `total`, is ignored)
  - `TestHV2_1` (two requests in an hour make one fetch)
  - `TestHV2_2` (a slow server doesn't delay past the timeout)
  - `TestHV3_2` (no copy at all gives `ErrHistoryUnavailable`)
- [x] 2.2 Build the derived data in one replay per copy: image sets per line, new vs. back images in archive lines, `firstSeen` and date precision (design decision 3; HV-5, HV-7, HV-9, HV-10). Verify with unit tests on the fixture:
  - `TestHV5_1` (22 points, 675 to 3,171, free absent until 2026-10-10 then 59)
  - `TestHV7_1` (the 2026-10-10 changes span 2026-10-08 to 2026-10-10)
  - `TestHV7_2` and `TestHV7_3` (on fixtures made with consecutive and gapped live dates)
  - `TestHV9_1` (15 not in the 2026-10-08 capture)
  - `TestHV9_2` (`argocd-fips-repo-server` not in the 2024-07-17 capture, back on 2024-11-17)
  - `TestHV10_1` to `TestHV10_4` (`python`, `verdaccio`, `argocd-fips-repo-server`, an unknown name)
- [x] 2.3 Build the report: query filtering, timeline entries newest first with collapsed no-change runs, `since` filtering and totals, data age and the stalled note (design decisions 3 and 4; HV-4, HV-6, HV-8, HV-12.1). Verify with unit tests on the fixture:
  - `TestHV4_1` and `TestHV4_2` (today 2026-10-14 vs. 2026-10-17)
  - `TestHV6_1` (`fips`: 204 to 1,315)
  - `TestHV6_2` (`verdaccio`: 0 until 2026-10-08, then 1; one change)
  - `TestHV8_1` (the "2026-10-11 to 2026-10-14: no changes" entry, then the 5 added)
  - `TestHV8_2` (first capture, 675 images)
  - `TestHV8_3` (the 781-name list and its 3 back are complete in the report)
  - `TestHV17_1` and `TestHV17_2` (a search's entries start at its first match; `fips` keeps the first capture)
  - `TestHV18_1` (every entry before 2026-10-10 is marked `archive`)
  - `TestHV12_1` (`since` `2026-10-09`: 5 added, 3,166 then and 3,171 now)
  - an entry spanning `since` is kept

## 3. Service, API and MCP

- [x] 3.1 Wire `Service.History` and `Service.CatalogHistory` with `since` validation; give `newFakeBackend` the fixture as its embedded copy and an unreachable URL; add `history_unavailable` (503) to `writeError` (design decisions 5 and 7). Verify: the existing Go suite still passes, plus a test that `since=yesterday` gives an `*InputError`.
- [x] 3.2 Add `GET /api/history` to `apiRoutes` and the README API table (HV-11). Verify in `web_test.go`:
  - `TestHV11_1` (index entry with `query` and `since`)
  - `TestHV11_2` (400 `invalid_input`)
  - 503 `history_unavailable` with no copy (HV-3.2)
  - `TestAPIIndex` passes
- [x] 3.3 Add the `catalog_changes` MCP tool (HV-12). Verify in `tools_test.go`:
  - `TestHV12_1`
  - `TestHV12_2` (a fixture whose last snapshot is old, with a clock after it)
  - the description mentions archive-era ranges and the data's age
- [x] 3.4 Add `catalog_since` to `ImageDetails` and fill it in `Service.ImageDetails` without ever failing the call (HV-10, HV-13). Verify:
  - `TestHV13_1` (`python`: since at least 2024-04-19) through `get_image_details`
  - `TestHV13_2` (no copy: details still load without the field)
  - the existing details tests still pass

## 4. Builds

- [x] 4.1 Download the history in `.goreleaser.yaml`'s `before` hooks (failing the release on error), and in the `Dockerfile`'s web stage with `wget`, tolerating failure and copying `historydata/` into the Go stage (design decision 1; HV-3). Verify:
  - `docker build -t chainguard-mcp .` succeeds and the image serves `/api/history` with 22 or more snapshots while run with `--network none` (`HV-3.1`)
  - the GoReleaser snapshot config check passes (`goreleaser check`, or CI's `Release config check`)
- [ ] 4.2 Extend CI's Docker smoke test to request `/api/history` and expect 200. Verify: CI passes on the implementing commit.

## 5. Web UI

- [x] 5.1 Mirror the report, entry and `CatalogSince` types in `web/src/api.ts`, and generate `web/src/test/fixtures/historyReport.json` from the Go report for the fixture with `UPDATE_FIXTURES=1` (design decision 7). Verify: `npm --prefix web run typecheck` passes, and the Go fixture test fails if the JSON is stale.
- [x] 5.2 Add the History view to the catalog page: the `view=history` button, hidden list-only controls, and `HistoryView` with the age line, the three tiles, the timeline (live entries first, the archive era in a collapsed `<details>`, 10 names per list with the rest in a nested `<details>`) and the empty and unavailable states (design decision 6; HV-4, HV-8, HV-14, HV-17, HV-18, HV-3.2). Verify with page tests:
  - `HV-14.1` (the URL, the tiles "3,171", "59" and "+2,496", the chart and the timeline)
  - `HV-14.2` (search `verdaccio`)
  - `HV-14.3` (search `traefik`: no changes recorded since 2024-04-19)
  - `HV-17.1` (`verdaccio`: two entries, nothing before 2026-10-08)
  - `HV-17.2` (`fips` ends with the first capture, 204 matching images)
  - `HV-18.1` (two live entries visible, 17 behind the collapsed heading, which opens by keyboard)
  - `HV-4.2` (stalled note)
  - `HV-8.3` (expand shows all 781)
  - `HV-3.2` (unavailable message)
- [x] 5.3 Add `GrowthChart` (plain SVG, time axis, total and free lines, hollow archive points) with its table of points (HV-15). Verify with component tests:
  - `HV-15.1` (no free point before 2026-10-10)
  - `HV-15.2` (22 table rows; the SVG is `aria-hidden`)
- [x] 5.4 Show "In the catalog: …" on the image page from `catalog_since` (HV-16.1). Verify with a page test for `verdaccio`'s range wording and for a details response without the field.
- [x] 5.5 Update the README's catalog history section (the server now reads the history, how builds embed it, and the `curl` one-liner for local builds), CLAUDE.md's catalog history notes, and idea 6's status in `docs/visualization-ideas.md`. Verify: the README no longer says nothing reads the history.

## 6. Verification

- [x] 6.1 Run `gofmt` on the changed files (`tr -d '\r' < file | gofmt -l`, since the checkout uses CRLF), `go vet ./...`, `go test ./...`, `npm --prefix web run lint && npm --prefix web run typecheck && npm --prefix web test`, and `openspec validate --all --strict`. Verify: all pass.
- [x] 6.2 Build the UI and server with the history downloaded into `historydata/`, run with `-http 127.0.0.1:8080`, and exercise it in the browser:
  - the History view without a search, then with `verdaccio`, `fips` and `python`
  - expanding the 781-name list
  - the `verdaccio` and `python` image pages
  - a run with an unreachable URL, still showing the embedded copy
  - 375 px in dark mode with no CSP violations (`HV-16.2`)

  Call `catalog_changes` and `get_image_details` through the built binary's MCP endpoint. Verify: screenshots and the MCP output match the specs.

## Workflow follow-up

- Don't archive this change in the implementing session. The main session reviews the result, checks CI on the implementing commit, archives with spec sync and archives the session.
- Ask the user before every commit and push.
