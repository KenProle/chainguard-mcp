# catalog-map Specification

## Purpose

Shows the whole Chainguard catalog at a glance: images grouped by variant type or name prefix and drawn as a labeled treemap colored by free-tier status, with a table of image families (an image and its variants) as its text equivalent and keyboard route.

CM-1 to CM-4 define the families and the API that returns them; CM-5 to CM-9 the Map view and its treemap; CM-10 to CM-13 the text equivalent, phone layout, accessibility and security policy; CM-14 to CM-18 the treemap's groups, their API, headers and name-prefix blocks; and CM-19 to CM-22 zooming into a group, its breadcrumb, its URL parameter and focus. Live figures were checked on 2026-10-09 (3,166 images, 59 free).

## Requirements

### Requirement: CM-1 Families of an image and its variants
Every catalog image SHALL belong to exactly one family. The family's name SHALL be the image name with trailing variant suffixes removed (the suffixes `find_alternative` treats as variants, such as `fips`, `iamguarded`, `crac`, `openssl`, `lts`, `msft`, `geomys`, `slim`, `glibc` and `musl`), never removing the first name part. Images whose remaining name differs SHALL be in different families.

#### Scenario: CM-1.1 nginx and its variants
- **WHEN** the catalog contains `nginx`, `nginx-fips`, `nginx-iamguarded` and `nginx-iamguarded-fips`
- **THEN** the `nginx` family contains exactly those four images

#### Scenario: CM-1.2 Shared prefixes stay separate
- **WHEN** the catalog contains `node`, `node-fips`, `node-local-dns`, `node-problem-detector` and `node-problem-detector-fips`
- **THEN** the `node` family contains only `node` and `node-fips`
- **AND** `node-local-dns` is a family of one, and `node-problem-detector` a family of two

#### Scenario: CM-1.3 Several suffixes
- **WHEN** the catalog contains `go`, `go-fips`, `go-geomys-fips`, `go-msft-fips`, `go-openssl` and `go-openssl-fips`
- **THEN** all six are in the `go` family

#### Scenario: CM-1.4 Same variants as find_alternative
- **WHEN** `find_alternative` is called for `nginx`
- **THEN** its recommended image and related images are exactly the `nginx` family's images

### Requirement: CM-2 Family without a base image
A family whose name is not itself a catalog image SHALL keep that name, and SHALL link to its first image in name order.

#### Scenario: CM-2.1 os-shell
- **WHEN** the catalog contains `os-shell-iamguarded` and `os-shell-iamguarded-fips` but no `os-shell`
- **THEN** they form the family `os-shell`, which links to `os-shell-iamguarded`

#### Scenario: CM-2.2 gcc-glibc
- **WHEN** the catalog contains `gcc-glibc` but no `gcc`
- **THEN** `gcc-glibc` is the only image in the family `gcc`, which links to `gcc-glibc`

### Requirement: CM-3 Families API
The web API SHALL return the catalog's families at `GET /api/families`, with the number of images and, per family, its name and its images in name order. Families SHALL be ordered by image count (descending), then name. The response SHALL NOT wait for free-tier checks.

#### Scenario: CM-3.1 Whole catalog
- **WHEN** `GET /api/families` is requested and the catalog has 3,166 images (as on 2026-10-09)
- **THEN** the response has 3,166 images in 1,751 families
- **AND** the first family is `go` (6 images), and families of 4 such as `argocd` and `nginx` come before families of 3

#### Scenario: CM-3.2 Images in name order
- **WHEN** the `nginx` family is returned
- **THEN** its images are `nginx`, `nginx-fips`, `nginx-iamguarded`, `nginx-iamguarded-fips`, in that order

#### Scenario: CM-3.3 No free-tier checks
- **WHEN** `GET /api/families` is requested and no image's free-tier status is cached
- **THEN** the response is returned without any request to the registry's token endpoint

#### Scenario: CM-3.4 Catalog unavailable
- **WHEN** the catalog can't be fetched and nothing is cached
- **THEN** the endpoint responds with status 502, as the image list endpoint does

### Requirement: CM-4 Search filter
`GET /api/families` SHALL accept the same `query` as the image list, keep only matching images, and leave out families with no matching image. Families SHALL be formed from the whole catalog before filtering, so a query never changes which family an image is in.

#### Scenario: CM-4.1 nginx
- **WHEN** `GET /api/families?query=nginx` is requested (as on 2026-10-09)
- **THEN** the response has 31 images in 13 families, the same 31 images the image list returns for `nginx`
- **AND** `nginx-prometheus-exporter` and `ingress-nginx` are families of their own, not part of `nginx`

#### Scenario: CM-4.2 Partial family
- **WHEN** `GET /api/families?query=fips` is requested
- **THEN** the `nginx` family contains only `nginx-fips` and `nginx-iamguarded-fips`

### Requirement: CM-5 Map view on the catalog page
The catalog page SHALL offer a List view and a Map view, with List as the default, and in Map view a "Group by" choice of Variant (default) or Name prefix. The view and grouping SHALL be kept in the URL. The search box SHALL apply to both views. The "Free only" checkbox and the list's pagination SHALL be hidden in Map view.

#### Scenario: CM-5.1 Switching to the map
- **WHEN** the user chooses Map on the catalog page
- **THEN** the URL records the Map view, and the page shows the map grouped by variant and the family table instead of the image list

#### Scenario: CM-5.2 Shared link
- **WHEN** a URL with the Map view, grouping by name prefix and the search `nginx` is opened
- **THEN** the Map view opens with `nginx` in the search box, the Name prefix grouping selected, and 13 families in the table

#### Scenario: CM-5.3 List view unchanged
- **WHEN** the catalog page is opened without a view in the URL
- **THEN** it shows the List view exactly as before, with "Free only" and pagination

#### Scenario: CM-5.4 Switching the grouping
- **WHEN** the user chooses Name prefix in Map view
- **THEN** the URL records the grouping and the treemap redraws with the prefix groups, while the family table stays the same

### Requirement: CM-6 Treemap of groups
On wide screens the Map view SHALL show a treemap with one rectangle per group (CM-14, CM-15), in the order the groups API returns, filling the drawing area, with gaps in the background color between groups. Each image SHALL get one unit of equal area inside its group's rectangle, below the group's header if it has one, laid out block by block (CM-18), so a group's area is proportional to its image count. While zoomed into a group, the treemap SHALL show that group alone (CM-19).

#### Scenario: CM-6.1 Area by image count
- **WHEN** the map shows the whole catalog grouped by variant (as on 2026-10-09)
- **THEN** the Base images rectangle has 1,730/3,166 of the drawing area and the FIPS rectangle 1,210/3,166
- **AND** the FIPS rectangle holds 1,210 equal units, one per image

#### Scenario: CM-6.2 Units tile their group
- **WHEN** any group's rectangle is drawn
- **THEN** its units don't overlap, stay inside it below its header, and together cover that space

#### Scenario: CM-6.3 Groups tile the map
- **WHEN** the treemap is drawn
- **THEN** group rectangles don't overlap, stay inside the drawing area, and together cover it

#### Scenario: CM-6.4 Stable layout
- **WHEN** free-tier statuses arrive
- **THEN** only unit colors change; no unit moves

### Requirement: CM-7 Free-tier colors and legend
Each unit SHALL be colored by its image's status: free, subscription, or not known. Free SHALL be the only saturated color and subscription a muted blue-gray, and groups SHALL NOT get colors of their own. A legend SHALL name each color in text, with the number of images in each status among the images the map shows: the whole search, or the zoomed group's images while zoomed (CM-19).

#### Scenario: CM-7.1 nginx family
- **WHEN** the statuses of the `nginx` family's images are known (as on 2026-10-09)
- **THEN** the `nginx` unit has the free color and the `nginx-fips`, `nginx-iamguarded` and `nginx-iamguarded-fips` units the subscription color

#### Scenario: CM-7.2 Legend counts
- **WHEN** every status in the catalog is known (as on 2026-10-09)
- **THEN** the legend reads "Free: 59", "Subscription: 3,107" and does not show a not-known count of zero

#### Scenario: CM-7.3 A failed check
- **WHEN** an image's free-tier check fails, so the image list returns it without a status
- **THEN** its unit keeps the not-known color and it is counted under "Not known"

#### Scenario: CM-7.4 Legend of a zoomed group
- **WHEN** the map is zoomed into Base images, grouped by variant, with every status in the catalog known (as on 2026-10-09)
- **THEN** the legend reads "Free: 58" and "Subscription: 1,672"
- **AND** the status line above it still reads "3,166 images in 1,751 families · 59 free"

### Requirement: CM-8 Colors fill in progressively
The Map view SHALL draw the treemap as soon as the families are loaded, before any free-tier status is known, then request statuses in batches of at most 1,000 images, one batch at a time, coloring each batch's units as it arrives. A status line SHALL report progress and SHALL be announced politely to screen readers.

#### Scenario: CM-8.1 First visit
- **WHEN** the Map view is opened for the whole catalog and no status is cached
- **THEN** the treemap appears with every unit in the not-known color
- **AND** the status line says "Checking free-tier status: 0 of 3,166 images" and that the first check can take about 30 seconds

#### Scenario: CM-8.2 Batches
- **WHEN** statuses are loaded for 3,166 images
- **THEN** four image list requests are made with a limit of 1,000 and offsets 0, 1,000, 2,000 and 3,000, each starting after the previous one finished
- **AND** after the first returns, the status line says "Checked 1,000 of 3,166 images" and those 1,000 units are colored

#### Scenario: CM-8.3 Done
- **WHEN** every batch has returned
- **THEN** the status line says "3,166 images in 1,751 families · 59 free"

#### Scenario: CM-8.4 A batch fails
- **WHEN** a batch request fails
- **THEN** the treemap and the statuses already loaded stay on screen, an error with a retry button is shown, and retrying requests only the batches not yet loaded

#### Scenario: CM-8.5 Search changes
- **WHEN** the search changes while batches are loading
- **THEN** the remaining batches for the old search are not requested, and statuses load for the new search's images

### Requirement: CM-9 Pointer interaction
Hovering a unit SHALL show, without a delay, a tooltip inside the map with the image's name, status and group. Clicking a unit SHALL open that image's page.

#### Scenario: CM-9.1 Hover
- **WHEN** the pointer moves onto the `nginx-fips` unit after statuses load, grouped by variant
- **THEN** a tooltip reading "nginx-fips · subscription · FIPS" appears at once, and disappears when the pointer leaves the map

#### Scenario: CM-9.2 Click
- **WHEN** the user clicks the `nginx` unit
- **THEN** the image page for `nginx` opens

### Requirement: CM-10 Family table
The Map view SHALL show a family table with each family's name, image count, free image count and images, paginated 50 families per page. It SHALL be sortable by image count (default, the treemap's order), free image count (then image count, then name) or name. The family name SHALL link as in CM-2, and each image SHALL be a link with its status as text. While zoomed (CM-19), the table SHALL list only the zoomed group's images, leaving out families with none of them.

#### Scenario: CM-10.1 nginx row
- **WHEN** the table shows the `nginx` family after statuses load
- **THEN** its row reads `nginx`, 4 images, 1 free, and lists `nginx` (free), `nginx-fips`, `nginx-iamguarded` and `nginx-iamguarded-fips` (subscription), each linking to its image page

#### Scenario: CM-10.2 Sort by free images
- **WHEN** the user sorts by free images with the whole catalog loaded
- **THEN** the 59 families with a free image come first, starting with `go` (6 images, 1 free) and then the four-image families among them, beginning with `haproxy` and `jdk`

#### Scenario: CM-10.3 Pages
- **WHEN** the whole catalog is shown
- **THEN** the table has 36 pages of 50 families, and the page and sort are kept in the URL

#### Scenario: CM-10.4 Status not known yet
- **WHEN** an image's status isn't known yet
- **THEN** its row shows the image without a status and the free count as "—" until the family's statuses are all loaded

#### Scenario: CM-10.5 Zoomed into Other variants
- **WHEN** the whole catalog is grouped by variant and the map is zoomed into Other variants (as on 2026-10-09)
- **THEN** the table has 8 families on one page, starting with `go`, whose row reads 4 images and lists only `go-geomys-fips`, `go-msft-fips`, `go-openssl` and `go-openssl-fips`
- **AND** the `gcc` row lists `gcc-glibc` (free) with 1 free

#### Scenario: CM-10.6 Zoomed into FIPS
- **WHEN** the whole catalog is grouped by variant and the map is zoomed into FIPS (as on 2026-10-09)
- **THEN** the table has 1,210 families of one image each on 25 pages, and the `nginx` row lists only `nginx-fips`

### Requirement: CM-11 Phone layout
Below 640 px wide the Map view SHALL replace the treemap with one stacked bar per group, in the same order, each split into free, subscription and not-known image counts on a shared scale, with the same legend, status line and family table. Each group's bar SHALL be a button that zooms into the group (CM-19). While zoomed, the bars SHALL be the group's 10 largest name-prefix blocks, then one bar for the rest if there are more. Each bar SHALL show its counts as text.

#### Scenario: CM-11.1 375 px wide
- **WHEN** the Map view is shown 375 px wide, grouped by variant, with every status known
- **THEN** no treemap is visible, and five bars show Base images ("58 of 1,730 free"), FIPS ("0 of 1,210 free"), IAM-guarded (0 of 117), IAM-guarded FIPS (0 of 95) and Other variants (1 of 14)
- **AND** the family table is usable without horizontal page scrolling

#### Scenario: CM-11.2 Tapping a bar
- **WHEN** the user taps the FIPS bar at 375 px wide
- **THEN** the map zooms into FIPS, with the breadcrumb (CM-20) above the bars and the family table narrowed as in CM-10.6

#### Scenario: CM-11.3 Bars of a zoomed group
- **WHEN** the map is zoomed into FIPS at 375 px wide, grouped by variant (as on 2026-10-09)
- **THEN** eleven bars show `crossplane` 194, `prometheus` 25, `kubernetes` 24, `kubeflow` 22, `knative` 21, `aws` 19, `gitlab` 18, `cert` 14, `kube` 14, `calico` 13, and "450 more prefixes" 846
- **AND** these bars are not buttons
- **AND** each bar shows its counts as text, such as "0 of 194 free", and a screen reader can read each bar's name and counts, such as "crossplane: 194 images, 0 free"

### Requirement: CM-12 Keyboard and screen reader access
The treemap's units SHALL be hidden from assistive technology and SHALL NOT add tab stops; the treemap SHALL have a text summary. Each group of the whole map SHALL be one tab stop, a button named for the group and its image count, even when it has no header. The view and grouping switches, breadcrumb, sort controls, pagination and every family and image link in the table SHALL be reachable and operable by keyboard.

#### Scenario: CM-12.1 Accessible summary
- **WHEN** a screen reader reaches the treemap grouped by variant with every status known
- **THEN** it reads "Treemap of 3,166 images in 5 variant groups: 59 free. The family table below lists every family."
- **AND** zoomed into FIPS it reads "Treemap of the FIPS group: 1,210 images in 460 name-prefix blocks: 0 free. The family table below lists its families."

#### Scenario: CM-12.2 Keyboard route
- **WHEN** a keyboard user tabs through the Map view grouped by variant
- **THEN** focus moves from the switches and search to five group buttons in the treemap's order, named "Zoom into Base images, 1,730 images" through "Zoom into Other variants (7 kinds), 14 images", then to the sort controls and table links, never stopping on a unit
- **AND** Enter on the `nginx` link opens its image page

#### Scenario: CM-12.3 Visible focus
- **WHEN** a group button has keyboard focus, in light or dark mode
- **THEN** an outline in the focus color is drawn around that group's whole rectangle

### Requirement: CM-13 Theme and security policy
The Map view SHALL be legible in light and dark mode and SHALL work under the app's Content Security Policy, with no inline styles or injected style elements.

#### Scenario: CM-13.1 Dark mode at phone and desktop width
- **WHEN** the Map view is shown in dark mode at 375 px and at 1280 px wide
- **THEN** the free, subscription and not-known colors and the legend text are distinguishable from the background and each other
- **AND** the browser console shows no Content Security Policy violations

### Requirement: CM-14 Variant groups
In the variant grouping, an image's group SHALL be the variant suffixes that CM-1 removes to find its family name, in order (none for a base image). Groups SHALL be labeled "Base images", "FIPS", "IAM-guarded", "IAM-guarded FIPS", or by their suffixes otherwise, and groups too small to name (CM-16) SHALL fold into "Other variants".

#### Scenario: CM-14.1 Whole catalog
- **WHEN** the catalog (3,166 images on 2026-10-09) is grouped by variant
- **THEN** the groups are Base images 1,730 (58 free), FIPS 1,210, IAM-guarded 117, IAM-guarded FIPS 95, and Other variants 14 (7 suffix combinations such as `openssl-fips` and `crac`, with `gcc-glibc` its only free image)

#### Scenario: CM-14.2 Image suffixes
- **WHEN** `nginx`, `nginx-fips`, `nginx-iamguarded-fips` and `node-local-dns` are grouped by variant
- **THEN** `nginx` and `node-local-dns` are in Base images, `nginx-fips` in FIPS and `nginx-iamguarded-fips` in IAM-guarded FIPS

#### Scenario: CM-14.3 nginx search
- **WHEN** the images matching `nginx` (31) are grouped by variant
- **THEN** the groups are Base images 13, FIPS 10, IAM-guarded 4 and IAM-guarded FIPS 4

### Requirement: CM-15 Name-prefix groups
In the name-prefix grouping, an image's group SHALL be the part of its name before the first hyphen (the whole name if it has none), labeled with that prefix. Groups too small to name (CM-16) SHALL fold into "Other".

#### Scenario: CM-15.1 Whole catalog
- **WHEN** the catalog (3,166 images on 2026-10-09) is grouped by name prefix
- **THEN** the groups are Other 2,335 (690 prefixes, including all 59 free images), `crossplane` 414, `prometheus` 72, `kubeflow` 61, `kubernetes` 51, `knative` 45, `aws` 43, `cert` 37, `harbor` 37, `gitlab` 36 and `kube` 35

#### Scenario: CM-15.2 nginx search
- **WHEN** the images matching `nginx` (31) are grouped by name prefix
- **THEN** the groups are `nginx` 14, `ingress` 9, `commercial` 4, `privatebin` 2 and `zabbix` 2, with no Other group

### Requirement: CM-16 Groups API
The web API SHALL return groups at `GET /api/groups`, with `group_by` set to `variant` (default) or `prefix` and the image list's `query`. A group SHALL be named only if it has at least 1% of the matching images (rounded up) and at least 2 images; the rest SHALL form one Other group. Groups SHALL be ordered by image count (descending), then label, each with its name-prefix blocks (CM-18) and its images ordered block by block. The response SHALL NOT wait for free-tier checks.

#### Scenario: CM-16.1 Default grouping
- **WHEN** `GET /api/groups` is requested for the whole catalog
- **THEN** the response is grouped by variant, with 3,166 images in 5 groups, Base images first

#### Scenario: CM-16.2 Other group
- **WHEN** `GET /api/groups?group_by=prefix` is requested for the whole catalog
- **THEN** Other comes first with 2,335 images and reports that it holds 690 prefixes, followed by `crossplane`

#### Scenario: CM-16.3 Invalid grouping
- **WHEN** `GET /api/groups?group_by=size` is requested
- **THEN** the endpoint responds with status 400 and the code `invalid_input`

#### Scenario: CM-16.4 No free-tier checks
- **WHEN** `GET /api/groups` is requested and no image's free-tier status is cached
- **THEN** the response is returned without any request to the registry's token endpoint

### Requirement: CM-17 Group headers
Each group rectangle with room for it SHALL have a header band naming the group and its image count. When the full text doesn't fit, the header SHALL show the name alone, or the name shortened with an ellipsis, and SHALL never be empty. Hovering a header SHALL show the group's full name and image count and that a click zooms in, and clicking it SHALL zoom into the group (CM-19). A group too small for a header SHALL have none; its units then fill the rectangle.

#### Scenario: CM-17.1 Large groups
- **WHEN** the whole catalog is shown grouped by variant on a wide screen
- **THEN** the Base images and FIPS groups have headers reading "Base images: 1,730" and "FIPS: 1,210"
- **AND** grouped by name prefix, every group has a header, including the smaller `cert`, `harbor`, `gitlab` and `kube` groups, with `cert`'s reading "cert: 37"

#### Scenario: CM-17.2 Small group
- **WHEN** a group's rectangle is too short or too narrow for a header
- **THEN** it has no header, and hovering its units still names the group

#### Scenario: CM-17.3 Name too long for its header
- **WHEN** the whole catalog is shown grouped by name prefix on a wide screen (as on 2026-10-09) and the `kubernetes` group's header is too narrow for its name
- **THEN** the header shows the start of the name followed by "…", never an empty band
- **AND** hovering the header shows "kubernetes: 51 images · click to zoom in"

#### Scenario: CM-17.4 Pointer over a header
- **WHEN** the pointer is over a group's header
- **THEN** the pointer shows that the header is clickable, and clicking it zooms into that group

### Requirement: CM-18 Name-prefix blocks within groups
Within each group, images sharing a name prefix (CM-15) SHALL be laid out together as a block, one block per prefix, including prefixes with a single image. Blocks SHALL be ordered by image count (descending), then prefix, with images in name order within a block. Blocks SHALL be separated by white space wider than the gap between units and narrower than the gap between groups, with no headers or outlines.

#### Scenario: CM-18.1 Variant groups
- **WHEN** the whole catalog is grouped by variant (as on 2026-10-09)
- **THEN** Base images is divided into 696 blocks, starting with `crossplane` 220, `kubeflow` 39 and `kubernetes` 25, of which 486 hold a single image
- **AND** FIPS is divided into 460 blocks, starting with `crossplane` 194

#### Scenario: CM-18.2 Other in the prefix view
- **WHEN** the whole catalog is grouped by name prefix
- **THEN** the Other group is divided into 690 blocks, starting with `flux` 30, `sigstore` 30 and `kserve` 28
- **AND** the `crossplane` group is one block with no inner gaps

#### Scenario: CM-18.3 nginx search
- **WHEN** the images matching `nginx` are grouped by variant
- **THEN** Base images has the blocks `nginx` 5, `commercial` 3, `ingress` 3, `privatebin` 1 and `zabbix` 1

#### Scenario: CM-18.4 Area by block size
- **WHEN** a group is divided into blocks
- **THEN** each block's area, including its share of the gaps, is proportional to its image count, and its units are equal and stay inside it
- **AND** hovering a unit still names its group

### Requirement: CM-19 Zoom into a group
Activating a group's header or group button SHALL zoom the map into that group without a new request: its name-prefix blocks SHALL fill the whole drawing area in CM-18's order, with units of equal area and gaps between blocks as between groups. If the group has more than one block, each block with room SHALL get a header naming its prefix and image count, following CM-17's shortening rules. Units SHALL keep their tooltip and click (CM-9).

#### Scenario: CM-19.1 Zooming into FIPS
- **WHEN** the user clicks the FIPS header with the whole catalog grouped by variant (as on 2026-10-09)
- **THEN** the treemap shows only FIPS's 1,210 units in 460 blocks filling the drawing area, the largest being `crossplane` with 194 units under a header reading "crossplane: 194"
- **AND** no request is made to the server

#### Scenario: CM-19.2 A group of one block
- **WHEN** the user zooms into the `crossplane` group grouped by name prefix
- **THEN** its 414 units fill the drawing area as one block with no block header

#### Scenario: CM-19.3 Units in a zoomed group
- **WHEN** the pointer moves onto the `nginx-fips` unit while zoomed into FIPS, and then the user clicks it
- **THEN** the tooltip reads "nginx-fips · subscription · FIPS", and the click opens the image page for `nginx-fips`

#### Scenario: CM-19.4 Block header tooltip
- **WHEN** the pointer is over a block header while zoomed into Other, grouped by name prefix
- **THEN** a tooltip names the prefix and its image count, such as "flux: 30 images", and clicking the header does nothing

#### Scenario: CM-19.5 Statuses still loading
- **WHEN** the user zooms into a group before every free-tier batch has returned
- **THEN** the batches keep loading, the status line keeps counting the whole search, and the zoomed units are colored as their batches arrive

### Requirement: CM-20 Breadcrumb and zooming out
While zoomed, the Map view SHALL show a breadcrumb above the treemap or bars reading "All groups › <group label>", where "All groups" is a button that zooms out to the whole map. Escape pressed while focus is in the breadcrumb, treemap or bars SHALL also zoom out. The breadcrumb SHALL NOT be shown when not zoomed.

#### Scenario: CM-20.1 Breadcrumb
- **WHEN** the map is zoomed into IAM-guarded FIPS
- **THEN** a breadcrumb reads "All groups › IAM-guarded FIPS"

#### Scenario: CM-20.2 All groups
- **WHEN** the user activates "All groups" while zoomed into FIPS
- **THEN** the treemap shows every group again, the breadcrumb disappears, and the table lists every family

#### Scenario: CM-20.3 Escape
- **WHEN** the user presses Escape while the "All groups" button has focus
- **THEN** the map zooms out as in CM-20.2

### Requirement: CM-21 Zoom in the URL
The zoomed group SHALL be kept in the URL as the `zoom` parameter holding its label, together with the search and grouping. Zooming in or out SHALL add a browser history entry and reset the table to its first page. Changing the grouping SHALL zoom out. When the groups for the current search and grouping have loaded and none has the `zoom` label, the map SHALL show every group and drop `zoom` from the URL.

#### Scenario: CM-21.1 Shared link
- **WHEN** the URL `?view=map&q=nginx&zoom=FIPS` is opened (as on 2026-10-09)
- **THEN** the map opens zoomed into FIPS with 10 units in the blocks `nginx` 5, `ingress` 2, `commercial` 1, `privatebin` 1 and `zabbix` 1, and the table lists 10 families

#### Scenario: CM-21.2 Browser Back
- **WHEN** the user zooms into FIPS from the whole map and then presses the browser's Back button
- **THEN** the map shows every group again and `zoom` is gone from the URL

#### Scenario: CM-21.3 Search keeps an existing group
- **WHEN** the map is zoomed into FIPS for the whole catalog and the user searches for `nginx`
- **THEN** the map stays zoomed into FIPS, now with 10 images

#### Scenario: CM-21.4 Search without the group
- **WHEN** the map is zoomed into Other variants for the whole catalog and the user searches for `nginx`, whose images form only Base images, FIPS, IAM-guarded and IAM-guarded FIPS groups
- **THEN** the map shows those four groups, no breadcrumb, and the URL has no `zoom`

#### Scenario: CM-21.5 Changing the grouping
- **WHEN** the map is zoomed into FIPS and the user chooses Name prefix
- **THEN** the map shows every name-prefix group and the URL has no `zoom`

#### Scenario: CM-21.6 Label not in the grouping
- **WHEN** the URL `?view=map&group=prefix&zoom=FIPS` is opened
- **THEN** the map shows every name-prefix group and the URL has no `zoom`

### Requirement: CM-22 Focus on zooming
Zooming in from a group's header, group button or bar SHALL move keyboard focus to the breadcrumb's "All groups" button, whose accessible name SHALL include the zoomed group's label. Zooming out with "All groups" or Escape SHALL move focus to the button of the group just left. A zoom that comes from the URL (a shared link or the browser's Back button) SHALL NOT move focus.

#### Scenario: CM-22.1 Zooming in by keyboard
- **WHEN** a keyboard user presses Enter on the FIPS group button
- **THEN** focus is on the "All groups" button, which a screen reader announces as "All groups, zoomed into FIPS"

#### Scenario: CM-22.2 Zooming out by keyboard
- **WHEN** the user then presses Enter on "All groups"
- **THEN** focus is on the FIPS group button in the whole map

#### Scenario: CM-22.3 On a phone
- **WHEN** the user taps the FIPS bar at 375 px wide and then "All groups"
- **THEN** focus goes to "All groups" and then back to the FIPS bar
