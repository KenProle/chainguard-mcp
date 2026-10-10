# Design

## Context

See `proposal.md` for why, and `specs/catalog-map/spec.md` for the requirements (CM-1 to CM-17).

Revised on 2026-10-09 after the first build: the treemap is now drawn by variant or name-prefix group (decisions 9 to 11) instead of by family. Decisions 1 to 3 stand, and 4 to 8 are revised in place. Revised again the same day: each group is divided into name-prefix blocks (decision 12, CM-18).

Current state:
- `Catalog.Images` (`catalog.go`) returns the sorted image names from the sitemap, cached for 1 hour. 3,166 names on 2026-10-09.
- `Service.ListImages` (`service.go`) filters by a lowercased substring `query`, pages the matches (limit up to 1,000) and calls `Registry.PublicStatus` on the page only, unless `free_only` is set, in which case it checks every match first. `PublicStatus` runs 8 workers over `IsPublic`, which asks `cgr.dev/token` once per image and caches the answer for 24 hours. A failed check leaves `free` out of that image's summary. `GET /api/images` maps straight onto it.
- `variantSuffixes` and `isVariant(img, base)` (`alternatives.go`) decide which images `find_alternative` lists as variants of a base name.
- `CatalogPage.tsx` keeps `q`, `free` and `page` in the URL, building updates from a ref holding the latest params (see CLAUDE.md), and renders a paginated list with `FreeBadge` (green "Free", amber "Subscription", nothing when unknown).
- `HorizontalBars` already supports stacked `segments` and is `aria-hidden` without `onSelect`.
- `newFakeService` (`fake_test.go`) has 11 images: `go`, `jdk`, `jre`, `loki-fips`, `node`, `node-fips`, `node-lts`, `python`, `python-fips`, `static` and `wolfi-base`.

**The Go server and web API change:** new `Service.ImageFamilies` and `Service.ImageGroups` methods and `GET /api/families` and `GET /api/groups` endpoints. MCP tools are unchanged.

## Goals / Non-Goals

**Goals:**
- One definition of "family" and of "variant group", in Go, shared with `find_alternative` and tested against the real 2026-10-09 catalog.
- The map is on screen as soon as the catalog loads, with statuses filling in after it.
- The treemap layout is a pure, unit-tested function with no dependency.

**Non-Goals:**
- Faster or cheaper free-tier checks. The Map view uses exactly the checks and cache that "Free only" uses.
- Making individual treemap units keyboard-focusable (CM-12).

## Decisions

### 1. Family grouping in Go, next to `variantSuffixes`
`alternatives.go` gains:
- `familyName(image string) string`: split on `-` and drop trailing parts while they are in `variantSuffixes`, never dropping the first part (CM-1). `node-local-dns` stays itself because `dns` isn't a suffix. `gcc-glibc` becomes `gcc` (CM-2.2).
- `groupFamilies(images []string) []ImageFamily`: one entry per family name with its images in name order, ordered by image count descending, then name (CM-3). Since a family's name is a prefix of each of its images, its base image, if it exists, sorts first, so the link target in CM-2 is always `Images[0]`. The UI uses that rule rather than asking the server for a link.

By construction, for every image `i` in family `F` other than `F` itself, `isVariant(i, F)` holds, which is CM-1.4. A unit test checks both directions over the real catalog. *Alternative:* grouping in TypeScript. Rejected: a second copy of `variantSuffixes` would drift, and `openspec/config.yaml` forbids front ends growing their own logic.

### 2. `Service.ImageFamilies` and `GET /api/families`
```go
type ImageFamiliesInput struct { Query string `json:"query,omitempty"` }
type ImageFamily struct {
    Name   string   `json:"name"`
    Images []string `json:"images"`
}
type ImageFamiliesOutput struct {
    Total    int           `json:"total"`    // images matching the query
    Families []ImageFamily `json:"families"`
}
```
It groups the whole catalog, then keeps the images matching the query and drops empty families (CM-4), so a query never regroups an image. The query match is pulled out of `ListImages` into a shared helper, so both endpoints return the same images for a query (CM-4.1). It never calls the registry (CM-3.3). Grouping 3,166 names takes well under a millisecond, so there is no cache beyond the catalog's own. `web.go` adds `GET /api/families?query=` with `respond`, so errors map as for `/api/images` (CM-3.4). `web/src/api.ts` mirrors the types and adds `api.families(query, signal)`.

*No MCP tool:* `find_alternative` already answers "what are the variants of X, and are they free?", and `list_images` handles search. The full grouping (~1,751 families) is the shape of a page, not of an answer. *No `free` in this response:* adding it would make the endpoint wait about 30 seconds on first use, which is what CM-8 avoids.

### 3. Statuses from the existing image list, in sequential batches
The Map view loads statuses with TanStack Query's `useInfiniteQuery` over `api.listImages({ query, limit: 1000, offset })`, keyed `['freeStatus', query]`, with the next offset taken from the previous page while `offset + count < total`. An effect calls `fetchNextPage` whenever the last page has arrived and there is a next one, so batches run one at a time (CM-8.2). Each page's `free` values go into a `Map<string, boolean>`; an image in a returned page without `free` is a failed check (CM-7.3). Images not yet in any page are "not checked yet". Both show the not-known color.

A failed batch stops the chain. The page shows `ErrorState` whose retry calls `fetchNextPage`, which refetches only the missing batch (CM-8.4). A new search changes the query key, so the old chain is abandoned and a new one starts (CM-8.5).

*Why the existing endpoint:* no new status code, and the load on `cgr.dev/token` equals one "Free only" use: 3,166 requests the first time, 8 concurrent, then nothing for 24 hours. *Alternatives:* `free_only=true` in one request (no progress for 30 seconds, and failed checks indistinguishable from subscription images); server-side background checks with polling (new concurrency and lifecycle code). Batches of 1,000 because that is `ListImages`' maximum limit. Four batches give visible progress without extra requests.

### 4. Pure layout in `web/src/treemap.ts`, no `d3-hierarchy` (revised)
```ts
type Rect = { x: number; y: number; w: number; h: number }
squarify(values: number[], area: Rect): Rect[]
layoutGroups(counts: number[], area: Rect, opts: { gap: number; header: number }): GroupLayout[]
// GroupLayout = { rect: Rect; header?: Rect; units: Rect[] }
```
`squarify` is the squarified algorithm (Bruls, Huizing and van Wijk): values are laid in rows along the shorter side, adding a value to the current row while that doesn't worsen the row's worst aspect ratio. Values must be positive and sorted descending; it returns rectangles in input order whose areas are proportional to the values and that exactly tile `area`.

`layoutGroups` squarifies the group counts (CM-6.1, CM-6.3), insets each group's rectangle by half the gap on every side so neighbors are separated by the background color (never more than a quarter of its width or height, so a sliver of a group keeps its units), takes a header band of fixed height off the top when the inset rectangle is at least two headers tall and 40 units wide (CM-17.2), and squarifies `count` equal units into what's left (CM-6.2). Group order comes from the API (count descending), and units are in the order of the group's images (name order), so the layout depends only on the groups and never on statuses (CM-6.4). Insetting makes each group's drawn area slightly smaller than its share; CM-6.1 is tested on the un-inset rectangles, and units within a group stay exactly equal.

The drawing area is a fixed `viewBox="0 0 1000 600"`, with a 3-unit gap between groups and a 22-unit header. Every image gets about 190 square units, about 14 × 14 px at 1,000 px wide. The SVG scales with its container through width and height attributes.

*Why not `d3-hierarchy`:* it's pure layout math with no DOM or style injection, so it would satisfy the CSP, and it has nested treemaps with padding. But the work here is two levels over small positive integers, under 100 lines, and its tests (tiling, proportional areas, header rules) are needed either way to prove CM-6 and CM-17. **No new dependency is added.**

### 5. Treemap component, tooltip and colors (revised)
`web/src/components/CatalogTreemap.tsx` takes the groups, the status map, the summary and the grouping's name. It renders:
- per group: the header band (if any) as a filled `rect` with a `text` label chosen by `headerText` (decision 6): "FIPS: 1,210", else the label alone, else the label cut short with "…" (`kubern…`), so a band is never empty; hovering a header shows the tooltip "kubernetes: 51 images" (CM-17). *Rejected:* dropping the text when the label doesn't fit, which left empty bands in the first build of the prefix view. Because long names shorten, the minimum size for a header is low (two header heights tall, 40 units wide); a first rule of three headers tall and 80 wide left `cert`, `harbor`, `gitlab` and `kube` without headers in the prefix view
- per image: one `rect` unit with a fill class for its status and a thin stroke in the card's background color, so units are separated by hairline gaps (CM-7); `data-image` and `data-group` attributes for the pointer handlers
- `role="img"` and an `aria-label` with the CM-12.1 summary on the `svg`, which makes its children presentational and adds no tab stops

The units are a memoized child component keyed by the groups, layout and statuses, so hovering (which changes only tooltip state) never re-renders the 3,166 `rect`s, and neither does sorting or paging the table.

**Tooltip (CM-9.1):** `onPointerMove` on the `svg` reads the unit under the pointer from `data-image` and converts the pointer position to SVG coordinates with `svg.getScreenCTM().inverse()`. The tooltip is a `g` drawn last in the SVG, with a dark rounded `rect` and one `text` line "nginx-fips · subscription · FIPS", placed 12 units right of and below the pointer and flipped to the other side near the right or bottom edge. Its width is estimated from the text length (the same estimate as the header text), which is enough for a background box. It has `pointer-events="none"`, appears on the first move without delay, and disappears on `pointerleave`. Everything is SVG attributes and classes, so the CSP holds (CM-13). The `<title>` elements are removed, so the browser's slow built-in tooltip no longer competes. *Alternative:* an HTML tooltip positioned with `style.left/top`, rejected because inline style attributes would conflict with the strict `style-src` policy.

Clicks are delegated as before: a single `onClick` on the `svg` reads the unit's `data-image` and navigates to `/images/<name>` (CM-9.2).

Colors in `web/src/styles.ts` as `freeStatusColor: Record<'free' | 'subscription' | 'unknown', { fill: string; swatch: string }>`: free `fill-emerald-500 dark:fill-emerald-400` (the only saturated color), subscription `fill-slate-300 dark:fill-slate-700` (blue-gray), not known `fill-zinc-200 dark:fill-zinc-800` (light enough to read as empty, dark enough that the map's shape shows before any status arrives), with matching `bg-*` swatches. Headers are `fill-slate-700 dark:fill-slate-300` with `fill-white dark:fill-slate-900` text. Groups get no colors of their own (CM-7). The table's `FreeBadge` keeps its green and amber badges; they are text labels, not the map's colors.

### 6. Counting and sorting in `web/src/catalogMap.ts`
Pure functions, unit-tested like `fixes.ts`:
- `imageStatus(name, statuses)` → `'free' | 'subscription' | 'unknown'`; statuses are one map from name to `true`, `false` or `null` (checked, failed), absent meaning not checked yet
- `statusCounts(images, statuses)` → `{ free, subscription, unknown }` (legend, CM-7.2; phone bars, CM-11)
- `familyRows(families, statuses)` → rows with `imageCount` and `freeCount: number | null`, null while any image is unchecked (CM-10.4)
- `sortFamilyRows(rows, 'images' | 'free' | 'name')` (CM-10.2)
- `mapStatusText(...)`, `treemapSummary(...)`, `tooltipText(...)` and `headerText(label, count, width)` for the status line, the `aria-label`, the tooltip and the headers (CM-8, CM-12.1, CM-9.1, CM-17)

### 7. Catalog page layout (revised)
`CatalogPage` keeps `view=map` in the URL (CM-5), set by a two-button "List / Map" switch with `aria-pressed`, through the existing `update` helper. Switching view clears `page`. In Map view the page hides "Free only" and the list pagination, and renders a `CatalogMap` component with:
1. a "Group by" switch (Variant / Name prefix, `aria-pressed`), kept in the URL as `group=prefix` and omitted for the default (CM-5.4)
2. the status line (`aria-live="polite"`) and legend with counts
3. `hidden sm:block`: the treemap; `sm:hidden`: `HorizontalBars` with one segmented bar per group (free, subscription, not known), labeled with the group's label and "N free" (CM-11)
4. a batch error, if any, with retry (CM-8.4)
5. `FamilyTable`: unchanged (CM-10). Families come from `useQuery(['families', query])`, groups from `useQuery(['groups', query, groupBy])`; while either loads, the existing `Loading` component shows.

Changing the grouping refetches only the groups (a fast, cached catalog read); statuses and families are unaffected. Both the treemap and the bars are in the DOM and CSS picks one, so tests see both.

### 8. Test data (revised)
- `testdata/catalog-2026-10-09.txt`: the 3,166 image names from the live sitemap, and `testdata/free-2026-10-09.txt`: the 59 free names.
- Go unit tests group the real list for CM-1 to CM-4 and CM-14 to CM-16. The fake service covers the APIs (`node` family: `node`, `node-fips`, `node-lts`; `loki` family: `loki-fips` only). The fake counts token requests for CM-3.3 and CM-16.4.
- `TestCatalogFamiliesFixture` writes, with `UPDATE_FIXTURES=1`, and otherwise checks, the web fixtures generated from the real list: `catalogFamilies.json` (`/api/families`), `catalogFree.json`, and `catalogGroups.json` holding the `/api/groups` responses for the whole catalog and for the search `nginx`, by variant and by prefix. So the web fixtures can't drift from the Go grouping, and no grouping logic is copied into TypeScript test helpers.
- Test names start with their scenario ID (`CM-1.1 …`); Go uses `t.Run("CM-1.1 …")` subtests.

### 9. Variant and prefix grouping in Go (new)
A new `groups.go` holds:
- `variantKind(image) string`: the trailing variant suffixes that `familyName` removes, joined by `-` (`"iamguarded-fips"`; `""` for a base image). It shares one helper with `familyName`, so an image's family and variant group always agree (CM-14).
- `namePrefix(image) string`: the text before the first `-` (CM-15).
- `groupImages(images []string, by string) []ImageGroup`: buckets images by key, names a bucket only if it has at least `max(2, ceil(1% of len(images)))` images, folds the rest into one Other group, and orders groups by image count, then label (CM-16).
- labels: `""` → "Base images", and otherwise each suffix word mapped (`fips` → "FIPS", `iamguarded` → "IAM-guarded", others as written) and joined with spaces; prefix groups are labeled with the prefix; Other is "Other variants" or "Other".

```go
type ImageGroupsInput struct {
    Query   string `json:"query,omitempty"`
    GroupBy string `json:"group_by,omitempty"` // "variant" (default) or "prefix"
}
type ImageGroup struct {
    Label  string   `json:"label"`
    Folded int      `json:"folded,omitempty"` // groups merged into this Other group
    Images []string `json:"images"`
}
type ImageGroupsOutput struct {
    GroupBy string       `json:"group_by"`
    Total   int          `json:"total"`
    Groups  []ImageGroup `json:"groups"`
}
```
`Service.ImageGroups` validates `group_by` (anything else is an `InputError`, so 400 `invalid_input`, CM-16.3), filters the catalog with the shared query helper, then groups the matches. Unlike families, groups are formed after filtering, because the 1% rule is relative to what's shown (CM-15.2: a search for `nginx` names `zabbix` with 2 images). It never calls the registry (CM-16.4). `web.go` adds `GET /api/groups`.

*Why a second endpoint rather than a parameter on `/api/families`:* the responses have different shapes and different filtering rules (families are formed before filtering, groups after). *Why server-side:* the variant suffixes live in Go, and the rule for naming groups is logic both front ends would need if an MCP tool were ever added.

### 10. Why group by variant, and the 1% rule (new)
See the proposal's revised decisions for the product reasons. The rule "at least 1% of the matching images and at least 2" was checked against live data on 2026-10-09: it gives 5 variant groups and 11 prefix groups for the whole catalog, 4 and 5 for `nginx`, and 3 and 12 for `fips`, all small enough to label and large enough to matter. *Rejected:* a fixed minimum of 25 images, which leaves searches with only an Other group.

### 11. Phone bars (new)
Below 640 px, `HorizontalBars` shows one bar per group with three segments (free, subscription, not known), on its shared linear scale from zero. With Base images at 1,730 the smaller groups are short bars, but each prints "N free" and the labels name the groups, which carries the grouping's story without the treemap's 8-pixel units (CM-11).

### 12. Name-prefix blocks (new, CM-18)
`groupImages` orders each group's images block by block: one block per name prefix, blocks by image count (descending), then prefix, images in name order within a block. `ImageGroup` gains `Blocks []ImageBlock`, where `ImageBlock{Prefix string; Count int}` describes consecutive runs of `Images`, so the UI slices `images` by the counts and never derives prefixes itself, and no name is sent twice.

`layoutGroups` takes each group's block counts instead of a single count: it squarifies the groups by their totals as before, takes the header off, squarifies the block counts into the body, insets each block by 1 unit on each side (a 2-unit gap between blocks, capped at a quarter of the block's size like the group inset) when the group has more than one block, and squarifies equal units into each block. `GroupLayout` gains `blocks: Rect[]`, and `units` stays one array in the order of `images`, so the component and tests address units as before. Block gaps are plain background, with no outline or tint. The gaps, 0.75 between units (their stroke), 2 between blocks and 3 between groups, keep the levels distinct.

*Rejected:* gray or soft outlines and tinted block backgrounds, which the user compared in mockups; a block per family, which would be 1 to 6 images; pooling single-image prefixes into one block per group, which the user tried in a build and reversed, because the pooled block was usually its group's largest and grouped unrelated images.

## Risks / Trade-offs

- **[Many tiny blocks]** Base images has 486 single-image blocks and Other 195, each with its own gap. → At 2 units a single-image block reads as a square with a little more space; the large blocks carry the structure.
- **[Free images are scattered]** Units are in block order, then name order, so the 58 free base images are spread through the Base group rather than clustered. → Accepted for a stable layout (CM-6.4): clustering free images first would move units as statuses arrive. They still stand out as the only saturated color.
- **[Other dominates the prefix view]** For the whole catalog, Other holds 2,335 of 3,166 images and all 59 free ones. → Accepted: the prefix view answers "what is the catalog made of", and its header says how many prefixes it folds ("690 prefixes" in the tooltip and phone bar). The variant view is the default.
- **[Prefixes aren't projects]** `node-local-dns` lands in the `node` prefix group. → The option is named "Name prefix", not "Project", so it claims nothing more than it does.
- **[First load hits the token endpoint 3,166 times]** → Same as "Free only" today: 8 at a time, cached 24 hours and shared by every client of the server. Two browsers opening the map at once during the first check would duplicate some requests, because `IsPublic` doesn't coalesce concurrent checks of one image. That already applies to "Free only" and is left out of scope.
- **[Tiny units]** About 14 px per image at 1,000 px. → The tooltip and the table give names. Below 640 px the bars replace the map.
- **[Tooltip width is estimated]** The tooltip box is sized from the text length, not measured, so it can be a little wide or narrow for unusual characters. → Image names are lowercase ASCII; the estimate is tested to cover the longest name in the catalog.
- **[Mouse-only treemap]** → Every link in the map is in the table (CM-12), and the summary tells screen reader users so.
- **[The catalog changes daily]** → Spec figures are dated; tests use the dated testdata; the live check compares shapes, not exact counts.
- **[Suffix list changes]** Adding a suffix to `variantSuffixes` regroups the map and the families. → Intended, since `find_alternative` changes the same way. The fixture comparison test flags it.

## Migration Plan

Additive: two new endpoints and a new view behind a switch; the List view and every existing endpoint and tool are unchanged. Ships in the embedded UI on the next build. Rollback is reverting the change; no data or configuration migration. **No new dependencies are added.**
