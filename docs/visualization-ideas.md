# Visualization ideas

A backlog of chart ideas for the web UI, first proposed on 2026-10-08. Each idea uses data the server already has unless noted. When one is picked up, propose it as an OpenSpec change (`/opsx:propose`) and update its status here.

| # | Idea | Where | Status |
|---|---|---|---|
| 1 | Minimal vs. `-dev` comparison | Image page | Done: the Compare tab (`image-variant-comparison`, prefix `VC`) |
| 2 | License composition | Packages tab | Done: the license breakdown (`image-license-breakdown`, prefix `LB`) |
| 3 | Security fixes per package | Security tab | Next |
| 4 | Catalog map | Catalog page | Idea |
| 5 | Size vs. the upstream image | Alternatives page | Idea; riskiest |

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

## Skipped: charts over time

"Fixes over time" or "image freshness" timelines. The security database records which *versions* fixed each CVE, not *dates*, so a timeline would be guesswork. Real trends would need snapshots saved over weeks, which is a feature of its own.

## How to build them

- Plain SVG in React, sized with attributes so it works under the strict CSP (no chart libraries that inject `<style>` tags). The Compare tab and license breakdown use `web/src/components/HorizontalBars.tsx`, with no new dependencies.
- Every chart has a text equivalent on the page (a table or summary), and works in light and dark mode and at 375 px wide.
