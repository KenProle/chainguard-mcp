# Proposal

## Why

The History view's growth chart draws the free count (59) on the same axis as the total (3,171), so in "All time" the free line hugs the bottom of the chart, and in "Since daily tracking" (zoomed to about 3,160 to 3,180) it isn't drawn at all (HV-22). The user asked to see the free trend on its own: "it's hard to make out b/c those counts are so much lower".

## What Changes

- **A "Show" toggle above the chart: "All images" and "Free images".**
  - "All images" is today's chart.
  - "Free images" draws only the free count of each daily snapshot, on its own axis around those values, so a change from 59 to 62 is plain to see.
- **Free counts start on 2026-10-10.** Free-tier status is known only from the first live snapshot, so in "Free images" the range buttons are hidden and a note says the free count starts then.
- **The choice is kept in the URL** as `series=free`, like `range=daily`.
- **In "Since daily tracking" with "All images",** the note now points to "Free images" for the free trend.
- **The tooltip is unchanged.** It still gives both counts in either mode.

## Capabilities

### New Capabilities
None.

### Modified Capabilities
- `catalog-history-view`:
  - **New HV-25:** the series choice.
  - **HV-22 modified:** its free-count note names "Free images".

## Decisions

Made with the user on 2026-10-15:
- **A toggle inside the chart.** *Rejected:* reusing the catalog's free-only filter for the whole History view, which would also have to filter the tiles and timeline to free images, with no free data before 2026-10-10.
- **One series at a time, each on its own scale.** A second y-axis on one chart would mislead.

Made in this proposal (for the user to review):
- **Hide the range buttons in "Free images",** rather than show two buttons that draw the same thing.
- **Offer "Free images" only when at least 2 snapshots have a free count,** as with "Since daily tracking".

## Out of Scope

- Filtering the tiles or the timeline to free images.
- Free-tier history before 2026-10-10, which doesn't exist.
- Any API change. The points already carry `free`.

## Impact

- **Web UI only:**
  - `history.ts` lets `chartLayout` lay out either series.
  - `GrowthChart.tsx` draws the chosen series.
  - `HistoryView.tsx` gains the toggle.
  - `CatalogPage.tsx` keeps `series` in the URL.
- **No API changes. No new dependencies.**
