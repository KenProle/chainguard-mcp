# Spec Delta

## ADDED Requirements

### Requirement: HV-19 Tooltip for the nearest snapshot
Pointing at the growth chart, or tapping it, SHALL highlight the snapshot nearest the pointer on the time axis with a guide line, and SHALL show a card with its date, its source, its images, its free images ("unknown" before the first live snapshot) and its change since the previous snapshot. Leaving the chart, or tapping outside it, SHALL hide the card.

#### Scenario: HV-19.1 An Internet Archive capture
- **WHEN** the pointer is nearest the 2026-10-08 point without a search
- **THEN** the card reads 2026-10-08, Internet Archive capture, 3,166 images, free unknown, +769 since 2026-05-10

#### Scenario: HV-19.2 The first capture
- **WHEN** the pointer is nearest the 2024-04-19 point
- **THEN** the card reads 675 images and says it is the first capture instead of giving a change

#### Scenario: HV-19.3 A daily snapshot
- **WHEN** the pointer is nearest the 2026-10-12 point in "Since daily tracking"
- **THEN** the card reads daily snapshot, 3,171 images, 59 free, no change since 2026-10-11

#### Scenario: HV-19.4 Leaving the chart
- **WHEN** the pointer leaves the chart
- **THEN** the card and guide line disappear

### Requirement: HV-20 Snapshots too close to tell apart
When other snapshots are drawn within 6 pixels of the highlighted one (in the chart's own coordinates), the card SHALL say how many. In "All time", when "Since daily tracking" is offered and the highlighted snapshot is a daily one, it SHALL also suggest that range.

#### Scenario: HV-20.1 The daily snapshots in "All time"
- **WHEN** the pointer is nearest the 2026-10-14 point in "All time"
- **THEN** the card says 5 more snapshots are drawn close to this one (2026-10-08 to 2026-10-13) and suggests "Since daily tracking"

#### Scenario: HV-20.2 Spread-out snapshots
- **WHEN** the pointer is nearest the 2025-05-04 point in "All time"
- **THEN** the card mentions no nearby snapshots

### Requirement: HV-21 Keyboard and screen readers
The chart SHALL be focusable. ←/→ SHALL move between snapshots and Home/End SHALL go to the first and last, showing the same card as the pointer, and each move SHALL announce the snapshot's text in a polite live region. Escape SHALL hide the card. The table of snapshots (HV-15.2) SHALL remain.

#### Scenario: HV-21.1 Stepping back from the end
- **WHEN** the chart has focus and the user presses End, then ←
- **THEN** the card shows 2026-10-13, and the live region reads its date, source, images, free images and change

#### Scenario: HV-21.2 Focus is visible
- **WHEN** the chart receives keyboard focus
- **THEN** it shows a visible focus outline and a hint that the arrow keys move between snapshots

### Requirement: HV-22 Range choice
The chart SHALL offer "All time" and, when there are at least 2 daily snapshots, "Since daily tracking". The latter SHALL rescale both axes to the daily snapshots, label its y-axis as not starting at zero, draw only the total, and say the free count isn't plotted. The choice SHALL be kept in the URL as `range=daily`.

#### Scenario: HV-22.1 Zooming in
- **WHEN** the user picks "Since daily tracking" without a search
- **THEN** the URL gets `range=daily`, the x-axis runs from 2026-10-10 to 2026-10-14 with the 5 points spread across the plot, the y-axis is labelled as not starting at zero, and no free line is drawn

#### Scenario: HV-22.2 Back to all time
- **WHEN** the user picks "All time"
- **THEN** `range` leaves the URL and the chart shows all 22 snapshots on an axis from zero

#### Scenario: HV-22.3 Not enough daily snapshots
- **WHEN** a history has fewer than 2 daily snapshots
- **THEN** only "All time" is offered, and `range=daily` in the URL is ignored

#### Scenario: HV-22.4 Flat values
- **WHEN** every point in the range has the same total, as 3,171 on all 5 days
- **THEN** the line is drawn in the middle of a y-axis with distinct tick labels, not on an edge
