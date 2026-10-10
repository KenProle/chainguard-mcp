# Spec Delta

## Purpose

Records the history of Chainguard's image catalog (which images exist and which are free-tier) in a file that other features and tools can read, so the project can show what was added, retired, or moved between free and subscription, and how the catalog has grown.

CH-1 to CH-4 define the history file and its snapshots; CH-5 to CH-7 how a daily snapshot is taken safely; CH-8 the backfill from the Internet Archive; CH-9 and CH-10 the daily schedule and where the file is published. Figures were checked on 2026-10-10: 3,171 images, 59 free; Internet Archive sitemap captures exist for 17 months from April 2024 (675 images) to October 2026 (3,166 images).

## ADDED Requirements

### Requirement: CH-1 History file
The catalog history SHALL be a text file with one JSON object per line, one line per snapshot, in date order. Each line SHALL have the snapshot's date (UTC, `YYYY-MM-DD`), its source (`archive` or `live`), the number of images, the number of free images (null when unknown), and the image names added and removed, and the names that became free and that became subscription-only since the previous snapshot, each list sorted by name.

#### Scenario: CH-1.1 A live line after the backfill
- **WHEN** the first live snapshot is taken on 2026-10-10 after the archive line for 2026-10-08 (3,166 images), and finds 3,171 images, 59 free
- **THEN** the line has date `2026-10-10`, source `live`, 3,171 images, 59 free, added `openstack-barbican-oshelm`, `openstack-keystone-oshelm-fips`, `openstack-manila-oshelm`, `verdaccio` and `whereabouts-fips`, and an empty removed list

#### Scenario: CH-1.2 Archive lines have no free count
- **WHEN** a line comes from an Internet Archive capture
- **THEN** its free count is null and its became-free and became-subscription lists are empty

### Requirement: CH-2 Baselines
The first line of the file SHALL list every image name it saw, so that the full image list at any snapshot can be rebuilt from the first line and the later lines' added and removed lists. The first live snapshot SHALL list every image that was free as its free-tier baseline, and those images SHALL NOT be counted as having become free.

#### Scenario: CH-2.1 Rebuilding any snapshot
- **WHEN** the full image list of any snapshot is rebuilt from the first line and the added and removed lists up to that snapshot
- **THEN** it equals the list that snapshot saw, and its length equals that line's image count

#### Scenario: CH-2.2 First live snapshot
- **WHEN** the first live snapshot follows the archive backfill and finds 59 free images
- **THEN** its line lists those 59 images as its free-tier baseline, its free count is 59, and its became-free list is empty

### Requirement: CH-3 Changes between snapshots
Each line after the first SHALL compare with the previous snapshot: an image is added if it is listed now but wasn't before, and removed if it was listed before but isn't now. From the first live snapshot onward, an image became free if it is free now and was not free (or did not exist) in the previous snapshot, and became subscription-only if it existed and was free before and is now subscription-only.

#### Scenario: CH-3.1 A new free image
- **WHEN** an image appears for the first time and is free
- **THEN** it is listed both as added and as became-free

#### Scenario: CH-3.2 An image moves to subscription
- **WHEN** an image that was free in the previous snapshot is subscription-only now
- **THEN** it is listed as became-subscription, and not as added or removed

#### Scenario: CH-3.3 A retired image
- **WHEN** `jaeger-query`, listed in the previous snapshot, is no longer in the sitemap
- **THEN** it is listed as removed, and not as became-subscription

### Requirement: CH-4 Unknown free-tier status
If an image's free-tier check fails, the snapshot SHALL treat its status as unchanged from the previous snapshot (or as not free if it is new), SHALL NOT list it as a change of status, and SHALL record its name in the line's list of unchecked images.

#### Scenario: CH-4.1 A failed check
- **WHEN** the free-tier check for `python`, free in the previous snapshot, fails
- **THEN** `python` is not listed as became-subscription, still counts as free, and is listed as unchecked

### Requirement: CH-5 One snapshot per day
Taking a live snapshot SHALL add at most one line per UTC date; if the file already ends with a line for the current date, it SHALL leave the file unchanged and report that.

#### Scenario: CH-5.1 A second run on the same day
- **WHEN** a snapshot is taken twice on 2026-10-12
- **THEN** the file has one line for 2026-10-12 and the second run reports it was skipped

### Requirement: CH-6 Implausible snapshots refused
A live snapshot SHALL be refused, leaving the file unchanged and reporting an error, if it finds no images, or if it would remove more than 5% of the previous snapshot's images, since that indicates a broken or partial sitemap rather than real retirements. (Real removals measured from the archive were 60 images over seven months.)

#### Scenario: CH-6.1 A truncated sitemap
- **WHEN** the sitemap returns 1,500 images and the previous snapshot had 3,171
- **THEN** no line is added and the run fails with an error naming both counts

#### Scenario: CH-6.2 Normal retirements
- **WHEN** a snapshot removes 15 of 3,171 images
- **THEN** the line is added with those 15 images as removed

### Requirement: CH-7 Source of a live snapshot
A live snapshot SHALL read the image list from the `images.chainguard.dev` sitemap and each image's free-tier status from Chainguard's anonymous token endpoint, the same way the image list and the "Free only" filter do, and SHALL make no more free-tier requests than one check of every image.

#### Scenario: CH-7.1 Same answer as the catalog
- **WHEN** a live snapshot is taken against the test backend
- **THEN** its image count and free images match what the image list reports with `free_only`

### Requirement: CH-8 Backfill from the Internet Archive
A backfill SHALL build the history's archive lines from the Internet Archive's captures of `images.chainguard.dev/sitemap.xml`, taking the first capture each month that parses as a sitemap with at least one image page and skipping any other (such as HTML error pages), dated by the capture date. It SHALL only write to an empty or missing history file, and SHALL pause between requests to the Archive.

#### Scenario: CH-8.1 Backfilled months
- **WHEN** the backfill runs against the Archive as of 2026-10-10
- **THEN** it writes one archive line for each of 17 months from 2024-04 to 2026-10, the first listing 675 images and the last 3,166

#### Scenario: CH-8.2 An HTML capture
- **WHEN** a month's capture is an HTML page instead of a sitemap
- **THEN** no line is written for it, and the next capture in that month is used if there is one

#### Scenario: CH-8.3 Existing history
- **WHEN** the backfill is run on a history file that already has lines
- **THEN** it changes nothing and reports an error

### Requirement: CH-9 Daily schedule
A scheduled job SHALL take a live snapshot once a day without anyone running the server, and SHALL also be startable by hand. A run that adds a line SHALL publish it; a run that is skipped (CH-5) or refused (CH-6) SHALL publish nothing, and a refused run SHALL be reported as failed.

#### Scenario: CH-9.1 A normal day
- **WHEN** the daily job runs and the snapshot adds a line
- **THEN** the updated history file is published with one new line

#### Scenario: CH-9.2 A refused snapshot
- **WHEN** the snapshot is refused because the sitemap looks truncated
- **THEN** nothing is published and the run is shown as failed

### Requirement: CH-10 Where the history is published
The history file SHALL be published on the repository's `data` branch, never on `main`, so that the daily commits don't trigger the code's CI or bypass `main`'s review rules.

#### Scenario: CH-10.1 Daily commit
- **WHEN** the daily job publishes a snapshot
- **THEN** one commit is added to `data`, touching only the history file, and `main` is unchanged
