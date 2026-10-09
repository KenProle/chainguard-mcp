# Proposal

## Why

The Security tab's "Recorded fixes" card shows three totals and a table with one row per source package, in alphabetical order. For `python:latest` (checked live on 2026-10-09, digest `sha256:6cc531a9…`) that table has 21 rows. Nine have no records at all, and 97% of the 336 recorded fixes sit in five packages:

| Source package | Fixes included | Never affected |
|---|---|---|
| `openssl-4.0` (records from `openssl`) | 147 | 6 |
| `python-3.14` | 74 | 4 |
| `expat` | 54 | 4 |
| `py3-pip` | 30 | 9 |
| `glibc-2.44` | 20 | 12 |
| `zlib` | 10 | 2 |
| `gcc` | 1 | 0 |
| `sqlite`, `util-linux` | 0 | 4 each |
| `brotli`, `ncurses`, `xz` | 0 | 2 each |
| 9 others (`bzip2`, `readline`, `zstd`, …) | 0 | 0 |

`python:latest-dev` has 56 rows, 25 with records (546 fixes included, 74 never affected). The alphabetical table hides where the security work went. A fix that a newer package version has but this image lacks (a "pending fix") shows only in a separate box above the table. None of the images checked on 2026-10-09 (`python`, `tomcat`, `node`, `nginx`) had one, because Chainguard rebuilds images as fixes land. They do happen between rebuilds, though, and they're what a reader most needs to see.

Users:
- A security reviewer wants to see which source packages carry an image's recorded fixes, to judge where its exposure has been.
- A developer wants a fix the image lacks to stand out, rather than hunting for it in a long table.

## What Changes

- The Security tab's "Recorded fixes" card shows a **fixes chart** between the totals and the table: one stacked horizontal bar per source package with at least one record, with segments for fixes included, never affected and fixes not yet installed.
- Bars are sorted with packages that have fixes not yet installed first, then by fixes included. Fixes not yet installed get their own highlight color, so they stand out.
- Each bar prints its fixes-included count as text. A legend names the three segments, and one line counts the source packages that have no records, in wording that never implies those packages are free of vulnerabilities.
- The existing totals, pending-fix list, per-package table and note that the data isn't a full scan stay as they are. The table is the chart's text equivalent.
- `HorizontalBars` gains optional stacked segments. Its existing callers (the Compare tab and the license breakdown) are unchanged.

## Capabilities

### New Capabilities
- `image-security-fixes`: showing how an image's recorded security fixes and fixes not yet installed are distributed across its source packages in the web UI.

### Modified Capabilities
None. `image-variant-comparison` and `image-license-breakdown` keep their requirements; this change only extends the bar chart component they share.

## Decisions

Proposed on 2026-10-09 and approved by the user the same day:
- **Which packages get a bar:** only source packages with at least one record (fixed, never affected or not yet installed). The rest are counted in one line ("9 more source packages have no records in the Wolfi security database") and still appear in the table. *Rejected:* a bar for every package. Nine of python's 21 bars, and 31 of `latest-dev`'s 56, would be empty rows.
- **Sort order:** packages with fixes not yet installed first, then fixes included, then never affected (both descending), then name. *Rejected:* sorting by total records, which can rank a package with only "never affected" records above one with real fixes.
- **Segment order and colors:** fixes included (indigo), never affected (gray), not yet installed (rose). Indigo, not green, so the chart doesn't read as an "all clear". Rose matches the red border of the existing pending-fix box.
- **Value label:** each bar prints its fixes included ("147 fixed"). The never-affected count is in the table, and fixes not yet installed are listed with their IDs in the existing box. *Rejected:* printing all three counts per bar, which doesn't fit at 375 px.
- **Linear scale shared by all bars, starting at zero.** `gcc`'s 1 fix is a sliver next to `openssl-4.0`'s 153 records. Its count is printed as text. *Rejected:* a log scale, which misrepresents stacked segments.
- **Decorative chart, not a filter:** the chart is hidden from assistive technology and the table carries the same numbers. *Rejected for now:* selectable bars that filter the table, as the license breakdown has. With one table row per bar there is nothing useful to filter to. Ordering the table like the chart is a possible follow-up.
- **Computed in the browser** from the summary response the tab already fetches, in a pure, unit-tested module, as `compare.ts` and `licenses.ts` are. The ordering and counting are presentation. The MCP `check_vulnerabilities` tool already returns the same per-package numbers, so there is no logic to keep in sync with Go.
- **Pending-fix scenarios use a hand-built report.** No live image had a pending fix on 2026-10-09, so those scenarios modify the captured python data rather than claim live values.

## Out of Scope

- Any change to the Go server, web API or MCP tools.
- Listing open (unfixed) vulnerabilities, severity or CVSS scores. The Wolfi data has none of them.
- Fixes over time. The data records versions, not dates (see `docs/visualization-ideas.md`).
- Showing the packages outside Wolfi that the data doesn't cover (`uncovered` in the response). The tab doesn't show them today either.
- Filtering or reordering the table from the chart.
- The per-ID lookup card at the top of the tab.

## Impact

- **Web UI:** the Security tab's "Recorded fixes" card gains the chart, legend and no-records line. `HorizontalBars` gains optional per-bar segments. No API or Go changes; `design.md` confirms.
- **Tests:** a new fixture with python's vulnerability summaries for `latest` and `latest-dev`, captured on 2026-10-09.
- **Dependencies:** none.
