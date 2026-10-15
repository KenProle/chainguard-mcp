# Proposal

## Why

The History view's growth chart (HV-15) is a static picture. Reading an exact count means opening the table, and the recent data is unreadable on the chart: the axis spans 2024-04-19 to 2026-10-14 (about 910 days), so the five daily snapshots (2026-10-10 to 2026-10-14) fall within 2 to 3 pixels of a 424-pixel plot and draw as one blob. As daily snapshots accumulate, that recent stretch is where the interesting changes will be.

## What Changes

- **Tooltips on the growth chart:**
  - Pointing at or tapping the chart shows the nearest snapshot: a guide line and a card with its date, source (Internet Archive capture or daily snapshot), images, free images (or "unknown" before 2026-10-10) and the change since the previous snapshot.
  - When other snapshots sit within a few pixels of it, the card says so and points to the zoomed range.
- **Keyboard and screen readers:** the chart can be focused. ←/→ (and Home/End) step through snapshots, and each one is announced in a polite live region. The table of snapshots stays the full text equivalent.
- **Range buttons above the chart: "All time" and "Since daily tracking".**
  - The second rescales both axes to the daily snapshots, with a y-axis that doesn't start at zero and is labelled as such.
  - In that range only the total is drawn, with a note that the free count isn't plotted there.
  - The choice lives in the URL (`range=daily`) like the catalog's other view settings.

## Capabilities

### New Capabilities
None.

### Modified Capabilities
- `catalog-history-view`: the growth chart gains tooltips, keyboard stepping and a range choice (new requirements HV-19 to HV-22). HV-15 itself is unchanged, since everything it requires still holds.

## Decisions

Made with the user on 2026-10-15:
- **Range buttons** for the squeezed recent points. *Rejected:*
  - a split axis giving the daily era a fixed share of the width (the slope across the break is meaningless)
  - evenly spaced points (a five-month gap would look like a day, against the project's honesty rules)
  - drag-to-zoom (hard to discover, awkward on touch, too much for 22 points)
- **Tooltips that snap to the nearest snapshot**, drawn inside the SVG, with keyboard stepping and touch support.

Made in this proposal (for the user to review):
- **The zoomed range draws only the total.** Free (59) and total (3,171) can't share a zoomed scale, and a second y-axis would be misleading. Free stays in the tooltip, the table and the tiles. *Alternative:* a scale covering both, which shows nothing new when zoomed.
- **"Since daily tracking" is offered only when there are at least 2 daily snapshots** (there are 5 today).
- **The range is kept in the URL** (`range=daily`), matching how the catalog keeps its view, search and zoom.

## Out of Scope

- Ranges other than these two (e.g. "Last 30 days"). They can be added once months of daily data exist.
- Tooltips or zoom on other charts in the app.
- Changes to the API, the history data or the timeline.

## Impact

- **Web UI only:**
  - `GrowthChart.tsx` gains its interaction.
  - `history.ts` gains a range-aware layout and the tooltip wording.
  - `HistoryView` and `CatalogPage` read and write `range`.
- **No API changes. No new dependencies.**
