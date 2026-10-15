# catalog-history-view Specification

## Purpose

Reads the project's catalog history (written daily by `catalog-history`) and presents it in the web UI, the web API and MCP: what changed in Chainguard's catalog and when, how the catalog grew, and when each image first appeared. Dates are only as precise as the snapshots behind them, and the data's age is always visible.

HV-1 to HV-4 define how the server obtains the history and reports its age; HV-5 to HV-10 how growth, changes and first-seen dates are derived and worded; HV-11 to HV-13 the web API, the MCP tool and image details; HV-14 to HV-18 the web UI; HV-19 to HV-23 the growth chart's tooltips, keyboard stepping, range choice and narrow-screen layout. Figures are from the `data` branch on 2026-10-14: 22 lines, 17 Internet Archive captures from 2024-04-19 (675 images) to 2026-10-08 (3,166), then live snapshots from 2026-10-10 to 2026-10-14 (3,171 images, 59 free).

## Requirements

### Requirement: HV-1 Embedded and fetched copies
The server SHALL use the newest valid copy of the history it has: the copy embedded at build time, or one fetched from the `data` branch. A fetched copy SHALL be used only if it parses, replays without error and its last snapshot is not older than the copy in use. Otherwise the server SHALL keep its copy.

#### Scenario: HV-1.1 Newer copy online
- **WHEN** the embedded copy ends on 2026-10-12 and the `data` branch's copy ends on 2026-10-14
- **THEN** history responses use the copy ending 2026-10-14 and say it came from the `data` branch

#### Scenario: HV-1.2 Offline
- **WHEN** the fetch fails
- **THEN** history responses use the embedded copy and say so

#### Scenario: HV-1.3 Damaged copy
- **WHEN** the fetched copy has a line whose `total` doesn't match the replayed image list
- **THEN** it is ignored and the copy in use is kept

### Requirement: HV-2 Fetching at most hourly
The server SHALL fetch the `data` branch's copy at most once an hour, on demand, and SHALL NOT delay a response by more than the fetch's timeout when the fetch is slow or fails.

#### Scenario: HV-2.1 Repeat request
- **WHEN** history is requested twice within an hour
- **THEN** the `data` branch is fetched at most once

#### Scenario: HV-2.2 Slow source
- **WHEN** the fetch doesn't answer within its timeout
- **THEN** the response uses the copy in use and arrives without waiting longer

### Requirement: HV-3 Builds embed the history
Release binaries and the Docker image SHALL embed the `data` branch's history file as it was when they were built. A build without it, such as a plain local `go build`, SHALL still compile and run.

#### Scenario: HV-3.1 Docker image offline
- **WHEN** a Docker image built on 2026-10-14 runs without network access
- **THEN** the History view shows 22 snapshots ending 2026-10-14

#### Scenario: HV-3.2 No history at all
- **WHEN** a build without an embedded copy runs offline
- **THEN** the history endpoint answers 503 with code `history_unavailable`, the History view says the history couldn't be loaded, and every other feature works as before

### Requirement: HV-4 Age of the data
Every history response SHALL give the date of its last snapshot and whether the copy came from the build or the `data` branch. When the last snapshot is more than 2 days old, the UI and MCP output SHALL say the daily collector may have stalled.

#### Scenario: HV-4.1 Current
- **WHEN** the last snapshot is 2026-10-14 and today is 2026-10-14
- **THEN** the History view reads "Last snapshot: 2026-10-14" and shows no stalled note

#### Scenario: HV-4.2 Stalled
- **WHEN** the last snapshot is 2026-10-14 and today is 2026-10-17
- **THEN** the view also says the last snapshot is 3 days old and the daily collector may have stalled

### Requirement: HV-5 Growth over time
The history SHALL give one point per snapshot with its date, the number of images and the number of free images. The free count SHALL be absent before the first live snapshot, never 0 or estimated.

#### Scenario: HV-5.1 Whole catalog
- **WHEN** the history is requested without a search
- **THEN** it has 22 points, from 675 images on 2024-04-19 to 3,171 on 2026-10-14
- **AND** the free count is absent until 2026-10-10, then 59

### Requirement: HV-6 Search
With a search, each point SHALL count only images whose names contain the search text (case-insensitive, as the catalog search does), and the changes SHALL list only those images.

#### Scenario: HV-6.1 fips images
- **WHEN** the history is requested with the search `fips`
- **THEN** the points grow from 204 on 2024-04-19 to 1,315 on 2026-10-14

#### Scenario: HV-6.2 One image
- **WHEN** the history is requested with the search `verdaccio`
- **THEN** the points are 0 until 2026-10-08 and 1 from 2026-10-10
- **AND** the only change is `verdaccio` added between 2026-10-08 and 2026-10-10

### Requirement: HV-7 How precise a change's date is
A snapshot's changes SHALL be dated to that day only when the previous snapshot is from the day before. Otherwise they SHALL be dated as a range from the previous snapshot's date to this one's.

#### Scenario: HV-7.1 First live snapshot
- **WHEN** the changes in the 2026-10-10 snapshot are shown
- **THEN** they read as between 2026-10-08 and 2026-10-10, since the previous snapshot is the 2026-10-08 capture

#### Scenario: HV-7.2 Consecutive days
- **WHEN** a live snapshot follows one from the day before
- **THEN** its changes are dated to its own day

#### Scenario: HV-7.3 Gap in the collector
- **WHEN** a live snapshot's previous snapshot is three days earlier
- **THEN** its changes read as a range between the two dates

### Requirement: HV-8 Timeline of changes
The timeline SHALL list snapshots with changes newest first, each with its date or range (HV-7) and its added, removed, newly free and newly subscription images, by name. Consecutive snapshots without changes SHALL collapse into one "no changes" entry, and the first snapshot SHALL read as the first capture with its image count.

#### Scenario: HV-8.1 The week of 2026-10-14
- **WHEN** the timeline is shown without a search
- **THEN** the newest entry reads "2026-10-11 to 2026-10-14: no changes"
- **AND** the next reads "Between 2026-10-08 and 2026-10-10: 5 added" with `openstack-barbican-oshelm`, `openstack-keystone-oshelm-fips`, `openstack-manila-oshelm`, `verdaccio` and `whereabouts-fips`

#### Scenario: HV-8.2 First capture
- **WHEN** the oldest entry is shown
- **THEN** it reads "2024-04-19: first capture, 675 images"

#### Scenario: HV-8.3 Long lists
- **WHEN** an entry has more than 10 names in one list, as "Between 2026-05-10 and 2026-10-08" has 781 added (and 3 back)
- **THEN** the entry gives the count, shows the first 10 names, and can be expanded to show all of them

### Requirement: HV-9 Wording for archive-era removals
An image missing from an Internet Archive capture SHALL be described as not in that capture, never as removed, since captures miss images that later reappear. An image listed again after being missed SHALL read as back, not as added. Only a live snapshot's removals SHALL read as removed. Free-tier changes SHALL be shown only from the first live snapshot on.

#### Scenario: HV-9.1 Capture gap
- **WHEN** the entry for the 2026-10-08 capture is shown
- **THEN** its 15 missing images read "not in the 2026-10-08 capture", not "removed"

#### Scenario: HV-9.2 Image that reappeared
- **WHEN** `argocd-fips-repo-server`, missing from the 2024-07-17 capture and listed again on 2024-11-17, is shown in those entries
- **THEN** the 2024-07-17 entry lists it as not in that capture, and the 2024-11-17 entry lists it as back, not as added

### Requirement: HV-10 When an image first appeared
For each image, the history SHALL give its first appearance: "since at least <date> (the earliest capture)" if it is in the first snapshot, otherwise the date or range of the snapshot that first lists it (HV-7). An image the history doesn't list SHALL be reported as not yet recorded.

#### Scenario: HV-10.1 In the first capture
- **WHEN** the first appearance of `python` is requested
- **THEN** it reads "since at least 2024-04-19 (the earliest capture)"

#### Scenario: HV-10.2 Added later
- **WHEN** the first appearance of `verdaccio` is requested
- **THEN** it reads "added between 2026-10-08 and 2026-10-10"

#### Scenario: HV-10.3 Missing from a later capture
- **WHEN** the first appearance of `argocd-fips-repo-server` is requested
- **THEN** it reads "since at least 2024-04-19 (the earliest capture)", ignoring the captures it was missing from

#### Scenario: HV-10.4 Newer than the history
- **WHEN** an image in today's catalog isn't in the history yet
- **THEN** its first appearance reads "not yet recorded in the catalog history"

### Requirement: HV-11 Web API endpoint
The server SHALL serve `GET /api/history` with optional query parameters `query` (search text) and `since` (a date, `YYYY-MM-DD`, limiting the changes to later snapshots), returning the points (HV-5, HV-6), the timeline entries (HV-8) and the data's age (HV-4) as JSON.

#### Scenario: HV-11.1 Listed in the index
- **WHEN** a client sends `GET /api/`
- **THEN** the index has an entry for `/api/history` with `query` and `since` as optional query parameters

#### Scenario: HV-11.2 Invalid date
- **WHEN** a client requests `/api/history?since=yesterday`
- **THEN** the response is 400 with code `invalid_input`

### Requirement: HV-12 MCP tool catalog_changes
A `catalog_changes` tool SHALL take an optional `since` date and an optional `query`, and SHALL return the changes after `since` (all changes when omitted), the totals at `since` and now, the data's age, and a note that archive-era dates are ranges between captures.

#### Scenario: HV-12.1 Since 2026-10-09
- **WHEN** `catalog_changes` is called with `since` `2026-10-09`
- **THEN** it returns the 5 images added between 2026-10-08 and 2026-10-10, and totals of 3,166 at the snapshot before `since` and 3,171 now

#### Scenario: HV-12.2 Stalled collector
- **WHEN** the last snapshot is more than 2 days old
- **THEN** the result says so, as in HV-4

### Requirement: HV-13 First appearance in image details
`get_image_details` and `GET /api/images/{name}/details` SHALL include the image's first appearance (HV-10) as `catalog_since`, and SHALL still answer when no history is available, leaving the field out.

#### Scenario: HV-13.1 python
- **WHEN** `get_image_details` is called for `python`
- **THEN** `catalog_since` says since at least 2024-04-19, the earliest capture

#### Scenario: HV-13.2 No history
- **WHEN** no history is available (HV-3.2)
- **THEN** image details still load, without `catalog_since`

### Requirement: HV-14 History view
The catalog page SHALL offer a History view next to List and Map, kept in the URL like the other views. It SHALL show the data's age, three summary tiles (images now, free now, change since the first capture), the growth chart and the timeline, all following the catalog's search. When a search's first match comes after the first capture, the third tile counts from that match and reads "Since first match".

#### Scenario: HV-14.1 Opening the view
- **WHEN** the user picks History on the catalog page
- **THEN** the URL gets `view=history`, and the view shows the last snapshot's date, the tiles "Images 3,171", "Free 59" and "Since first capture +2,496 (from 675 on 2024-04-19)", a chart of total and free images, and the timeline

#### Scenario: HV-14.2 Search
- **WHEN** the user searches `verdaccio` in the History view
- **THEN** the chart and the timeline show only `verdaccio`, as in HV-6.2

#### Scenario: HV-14.4 Search first matching after the first capture
- **WHEN** the user searches `verdaccio` in the History view
- **THEN** the third tile reads "Since first match +0" with "from 1 on 2026-10-10"

#### Scenario: HV-14.3 Search without changes
- **WHEN** a search matches only images that never changed, such as `traefik` (`traefik` and `traefik-fips`, both in the first capture)
- **THEN** the timeline says no changes are recorded for matching images since the first capture, 2024-04-19

### Requirement: HV-15 Growth chart
The chart SHALL plot total images on a time axis, with a point per snapshot, and free images from the first live snapshot on. It SHALL mark which points are Internet Archive captures and SHALL have a table of every point as its text equivalent.

#### Scenario: HV-15.1 Free line
- **WHEN** the chart is shown without a search
- **THEN** the free line starts on 2026-10-10 at 59 and no free value is drawn before it

#### Scenario: HV-15.2 Text equivalent
- **WHEN** the chart is shown
- **THEN** a table lists each of the 22 snapshots with its date, source, total and free count, and the chart itself is hidden from assistive technology

### Requirement: HV-16 Since line, layout and security policy
A free image's Overview tab SHALL show its first appearance (HV-10) under the image's name. The Packages and Security tabs make no extra request for it. The History view and the since line SHALL be usable at 375 px wide, in light and dark mode, by keyboard, and without Content Security Policy violations.

#### Scenario: HV-16.1 Image page
- **WHEN** the Overview tab of `chainctl` (free, in the 2026-05-10 and not the 2026-02-05 capture) is opened
- **THEN** it reads "In the catalog: added between 2026-02-05 and 2026-05-10"

#### Scenario: HV-16.2 Phone width in dark mode
- **WHEN** the History view is viewed at 375 px wide in dark mode
- **THEN** nothing overflows horizontally, the timeline and the table are readable, and the console shows no Content Security Policy violations

### Requirement: HV-17 A search's timeline starts at its first match
With a search, the timeline SHALL start at the first snapshot that lists a matching image. Earlier snapshots SHALL NOT appear, not even as a "no changes" entry or a first capture with 0 images.

#### Scenario: HV-17.1 verdaccio
- **WHEN** the timeline is shown for the search `verdaccio`
- **THEN** it has two entries: "2026-10-11 to 2026-10-14: no changes" and "Between 2026-10-08 and 2026-10-10: 1 added" (`verdaccio`)
- **AND** nothing dated before 2026-10-08 is shown

#### Scenario: HV-17.2 Search matching the first capture
- **WHEN** the timeline is shown for the search `fips`, which matches 204 images in the first capture
- **THEN** it ends with "2024-04-19: first capture, 204 matching images"

### Requirement: HV-18 Archive era collapsed
The timeline SHALL show entries from the first live snapshot on directly, and SHALL group earlier entries under a collapsed "Before daily tracking (Internet Archive captures)" heading that opens on demand and states how many entries it holds.

#### Scenario: HV-18.1 Without a search
- **WHEN** the timeline is shown without a search
- **THEN** the first two entries (no changes from 2026-10-11 to 2026-10-14, and the 5 added between 2026-10-08 and 2026-10-10) are visible
- **AND** the 17 archive-era entries are behind the collapsed heading, which opens with the keyboard as well as the mouse

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

### Requirement: HV-23 Narrow screens
Below 640 pixels wide, the selected snapshot's text SHALL appear in a line of the page's normal text size directly below the chart, with the nearby-snapshot note and range suggestion of HV-20 where they apply, instead of in a card inside the chart, whose text is too small there. The guide line and the ringed point SHALL be drawn at every width.

#### Scenario: HV-23.1 Selecting on a phone
- **WHEN** the screen is 375 pixels wide and the user selects the 2026-10-08 point
- **THEN** its text is shown below the chart at the page's normal text size and no card is shown inside the chart
