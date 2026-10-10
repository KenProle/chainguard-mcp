# Design

## Context

See `proposal.md` for why, and `specs/catalog-map/spec.md` for the requirements (CM-6, CM-7, CM-10, CM-11, CM-12 and CM-17 modified; CM-19 to CM-22 added).

Current state:
- `GET /api/groups` (`groups.go`) returns, per group, its `label`, `folded` count, `images` ordered block by block, and `blocks` (`{prefix, count}`) describing consecutive runs of `images`. That is everything a zoomed group needs: its units, its block sizes and its block prefixes.
- `CatalogPage.tsx` keeps the catalog's state in the URL (`q`, `view`, `group` for "Group by", `sort`, `page`) through `update()`, which builds from a ref holding the latest params (see CLAUDE.md) and always calls `setParams(next, { replace: true })`.
- `CatalogMap.tsx` loads families, groups and the free-tier batches, and renders the "Group by" switch, status line, legend, the treemap (`hidden sm:block`), the phone bars (`sm:hidden`) and the `FamilyTable`.
- `CatalogTreemap.tsx` is an SVG with `role="img"` and the summary as its name. Pointer handling is delegated from the root: `data-group` / `data-image` attributes identify the hovered unit or header, a click on a unit navigates, and a header only shows a tooltip.
- `layoutGroups` (`treemap.ts`) takes groups as arrays of block sizes, squarifies them, gives a group a header when its inset rectangle is at least two headers tall and `minHeaderWidth` wide, and squarifies the blocks and their units below it.
- `HorizontalBars` draws stacked bars; with `onSelect` each row is a toggle button with `aria-pressed`. `LicenseBreakdown` and `FixesChart` use that toggle mode.
- The page tests (`web/src/pages/CatalogMap.test.tsx`) stub fetch with fixtures captured from the real 2026-10-09 catalog (`web/src/test/fixtures/catalog.ts`, `catalogGroups.json`), so scenarios with real numbers can be asserted exactly.

**No API changes.** The Go server, the web API and the MCP tools are unchanged; zooming works on the groups response already loaded.

**No new dependencies.**

## Goals / Non-Goals

**Goals:**
- Zooming is instant and makes no request (CM-19.1): it only changes which part of the loaded data is drawn.
- One source of truth for the zoom: the URL. Everything else (breadcrumb, layout, legend, table, phone bars) derives from it.
- The zoomed layout reuses `layoutGroups` unchanged.

**Non-Goals:**
- Making units or block headers focusable. Units stay pointer-only; the table is their keyboard route (CM-12).
- Remembering a zoom per grouping or per search.

## Decisions

### 1. The zoom is the `zoom` URL parameter, holding the group's label
`CatalogPage` reads `zoom` like its other params and passes `zoom` and `onZoom(label | '')` to `CatalogMap`. The label is the API's `label` ("FIPS", "Base images", "Other variants", "crossplane"), which is unique within a grouping and is what users see, so `?view=map&zoom=FIPS` is readable. `group` is taken by "Group by", hence the new name.

`update()` gains an option `{ push: true }` that calls `setParams` without `replace`. Zooming in and out pushes, so the browser's Back button undoes a zoom (CM-21.2); every other update still replaces, as today. Zooming also clears `page` (CM-21), and the "Group by" handler clears `zoom` along with setting `group` (CM-21.5), in one update.

*Alternatives:* the variant kind (`fips`, `iamguarded-fips`, `""` for base) as the key would survive label wording changes, but the API doesn't return it and the base group's key would be empty; adding it is an API change for no user-visible gain. Component state instead of the URL can't be shared and wouldn't work with Back. Replacing instead of pushing would make Back leave the catalog from a zoomed map.

### 2. A zoom that names no group is dropped
`CatalogMap` looks the label up in the loaded groups. If it isn't there, it draws the whole map with no breadcrumb. An effect then calls `onZoom('')` with replace (not push, so it doesn't add a history entry), but only when the groups data is the current search's and grouping's (`!groups.isPlaceholderData` and `groups.data.group_by === groupBy`), because `keepPreviousData` briefly shows the old groups while a new grouping loads. This covers a search that removes the group (CM-21.4), a label from another grouping (CM-21.6) and hand-edited URLs. A search that keeps the group keeps the zoom (CM-21.3), because the label still matches.

### 3. Zoomed layout: blocks become the top level
For the zoomed group, a small pure `layoutBlocks(blockCounts, area, opts)` in `treemap.ts` calls `layoutGroups(blockCounts.map((n) => [n]), area, { gap, header, minHeaderWidth })`, treating each block as a one-block group, and `CatalogTreemap` draws its result. That gives exactly CM-19: blocks fill the area in API order (count descending), with the group gap between them and a header band on each block with room, and equal units inside. Units map to images by walking `group.images` block by block, as today. For a group with a single block (every named prefix group, e.g. `crossplane`), `minHeaderWidth: Infinity` turns the block header off (CM-19.2); the breadcrumb already names it.

Block headers use `headerText(prefix, count, width)` and the tooltip `headerTooltip(prefix, count)` ("flux: 30 images"). They are not buttons and have no click (CM-19.4), since deeper zoom is out of scope. Unit tooltips keep the group's label (CM-19.3) by passing the zoomed group's label for every unit.

The layout is memoized on the groups and the zoom, so statuses arriving still never move a unit (CM-6.4).

*Alternative:* scaling the whole-map layout so the group's rectangle fills the view (a geometric zoom). Rejected: the group's units would stay in their whole-map arrangement (a long strip for FIPS), and blocks would still have no room for headers.

### 4. Group headers as SVG buttons
In the whole map, each group renders a `<g role="button" tabIndex={0} aria-label="Zoom into FIPS, 1,210 images" data-zoom-group="FIPS">` containing the header band (if any) and a focus outline: a `rect` over the group's inset rectangle with `fill-none` and a stroke in the focus color, hidden unless the button has `:focus-visible` (Tailwind's `group` / `group-focus-visible:` classes, so no inline styles; CM-13). The outline has `pointer-events="none"`, so clicks on units underneath still reach them. Enter and Space on the button, and a click on its header band, call `onZoom(label)`. A group too small for a header still gets the button, with only the outline, so it is a keyboard stop (CM-12); pointer users have no handle for it (see Risks), which is acceptable for a group too small to label. Header bands get `cursor-pointer`, which units already have (CM-17.4).

Units go in a separate `<g aria-hidden="true">`, so the buttons are the only content assistive technology sees inside the map. The SVG's role becomes `group` (named by the summary) in the whole map, because a `role="img"` hides its children, including the buttons; zoomed, it stays `role="img"`, since nothing in it is interactive. The accessible name text comes from `treemapSummary` (unchanged wording, CM-12.1) and a new `zoomedSummary(label, images, blocks, free)`.

Tab order is DOM order, which is the API's group order (CM-12.2). At most 11 groups exist (prefix view), so plain tab stops are fine.

*Alternatives:* HTML buttons positioned over the SVG would need inline `style` for coordinates, which the CSP forbids. A roving tab index with arrow keys gives one tab stop, but is more code and less discoverable for at most 11 items. A separate list of "Zoom into" buttons outside the map duplicates the headers and leaves pointer and keyboard users with different controls.

### 5. Breadcrumb, Escape and focus
`CatalogMap` renders, when zoomed, `<nav aria-label="Map zoom">` with an "All groups" button, a `›` separator (`aria-hidden`) and the group label with `aria-current="location"`. The button's visible text is "All groups"; its `aria-label` is "All groups, zoomed into FIPS" (CM-22.1), which keeps the visible text at the start of the name.

Escape is handled by an `onKeyDown` on the wrapper around the breadcrumb and the map card (CM-20.3).

Focus follows CM-22 with a one-shot ref in `CatalogMap`, `pendingFocus: 'back' | { group: string } | null`, set only by the user actions that zoom (header click or key, bar tap, "All groups", Escape) right before calling `onZoom`. After the next render, an effect focuses the target and clears the ref: the "All groups" button, or the element with `data-zoom-group="<label>"` in the treemap or in the phone bars. Both are in the DOM (one is `display: none`), so the choice can't rely on layout, which jsdom doesn't have: zooming in records whether it came from the treemap or a bar, and zooming out returns focus to the same kind. When the zoom came from the URL, zooming out picks by `window.matchMedia('(min-width: 640px)')`, the same breakpoint as the `sm:` classes. A zoom that arrives from the URL never sets the ref, so it moves no focus.

### 6. The table and legend follow the zoom
A pure function in `catalogMap.ts`, `familiesInGroup(families, images)`, keeps each family's images that are in the zoomed group (a `Set` of `group.images`) and drops families left empty, preserving order. `CatalogMap` passes its result to `familyRows`, so sorting, paging, the free count and "—" behave exactly as before (CM-10.5, CM-10.6). The legend uses `statusCounts([zoomedGroup], statuses)` instead of the families' counts (CM-7.4). The status line keeps counting the whole search, since that's what the free-tier check covers (CM-19.5).

This is filtering server-provided lists by membership in another server-provided list, not grouping: the variant and prefix rules stay in Go, as `openspec/config.yaml` requires. *Alternative:* a `zoom` parameter on `GET /api/families` would put this in a `Service` method, but it means a request per zoom, a loading state where none is needed, and the server re-deriving groups it already returned.

### 7. Phone bars
`HorizontalBars` gets an optional `pressable` flag defaulting to true; when false, its buttons omit `aria-pressed` and the selection ring, so they read as plain buttons. Each bar's button also carries `data-zoom-group` for focus (decision 5). The existing toggle callers are unchanged.

Not zoomed, `CatalogMap` passes `onSelect={(label) => zoom(label)}`, `pressable={false}` and `accessibleLabel: "Zoom into FIPS, 1,210 images, 0 free"`. The bar label stays `groupLabel(g)`, so `onSelect` receives the display label; `CatalogMap` maps it back to the API label (they differ only for a folded Other group, e.g. "Other variants (7 kinds)").

Zoomed, a pure `blockBars(group, statuses, 10)` in `catalogMap.ts` returns the 10 largest blocks (the API's order) and, if there are more, one "N more prefixes" bar summing the rest, each with free / subscription / not-known segments; these are rendered without `onSelect`, so they stay decorative and `aria-hidden`, with the table as their text equivalent (CM-11.3). Ten keeps the zoomed phone view about as tall as the five-to-eleven group bars it replaces.

### 8. No progress indicator for zooming
Zooming makes no request, so there is no wait to report. The existing status line keeps reporting the free-tier check's progress while zoomed (CM-19.5), which satisfies the rule that waits over about 3 seconds show progress.

## Risks / Trade-offs

- [A label-keyed URL breaks if a group's wording changes in Go, e.g. "Base images" renamed] → the old link falls back to the whole map (decision 2) rather than erroring; renames are rare and visible in review.
- [Rendering all 1,730 Base-images units in jsdom is slow, as the existing tests already note] → zoom tests mostly use the `nginx` search fixture (31 images), and whole-catalog zoom assertions reuse the existing longer timeout.
- [Focus outlines on SVG `<g>` elements depend on the browser's `:focus-visible` support for SVG] → the outline is a child `rect` styled through the parent's state, not the element's own `outline`; the browser check (task 6) confirms it in Chrome, Firefox and Edge on the user's machine.
- [Search updates replace history while zoom pushes it, so Back after zoom-then-search returns to the unzoomed map with the earlier search] → consistent with how search already behaves (it never adds entries); acceptable.
- [Groups too small for a header can be zoomed by keyboard but not by pointer] → such groups are a few dozen square units at most; pointer users can still read every image in the table. Revisit if users ask.

## Migration Plan

None: a client-only change. Existing URLs without `zoom` behave exactly as before. Rollback is reverting the commit.
