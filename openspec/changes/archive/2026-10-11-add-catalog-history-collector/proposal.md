# Proposal

## Why

Chainguard's catalog changes constantly, but its public endpoints only ever show the present. Checked on 2026-10-10, the catalog had 3,171 images, 59 free; Internet Archive copies of its sitemap show 675 images in April 2024 and 2,397 in May 2026, and between May and October 2026 alone 784 images were added and 15 removed (including the whole Jaeger set). Nothing records what was added, retired, or moved between free and subscription. Free-tier status can't be recovered later, so every day without a collector is history lost for good. This change starts recording now; showing the history is a follow-up.

## What Changes

- A **daily catalog snapshot**, taken by a new mode of the existing binary: the image names from the sitemap and each image's free-tier status, appended as one JSON line to a history file, recording the images added and removed and those that became free or subscription-only since the previous snapshot.
- A **scheduled GitHub Action** that takes the snapshot every day and commits it to a dedicated `data` branch. `main` stays code-only: its ruleset requires pull requests and passing CI, which a scheduled job can't satisfy.
- A **one-time backfill** from the Internet Archive's captures of `images.chainguard.dev/sitemap.xml` (17 months with a usable capture, April 2024 to October 2026), giving the history image names back to 2024. Free-tier status starts with the first live snapshot.
- A **narrow exception** in `openspec/config.yaml` to the rule that data comes only from Chainguard's public endpoints: this project's own catalog history file (published from the `data` branch), and the Internet Archive copies of Chainguard's sitemap used to build it.

## Capabilities

### New Capabilities
- `catalog-history`: recording the catalog's history (snapshots, their changes, the backfill and the daily schedule) in a file other features and tools can read.

### Modified Capabilities
None.

## Decisions

Made with the user in `/opsx:explore` on 2026-10-10:
- **Goal:** knowing what changed (added, removed, free/subscription changes), plus a chart of total vs. free over time. *Rejected:* totals only (misses what changed), and alerts (a different feature).
- **Collect outside the server**, on a schedule, so history has no gaps when nobody runs the server. *Rejected:* SQLite or a file written by the server (records only while it runs; SQLite also needs persistent disk and a new dependency).
- **Store the history in git, on a `data` branch.** *Rejected:* a database (unneeded at a few hundred bytes a day), and committing to `main` (blocked by its ruleset, and would run CI daily).
- **Backfill names from the Internet Archive**, accepting that free status starts on the first live day and that dates before then are only as precise as the monthly captures.
- **Allow the server to read this file from GitHub** (in the follow-up), with the config exception recorded here.
- **Split the work into two changes**, so collection starts as soon as possible.

## Out of Scope

- Anything that reads or shows the history: the `Service` method, `GET /api/history`, the MCP `catalog_changes` tool and first-seen date in `get_image_details`, the catalog History view and the per-image "in the catalog since" line. These are the follow-up change, `add-catalog-history-view`, recorded in `docs/visualization-ideas.md`.
- Embedding the history in builds and fetching newer copies (the follow-up).
- Free-tier status before collection starts (not recoverable).
- Notifications or alerts.

## Impact

- **Go:** a new `history.go` (snapshots, change lists, backfill), a `-record-history` and a `-backfill-history` flag in `main.go`, and tests with the existing fake backend. No change to the MCP tools, the web API or the web UI.
- **CI/CD:** a new workflow, `.github/workflows/history.yml`, with write access to the repository's contents; a new `data` branch.
- **Planning:** one exception added to `openspec/config.yaml`.
- **External load:** about 3,200 anonymous token requests to Chainguard a day (the same as one first "Free only" check), and about 20 one-time requests to the Internet Archive.
- **Dependencies:** none.
