# Proposal

## Why

The catalog Map view shows the whole catalog at once, but its biggest groups are too dense to read. Grouped by variant (catalog captured 2026-10-09), the FIPS group holds 1,210 images in 460 name-prefix blocks, and the Base images group 1,730 in 696 blocks, so on a 1280 px screen each unit is a few pixels wide and the blocks inside a group have no labels: you can see that FIPS is big, but not what it's made of. The group header bands (CM-17) already name each group, so they are the natural handle for "show me just this one". Today they only show a tooltip, and the treemap is a single image with no tab stops (CM-12), so keyboard users can't explore groups at all.

Users:
- Someone evaluating Chainguard wants to see what a group contains: which projects make up FIPS (`crossplane` 194, `prometheus` 25, `kubernetes` 24, …), or which 14 images are "Other variants".
- A developer looking for a free image wants to narrow the map and table to Base images, where 58 of the 59 free images are.

## What Changes

- **Group headers become buttons.** Clicking a group's header zooms the treemap into that group: its name-prefix blocks fill the whole drawing area, and blocks with room get a header of their own ("crossplane: 194").
- **A breadcrumb above the map** ("All groups › FIPS") shows where you are; its "All groups" button zooms back out. Escape in the map does the same.
- **The zoomed group is kept in the URL** as `zoom=<group label>` (e.g. `?view=map&zoom=FIPS`), so a zoomed map can be shared and the browser's Back button zooms out. Changing "Group by" zooms out; changing the search keeps the zoom while the group still exists.
- **The family table and the legend follow the zoom**: zoomed into FIPS, the table lists only the FIPS images (1,210 families of one image each) and the legend counts only them. The status line still reports the free-tier check for the whole search.
- **Keyboard and screen readers:** each group is a focusable button named for the group ("Zoom into FIPS, 1,210 images"); focus moves to "All groups" on zooming in and back to the group's button on zooming out.
- **Phone layout:** the per-group bars become buttons that zoom in too. Zoomed, the phone shows a bar for each of the group's 10 largest name-prefix blocks plus one bar for the rest.
- No Go or API change: the groups response already carries each group's images and blocks.

Out of scope:
- Zooming further, into a single name-prefix block or family.
- Animated transitions between the whole map and a group.
- Zooming by clicking a group's units: units keep opening their image page (CM-9).
- Any change to the List view, the groups or families API, or the MCP tools.

## Capabilities

### New Capabilities
None.

### Modified Capabilities
- `catalog-map`: group headers and phone bars zoom into a group (new), with the zoom in the URL, a breadcrumb back, block headers inside the zoomed group, and the family table and legend following the zoom. CM-6 (treemap of groups), CM-7 (legend), CM-10 (family table), CM-11 (phone layout), CM-12 (keyboard and screen reader access) and CM-17 (group headers) are modified; CM-19 to CM-22 are added.

## Decisions

Proposed on 2026-10-10 for the user's approval:
- **Zoom in place, not a new page.** The zoomed group replaces the whole map in the same drawing area, with a breadcrumb back. *Rejected:* opening a separate route per group (loses the shared search, sort and grouping state), and highlighting the group while dimming the rest (gives the group no more room, which is the point).
- **The zoom lives in the URL as a `zoom` parameter holding the group's label**, like the catalog's other view state. `group` already means the "Group by" choice, so it can't be reused. *Rejected:* keeping the zoom only in component state (can't be shared, and Back would leave the catalog instead of zooming out).
- **Zooming pushes a browser history entry**; search, sort and paging keep replacing it. Drilling in reads as navigation, so Back should undo it.
- **The family table and legend follow the zoom.** The table is the map's text equivalent, so it should list what the map shows. *Rejected:* leaving the table on the whole search (the keyboard route into a group would then be missing).
- **Every group is a keyboard stop, including groups too small for a header**, so no group is reachable by pointer only. *Rejected:* a roving tab stop with arrow keys across groups, which is more code for at most 11 groups.
- **Phone bars drill in too, and the zoomed phone view shows the 10 largest blocks plus "the rest".** *Rejected:* one bar per block (450+ bars for FIPS) and not drilling in on phones (the narrowed table is most useful exactly there).
- **No API change.** The zoomed view uses the group's images and blocks from the groups response already loaded, so zooming makes no request.

## Impact

- Web UI: `web/src/components/CatalogMap.tsx` (breadcrumb, zoom state, table and legend filtering, phone bars), `CatalogTreemap.tsx` (header buttons, zoomed layout with block headers, focus), `web/src/pages/CatalogPage.tsx` (`zoom` URL parameter, history push), `web/src/catalogMap.ts` (filtering families to a group, zoom labels and summaries, phone bars), `web/src/treemap.ts` (a small wrapper so the zoomed layout reuses `layoutGroups`), `web/src/components/HorizontalBars.tsx` (plain, non-toggle buttons), and tests in `web/src/catalogMap.test.ts`, `web/src/treemap.test.ts`, `web/src/components/HorizontalBars.test.tsx` and `web/src/pages/CatalogMap.test.tsx`. README's Map view description and CLAUDE.md's note on URL state.
- Go server, web API and MCP tools: unchanged.
- Dependencies: none added.
