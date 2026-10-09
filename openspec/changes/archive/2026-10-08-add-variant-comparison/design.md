# Design

## Context

See `proposal.md` for why, and `specs/image-variant-comparison/spec.md` for the requirements (VC-1 to VC-14).

Current state of the web UI (`web/`):
- `ImagePage.tsx` fetches the image's tags (a 403 there already shows the subscription message and stops), then renders three tabs. The active tab and the single selected tag live in the URL (`?tab=…&tag=…`).
- The tabs fetch through TanStack Query with shared cache keys: `['details', name, tag]` and `['packages', name, tag, arch]`.
- `GET /api/images/{name}/details?tag=` returns every platform's compressed size in one response; `GET /api/images/{name}/packages?tag=&arch=` returns one architecture's packages plus `has_shell` and `has_apk`.
- Charts must be CSP-safe (no inline `<style>`), and `formatBytes` (`web/src/styles.ts`) currently formats in binary units.

**No API changes.** The Go server and web API are untouched; everything the comparison needs is in the existing details and packages responses.

## Goals / Non-Goals

**Goals:**
- Comparison logic (choosing the pair, diffing packages, size difference) lives in pure functions that are unit-tested directly against the scenarios.
- The Compare tab shares the query cache with the other tabs, so switching tabs doesn't refetch.
- A small bar-chart component that the license breakdown (spec 002) can reuse.

**Non-Goals:**
- Comparing more than two tags at once.
- Any server-side comparison endpoint or MCP tool.

## Decisions

### 1. Logic in a pure module, `web/src/compare.ts`
Three functions, each mapping to requirements:
- `defaultPair(tags)` → `latest`/`latest-dev` if both exist, otherwise the first base tag `X` (alphabetical) with an `X-dev`, otherwise `null` (VC-1, VC-3).
- `diffPackages(a, b)` → `onlyInA`, `onlyInB`, `versionChanged`, matched by package name (VC-7).
- `sizeDifference(aBytes, bBytes)` → absolute difference, ratio of larger to smaller, and which side is larger; `nearlyEqual` when within 1% (VC-6).

*Why:* the scenarios become fast unit tests with exact numbers (e.g. VC-6.1: 26,882,525 and 272,631,719 bytes give "+245.7 MB" and "10.1×"), and the component stays thin. *Alternative:* logic inside the component, tested only through rendering, which is slower and makes exact-number tests awkward.

### 2. Data: two details and two packages queries, reusing the cache keys
For pair (a, b) and architecture `arch`: `details(a)`, `details(b)`, `packages(a, arch)`, `packages(b, arch)`, using the same query keys as the other tabs. Sizes come from the details response for platform `linux/<arch>`, so switching architecture refetches only the two packages queries (VC-10).

Request budget: 1 tags (already loaded by `ImagePage`) + 2 details + 2 packages = 5, as the proposal requires. *Alternative:* a new `/api/images/{name}/compare` endpoint returning a precomputed diff. Rejected: it duplicates logic that's already client-side for the other tabs, and needs Go changes the proposal doesn't call for.

### 3. URL state: `?tab=compare&a=<tag>&b=<tag>&arch=<arch>`
The pair and architecture are query parameters, as for the other tabs (VC-4). The Compare tab ignores the page's single `tag` parameter, and the page-level tag selector is hidden on this tab so there's only one way to pick tags.

Rules:
- `a`/`b` absent → `defaultPair(tags)`; if that's `null` → the no-variant message (VC-3).
- `a === b`, or a tag that doesn't exist → treated as absent.
- When the user changes `a` to `X` and `X-dev` exists, `b` becomes `X-dev` (VC-1.2). Otherwise `b` is kept, unless it now equals `a`, in which case it becomes the first other tag.
- The `b` selector never offers `a`'s current value (VC-2.2).
- With a single tag, no selectors are shown (VC-3.2).

### 4. Size chart: SVG bars, no chart library
A `HorizontalBars` component in `web/src/components/` draws labeled bars as SVG `<rect>`s whose `width` attributes come from a linear scale starting at zero over the largest value (VC-9). It takes `{ label, value, valueLabel }[]`, so the license breakdown can reuse it.

*Why SVG attributes:* they aren't styles, so they're unaffected by the strict `style-src 'self'` policy (VC-14). *Alternatives:* a chart library (Recharts and others) or D3. Rejected: a two-bar chart doesn't need either, and some chart libraries inject `<style>` tags the CSP blocks. **No new dependencies are added.**

The SVG is marked `aria-hidden="true"`, because the same information is on the page as text (decision 5).

### 5. Text first: a summary table, with the chart as an enhancement
The tab renders, in order:
1. a sentence, e.g. "latest-dev is 10.1× larger than latest (+245.7 MB) on amd64"
2. a summary table with one row per tag: packages, download size, shell, `apk` (VC-5, VC-8)
3. the bar chart (VC-9)
4. the three package groups, each with a count, listing package names (with both versions in the changed group), or "None" (VC-7)

This satisfies VC-13 without hidden screen-reader-only text: everything in the chart is already visible as text above it.

### 6. Loading and errors: all-or-nothing
The comparison renders only when all four queries succeed. While any is pending, a loading indicator is shown. If any fails, its `ErrorState` is shown with a retry that refetches only the failed queries, and no partial comparison is shown (VC-12). Subscription-only images never reach the tab, because `ImagePage` stops at the tags 403 (VC-11). A tag without the selected architecture fails its packages query with `not_found`, shown as "latest-dev has no arm64 variant".

### 7. Decimal megabytes everywhere (decided with the user, 2026-10-08)
`formatBytes` switches from binary to decimal units (1 MB = 1,000,000 bytes) with one decimal place, matching Docker's CLI, Docker Hub and the spec's figures. This also changes the Overview tab, e.g. python from "26 MB" to "26.9 MB", so both tabs agree. *Rejected:* binary MiB everywhere (would change the spec's numbers and differs from what Docker shows), and decimal only on the Compare tab (the same image would show different sizes on two tabs).

### 8. Test fixtures, not live data
Component tests use fixtures built from the real October 2026 python data (29 and 76 packages, the real sizes), served by `mockApi`. `mockApi` (`web/src/test/render.tsx`) currently picks a response by path only; it gains an optional match on query parameters, so `tag=latest` and `tag=latest-dev` can return different data. Test names start with their scenario ID (`VC-6.1 …`).

## Risks / Trade-offs

- **[Live data drifts from the spec's examples]** Chainguard rebuilds images daily, so the counts in VC-5.1 and VC-7.1 will change. → Tests use fixtures; the scenarios document the behavior with a dated example, not a permanent fact.
- **[Diffing by name misses renames]** A package renamed between tags (e.g. `libssl3` → `openssl-4.0-libssl`) shows as one removal and one addition. → Acceptable: that's accurate at the package level, and tags of one image are built from the same package set at the same time.
- **[Large groups]** A `-dev` variant can add dozens of packages (47 for python). → The groups render as compact wrapped lists, so the page stays scannable at 375 px.
- **[Changing `formatBytes` affects existing UI and tests]** → The Overview test fixture expecting "30 MB" is updated to the decimal value in the same change, and the Overview tab is rechecked in the browser.
- **[Ratio wording when the second tag is smaller]** e.g. comparing `latest-dev` with `latest`. → `sizeDifference` reports which side is larger, and the sentence always names the larger tag first ("latest-dev is 10.1× larger than latest").

## Migration Plan

Frontend only, shipped in the embedded UI on the next build. Rollback is reverting the change; there's no data, API or configuration migration.
