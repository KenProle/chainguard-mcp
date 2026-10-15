# Proposal

## Why

Since 2026-10-10 the project has recorded Chainguard's catalog daily on its `data` branch (`catalog-history`, prefix CH), with 17 earlier snapshots rebuilt from the Internet Archive, but nothing reads the history yet. Chainguard's own endpoints only show the catalog as it is today. The history can answer questions they can't: what was added, retired, or moved between free and subscription, when an image first appeared, and how the catalog grew. On 2026-10-14 the file had 22 lines (96 KB, about 19 KB gzipped):

- **Growth:** 675 images on 2024-04-19 (the first capture) to 3,171 on 2026-10-14. 59 are free; free-tier status is known only from the first live snapshot, 2026-10-10.
- **Live era:** 5 live days so far. Only 2026-10-10 had changes (5 added, e.g. `verdaccio` and `openstack-barbican-oshelm`), and 2026-10-11 to 2026-10-14 had none.
- **Archive era:** the captures are irregular, from 3 weeks to 5 months apart (2026-05-10 to 2026-10-08 added 781 images, and 3 more came back). Their removals aren't reliable: 37 images "removed" from one capture reappear in a later one, and the 2025-10-02 capture lacks 120 images.

## What Changes

- **A History view on the catalog page**, next to List and Map. Everything in it follows the catalog's search, as the other views do.
  - Three summary tiles: images now, free now, and the change since the first capture.
  - A chart of total and free images over time (the free line starts on 2026-10-10).
  - A timeline of what changed ("Between 2026-10-08 and 2026-10-10: 5 added: verdaccio, …"). Days without changes collapse into one entry ("2026-10-11 to 2026-10-14: no changes"). Archive-era entries read as ranges between captures and sit under a collapsed "Before daily tracking" heading. With a search, the timeline starts at the first matching image.
- **"In the catalog since …" on each free image's page**, e.g. "since at least 2024-04-19 (the earliest capture)" for `python`, or "added between 2026-10-08 and 2026-10-10" for `verdaccio`.
- **How old the data is**, shown wherever history appears: the last snapshot's date. When the last snapshot is more than 2 days old, a note says the daily collector may have stalled.
- **A new web API endpoint, `GET /api/history`**, and **a new MCP tool, `catalog_changes`** (`since` date and optional `query`): changes and totals from one `Service` method.
- **A `catalog_since` field in `get_image_details`** and in the image details endpoint.
- **How the server gets the history:**
  - Release binaries and the Docker image embed a copy of the history taken at build time.
  - A running server fetches a newer copy from the `data` branch, at most once an hour.
  - If the fetch fails, or the fetched copy is invalid, the server falls back to the copy it has.
  - A build without an embedded copy (a plain local `go build`) shows a clear "no history available" state when offline.

## Capabilities

### New Capabilities
- `catalog-history-view`: reading the catalog history and presenting it, in the web UI, the web API and MCP: how the server obtains the file, how changes, growth and first-seen dates are derived and worded, and how the data's age is shown.

### Modified Capabilities
None. The collector (`catalog-history`) and its file format are unchanged. The new API endpoint is covered by WA-2's "plus any API endpoint added since".

## Decisions

Made with the user in `/opsx:explore` on 2026-10-10 (recorded under idea 6 in `docs/visualization-ideas.md`):
- **Goal:** knowing what changed, plus a chart of total vs. free images over time.
- **Delivery:** the history is embedded at build time and refreshed from GitHub when online, with the API saying how old its data is. The browser can't fetch from GitHub under the CSP, so the server serves the history. *Rejected:* fetching only at runtime (no history offline); embedding only (always stale between releases).
- **One `Service` method** behind `GET /api/history` and the MCP tools.
- **UI:** a History view next to List and Map, plus an "in the catalog since" line on image pages. **MCP:** `catalog_changes`, and a first-seen date in `get_image_details`.
- **Honesty:** archive-era dates are only as precise as the captures, so they read as ranges, never exact days, and a stalled collector is visible.

Made in this proposal (for the user to review):
- **Its own capability, `catalog-history-view` (prefix HV),** separate from the collector's `catalog-history`. One describes how the file is written, the other how it's read and shown.
- **Archive-era removals read "not in the next capture"**, not "removed". The captures miss images that come back later, so "removed" would overclaim. Live-era removals read "removed".
- **The search filters the chart as well as the timeline.** The chart then counts the matching images over time, e.g. how many `-fips` images there were. *Alternative:* a chart of the whole catalog only.
- **"In the catalog since" appears only on free images' pages,** where image details load. Subscription images keep the existing subscription message.

Layout, decided with the user on 2026-10-14 after a mockup with the real data:
- **Keep the chart's free line as it is,** although it covers only the last days of a 2½-year axis. The "Free" tile carries the current number. *Rejected:* a second chart of the daily-tracking period only.
- **A search's timeline starts at its first match** (HV-17), with no empty archive-era entries for images that didn't exist yet.
- **The archive era is collapsed** under "Before daily tracking (Internet Archive captures)" (HV-18), so daily changes come first.
- **Three summary tiles above the chart** (HV-14). *Rejected:* the chart alone.

## Out of Scope

- Changes to the collector, the history file's format or the daily workflow.
- Free-tier history before 2026-10-10 (it can't be reconstructed).
- Per-image version, tag or vulnerability history.
- Notifications or feeds of changes (RSS, webhooks).
- Correcting archive-era capture gaps. They're shown as captured, with the wording above.

## Impact

- **Go:**
  - New: a history reader (`Service.History`), a fetcher for the `data` branch's raw file, and a build-time embed with a committed placeholder (like `web/dist/.gitkeep`).
  - New: a `GET /api/history` route, a `catalog_changes` tool, and a `catalog_since` field on `ImageDetails`.
- **Builds:**
  - `.goreleaser.yaml` and the `Dockerfile` fetch the history before building.
  - CI's Docker smoke test checks the History view loads.
- **Web UI:** a History view (chart and timeline), the image page's since line, and new types in `web/src/api.ts`.
- **Docs:**
  - The README's catalog history section ("Nothing in the server or UI reads the history yet") and its API table.
  - CLAUDE.md's catalog history notes.
  - Idea 6's status in `docs/visualization-ideas.md`.
- **External load:** at most one request an hour per running server to `raw.githubusercontent.com` for a file of about 100 KB. This is already allowed as the project's own history file (`openspec/config.yaml`).
- **Dependencies:** none.
