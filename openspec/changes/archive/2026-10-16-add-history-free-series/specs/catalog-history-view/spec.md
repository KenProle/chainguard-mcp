# Spec Delta

## ADDED Requirements

### Requirement: HV-25 Series choice
When at least 2 snapshots have a free count, the chart SHALL offer "All images" and "Free images". "Free images" SHALL draw only the daily snapshots' free counts, on a y-axis around them (labelled when it doesn't start at zero), hide the range choice, and say free counts start on the first live snapshot's date. The choice SHALL be kept in the URL as `series=free`.

#### Scenario: HV-25.1 Free images
- **WHEN** the user picks "Free images" without a search
- **THEN** the URL gets `series=free`, the chart draws the 5 daily snapshots from 2026-10-10 to 2026-10-14 at 59 each, mid-plot on an axis that doesn't start at zero, the range buttons are hidden, and a note says free counts start on 2026-10-10

#### Scenario: HV-25.2 Back to all images
- **WHEN** the user picks "All images"
- **THEN** `series` leaves the URL and the chart is drawn as before, with the range buttons shown

#### Scenario: HV-25.3 Not enough free counts
- **WHEN** fewer than 2 snapshots have a free count
- **THEN** only "All images" is offered, and `series=free` in the URL is ignored

#### Scenario: HV-25.4 Tooltip in free images
- **WHEN** the pointer is nearest the 2026-10-12 point in "Free images"
- **THEN** the card gives the same text as in "All images": daily snapshot, 3,171 images, 59 free, no change since 2026-10-11

## MODIFIED Requirements

### Requirement: HV-22 Range choice
The chart SHALL offer "All time" and, when there are at least 2 daily snapshots, "Since daily tracking". The latter SHALL rescale both axes to the daily snapshots, label its y-axis as not starting at zero, draw only the total, and say the free count isn't plotted and that "Free images" shows it. The choice SHALL be kept in the URL as `range=daily`.

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

#### Scenario: HV-22.5 Pointing to the free series
- **WHEN** "Since daily tracking" is shown with "All images"
- **THEN** the note under the chart says free images aren't plotted in this range and that "Free images" shows them
