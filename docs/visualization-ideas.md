# Visualization ideas

A backlog of chart ideas for the web UI, first proposed on 2026-10-08. Each idea uses data the server already has unless noted. When one is picked up, propose it as an OpenSpec change (`/opsx:propose`) and update its status here.

| # | Idea | Where | Status |
|---|---|---|---|
| 1 | Minimal vs. `-dev` comparison | Image page | Done: the Compare tab (`image-variant-comparison`, prefix `VC`) |
| 2 | License composition | Packages tab | Done: the license breakdown (`image-license-breakdown`, prefix `LB`) |
| 3 | Security fixes per package | Security tab | Done: the fixes chart (`image-security-fixes`, prefix `SF`) |
| 4 | Catalog map | Catalog page | Done: the Map view (`catalog-map`, prefix `CM`) |
| 5 | Size vs. the upstream image | Alternatives page | Idea; riskiest |
| 6 | Catalog history | Catalog page, image page, MCP | Scoped: collector first (`add-catalog-history-collector`, prefix `CH`), then `add-catalog-history-view` |

## 1. Minimal vs. `-dev` comparison

The packages and download size a `-dev` variant adds over the minimal image, e.g. `python:latest` vs `python:latest-dev`. It shows Chainguard's core idea, minimal runtime images, in one picture.

## 2. License composition

Package counts per license family (permissive vs. weak and strong copyleft), with each bar linked to its packages in the table. Answers "what licenses am I shipping?", which the package table buries.

## 3. Security fixes per package

A sorted, stacked horizontal bar per source package: fixes included vs. never affected, with any pending fixes highlighted. For python, nearly all recorded fixes are in OpenSSL, Expat and Python itself, which the current table (21 rows, mostly zeros) hides.

- Data: the existing vulnerabilities endpoint (`packages[]` with `fixed_count`, `not_affected_count`, `pending_fixes`).
- Reuse `HorizontalBars`; it needs stacked segments.
- Keep the security wording rule: the Wolfi data records fixes, not open vulnerabilities, so the chart must not suggest an image is vulnerability-free.

## 4. Catalog map

A treemap of all images (about 3,166) grouped into families, such as `nginx` with its `-fips` and `-iamguarded` variants. Rectangle size is the number of variants; color is free vs. subscription. A strong landing visual for demos.

- Catch: coloring needs free-tier status for every image. That takes about 30 seconds the first time and is cached for 24 hours, so the page needs a loading state (or colors that fill in progressively).
- Family grouping can reuse the variant-suffix rules in `alternatives.go` (`variantSuffixes`).
- The layout math is the one place a small dependency (`d3-hierarchy`) may be worth it; justify it in the design, as `openspec/config.yaml` requires.

## 5. Size vs. the upstream image

A bar comparing an upstream image (e.g. Docker Hub's `node:20`) with its Chainguard replacement. The most persuasive comparison, but the most work: it needs new Go code that reads Docker Hub's registry, which rate-limits anonymous requests.

## 6. Catalog history

What Chainguard added, retired, or moved between free and subscription, and how the catalog has grown (675 images in April 2024, 3,171 on 2026-10-10, 59 free). Scoped in `/opsx:explore` on 2026-10-10 and split in two:

1. **`add-catalog-history-collector`** (the `catalog-history` capability, prefix `CH`): a daily snapshot by a scheduled GitHub Action, one JSON line per day on the `data` branch, with a backfill of image names from 17 monthly Internet Archive captures of the sitemap since 2024-04. Free-tier status can't be backfilled, so it starts on the first live day. Done first so history starts accumulating.
2. **`add-catalog-history-view`** (not yet proposed). Decided scope:
   - **Goal:** knowing what changed (added, removed, became free or subscription), plus a chart of total vs. free images over time.
   - **Delivery:** the server embeds the history at build time and fetches a newer copy of the file from GitHub when online, falling back to the embedded copy; the API says how old its data is. The `openspec/config.yaml` exception for this file is added by the collector change. The browser can't fetch from GitHub under the CSP, so the server serves the history through its own API.
   - **Logic** in a `Service` method, shared by the API and MCP (`GET /api/history`).
   - **Web UI:** a **History** view on the catalog page, next to List and Map: the chart at the top (the free line starts on the first live day), then a timeline of changes ("Oct 10: 5 added: verdaccio, …"), filtered by the catalog's search. Plus an "in the catalog since …" line on each image page.
   - **MCP:** a `catalog_changes` tool (`since` date, optional `query`; returns added, removed, became free or subscription, and totals), and a first-seen date in `get_image_details`.
   - **Honesty:** before the first live snapshot, dates are only as precise as the monthly captures, so they read "between 2025-02 and 2025-03", never an exact day. Show the last snapshot's age, so a stalled collector is visible.

## Skipped: charts over time

"Fixes over time" or "image freshness" timelines. The security database records which *versions* fixed each CVE, not *dates*, so a timeline would be guesswork. Real trends need snapshots saved over time, which is what idea 6 does for the catalog.

## How to build them

- Plain SVG in React, sized with attributes so it works under the strict CSP (no chart libraries that inject `<style>` tags). The Compare tab and license breakdown use `web/src/components/HorizontalBars.tsx`, with no new dependencies.
- Every chart has a text equivalent on the page (a table or summary), and works in light and dark mode and at 375 px wide.
