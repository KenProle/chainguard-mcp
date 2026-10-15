# Proposal

## Why

The History view's growth chart is drawn in a fixed 480×240 coordinate space (`viewBox`) and stretched to the full width of its card. Its text and tooltip card therefore scale with the window:
- **Desktop:** on a card about 935 px wide, everything is drawn at about 1.95×, so the 12-unit axis labels and tooltip text render at about 23 px and the tooltip card is about 390 px wide. The user found the chart and tooltips "huge".
- **Phone:** at 375 px wide, the chart is drawn at 0.64×, so the axis labels render at about 7.7 px. HV-23 already moved the tooltip text below the chart on phones for this reason, but the axis labels are still too small.

## What Changes

- **The chart is drawn at its real width.** It measures its own rendered width and uses that many units for its coordinate space, with a fixed height of 240. One unit is then one CSS pixel at every width:
  - axis labels and tooltip text render at 12 px everywhere
  - the tooltip card keeps one size
  - the plot itself gets wider or narrower
- **It re-lays itself out when its width changes**, for example on a window resize or rotation.
- **"Drawn close" (HV-20) is now measured in real pixels.** On a wide chart, fewer snapshots count as close, because they really are farther apart on screen.

## Capabilities

### New Capabilities
None.

### Modified Capabilities
- `catalog-history-view`:
  - **New HV-24:** the chart's text and tooltip card keep their design size at every width.
  - **HV-20 modified:** its scenarios state the chart width they assume, since what counts as "close" now depends on it.
  - **Unchanged:** HV-15, and HV-23's narrow-screen text line. At 375 px the plot area is about 250 px wide, which a 200 px tooltip card would almost cover.

## Decisions

Made with the user on 2026-10-15:
- **Draw at the real width.** *Rejected:* capping the chart's width at about 640 px. That fixes desktop only, and phone labels would stay at about 7.7 px.

Made in this proposal (for the user to review):
- **Keep HV-23's text line below the chart on narrow screens.**
- **A minimum drawing width of 280 units.** Narrower containers scroll horizontally inside the chart's own box rather than squashing the axis labels. This can't happen at 375 px, where the chart is about 309 px wide.

## Out of Scope

- Other charts in the app. They use the `HorizontalBars` component, which is sized differently and reported no problems.
- The chart's height, which stays 240 px.
- Any change to the chart's data, ranges or tooltip wording.

## Impact

- **Web UI only:**
  - `GrowthChart.tsx` gains a width measurement with a `ResizeObserver` and passes the width down.
  - `history.ts` makes `chart` and `chartLayout` take the width.
  - The pointer conversion no longer needs to scale.
- **No API changes. No new dependencies.**
