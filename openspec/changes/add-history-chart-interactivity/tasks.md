# Tasks

Commands are for Git Bash on Windows as in CLAUDE.md (`npm.cmd` from PowerShell). Test names start with the scenario ID they cover (e.g. "HV-19.1 …"). Design decisions are numbered as in `design.md`. Figures come from `web/src/test/fixtures/historyReport.json`, the real history of 2026-10-14.

## 1. Chart helpers

- [x] 1.1 Add range-aware `chartLayout(points, range)`, `niceDomain`, `nearestIndex`, `neighbours` and `pointText` to `web/src/history.ts` (design decision 1; HV-19, HV-20, HV-22). Verify with unit tests in `history.test.ts`:
  - `HV-19.1` (2026-10-08: 3,166 images, free unknown, +769 since 2026-05-10)
  - `HV-19.2` (first capture)
  - `HV-19.3` (2026-10-12: no change since 2026-10-11, 59 free)
  - `HV-20.1` (5 neighbours of 2026-10-14 in `all`)
  - `HV-20.2` (none for 2025-05-04)
  - `HV-22.4` (a flat series sits mid-plot with distinct ticks)
  - `all` still starts at zero

## 2. Interaction

- [x] 2.1 Add the pointer overlay, guide line and in-SVG card to `GrowthChart`, with colors in `styles.ts` (design decisions 2 and 3; HV-19, HV-20). Verify with component tests:
  - `HV-19.1`, `HV-19.4` and `HV-20.1` using dispatched pointer events and a mocked bounding box
  - a tap outside the chart clears the card
- [x] 2.2 Add keyboard stepping, the focus hint and the polite live region (design decision 4; HV-21). Verify with component tests:
  - `HV-21.1` (End then ← shows 2026-10-13 and announces it)
  - `HV-21.2` (focus shows the hint)
  - Escape clears the card
  - the SVG stays `aria-hidden` and the table still has 22 rows (HV-15.2)

## 3. Range

- [x] 3.1 Add the range control to `HistoryView` and `range` to `CatalogPage`'s URL state; in `daily`, hide the free series and show the two notes (design decision 5; HV-22). Verify with page tests:
  - `HV-22.1` (URL `range=daily`, 5 points, the zero-axis label, no free line)
  - `HV-22.2` (back to all time, `range` removed)
  - `HV-22.3` (one live point: only "All time", and `range=daily` ignored)
  - the selection resets on range change
- [x] 3.2 Update CLAUDE.md's catalog history view notes (range in the URL state list) and the README's History view description. Verify: both mention the range and tooltips.

- [x] 3.3 Below `sm`, hide the in-SVG card and show the selected snapshot as a line of text under the chart, keeping the guide line and ring (HV-23). Verify with a component test of `HV-23.1` and in the browser at 375 px in dark mode.

## 4. Verification

- [x] 4.1 Run `npm --prefix web run lint && npm --prefix web run typecheck && npm --prefix web test`, `go test ./...` and `openspec validate --all --strict`. Verify: all pass.
- [x] 4.2 Build the UI and server (`go build -o chainguard-mcp.exe .` on Windows), run with `-http`, and exercise the History view in the browser:
  - hover across "All time", including the 2026-10-14 blob with its 5-neighbour note
  - the 2026-10-08 card
  - "Since daily tracking" and back
  - a search (`fips`) in both ranges
  - keyboard stepping and Escape
  - at 375 px in dark mode: the card fits, nothing overflows horizontally, and the console shows no CSP violations

  Verify: screenshots match the specs.

## Workflow follow-up

- Don't archive this change in the implementing session; the main session reviews, checks CI and archives.
- Ask the user before every commit and push.
