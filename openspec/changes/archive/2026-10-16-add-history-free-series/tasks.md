# Tasks

Commands are for Git Bash on Windows as in CLAUDE.md. Test names start with the scenario ID they cover. No design.md: a small web-only change following the existing range choice (HV-22), with no open technical decisions. Figures come from `web/src/test/fixtures/historyReport.json` (the history of 2026-10-14).

## 1. Layout

- [x] 1.1 Let `chartLayout(points, range, width, series)` lay out the free series: daily snapshots with a free count, valued by `free`, on `niceDomain` around them (HV-25). Verify with unit tests in `history.test.ts`: `HV-25.1` (5 points at 59, mid-plot, not starting at zero), and the total series' existing tests unchanged.

## 2. Chart and toggle

- [x] 2.1 Draw the chosen series in `GrowthChart` (its line and points in the free color; legend, notes and the card's "Try" suggestion per series), and add the "Show" toggle to `HistoryView` with `series=free` in `CatalogPage`'s URL state, hiding the range buttons in "Free images" (HV-25, HV-22). Verify with component and page tests:
  - `HV-25.1` (URL, 5 points, axis label, range hidden, note)
  - `HV-25.2` (back to all images)
  - `HV-25.3` (one free point: no toggle, and `series=free` ignored)
  - `HV-25.4` (the tooltip text)
  - `HV-22.5` (the daily-range note)
  - the existing HV tests still pass
- [x] 2.2 Update CLAUDE.md's growth chart note and the README's History view sentence to mention the series toggle. Verify: both mention it.

## 3. Verification

- [x] 3.1 Run the web lint, typecheck and tests, `go test ./...` and `openspec validate --all --strict`. Verify: all pass.
- [x] 3.2 In the running app's History view, switch between "All images" and "Free images", in both ranges, and with the search `fips`; hover a point. Check 375 px in dark mode and the console. Verify: the free series is readable on its own axis and nothing overflows.
