# Design

## Context

See `proposal.md` for why, and `specs/image-security-fixes/spec.md` for the requirements (SF-1 to SF-9).

Current state of the web UI (`web/`):
- `SecurityTab.tsx` receives `name` and `tag` from `ImagePage` (which keeps the tag in the URL) and fetches `['vulnSummary', name, tag]` through TanStack Query. On success, the "Recorded fixes" card shows three `Stat` tiles (fixes included, never affected, fixes not yet installed), a red-bordered box listing each package with `pending_fixes` as `package version: IDs`, and a table of `packages[]` (source package, version, fixes, not affected). A paragraph under the card says the data "doesn't list unfixed vulnerabilities, so it isn't a full scan".
- The summary response (`VulnReport` in `web/src/api.ts`, built by `analyzeVulns` in `vulns.go`) has one `PackageFixes` per Wolfi source package, sorted by name, with `fixed_count`, `not_affected_count` and optional `pending_fixes` and `records_from`. Packages with no records are included with zeros. `pending_fixes` holds IDs fixed only in a newer version than the one installed. For version-stream packages that borrow a base package's records (`openssl-4.0` from `openssl`), newer fixes are skipped rather than reported as pending, because they may belong to another stream.
- Subscription-only images never reach `SecurityTab`: `ImagePage` shows the subscription message when the tags request returns `not_public`.
- `HorizontalBars` (`web/src/components/HorizontalBars.tsx`) draws `{ label, value, valueLabel, colorClass?, accessibleLabel? }[]` as one SVG `rect` per bar in a `viewBox="0 0 100 10"`, sized by attributes on a zero-based linear scale. Without `onSelect` it is wrapped in `aria-hidden` (Compare tab); with `onSelect` each row is an `aria-pressed` button (license breakdown).
- `web/src/compare.ts` and `web/src/licenses.ts` are the model: pure exported functions with exact-number tests, thin components. Shared class strings live in `web/src/styles.ts`.

**No API changes.** The Go server, the web API and the MCP tools are untouched; the chart is computed from the summary response the tab already loads.

## Goals / Non-Goals

**Goals:**
- Row selection and ordering are a pure function, unit-tested against SF-1 and SF-3 with the real python summaries.
- `HorizontalBars` gains stacked segments without changing what the Compare tab or the license breakdown render. Their existing tests pass unchanged.

**Non-Goals:**
- Changing the existing totals, pending-fix box, table or note. They are the text equivalent (SF-4, SF-6, SF-7) and stay as they are.
- Making the chart interactive.

## Decisions

### 1. Logic in a pure module, `web/src/fixes.ts`
Exports:
- `type FixRow = { package: string; fixed: number; notAffected: number; pending: number }`. `pending` is the length of `pending_fixes`.
- `fixesChart(packages: PackageFixes[] = [])` → `{ rows: FixRow[]; withoutRecords: number }`. `rows` keeps packages whose three counts sum to more than zero (SF-1), sorted by `pending` desc, then `fixed` desc, `notAffected` desc and name ascending (SF-3). `withoutRecords` counts the rest (SF-7.1).
- `fixesWithoutRecordsText(withoutRecords, total)` → the no-records line, kept here so its wording is tested in one place (SF-7): "9 more source packages have no records in the Wolfi security database." (singular "1 more source package has…"), or, when no package has records, "None of this image's 21 source packages have records in the Wolfi security database." It returns nothing when every package has records.

*Why:* SF-1.1 (12 rows, 9 without records), SF-1.3 (25 rows) and SF-3.1's full order become exact unit tests, and the counts were checked against the live responses with a throwaway script while writing this change. *Alternative:* sort inside the component. Rejected for the same reasons as in the earlier charts: harder to test and mixes data and markup.

### 2. Optional stacked segments in `HorizontalBars`
Adds `Bar.segments?: { value: number; colorClass: string }[]`. When present, the bar is drawn as consecutive `rect`s, each starting where the previous one ended, with widths on the same scale as unsegmented bars. Segments with value 0 draw no `rect`, so a bar with no fixes not yet installed has no pending segment (SF-4.2). The scale is still each bar's `value`, so callers set `value` to the sum of its segments. `fixes.ts` does, and a unit test checks it. Without `segments` a bar renders exactly as today, a zero value still giving one zero-width `rect`, which the existing "zero-width bar" test checks.

The chart is used without `onSelect`, so it keeps the `aria-hidden` wrapper (SF-6).

*Why one component:* the two-column layout (label above the bar on phones, beside it from `sm`) and the value-label column are already right at 375 px (SF-9), and a third chart shouldn't fork them. *Alternatives:* a separate `StackedBars` component, rejected as duplicated layout; percentage-based segments, rejected because stacked bars must share the absolute scale for SF-2.2.

Segment colors are in `web/src/styles.ts` as `fixSegmentColor: Record<'fixed' | 'notAffected' | 'pending', { fill: string; swatch: string }>`, with complete class strings so Tailwind's scanner finds them: fixes included `fill-indigo-500 dark:fill-indigo-400`, never affected `fill-zinc-300 dark:fill-zinc-600`, not yet installed `fill-rose-500 dark:fill-rose-400`, with matching `bg-*` classes for the legend swatches. Indigo, the app's accent, rather than green, so "fixes included" doesn't read as "safe" (SF-7). Classes, not inline styles, keep the CSP intact (SF-9).

### 3. A `FixesChart` component in the "Recorded fixes" card
`web/src/components/FixesChart.tsx` takes `packages`, calls `fixesChart`, and renders, between the pending-fix box and the table:
1. a heading "Fixes per source package" and a legend: three `span` swatches with their text "Fixes included", "Never affected" and "Not yet installed" (SF-5). The legend wraps on phones.
2. `HorizontalBars` with one bar per row: label the package name, `value` the row's total, `segments` in the SF-2 order, `valueLabel` "147 fixed" (SF-5)
3. the no-records line from `fixesWithoutRecordsText` (SF-7.1, SF-7.2)

When no package has records, it renders the heading and the "None of…" line but no legend and no bars (SF-7.2). When `packages` is empty or missing (an image with no Wolfi packages), it renders nothing. It is rendered only in the summary's success branch, so loading and error states stay as they are and no request is added (SF-8). The chart recalculates when the tag changes because the summary's query key includes the tag (SF-8.2).

The pending-fix text that SF-4.1 asks for is the existing box, not new text. The value label stays "N fixed" even for packages with fixes not yet installed, because the box names those packages and their IDs.

### 4. Test fixtures
New `web/src/test/fixtures/pythonVulns.ts`: the `python` `latest` and `latest-dev` summaries captured through the MCP server on 2026-10-09 (digests `sha256:6cc531a9…` and `sha256:9876dbb8…`), with a header comment like `tomcat.ts`'s. Pending-fix scenarios (SF-2.3, SF-3.2, SF-4.1) derive a report from the `latest` fixture in the test, adding `pending_fixes: ['CVE-2026-0001']` to `zlib`, because no live image had one that day. `mockApi` already matches query parameters, so the tag switch (SF-8.2) needs no harness changes. Test names start with their scenario ID (`SF-1.1 …`).

## Risks / Trade-offs

- **[A green-light reading]** A chart full of fixes can look like reassurance. → No green; the no-records line says "no records", not "no vulnerabilities"; the existing note stays; SF-7.2 tests that the tab never says the image has no vulnerabilities or is secure.
- **[Small counts are invisible]** `gcc`'s 1 fix is about 1/150 of the width. → Every bar prints its count, and the table has exact numbers. A minimum segment width was considered and rejected because it breaks SF-2's proportionality.
- **[Long charts]** `latest-dev` has 25 bars, roughly 600 px tall on phones, where labels sit above bars. → Accepted. The table below it was already 56 rows. Capping to the top N with "show all" is a possible follow-up.
- **[Pending fixes are rare in live data]** The highlighted path is tested only with a modified fixture. → The modification is minimal and documented in the spec. The live check in task 4.2 confirms the other paths.
- **[Extending a shared component]** could change the Compare tab or the license breakdown. → `segments` is optional and the unsegmented code path is unchanged. `HorizontalBars.test.tsx`, `CompareTab.test.tsx` and `PackagesTab.test.tsx` must pass unchanged.
- **[Live data drifts]** Chainguard rebuilds daily. → Tests use the dated fixture; the spec's examples are dated.

## Migration Plan

Frontend only, shipped in the embedded UI on the next build. Rollback is reverting the change; there's no data, API or configuration migration. **No new dependencies are added.**
