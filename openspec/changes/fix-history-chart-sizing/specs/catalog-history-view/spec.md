# Spec Delta

## ADDED Requirements

### Requirement: HV-24 Text at its design size at every width
The growth chart SHALL be drawn in as many units as its rendered width in pixels, with a fixed height, so its axis labels and tooltip text render at their design size (12 px) and the tooltip card keeps one size at every width. It SHALL redraw when its width changes, and SHALL NOT draw narrower than 280 units.

#### Scenario: HV-24.1 Desktop
- **WHEN** the History view is shown in a window 1,280 pixels wide
- **THEN** the axis labels and the tooltip text render at 12 px (±0.5), and the tooltip card is 200 px wide

#### Scenario: HV-24.2 Tablet
- **WHEN** the window is 768 pixels wide
- **THEN** the axis labels and the tooltip text render at 12 px (±0.5)

#### Scenario: HV-24.3 Phone
- **WHEN** the window is 375 pixels wide
- **THEN** the axis labels render at 12 px (±0.5), nothing overflows the page horizontally, and the selected snapshot's text is below the chart (HV-23)

#### Scenario: HV-24.4 Resizing
- **WHEN** the window narrows from 1,280 to 768 pixels while the History view is open
- **THEN** the chart redraws to the new width, and its labels stay at 12 px

## MODIFIED Requirements

### Requirement: HV-20 Snapshots too close to tell apart
When other snapshots are drawn within 6 pixels of the highlighted one (in the chart's own coordinates), the card SHALL say how many. In "All time", when "Since daily tracking" is offered and the highlighted snapshot is a daily one, it SHALL also suggest that range.

#### Scenario: HV-20.1 The daily snapshots in "All time"
- **WHEN** the pointer is nearest the 2026-10-14 point in "All time", on a chart drawn 480 units wide
- **THEN** the card says 5 more snapshots are drawn close to this one (2026-10-08 to 2026-10-13) and suggests "Since daily tracking"

#### Scenario: HV-20.2 Spread-out snapshots
- **WHEN** the pointer is nearest the 2025-05-04 point in "All time", on a chart drawn 480 units wide
- **THEN** the card mentions no nearby snapshots
