# Visualization ideas

A backlog of chart ideas for the web UI, first proposed on 2026-10-08. Each idea uses data the server already has unless noted. When one is picked up, propose it as an OpenSpec change (`/opsx:propose`) and update its status here.

| # | Idea | Where | Status |
|---|---|---|---|
| 1 | Minimal vs. `-dev` comparison | Image page | Done: the Compare tab (`image-variant-comparison`, prefix `VC`) |
| 2 | License composition | Packages tab | Done: the license breakdown (`image-license-breakdown`, prefix `LB`) |
| 3 | Security fixes per package | Security tab | Done: the fixes chart (`image-security-fixes`, prefix `SF`) |
| 4 | Catalog map | Catalog page | Done: the Map view (`catalog-map`, prefix `CM`) |
| 5 | Size and known vulnerabilities vs. the upstream image | Alternatives page | Idea; data sources approved; explore before proposing |
| 6 | Catalog history | Catalog page, image page, MCP | Scoped: collector first (`add-catalog-history-collector`, prefix `CH`), then `add-catalog-history-view` |
| 7 | Open vulnerabilities on Chainguard images | Security tab, MCP | Idea; the user thinks it's worth doing |

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

**Extended on 2026-10-10: compare known vulnerabilities too**, where most of the value is. Checked that day:
- Docker Hub's official images publish SBOM attestations: `node:20` has an SPDX document (15.8 MB for amd64) and SLSA provenance, read through the same registry API the app uses for Chainguard.
- OSV.dev (free, no key) answers known vulnerabilities by package and version for Debian, Alpine and Ubuntu, and for Wolfi/Chainguard, so both sides can go through the same pipeline: SBOM → OS packages → OSV batch query.
- Anonymous Docker Hub access is limited to 100 requests an hour per IP (`x-ratelimit-limit: 100;w=3600`), so results need caching.

**Decided with the user:** Docker Hub and OSV.dev may be used as data sources. Add them as exceptions in `openspec/config.yaml` as the first task of the change that uses them.

**For the plan to settle:** compare OS packages only on both sides (upstream SBOMs also list language packages, such as npm modules); split counts by severity and by fix available vs. no fix yet, so distro "won't fix" low-severity issues don't make a one-sided number; word results as "known vulnerabilities per OSV.dev on <date>", never "secure"; say that each side is graded by its own maintainers (OSV collects Debian's security tracker for the upstream image and Chainguard's own advisories for its image; see idea 7 for that feed's limits), e.g. "per each distribution's published advisories, via OSV.dev"; show "no SBOM published" for images without one; and keep the comparison factual, since the project is unaffiliated. Possibly an MCP tool too. Before trusting the numbers, cross-check both sides with grype (see "Cross-checking with grype" below). Using OSV for *open* vulnerabilities on Chainguard images is out of scope here; it's idea 7.

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

## 7. Open vulnerabilities on Chainguard images

Today the Security tab (and `check_vulnerabilities`) can only show *recorded fixes*, because the Wolfi security database lists the vulnerabilities each package version fixed, not the ones still open. OSV.dev also covers the Wolfi and Chainguard ecosystems, so the app could also show known vulnerabilities that are **still open** in an image's installed packages, which is the question users actually ask.

**Real figures, python `latest` (amd64) on 2026-10-10**, from one OSV batch query of its 29 installed packages (ecosystem `Wolfi`):
- 20 advisories against 4 packages: `glibc-2.44`, `glibc-2.44-locale-posix` and `ld-linux-2.44` (all `2.44-r8`, 16 advisories between them) and `python-3.15` (`3.15.0_git20261010-r0`, 4 advisories).
- They are Chainguard's own advisories (`CGA-…` IDs), each tracking an upstream CVE (e.g. `CGA-2727-9j94-3x3p` → `CVE-2026-89092`), and the ones checked have the status `pending_upstream_fix`: known, but no fix has been released upstream yet.
- The image's OpenSSL library (`openssl-4.0-libcrypto 4.0.3-r6`) has no open advisories.
- (An earlier test that day returned 152 vulnerabilities for `openssl` 3.0.0-r0, a deliberately ancient version no current image ships. That only showed OSV knows the ecosystem; it isn't a figure for any image.)

**Where these records come from, and their limits.** Chainguard builds every package in its images from source, under its own package names (`glibc-2.44`, `openssl-4.0-libcrypto`), so no outside database knows them on its own. OSV can answer only because Chainguard publishes advisories for its own packages, in OSV format, keyed by those names: it triages each upstream CVE per package (`CVE-2026-89092` → `CGA-2727-9j94-3x3p` for `glibc-2.44`) and OSV collects the feed. Caveats a plan must carry:
1. **It is Chainguard's own assessment, not an independent scan.** OSV knows only what Chainguard reported, the same caution as for the "never affected" counts. Word results as "per Chainguard's published advisories, via OSV.dev", and cross-check with grype (see "Cross-checking with grype" below).
2. **Software bundled inside a package** (Go modules compiled into a binary, vendored Python libraries) appears only if Chainguard files an advisory against the APK. It often does (the GHSA IDs in the python results are language-library advisories mapped onto Chainguard packages), but coverage depends on its triage.
3. **Ask the right ecosystem per package:** free images use Wolfi packages, and subscription images can also contain packages from Chainguard's private repository. The SBOM's package URLs (the app's `distro` field) say which, so query `Wolfi` or `Chainguard` accordingly. Today's Security tab skips non-Wolfi packages, so this could close that gap too.

This is what the Security tab can't show today, and what the "never vulnerability-free" rule anticipates. A plan should show each advisory's status (such as "waiting on an upstream fix" vs. "fix available"), read from OSV's `ecosystem_specific.components[].latest_event_status`, and link the upstream CVE.

Noticed while scoping idea 5, which uses OSV for both sides of an upstream comparison; kept as its own change because it changes what an existing tab means. Things a plan would need to settle:
- **Wording:** "known open vulnerabilities per OSV.dev on <date>", and never "vulnerability-free" when the count is zero; the `config.yaml` rule against overclaiming still applies.
- **How it relates to the fixes chart (`image-security-fixes`, prefix `SF`):** alongside it, or merged into one view of fixed, never-affected and open.
- **Freshness:** open vulnerabilities change daily as advisories are published, so results need dates and caching.
- **Data source:** OSV.dev is approved as a data source (idea 5); its `config.yaml` exception goes in with whichever change uses OSV first.
- Building idea 5 first makes this cheaper, since the OSV client would already exist.

## Cross-checking with grype (for ideas 5 and 7)

Before trusting OSV-based vulnerability numbers in the UI, compare them with an independent scanner:
- **While planning idea 5 or 7:** run [grype](https://github.com/anchore/grype) by hand on a few images (for example `cgr.dev/chainguard/python:latest` and Docker Hub's `node:20`) and compare its findings with the OSV results for the same packages. Record the comparison in the change's design: matches, and anything OSV misses (such as vulnerabilities in Go modules compiled into binaries). A large gap means the plan must change, or its wording must say what the numbers leave out.
- **As a lasting check:** an optional live test, skipped unless `CHAINGUARD_LIVE=1` like the existing ones and not run in CI, that repeats the comparison on demand, asserting stable properties (OSV finds every OS-package vulnerability grype finds, give or take a stated tolerance) rather than exact counts. If grype isn't installed, the test is skipped.

**Why grype isn't a feature of the app:** it's a large Go dependency with its own vulnerability database (several hundred MB from Anchore, refreshed regularly), which would make this small, stateless server heavy and add another outside data source. For OS packages it largely reaches the same answers from the same distro feeds that OSV collects, and anyone wanting a full scan can already run it themselves (the README points to it), including alongside the MCP tools.

## Skipped: charts over time

"Fixes over time" or "image freshness" timelines. The security database records which *versions* fixed each CVE, not *dates*, so a timeline would be guesswork. Real trends need snapshots saved over time, which is what idea 6 does for the catalog.

## How to build them

- Plain SVG in React, sized with attributes so it works under the strict CSP (no chart libraries that inject `<style>` tags). The Compare tab and license breakdown use `web/src/components/HorizontalBars.tsx`, with no new dependencies.
- Every chart has a text equivalent on the page (a table or summary), and works in light and dark mode and at 375 px wide.
