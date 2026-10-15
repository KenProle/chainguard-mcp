# Design

## Context

See proposal.md for the motivation and the spec delta (HV-24, and HV-20 modified) for the behavior.

- `web/src/history.ts` exports `chart = { width: 480, height: 240, left: 44, right: 12, top: 12, bottom: 28 }`. `chartLayout(points, range)` reads `chart.width` to map dates to x.
- `GrowthChart.tsx` renders `<svg viewBox="0 0 480 240" className="h-auto w-full">`, so the drawing scales with the card. It converts pointer `clientX` to units with `(clientX - box.left) * chart.width / box.width`. The tooltip card is `CARD_WIDTH = 200` units and flips left near `chart.width - chart.right`.
- HV-23 already hides the in-SVG card below `sm` (640 px) and shows the selected snapshot's text under the chart.

**No API changes**: this is web-only.

## Goals / Non-Goals

**Goals:** one unit is one CSS pixel at every width, the layout updates on resize, and the existing tests keep working in jsdom, which has no layout.

**Non-Goals:** changing the height, the margins, the tooltip wording or other charts.

## Decisions

### 1. Width is a parameter, not a constant

- `history.ts` keeps `chart` for the margins and the height. Its `width` stays 480 and becomes the *default* width.
- `chartLayout(points, range, width = chart.width)` uses `width` wherever it used `chart.width`.
- The card placement in `GrowthChart` takes the width too.
- Pure functions stay testable at any width, and every existing test at 480 keeps passing. That's why HV-20's scenarios now say "480 units wide".

### 2. Measuring the rendered width

- A small hook, `useElementWidth(ref)`, lives in `web/src/history.ts` or a sibling non-component module (kept out of component files so fast refresh keeps working).
- It uses a `ResizeObserver` on the chart's wrapper and returns `Math.max(280, Math.round(contentRect.width))`, or `undefined` before the first measurement.
- `GrowthChart` draws with `width ?? chart.width`, so the first paint and jsdom (no `ResizeObserver`, no layout) use 480. The hook guards `typeof ResizeObserver === 'undefined'`.
- The SVG becomes `width={w} height={chart.height} viewBox={`0 0 ${w} ${chart.height}`}`, with the class `block` rather than `h-auto w-full`. One unit then equals one pixel.
- If a parent is narrower than 280, the wrapper gets `overflow-x-auto` so the page itself never scrolls horizontally.

*Alternative:* CSS `vector-effect` or `font-size` counter-scaling. Rejected: text would stay a fixed size, but the card, the margins and the hit areas would still scale, and CSS-based fixes would fight the CSP rule against inline styles.

### 3. Pointer conversion

`(clientX - box.left)` is already in units once the viewBox width equals the rendered width. The code keeps the ratio form, `* w / box.width`, so it stays correct during the brief window before a resize re-measures.

### 4. Resizing and selection

- The selected index survives a resize, since the same snapshots are drawn; only their x moves.
- The chart doesn't remount on resize, because the `key` in `HistoryView` stays `range|query|last_snapshot`.

### 5. Tests

- **Unit:** `chartLayout` at widths 480, 935 and 309. The plot spans `left` to `width - right`, and ticks and labels don't depend on width. `neighbours` at 935 for 2026-10-14 shows the count can drop when snapshots are really farther apart; assert the measured value and note it.
- **Component:** mock `ResizeObserver` (a test helper that calls the callback with a given width).
  - The SVG's `width` and `viewBox` follow the reported width.
  - A tooltip card is 200 wide at any width.
  - HV-24.4: a second callback re-lays it out.
- **Browser (task 3.2):** measure `getBoundingClientRect().height` of an axis `<text>` and the card's text at 1,280, 768 and 375 px windows, expecting about 12 px (±0.5). The previous review missed this; it's now an explicit check.

### Dependencies

None added.

## Risks / Trade-offs

- [The first paint is at 480, then jumps to the real width] → The measurement runs before the browser paints in practice (`ResizeObserver` fires before paint), and the chart's data arrives after a fetch anyway. Acceptable.
- [Very wide windows make long, flat plots] → It's still a time axis and still honest, and the "Since daily tracking" range covers the dense part. No max width, per the user's choice of option A over a cap.
- [HV-20 counts change with width] → Intended: "drawn close" should mean close on screen. The spec now states the width its scenarios use.
