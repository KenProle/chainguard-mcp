# Design

## Context

See proposal.md for the motivation and the spec delta (HV-19 to HV-22) for the behavior.

- `GrowthChart.tsx` draws plain SVG in a 480×240 viewBox. The plot is 424 pixels wide (`chart.left` 44, `chart.right` 12), and the SVG is `aria-hidden` with a table of points as its text equivalent.
- `chartLayout` in `web/src/history.ts` maps dates linearly to x and values to y from 0 up to `niceMax`. It has three ticks and a year mark per January 1.
- The CSP forbids inline styles and scripts, so everything is placed with SVG attributes and styled with Tailwind classes.
- URL state for the catalog lives in the query string, which `CatalogPage` updates through a ref holding the latest params.

**No API changes**: this is a web-only change using the existing `points` of `GET /api/history`.

## Goals / Non-Goals

**Goals:** pointer, touch and keyboard access to every snapshot's numbers, and a readable zoom for the daily era, with no new dependency.

**Non-Goals:**
- generic chart infrastructure for other charts
- animation
- more than two ranges

## Decisions

### 1. Pure helpers in `history.ts`, the component stays thin

- `chartLayout(points, range)` takes `'all' | 'daily'`.
  - `daily` filters to `source === 'live'` points.
  - Its y domain is `[lo, hi]` from `niceDomain(min, max)`: round numbers around the data, padded so a flat series sits mid-plot. For 3,171 on every day, that gives about 3,160 to 3,180 with a middle tick of 3,170 (HV-22.4).
  - `all` keeps today's `[0, niceMax]`.
  - It returns `startsAtZero` for the axis label.
- `nearestIndex(xs, px)`: binary search on the sorted x positions.
- `neighbours(xs, i, 6)`: the indexes within 6 viewBox units (HV-20).
- `pointText(points, i)`: the card and live-region wording, e.g. "2026-10-08 · Internet Archive capture · 3,166 images · free unknown · +769 since 2026-05-10". Its change part reads "first capture" for index 0 and "no change since …" for 0.
- All are unit-tested without a DOM.

### 2. Pointer and touch: pointer events on a transparent overlay

- A `<rect>` covering the plot area handles `onPointerMove` (mouse and pen) and `onPointerDown` (touch).
- The SVG's bounding box converts `clientX` to viewBox x (`(clientX - rect.left) * 480 / rect.width`), then `nearestIndex` picks the point.
- `onPointerLeave` clears the selection for mice. On touch, a document `pointerdown` listener outside the chart clears it (HV-19.4).
- No hover timers. The selection is plain React state, an index or null.

### 3. The card is drawn inside the SVG

- The card is a `<g>` holding:
  - a vertical guide `<line>` at the point's x
  - an enlarged ring on the point
  - a `<rect>` with up to five `<text>` lines
- It sits right of the point, flips left when it would cross `chart.width - chart.right`, and is clamped vertically.
- Its width is fixed (about 190 units) and the text is short enough to fit at 375 px, where the viewBox scales down. The lines are kept at font size 12 or above, so they stay legible after scaling (HV-16.2 still applies).
- Colors come from `historyChartColor` in `styles.ts` (new `card` and `guide` entries) for light and dark mode.
- No HTML overlay and no inline style, so it's CSP-safe by construction.
- Below Tailwind's `sm` (640 px) the SVG is scaled so small that the 12-unit text is about 8 px, so the card is `hidden sm:inline` and the same text is shown in a `text-sm sm:hidden` line under the chart (HV-23); the guide line and ring stay at every width.

### 4. Keyboard and screen readers

- A wrapping `<div role="group" tabIndex={0} aria-label="Growth chart: use the left and right arrow keys to move between snapshots">` takes focus with a visible `focus-visible` outline, and shows a one-line hint while focused (HV-21.2).
- The keys:
  - ←/→ move the index
  - Home and End go to the first and last snapshot
  - Escape clears the index
  - each handled key calls `preventDefault`
- A visually hidden `<p aria-live="polite">` gets `pointText` for the current index. Pointer moves don't write to it, so screen readers aren't flooded; only key presses do.
- The SVG stays `aria-hidden`, and the table stays (HV-15.2).

### 5. Range choice and URL

- `CatalogPage` reads `range` (`daily` or absent) and passes it to `HistoryView`. Setting it uses the existing ref-based updater and *replaces* the history entry, like every non-zoom update.
- `HistoryView` renders a small segmented control above the chart (reusing `segmentGroup` and `segmentButton`), with "Since daily tracking" only when at least 2 points are `live` (HV-22.3).
- When `range=daily` but the range isn't offered (for example a fixture with one live point), the chart falls back to "All time" without rewriting the URL.
- In the daily range:
  - the free polyline and free points aren't drawn
  - a line under the chart reads "Free images aren't plotted in this range; the tooltip and the table show them."
  - the y-axis label reads "Axis doesn't start at zero"
- The x labels are the range's first and last dates.
- The search, tiles and timeline don't change with the range. Only the chart does.
- The selection resets when the range or the points change.

### 6. Tests

- **Unit tests** in `history.test.ts`:
  - `niceDomain` on a flat series and a growing one
  - `nearestIndex` at edges and between points
  - `neighbours` with the real fixture: 5 within 6 units of 2026-10-14 in `all`, none for 2025-05-04
  - `pointText` for HV-19.1, HV-19.2 and HV-19.3
- **Component tests** in `GrowthChart.test.tsx`, using `historyReport.json`:
  - pointer events dispatched with mocked `getBoundingClientRect` and asserted on the card text
  - keyboard stepping, with the live-region text asserted
  - range buttons and the URL, through `HistoryView` and `CatalogPage` page tests
  - the hidden free line and the zero-axis label in the daily range

### Dependencies

None added.

## Risks / Trade-offs

- [Card text overflows at 375 px] → The card is fixed-width in viewBox units and uses short lines; checked in the browser at 375 px.
- [Six-unit neighbour threshold is arbitrary] → It matches the point radius plus stroke (about 5.5 units), so "drawn close" means visually overlapping. It's one constant in `history.ts`.
- [A zoomed axis exaggerates small changes] → The y-axis is labelled as not starting at zero, and "All time" stays the default.
- [Touch users can't hover] → A tap selects the nearest point and a tap elsewhere clears it; the table remains.
