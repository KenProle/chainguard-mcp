# Tasks

Commands are for Git Bash on Windows as in CLAUDE.md (`npm.cmd` from PowerShell; build the server with `go build -o chainguard-mcp.exe .` after `npm --prefix web run build`). Test names start with the scenario ID they cover. Design decisions are numbered as in `design.md`.

## 1. Layout at any width

- [ ] 1.1 Make `chartLayout` and the card placement take a width (default `chart.width`, 480) (design decision 1; HV-24, HV-20). Verify with unit tests in `history.test.ts`:
  - layouts at 480, 935 and 309 span `left` to `width - right`
  - the existing `HV-20.1` and `HV-20.2` tests still pass at 480
  - record what `neighbours` returns for 2026-10-14 at 935 in a test

## 2. Measuring and drawing

- [ ] 2.1 Add `useElementWidth` with a `ResizeObserver` (minimum 280, falling back to 480 when unavailable), and draw the SVG with `width`, `height` and `viewBox` equal to the measured size (design decisions 2 to 4; HV-24). Verify with component tests using a mocked `ResizeObserver`:
  - `HV-24.1` (at a reported 935, the SVG is 935×240 and the card rect is 200 wide)
  - `HV-24.3` (at 309, HV-23's text line still shows and the card is hidden below `sm`)
  - `HV-24.4` (a second callback at 600 redraws, and a selection survives)
  - all existing GrowthChart, HistoryView and page tests still pass
- [ ] 2.2 Update CLAUDE.md's growth chart note (drawn at its measured width, one unit per pixel). Verify: it says so.

## 3. Verification

- [ ] 3.1 Run `npm --prefix web run lint && npm --prefix web run typecheck && npm --prefix web test`, `go test ./...` and `openspec validate --all --strict`. Verify: all pass.
- [ ] 3.2 Rebuild the UI, then the server, and open the History view in the browser. At window widths of 1,280, 768 and 375 px, measure with `getBoundingClientRect()`:
  - the height of an axis label `<text>`, expecting 12 px (±0.5) at all three (`HV-24.1` to `HV-24.3`)
  - the tooltip card's text height and the card's width (200 px) at 1,280 and 768
  - at 375, no horizontal page overflow and the text line below the chart

  Then resize from 1,280 to 768 with the view open (`HV-24.4`). Check dark mode, and the console for CSP violations. Verify: the measurements are reported in the summary.

## Workflow follow-up

- Don't archive this change in the implementing session; the main session reviews, checks CI and archives.
- Ask the user before every commit and push; the user types the approval in the implementing session itself.
