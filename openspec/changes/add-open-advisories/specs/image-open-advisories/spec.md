# Spec Delta

## Purpose

Shows the known open advisories that Chainguard has published for a Chainguard image's installed packages, read through OSV.dev, in the Security tab and through MCP. It counts each vulnerability once, says what state each one is in, and never implies that an image without advisories is free of vulnerabilities.

OA-1 to OA-5 define what is looked up and how it is counted; OA-6 to OA-8 what is shown and how it is worded; OA-9 and OA-10 the MCP tool and the web API; OA-11 to OA-13 freshness, failures and loading; OA-14 layout; OA-15 accuracy. Figures were measured on 2026-10-11 for `python` `latest` (29 packages) and `latest-dev` (76 packages), amd64.

## ADDED Requirements

### Requirement: OA-1 Packages looked up
For an image and tag, open advisories SHALL be looked up on OSV.dev for each installed linux/amd64 package by its own name and version, in the ecosystem its package URL gives (`Wolfi` or `Chainguard`). Packages from any other source SHALL be listed as not covered, not looked up.

#### Scenario: OA-1.1 python latest
- **WHEN** open advisories are requested for `python` `latest`
- **THEN** all 29 installed packages are looked up in the `Wolfi` ecosystem
- **AND** no package is listed as not covered

#### Scenario: OA-1.2 Each package, not each source package
- **WHEN** open advisories are requested for `python` `latest`
- **THEN** CVE-2026-95818 is found, which on 2026-10-11 was filed only against `ld-linux-2.44` and not against `glibc-2.44`, the other package built from the same source

#### Scenario: OA-1.3 Package from another source
- **WHEN** an image's SBOM lists a package whose package URL is neither `wolfi` nor `chainguard`
- **THEN** that package is not looked up and is listed as not covered

### Requirement: OA-2 Counted once per vulnerability
Each vulnerability SHALL be counted once per image, however many advisories and packages it has, using the same rule as the upstream comparison (UC-16). An advisory stands for the CVE IDs it names (in `upstream` or `aliases`, or its own ID), else for the GHSA IDs it names, else for its own ID; an advisory that names several CVEs counts under each of them.

#### Scenario: OA-2.1 python latest
- **WHEN** open advisories are requested for `python` `latest`
- **THEN** its 20 advisories are reported as 7 vulnerabilities

#### Scenario: OA-2.2 Two advisories on one package
- **WHEN** `glibc-2.44` has two advisories, `CGA-2727-9j94-3x3p` and `CGA-4mgp-78rw-5337`, that both track CVE-2026-89092
- **THEN** CVE-2026-89092 is counted once, and both advisory IDs are listed under it

#### Scenario: OA-2.3 Advisory without a CVE
- **WHEN** an advisory names no CVE but names a GHSA ID, and another package's advisory names the same GHSA ID
- **THEN** both are counted once, under that GHSA ID

#### Scenario: OA-2.4 Advisory naming several CVEs
- **WHEN** an advisory names two CVEs
- **THEN** it counts as two vulnerabilities, and it is listed under each of them

### Requirement: OA-3 Status of an advisory
Each package's advisory SHALL be given a status from Chainguard's record for that package: **fix available** when it names a fixed version, otherwise **waiting on an upstream fix** (`pending_upstream_fix`), **fix not planned** (`fix_not_planned`), **not yet assessed** (`detection`) or **confirmed** (any other state, such as `true_positive_determination`).

#### Scenario: OA-3.1 python latest
- **WHEN** open advisories are requested for `python` `latest`
- **THEN** CVE-2026-89092, CVE-2026-8674, CVE-2026-86805 and CVE-2026-95818 are waiting on an upstream fix
- **AND** CVE-2026-97399, CVE-2026-12345 and CVE-2025-15367 are not yet assessed

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
- **THEN** the total is 26: 21 waiting on an upstream fix, 5 not yet assessed, and 0 fix available, confirmed or fix not planned

#### Scenario: OA-5.2 Counts add up
- **WHEN** any result is returned
- **THEN** the status counts add up to the total

### Requirement: OA-6 One row per vulnerability
The Security tab SHALL show an Open advisories section, above the Recorded fixes section, listing one row per vulnerability with its ID (linked to its OSV.dev page), its status, and each affected package with its installed version, its own status and its advisory IDs. Rows SHALL be ordered by status as in OA-4, then by ID.

#### Scenario: OA-6.1 CVE-2026-89092 on python latest
- **WHEN** the Open advisories section is shown for `python` `latest`
- **THEN** CVE-2026-89092's row says it is waiting on an upstream fix
- **AND** it lists `glibc-2.44`, `glibc-2.44-locale-posix` and `ld-linux-2.44`, each at `2.44-r8`
- **AND** its ID links to `https://osv.dev/vulnerability/CVE-2026-89092`

#### Scenario: OA-6.2 Order
- **WHEN** the section is shown for `python` `latest`
- **THEN** the four vulnerabilities waiting on an upstream fix come before the three not yet assessed

### Requirement: OA-7 Compiled-in components
When an advisory was found in software compiled or bundled into a package rather than in the package itself, the row SHALL name that component and its kind next to the package.

#### Scenario: OA-7.1 Rust crate in uv
- **WHEN** the section is shown for `python` `latest-dev`
- **THEN** CVE-2026-21895's row shows package `uv` with the component `rsa` (Rust crate)

#### Scenario: OA-7.2 Bundled Python package
- **WHEN** the section is shown for `python` `latest-dev`
- **THEN** CVE-2025-47273's row shows package `py3.15-pip-base` with the component `setuptools` (Python package)

### Requirement: OA-8 Wording and source
The section and the MCP output SHALL say that the results are known open advisories per Chainguard's published advisories, via OSV.dev, with the time they were checked. They SHALL NOT state or imply that an image or package has no vulnerabilities, is secure or is unaffected, including when the total is 0.

#### Scenario: OA-8.1 Source line
- **WHEN** the section is shown
- **THEN** it says the advisories are Chainguard's published advisories, via OSV.dev, and shows when they were checked

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
- **WHEN** `CVE-2026-89092` is looked up for `python` `latest`
- **THEN** the result says it is an open advisory waiting on an upstream fix for `glibc-2.44`, `glibc-2.44-locale-posix` and `ld-linux-2.44`

#### Scenario: OA-9.2 Looked up by advisory ID
- **WHEN** `CGA-2727-9j94-3x3p` is looked up for `python` `latest`
- **THEN** the result is the same vulnerability, CVE-2026-89092

#### Scenario: OA-9.3 Not an open advisory
- **WHEN** an ID that has no open advisory for the image is looked up
- **THEN** the result says no open advisory for it is published for the image's packages, without saying the image is unaffected

### Requirement: OA-10 MCP tool
`check_vulnerabilities` SHALL return the image's open advisories (OA-5 totals, each vulnerability with its packages, and the not-covered packages) alongside the recorded fixes it already returns. Its description SHALL say it reports open advisories as well as fixes.

#### Scenario: OA-10.1 Summary for python latest
- **WHEN** `check_vulnerabilities` is called for `python` without an `id`
- **THEN** the result has the per-package fixes as before, and open advisories totalling 7, with 4 waiting on an upstream fix and 3 not yet assessed

#### Scenario: OA-10.2 Description
- **WHEN** a client lists the tools
- **THEN** `check_vulnerabilities`' description mentions open advisories from Chainguard via OSV.dev, and no longer says the tool records only fixes

#### Scenario: OA-10.3 OSV.dev unavailable
- **WHEN** `check_vulnerabilities` is called for `python` and OSV.dev returns errors
- **THEN** the result still has the recorded fixes
- **AND** it says open advisories couldn't be checked, and gives no open-advisory counts

### Requirement: OA-11 Web API endpoint
The server SHALL serve `GET /api/images/{name}/advisories` with optional query parameters `tag` (default `latest`) and `id`, returning the open advisories as JSON. It SHALL use the existing error codes: 400 `invalid_input`, 403 `not_public`, 404 `not_found`, and 502 `source_failed` when OSV.dev fails.

#### Scenario: OA-11.1 Listed in the index
- **WHEN** a client sends `GET /api/`
- **THEN** the index has an entry for `/api/images/{name}/advisories` with `name` as a required path parameter and `tag` and `id` as optional query parameters

#### Scenario: OA-11.2 Subscription-only image
- **WHEN** a client requests `/api/images/loki-fips/advisories`
- **THEN** the response is 403 with code `not_public`

### Requirement: OA-12 Freshness
Open advisories for Wolfi and Chainguard packages SHALL be cached for at most 1 hour per package, ecosystem and version, and each result SHALL carry the time its data was checked.

#### Scenario: OA-12.1 Repeat within an hour
- **WHEN** the same image and tag are requested twice within an hour
- **THEN** the second request makes no requests to OSV.dev and reports the first check's time

#### Scenario: OA-12.2 After an hour
- **WHEN** the same image and tag are requested again more than an hour later
- **THEN** its packages are looked up on OSV.dev again

### Requirement: OA-13 Failures and loading
The Open advisories section SHALL load independently of the Recorded fixes section. If any package lookup fails, no partial result SHALL be shown; the section SHALL show an error with a retry, and Recorded fixes SHALL be unaffected. Switching the tag SHALL reload the section.

#### Scenario: OA-13.1 OSV.dev unavailable
- **WHEN** OSV.dev returns errors
- **THEN** the Open advisories section shows an error with a retry button and no counts
- **AND** the Recorded fixes section shows as usual

#### Scenario: OA-13.2 Subscription-only image
- **WHEN** the Security tab is opened for `loki-fips`
- **THEN** the Open advisories section shows the existing subscription message and no counts

#### Scenario: OA-13.3 Switching tag
- **WHEN** the user switches `python` from `latest` to `latest-dev`
- **THEN** the section shows `latest-dev`'s 26 vulnerabilities

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
