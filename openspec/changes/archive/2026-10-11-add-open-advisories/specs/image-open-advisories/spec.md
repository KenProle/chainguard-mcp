# Spec Delta

## Purpose

Shows the known open advisories that Chainguard has published for a Chainguard image's installed packages, in the Security tab and through MCP. Advisories are found through OSV.dev, and each one's current status is read from Chainguard's own advisory feed, because OSV.dev can serve older copies. It counts each vulnerability once, says what state each one is in, and never implies that an image without advisories is free of vulnerabilities.

OA-1 to OA-5 define what is looked up and how it is counted; OA-16 and OA-17 which advisories are open and where their status comes from; OA-6 to OA-8 what is shown and how it is worded; OA-9 and OA-10 the MCP tool and the web API; OA-11 to OA-13 freshness, failures and loading; OA-14 layout; OA-15 accuracy. Figures were measured on 2026-10-11 for `python` `latest` (29 packages) and `latest-dev` (76 packages), amd64, with statuses read from the advisory feed.

## ADDED Requirements

### Requirement: OA-1 Packages looked up
For an image and tag, advisories SHALL be looked up on OSV.dev for each installed linux/amd64 package by its own name and version, in the ecosystem its package URL gives (`Wolfi` or `Chainguard`). Packages from any other source SHALL be listed as not covered, not looked up.

#### Scenario: OA-1.1 python latest
- **WHEN** open advisories are requested for `python` `latest`
- **THEN** all 29 installed packages are looked up in the `Wolfi` ecosystem
- **AND** no package is listed as not covered

#### Scenario: OA-1.2 Each package, not each source package
- **WHEN** open advisories are requested for `python` `latest`
- **THEN** CVE-2025-15367 is found for `python-3.15`, which on 2026-10-11 had no advisory for `python-3.15-base`, the other package built from the same source

#### Scenario: OA-1.3 Package from another source
- **WHEN** an image's SBOM lists a package whose package URL is neither `wolfi` nor `chainguard`
- **THEN** that package is not looked up and is listed as not covered

### Requirement: OA-2 Counted once per vulnerability
Each vulnerability SHALL be counted once per image, however many advisories and packages it has, using the same rule as the upstream comparison (UC-16). An advisory stands for the CVE IDs it names (in `upstream` or `aliases`, or its own ID), else for the GHSA IDs it names, else for its own ID; an advisory that names several CVEs counts under each of them.

#### Scenario: OA-2.1 python latest-dev
- **WHEN** open advisories are requested for `python` `latest-dev`
- **THEN** OSV.dev lists 64 advisories, 25 of which are already fixed in the installed version (OA-16), and the 39 open ones are reported as 21 vulnerabilities

#### Scenario: OA-2.2 Two advisories on one package
- **WHEN** `python-3.15` has two advisories, `CGA-2w26-qvh7-3jhg` and `CGA-mp3h-w2rr-vq2f`, that both track CVE-2026-12345
- **THEN** CVE-2026-12345 is counted once, and both advisory IDs are listed under it

#### Scenario: OA-2.3 Advisory without a CVE
- **WHEN** an advisory names no CVE but names a GHSA ID, and another package's advisory names the same GHSA ID
- **THEN** both are counted once, under that GHSA ID

#### Scenario: OA-2.4 Advisory naming several CVEs
- **WHEN** an advisory names two CVEs
- **THEN** it counts as two vulnerabilities, and it is listed under each of them

### Requirement: OA-3 Status of an advisory
Each package's advisory SHALL be given a status from Chainguard's current record for that package (OA-17): **fix available** when it names a fixed version newer than the installed one, otherwise **waiting on an upstream fix** (`pending_upstream_fix`), **fix not planned** (`fix_not_planned`), **not yet assessed** (`detection`) or **confirmed** (any other state, such as `true_positive_determination`).

#### Scenario: OA-3.1 python latest and latest-dev
- **WHEN** open advisories are requested for `python` `latest`
- **THEN** CVE-2025-15367 and CVE-2026-12345 are not yet assessed
- **AND** when they are requested for `latest-dev`, CVE-2026-6844 (`binutils`) and CVE-2026-58470 (`wget`) are waiting on an upstream fix

#### Scenario: OA-3.2 Fixed version
- **WHEN** a package's advisory has a range with a fixed version (as `CGA-rc72-c57w-7cmr`'s fixed `1.20.2-r3` for `cilium-fips-1.20-operator-aws`) and the installed version is older
- **THEN** that advisory's status is fix available, and the fixed version is shown

#### Scenario: OA-3.3 Fix not planned
- **WHEN** a package's advisory is in the state `fix_not_planned` (as `CGA-vjm6-5xm3-7w39` for `argo-cd-fips-3.2-compat`)
- **THEN** its status is fix not planned

### Requirement: OA-4 One status per vulnerability
When a vulnerability's advisories have different statuses for different packages, the vulnerability SHALL take the first that applies of: fix available, waiting on an upstream fix, confirmed, fix not planned, not yet assessed. Each package SHALL keep its own status in the details.

#### Scenario: OA-4.1 Mixed statuses
- **WHEN** a vulnerability is not yet assessed for one package and waiting on an upstream fix for another
- **THEN** it counts as waiting on an upstream fix
- **AND** each package is shown with its own status

#### Scenario: OA-4.2 Fix for one package
- **WHEN** a vulnerability has a fix available for any of its packages
- **THEN** it counts as fix available

### Requirement: OA-5 Totals
The result SHALL give one total of open advisories, counted by vulnerability, and the count for each status. The status counts SHALL add up to the total, and statuses with no vulnerabilities SHALL be reported as 0.

#### Scenario: OA-5.1 python latest-dev
- **WHEN** open advisories are requested for `python` `latest-dev`
- **THEN** the total is 21: 17 waiting on an upstream fix, 4 not yet assessed, and 0 fix available, confirmed or fix not planned

#### Scenario: OA-5.2 Counts add up
- **WHEN** any result is returned
- **THEN** the status counts add up to the total

### Requirement: OA-6 One row per vulnerability
The Security tab SHALL show an Open advisories section, above the Recorded fixes section, listing one row per vulnerability with its ID (linked to its OSV.dev page), its status, and each affected package with its installed version, its own status and its advisory IDs. Rows SHALL be ordered by status as in OA-4, then by ID.

#### Scenario: OA-6.1 CVE-2026-12345 on python latest
- **WHEN** the Open advisories section is shown for `python` `latest`
- **THEN** CVE-2026-12345's row says it is not yet assessed
- **AND** it lists `python-3.15` at `3.15.0_git20261010-r0` with its advisories `CGA-2w26-qvh7-3jhg` and `CGA-mp3h-w2rr-vq2f`
- **AND** its ID links to `https://osv.dev/vulnerability/CVE-2026-12345`

#### Scenario: OA-6.2 Order
- **WHEN** the section is shown for `python` `latest-dev`
- **THEN** the 17 vulnerabilities waiting on an upstream fix come before the 4 not yet assessed, and each group is in ID order

### Requirement: OA-7 Compiled-in components
When an advisory was found in software compiled or bundled into a package rather than in the package itself, the row SHALL name that component and its kind next to the package.

#### Scenario: OA-7.1 Rust crate in uv
- **WHEN** the section is shown for `python` `latest-dev`
- **THEN** CVE-2026-21895's row shows package `uv` with the component `rsa` (Rust crate)

#### Scenario: OA-7.2 Bundled Python package
- **WHEN** the section is shown for `python` `latest-dev`
- **THEN** CVE-2025-47273's row shows package `py3.15-pip-base` with the component `setuptools` (Python package)

### Requirement: OA-8 Wording and source
The section and the MCP output SHALL say that the results are known open advisories per Chainguard's published advisories, found via OSV.dev, with each one's status read from Chainguard's advisory feed, and with the time they were checked. They SHALL NOT state or imply that an image or package has no vulnerabilities, is secure or is unaffected, including when the total is 0.

#### Scenario: OA-8.1 Source line
- **WHEN** the section is shown
- **THEN** it says the advisories are Chainguard's published advisories, found via OSV.dev with their status from Chainguard's advisory feed, and shows when they were checked

#### Scenario: OA-8.2 No open advisories
- **WHEN** the total is 0
- **THEN** the text says no open advisories are published for the image's packages as of the check time
- **AND** no text says the image has no vulnerabilities or is secure

#### Scenario: OA-8.3 Not yet assessed explained
- **WHEN** any vulnerability is not yet assessed
- **THEN** the section explains that these were detected but not yet assessed by Chainguard, so some may turn out not to apply

### Requirement: OA-9 Looking up one ID
When a vulnerability ID is looked up (a CVE, GHSA or CGA ID), the result SHALL also say whether it is among the image's open advisories, with its status and packages, in both the Security tab's lookup and `check_vulnerabilities`.

#### Scenario: OA-9.1 Open advisory found
- **WHEN** `CVE-2026-12345` is looked up for `python` `latest`
- **THEN** the result says it is an open advisory, not yet assessed, for `python-3.15`

#### Scenario: OA-9.2 Looked up by advisory ID
- **WHEN** `CGA-2w26-qvh7-3jhg` is looked up for `python` `latest`
- **THEN** the result is the same vulnerability, CVE-2026-12345

#### Scenario: OA-9.3 Not an open advisory
- **WHEN** an ID that has no open advisory for the image is looked up
- **THEN** the result says no open advisory for it is published for the image's packages, without saying the image is unaffected

#### Scenario: OA-9.4 Advisory already fixed in the installed version
- **WHEN** `CVE-2026-89092` is looked up for `python` `latest`, whose advisories OSV.dev still lists as pending
- **THEN** the result says no open advisory for it is published, because Chainguard's current records have it fixed in the installed version (OA-16)

### Requirement: OA-10 MCP tool
`check_vulnerabilities` SHALL return the image's open advisories (OA-5 totals, each vulnerability with its packages, and the not-covered packages) alongside the recorded fixes it already returns. Its description SHALL say it reports open advisories as well as fixes.

#### Scenario: OA-10.1 Summary for python latest
- **WHEN** `check_vulnerabilities` is called for `python` without an `id`
- **THEN** the result has the per-package fixes as before, and open advisories totalling 2, both not yet assessed (CVE-2025-15367 and CVE-2026-12345), and 0 waiting on an upstream fix

#### Scenario: OA-10.2 Description
- **WHEN** a client lists the tools
- **THEN** `check_vulnerabilities`' description mentions open advisories from Chainguard via OSV.dev, and no longer says the tool records only fixes

#### Scenario: OA-10.3 OSV.dev or the advisory feed unavailable
- **WHEN** `check_vulnerabilities` is called for `python` and OSV.dev, or the advisory feed, returns errors
- **THEN** the result still has the recorded fixes
- **AND** it says open advisories couldn't be checked, and gives no open-advisory counts

### Requirement: OA-11 Web API endpoint
The server SHALL serve `GET /api/images/{name}/advisories` with optional query parameters `tag` (default `latest`) and `id`, returning the open advisories as JSON. It SHALL use the existing error codes: 400 `invalid_input`, 403 `not_public`, 404 `not_found`, and 502 `source_failed` when OSV.dev or the advisory feed fails.

#### Scenario: OA-11.1 Listed in the index
- **WHEN** a client sends `GET /api/`
- **THEN** the index has an entry for `/api/images/{name}/advisories` with `name` as a required path parameter and `tag` and `id` as optional query parameters

#### Scenario: OA-11.2 Subscription-only image
- **WHEN** a client requests `/api/images/loki-fips/advisories`
- **THEN** the response is 403 with code `not_public`

### Requirement: OA-12 Freshness
Open advisories for Wolfi and Chainguard packages SHALL be cached for at most 1 hour per package, ecosystem and version, and each advisory read from the advisory feed for at most 1 hour per advisory ID. Each result SHALL carry the time its oldest data was checked. At most 10 requests SHALL be in flight at a time to OSV.dev and the feed together.

#### Scenario: OA-12.1 Repeat within an hour
- **WHEN** the same image and tag are requested twice within an hour
- **THEN** the second request makes no requests to OSV.dev or the advisory feed and reports the first check's time

#### Scenario: OA-12.2 After an hour
- **WHEN** the same image and tag are requested again more than an hour later
- **THEN** its packages are looked up on OSV.dev again, and each advisory is read from the feed again

#### Scenario: OA-12.3 Feed record shared between tags
- **WHEN** `python` `latest-dev` is requested within an hour of `latest`
- **THEN** advisories both tags have, such as `CGA-2w26-qvh7-3jhg`, are not read from the feed a second time

#### Scenario: OA-12.4 Concurrency limit
- **WHEN** an image with many packages and advisories is requested
- **THEN** no more than 10 requests to OSV.dev and the feed are in flight at once

### Requirement: OA-13 Failures and loading
The Open advisories section SHALL load independently of the Recorded fixes section. If any package lookup or advisory read fails (other than a missing feed record, OA-17), no partial result SHALL be shown; the section SHALL show an error with a retry, and Recorded fixes SHALL be unaffected. Switching the tag SHALL reload the section.

#### Scenario: OA-13.1 OSV.dev unavailable
- **WHEN** OSV.dev returns errors
- **THEN** the Open advisories section shows an error with a retry button and no counts
- **AND** the Recorded fixes section shows as usual

#### Scenario: OA-13.2 Subscription-only image
- **WHEN** the Security tab is opened for `loki-fips`
- **THEN** the Open advisories section shows the existing subscription message and no counts

#### Scenario: OA-13.3 Switching tag
- **WHEN** the user switches `python` from `latest` to `latest-dev`
- **THEN** the section shows `latest-dev`'s 21 vulnerabilities

#### Scenario: OA-13.4 Advisory feed unavailable
- **WHEN** the advisory feed returns a server error for any advisory
- **THEN** the section shows an error with a retry button and no counts, as when OSV.dev fails

### Requirement: OA-14 Layout, accessibility and security policy
The Open advisories section SHALL present every count and status as text, and SHALL be usable at 375 px wide, in light and dark mode, by keyboard, and without Content Security Policy violations.

#### Scenario: OA-14.1 Phone width in dark mode
- **WHEN** the Security tab for `python` `latest-dev` is viewed at 375 px wide in dark mode
- **THEN** nothing overflows horizontally, every ID, package and status is readable, and the console shows no Content Security Policy violations

### Requirement: OA-15 Accuracy against an independent scanner
The project SHALL include a live test, skipped unless `CHAINGUARD_LIVE=1` and skipped when grype isn't installed, that scans `cgr.dev/chainguard/python:latest-dev` with grype and fails if any CVE grype reports for an OS package is not among the image's open advisories or its recorded fixes not yet installed, beyond the tolerance recorded in the design.

#### Scenario: OA-15.1 python latest-dev
- **WHEN** the live test runs with grype available
- **THEN** every CVE grype reports for an OS package, within the stated tolerance, is either an open advisory or a fix not yet installed for the same image

### Requirement: OA-16 Advisories fixed in the installed version are not open
An advisory whose current record names a fixed version at or below the installed version of the package (compared with the Alpine version rules the recorded fixes use) SHALL NOT be reported as open. A vulnerability SHALL count only for the packages whose advisories are still open, and SHALL NOT count when none are left.

#### Scenario: OA-16.1 glibc fixed in 2.44-r8
- **WHEN** open advisories are requested for `python` `latest`, where `CGA-2727-9j94-3x3p` for `glibc-2.44` `2.44-r8` is fixed in `2.44-r8` in Chainguard's current record, though OSV.dev's copy still says it is pending an upstream fix
- **THEN** CVE-2026-89092 is not reported, and neither are CVE-2026-8674, CVE-2026-86805, CVE-2026-95818 or CVE-2026-97399, the other glibc advisories fixed in `2.44-r8`

#### Scenario: OA-16.2 Fix in a newer version stays open
- **WHEN** an advisory's current record names a fixed version newer than the installed one
- **THEN** it is reported, with the status fix available (OA-3.2)

#### Scenario: OA-16.3 Partly fixed vulnerability
- **WHEN** a vulnerability's advisory is fixed in the installed version for one package and still open for another
- **THEN** it is reported with only the package where it is still open

#### Scenario: OA-16.4 Record without the package
- **WHEN** an advisory's current record has no entry for the package's own name and ecosystem
- **THEN** it is still reported as open, because leaving it out could hide a real advisory
- **AND** its status is the one in OSV.dev's copy for that package when that copy has one, otherwise confirmed

### Requirement: OA-17 Status from Chainguard's advisory feed
Each distinct advisory found on OSV.dev SHALL be read again from Chainguard's advisory feed (`https://advisories.cgr.dev/chainguard/v3/osv/<id>.json`), and its status (OA-3) and fixed version SHALL come from that record's entry for the package's own name and ecosystem, not from OSV.dev's copy, which can be older.

#### Scenario: OA-17.1 OSV.dev copy is older
- **WHEN** `CGA-2727-9j94-3x3p` is found on OSV.dev as pending an upstream fix, and the feed's record (modified 2026-10-07) has it fixed in `2.44-r8`
- **THEN** the status comes from the feed's record, and the advisory is handled as in OA-16.1

#### Scenario: OA-17.2 Status from the feed
- **WHEN** `CGA-2w26-qvh7-3jhg` is found for `python-3.15`
- **THEN** its status is the one in the feed's record for `python-3.15` in ecosystem `Wolfi`: not yet assessed

#### Scenario: OA-17.3 Advisory missing from the feed
- **WHEN** the feed answers 404 for an advisory OSV.dev lists
- **THEN** OSV.dev's copy is used for it, the result lists its ID as one whose status couldn't be confirmed, and the result carries a note saying so

#### Scenario: OA-17.4 Feed failure
- **WHEN** the feed answers a server error for an advisory
- **THEN** the whole result fails with a 502 `source_failed` naming the feed as the source (OA-13.4)
