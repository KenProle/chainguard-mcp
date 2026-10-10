# Proposal

## Why

The catalog page is a paginated alphabetical list: 3,166 images on 64 pages of 50. It answers "is there an image called X?" but not "what's in the catalog?". Two facts the list hides were checked live on 2026-10-09:

- **Most images are variants.** Grouping each image with its variants (`nginx` with `nginx-fips`, `nginx-iamguarded` and `nginx-iamguarded-fips`) gives 1,751 families. Family sizes are 1 image (521 families), 2 (1,129), 3 (19), 4 (81) and 6 (`go` only: `go`, `go-fips`, `go-geomys-fips`, `go-msft-fips`, `go-openssl`, `go-openssl-fips`).
- **Very little is free.** Only 59 of the 3,166 images (1.9%) can be pulled anonymously. Every one of them is the base image of its family (`nginx` is free, its three variants aren't). 59 families have a free image, and 18 of those are entirely free (single-image families such as `apko` and `static`).

Free images are the only ones this app can inspect in detail, so finding them matters. Today the only way to see them is the "Free only" checkbox, which waits about 30 seconds the first time with nothing on screen.

The original idea (`docs/visualization-ideas.md` #4) sized each family's rectangle by its variant count and colored the family free or subscription. The live data changes two things about it:
- With sizes of only 1 to 6, a family-sized treemap is close to a grid of similar small squares with no room for labels (the first build confirmed it). Grouping by variant type gives a few large, labeled groups instead: base images 1,730, FIPS 1,210, IAM-guarded 117, IAM-guarded FIPS 95, and 14 others.
- A family is often mixed (1 free image of 4), so a single color per family would misreport it. Coloring each image instead keeps that honest.

Users:
- Someone evaluating Chainguard wants to see the catalog's shape at a glance: how many images, how they split into base images and FIPS or IAM-guarded variants, what projects make it up, and how few are free.
- A developer looking for a free image wants to find the free families quickly and open one.

## What Changes

- The catalog page gets a **List / Map** view switch, kept in the URL. The List view is unchanged.
- The Map view shows a **treemap of the catalog grouped by variant** (default) or **by name prefix**, chosen with a "Group by" switch kept in the URL. Each group's rectangle has a header naming it with its image count ("FIPS: 1,210"), and inside it each image is one equal-size unit, so a group's area is its image count. Groups too small to be worth naming fold into "Other".
- Units are colored **free (green), subscription (blue-gray) or not checked yet**, with a text legend. White gaps separate groups and units. Hovering a unit shows its name, status and group at once, and clicking it opens the image.
- **Colors fill in progressively.** The map's shape appears as soon as the catalog loads. Free-tier status arrives in batches of 1,000 images through the existing image list endpoint, and a status line counts progress ("Checked 2,000 of 3,166 images").
- A sortable, paginated **family table** under the map is its text equivalent and the keyboard route: one row per family (an image and its variants, such as `nginx` with `nginx-fips`), linking to its base image, with each image listed as a link with its status in words.
- The search box filters the map and table too ("nginx" → 31 images: Base 13, FIPS 10, IAM-guarded 4, IAM-guarded FIPS 4; 13 families).
- At phone width the treemap is replaced by one stacked bar per group (free, subscription, not checked yet), above the same family table.
- Two new endpoints, backed by new `Service` methods: **`GET /api/groups`** returns the catalog grouped by variant or prefix for the map, and **`GET /api/families`** returns its families for the table. Both use the variant-suffix rules that `find_alternative` uses for related images.

## Capabilities

### New Capabilities
- `catalog-map`: grouping the catalog's images by variant, name prefix and family, and showing the catalog as a treemap and family table, colored by free-tier status, in the web UI.

### Modified Capabilities
None. The catalog's List view and `list_images` behave as before.

## Decisions

Proposed on 2026-10-09 and approved by the user the same day:
- **Color each image, not each group.** Families and groups are often mixed (`nginx`: 1 free of 4), so one color per group would misreport them. *Rejected:* coloring a group by majority, or "any free", which would show `nginx-fips` as free.
- **Free is the saturated color and subscription is muted.** With 98% of images on a subscription, a muted background lets the 59 free images stand out, which is what a user hunting for them needs.
- **Grouping is server-side**, in `Service` methods next to `variantSuffixes`, exposed as `GET /api/groups` and `GET /api/families`. The suffix list lives in Go and `find_alternative` already depends on it. A TypeScript copy would drift.
- **No MCP tool.** `find_alternative` already returns any image's variants with their free status, and `list_images` covers search. The whole grouping is too much to be useful in an assistant's context. The `Service` methods make adding a tool later a few lines.
- **Free status comes from the existing `GET /api/images` endpoint**, in sequential pages of 1,000. That gives progressive colors with no new status-checking code, the same per-image 24-hour cache, and the same load on Chainguard's token endpoint as the "Free only" checkbox. *Rejected:* an endpoint that waits ~30 seconds for every status (a blank page meanwhile), and server-side background checks with polling (new concurrency code for a one-time 30-second wait).
- **No `d3-hierarchy`.** Squarified layout over a few groups, then equal units inside each, is about 80 lines of pure, unit-tested TypeScript. No new dependency.
- **The treemap is pointer-only and labeled as one image** for screen readers, with a summary. 3,166 tab stops would be unusable. The table offers every link the map does, by keyboard.
- **"Free only" is hidden in Map view.** Color already shows free status, and filtering would break the map's purpose as an overview of the whole catalog.

Revised on 2026-10-09 after the user reviewed the first build, which drew one rectangle per family, and approved the same day:
- **Group by variant (default) or by name prefix, not by family.** With 1,751 families of 1 to 6 images there was no room for labels, and the map read as a plain grid. Variant groups give five labeled groups that tell the catalog's main story: every free image but one (`gcc-glibc`) is a base image. The prefix option shows what the catalog is made of (`crossplane` alone is 414 images). Prefix grouping is offered as an option named for what it is, so `node-local-dns` under `node` is expected rather than a claim that they're related. Families stay in the table, where they work well.
- **A group is named only if it has at least 1% of the matching images and at least 2 images; the rest fold into "Other".** *Rejected:* a fixed minimum such as 25 images, which would put every image of a search like `nginx` into "Other".
- **Labeled group headers**, like a classic treemap's category headers, shortened or left out where a group is too small.
- **Free and subscription stay the only colors; groups are told apart by headers and gaps.** *Rejected:* coloring by group, which needs up to 11 hues and would compete with the free/subscription story.
- **Blue-gray for subscription and white gaps between groups and units.** *Rejected:* yellow for subscription and gray family outlines, which the user found didn't work.
- **A tooltip drawn in the map that appears immediately.** *Rejected:* the browser's built-in SVG tooltip, which takes about a second to appear.
- **Units are laid out in name order within a group**, so colors filling in never move them. The cost is that free images are scattered within the Base group rather than clustered.
- **Phone width gets one bar per group, not the treemap.** At 375 px the treemap would give each image about 8 × 8 px, too small to tap or read; per-group bars keep the grouping's story.
- *Rejected:* a separate "FIPS only" filter. FIPS images already form their own group, and the search box finds them.

Revised again on 2026-10-09, after the user reviewed the grouped build, and approved the same day:
- **Name-prefix blocks inside each group.** Images sharing a name prefix sit together in a block, separated by a 2-unit white gap with no header or outline. The user compared gray outlines, soft outlines, tinted blocks and gap widths in mockups and chose plain white space at 2.0 units. Blocks give the treemap varied rectangle sizes (`crossplane` 220 images in Base images, `flux` 30 in Other) and show what the large groups are made of.
- **Every prefix gets its own block, including single images.** At a 2-unit gap a single-image block reads as a square with a little more space around it. *Rejected:* pooling single-image prefixes into one block per group, which the user tried in a build and reversed: the pooled block (486 images in Base images) was usually the largest in its group and read as a meaningless blob.
- **Sizing by image size or another attribute was considered and rejected.** Only the 59 free images can be read anonymously; the registry refuses a token for subscription images, so their size, packages and tags aren't available, and sizing only free images would mix two measures in one map. The image directory's categories ("Application", "Base", "FIPS", …) are too coarse to group by and would mean downloading about 1.5 MB of HTML per image. Block sizes (images per prefix) are the attribute the public data supports.

## Out of Scope

- An MCP tool for groups or families, and any change to `list_images` or `find_alternative`.
- Changes to how free-tier status is checked or cached, and server-side background checking.
- Other per-image data in the map (size, package count, update date). Each would need per-image registry requests for 3,166 images.
- Grouping by project or category beyond the first name part, and nested groups (prefix within variant).
- Version-specific images and tags; the catalog lists image names only.
- Filtering by free status in Map view.

## Impact

- **Go:** family and variant grouping functions next to `variantSuffixes` in `alternatives.go`, `Service.ImageFamilies` and `Service.ImageGroups` with their output types in `service.go`, and `GET /api/families` and `GET /api/groups` routes in `web.go`. No change to MCP tools. Tests in `unit_test.go` and `web_test.go`.
- **Web UI:** view and group switches on `CatalogPage`, a treemap layout module with group and block levels, a treemap component with group headers and its own tooltip, a family table, per-group phone bars (reusing `HorizontalBars` segments), and `api.ts` types for the new endpoints.
- **Load on Chainguard:** unchanged per check. Opening the Map view triggers the same per-image token checks as "Free only" (cached 24 hours, 8 at a time).
- **Dependencies:** none.
