# upstream-comparison Specification

## Purpose

Compares an upstream container image a user runs today with its Chainguard replacement, by size, installed OS packages, known vulnerabilities and support status, so the value of switching is visible and stated fairly.

UC-1 to UC-3 define what is compared; UC-4 to UC-6 how vulnerabilities are counted and worded; UC-7 and UC-8 size, build dates and end of life; UC-9 to UC-11 loading, caching and errors; UC-12 the MCP tool; UC-13 accuracy; UC-14 layout and accessibility; UC-15 images whose packages can't be checked; UC-16 how a vulnerability is identified across advisories; UC-17 and UC-18 where the Chainguard advisories' status comes from and which are counted. Figures were measured on 2026-10-11 for Docker Hub's `node:20` (amd64) and `cgr.dev/chainguard/node:latest`, with the Chainguard advisories' status read from Chainguard's advisory feed.

## Requirements

### Requirement: UC-1 What is compared
For an image reference whose Chainguard alternative is found, the comparison SHALL compare the upstream image exactly as entered (repository and tag) with the recommended Chainguard image's `latest` tag, for one architecture: amd64 by default, or arm64 when chosen.

#### Scenario: UC-1.1 node:20
- **WHEN** the user looks up `node:20`
- **THEN** the comparison is between `docker.io/library/node:20` and `cgr.dev/chainguard/node:latest`, both on linux/amd64

#### Scenario: UC-1.2 No alternative
- **WHEN** no Chainguard alternative is found for the reference
- **THEN** no comparison is offered

### Requirement: UC-2 Upstream source and SBOM
The upstream image SHALL be read from Docker Hub, with references without a registry treated as Docker Hub and single names as official images (`node` is `library/node`). Its packages SHALL come from the image's published SBOM attestation for the chosen architecture. A reference on another registry SHALL get a message that only Docker Hub images can be compared, and an image without an SBOM attestation SHALL get a "no SBOM published" state that still shows sizes and build dates.

#### Scenario: UC-2.1 Official image
- **WHEN** `node:20` is compared
- **THEN** its packages are read from the SPDX SBOM attestation of `library/node:20` for linux/amd64

#### Scenario: UC-2.2 No SBOM
- **WHEN** `bitnami/redis:latest`, which publishes no SBOM attestation (as on 2026-10-11), is compared
- **THEN** the upstream side says no SBOM is published, shows no package or vulnerability counts, and still shows its size and build date

#### Scenario: UC-2.3 Another registry
- **WHEN** `ghcr.io/example/app:1` is looked up
- **THEN** the comparison says only Docker Hub images can be compared, and the Alternatives result is otherwise unchanged

### Requirement: UC-3 OS packages only
Both sides SHALL be compared on installed OS packages only (Debian, Ubuntu or Alpine packages upstream; Wolfi or Chainguard packages for the Chainguard image). The upstream package count SHALL count installed binary packages, not the source packages the SBOM also lists. Language packages (npm, PyPI, and so on) SHALL be excluded from all counts, with their number stated.

#### Scenario: UC-3.1 node:20 packages
- **WHEN** `node:20` is compared with Chainguard's `node` (as on 2026-10-11)
- **THEN** the upstream side shows 393 OS packages and notes 193 npm packages not compared, and the Chainguard side shows 28 OS packages

### Requirement: UC-4 Counting known vulnerabilities
Known vulnerabilities SHALL be looked up on OSV.dev for each OS package and version, in the ecosystem the SBOM gives for it (such as `Debian:12`, `Alpine:v3.20` or `Wolfi`), upstream by source package. Each vulnerability, identified as in UC-16, SHALL be counted once per image, however many packages or advisories list it. Advisories for the Linux kernel (Debian's `linux` source package, shipped in images only as kernel headers) SHALL be excluded, since they don't apply inside a container.

#### Scenario: UC-4.1 node:20 total
- **WHEN** `node:20` is compared (as on 2026-10-11)
- **THEN** it shows 833 known vulnerabilities, and none of the 1,614 advisories for the `linux` source package are counted

#### Scenario: UC-4.2 Counted once
- **WHEN** one vulnerability affects both `libssl3` and `openssl`, built from the same source
- **THEN** it counts as one vulnerability

#### Scenario: UC-4.3 Several advisories for one CVE
- **WHEN** Chainguard's `python` is compared (as on 2026-10-11), whose packages have 20 Chainguard advisories on OSV.dev, of which 16 are fixed in the installed versions per Chainguard's advisory feed (UC-18), and the 4 still open (two each for `python-3.15`) name 2 distinct CVEs
- **THEN** it shows 2 known vulnerabilities, CVE-2025-15367 and CVE-2026-12345
- **AND** a Debian `DSA-…` advisory that names CVEs which also have their own `DEBIAN-CVE-…` advisories adds nothing to the count

### Requirement: UC-5 Fix status
Each known vulnerability SHALL be classified, from the advisory for the installed package's own ecosystem and release, as **fix available** (a fixed version newer than the installed one exists), **won't fix** (marked as one its maintainers won't fix, such as Debian's "unimportant" or Chainguard's `fix_not_planned`), or **no fix yet** (otherwise, such as pending an upstream fix). A vulnerability fixable for any affected package, or in any advisory for it, counts as fix available.

#### Scenario: UC-5.1 node:20 split
- **WHEN** `node:20` is compared (as on 2026-10-11)
- **THEN** its 833 vulnerabilities split into 295 fix available, 325 no fix yet and 213 won't fix

#### Scenario: UC-5.2 Chainguard split
- **WHEN** Chainguard's `node` is compared (as on 2026-10-11), whose packages have 22 Chainguard advisories on OSV.dev, 20 of them fixed in the installed versions per Chainguard's advisory feed
- **THEN** it shows 1 known vulnerability, CVE-2026-93750 (two advisories for `npm-12`, pending an upstream fix), as no fix yet, and 0 fix available

#### Scenario: UC-5.3 Fix in another release only
- **WHEN** a Debian advisory has a fixed version for Debian 13 but none for Debian 12, and the image is Debian 12
- **THEN** it counts as no fix yet for that image

#### Scenario: UC-5.4 Chainguard fix not planned
- **WHEN** a Chainguard advisory's current record (UC-17) has no fixed version and its state for the installed package is `fix_not_planned`
- **THEN** it counts as won't fix for the Chainguard image

#### Scenario: UC-5.5 Other Chainguard states
- **WHEN** a Chainguard advisory's current record has no fixed version and its state is `pending_upstream_fix`, `detection` or any other state except `fix_not_planned`
- **THEN** it counts as no fix yet

### Requirement: UC-6 Headline and wording
The comparison SHALL lead with the fix-available counts and the OS package counts, then show the full split for both sides. It SHALL state its source and date as known vulnerabilities per each distribution's published advisories, via OSV.dev, with the Chainguard advisories' statuses from Chainguard's advisory feed, on the date checked, and SHALL NOT describe either image as secure or free of vulnerabilities.

#### Scenario: UC-6.1 Summary
- **WHEN** `node:20` is compared (as on 2026-10-11)
- **THEN** the summary says `node:20` has 295 known vulnerabilities with a fix available and Chainguard's `node` has 0, and 393 OS packages against 28

#### Scenario: UC-6.2 Zero on one side
- **WHEN** the Chainguard image has no known vulnerabilities at all
- **THEN** it reads "0 known vulnerabilities" with the source and date, never "vulnerability-free" or "secure"

#### Scenario: UC-6.3 Source names the feed
- **WHEN** any comparison is shown, in the web UI or through `compare_with_upstream`
- **THEN** its source says the Chainguard advisories' statuses come from Chainguard's advisory feed, and the counts are known vulnerabilities only

### Requirement: UC-7 Size and build date
The comparison SHALL show each image's compressed download size for the chosen architecture, and each image's build date from its configuration.

#### Scenario: UC-7.1 node:20 size and age
- **WHEN** `node:20` is compared (as on 2026-10-11)
- **THEN** it shows 398.4 MB built on 2026-04-22 against 65.9 MB built on 2026-10-09

### Requirement: UC-8 End-of-life note
When the upstream image's name and tag map confidently to a product and release cycle on endoflife.date whose end-of-life date has passed, the comparison SHALL note that the release reached end of life on that date and may no longer receive security updates. When the mapping isn't confident (an unknown image, or a tag such as `latest` or `lts`), it SHALL show no note.

#### Scenario: UC-8.1 Node.js 20
- **WHEN** `node:20` or `node:20-alpine` is compared
- **THEN** a note says Node.js 20 reached end of life on 2026-04-30

#### Scenario: UC-8.2 No confident mapping
- **WHEN** `node:latest` or an image with no known product is compared
- **THEN** no end-of-life note is shown

### Requirement: UC-9 Progress while loading
While a comparison is being computed, the web UI SHALL show its progress with the number of packages checked out of the total, and SHALL never show only a loading message for it.

#### Scenario: UC-9.1 First comparison
- **WHEN** `node:20` is compared and nothing is cached
- **THEN** a progress bar shows packages checked out of the total until the comparison appears

### Requirement: UC-10 Caching and load
SBOMs and package lists SHALL be cached by image digest; vulnerability lookups SHALL be cached for at most 24 hours per package, ecosystem and version, and for at most 1 hour for Wolfi and Chainguard packages; each advisory read from Chainguard's advisory feed SHALL be cached for at most 1 hour; end-of-life data SHALL be cached for 24 hours. At most 10 requests SHALL be in flight to OSV.dev and the feed together, and a comparison SHALL make no more than 10 requests to Docker Hub.

#### Scenario: UC-10.1 Repeat comparison
- **WHEN** `node:20` is compared again within an hour
- **THEN** the result appears without new requests to OSV.dev, to the advisory feed or for the SBOM

#### Scenario: UC-10.2 Repeat after an hour
- **WHEN** `node:20` is compared again more than an hour but less than 24 hours later
- **THEN** only the Chainguard image's packages are looked up on OSV.dev again and their advisories read from the feed again, and the upstream image's lookups come from the cache

#### Scenario: UC-10.3 Advisory shared with the Security tab
- **WHEN** Chainguard's `python` is compared within an hour of its Security tab being opened for `latest`
- **THEN** the advisories both have are not read from the feed a second time

### Requirement: UC-11 Errors
If Docker Hub, OSV.dev, Chainguard's advisory feed or endoflife.date fails, the comparison SHALL say which source failed and offer a retry, and SHALL NOT show partial vulnerability counts as complete. Docker Hub's rate limit SHALL be reported as such. An end-of-life lookup failure SHALL only omit the note, and an advisory missing from the feed is handled as in UC-17.

#### Scenario: UC-11.1 Rate limited
- **WHEN** Docker Hub answers with its rate-limit status
- **THEN** the comparison says Docker Hub's anonymous limit was reached and to try again later

#### Scenario: UC-11.2 OSV failure
- **WHEN** some OSV.dev lookups fail
- **THEN** no vulnerability counts are shown for that side, and the error offers a retry

#### Scenario: UC-11.3 Advisory feed failure
- **WHEN** the advisory feed answers a server error for any Chainguard advisory the comparison found
- **THEN** no comparison counts are shown, the error names "Chainguard advisories" as the failed source (502 `source_failed` in the web API), and it offers a retry

### Requirement: UC-12 MCP tool
An MCP tool `compare_with_upstream` SHALL return the same comparison as the web UI for an image reference and architecture, including the source and date, the fix-status split, sizes, build dates and any end-of-life note.

#### Scenario: UC-12.1 Same numbers
- **WHEN** `compare_with_upstream` is called for `node:20` against the test backend
- **THEN** its counts equal the web API's for the same reference

### Requirement: UC-13 Accuracy against an independent scanner
The project SHALL include a live test, skipped unless `CHAINGUARD_LIVE=1` and skipped when grype isn't installed, that compares the upstream vulnerabilities found for `node:20` with grype's scan of the same image, excluding the kernel, and SHALL fail if fewer than 95% of grype's CVEs are found.

#### Scenario: UC-13.1 Agreement on node:20
- **WHEN** the live test runs with grype available
- **THEN** at least 95% of grype's OS-package CVEs for `node:20` (excluding the kernel) are among the comparison's vulnerabilities (on 2026-10-11 it was all 817)

### Requirement: UC-14 Layout, accessibility and security policy
The comparison SHALL present every number as text, with any chart only as an addition, and SHALL be usable at 375 px wide, in light and dark mode, by keyboard, and without Content Security Policy violations.

#### Scenario: UC-14.1 Phone width in dark mode
- **WHEN** the comparison is viewed at 375 px wide in dark mode
- **THEN** nothing overflows horizontally, all counts are readable as text, and the console shows no Content Security Policy violations

### Requirement: UC-15 Unchecked images never read as zero
When an upstream image's SBOM lists no packages from a supported distribution, or lists packages from a distribution with no vulnerability data here (such as RPM-based Fedora), the comparison SHALL show that its known vulnerabilities could not be checked and why, with no package or vulnerability counts for it, and SHALL NOT present it as having 0 known vulnerabilities. Size and build date SHALL still be shown.

#### Scenario: UC-15.1 A static image
- **WHEN** `busybox:latest`, whose SBOM lists no distribution packages (as on 2026-10-11), is compared with Chainguard's `busybox`, whose 20 advisories on OSV.dev are all fixed in the installed versions per Chainguard's advisory feed
- **THEN** the upstream side says its SBOM lists no distribution packages so its known vulnerabilities can't be checked, that this is not the same as having none, and shows no counts, while the Chainguard side shows 0 known vulnerabilities with the source and date

#### Scenario: UC-15.2 An unsupported distribution
- **WHEN** an upstream image's SBOM lists RPM packages from Fedora 40
- **THEN** the upstream side says its packages come from a distribution with no vulnerability data here, names it, and shows no counts; no package is silently left out

### Requirement: UC-16 Identifying a vulnerability
An advisory SHALL stand for the CVE IDs it names (in its `upstream` or `aliases` fields, or its own ID); if it names no CVE, for the GHSA IDs it names; otherwise for its own ID. An advisory that names several CVEs SHALL count as each of them.

#### Scenario: UC-16.1 A DSA naming several CVEs
- **WHEN** a Debian `DSA-…` advisory names several CVEs (as DSA-6113-1 names 8 OpenSSL CVEs for `openssl` 3.0.17-1~deb12u2), each of which also has its own `DEBIAN-CVE-…` advisory
- **THEN** each of those CVEs counts once, and the DSA adds nothing to the count

#### Scenario: UC-16.2 Advisories naming only a GHSA ID
- **WHEN** two Chainguard advisories for different packages name no CVE but the same GHSA ID
- **THEN** they count as one vulnerability, identified by that GHSA ID

#### Scenario: UC-16.3 Advisory naming neither
- **WHEN** an advisory names no CVE and no GHSA ID
- **THEN** it counts as one vulnerability, identified by its own ID

### Requirement: UC-17 Chainguard advisories' status from Chainguard's advisory feed
For the Chainguard image, each distinct Chainguard advisory found on OSV.dev SHALL be read again from Chainguard's advisory feed (`https://advisories.cgr.dev/chainguard/v3/osv/<id>.json`), and its fix status and fixed version SHALL come from that record's entry for the installed package's own name and ecosystem, not from OSV.dev's copy, which can be older. The upstream image's advisories SHALL come from OSV.dev only.

#### Scenario: UC-17.1 OSV.dev copy is older
- **WHEN** Chainguard's `python` is compared and OSV.dev lists `CGA-2727-9j94-3x3p` for `glibc-2.44` `2.44-r8` as pending an upstream fix, while the feed's record (modified 2026-10-07) has it fixed in `2.44-r8`
- **THEN** the status comes from the feed's record, and the advisory is not counted (UC-18)

#### Scenario: UC-17.2 Advisory missing from the feed
- **WHEN** the feed answers 404 for a Chainguard advisory OSV.dev lists
- **THEN** OSV.dev's copy is used for it, and the comparison's notes name the advisory and say its status couldn't be confirmed with Chainguard's advisory feed

#### Scenario: UC-17.3 Record without the package
- **WHEN** an advisory's current record has no entry for the installed package's own name and ecosystem
- **THEN** OSV.dev's copy is used for that package, so the advisory still counts, because leaving it out could hide a real vulnerability

#### Scenario: UC-17.4 Upstream side unchanged
- **WHEN** `node:20` is compared
- **THEN** no Debian advisory is read from the feed, and the upstream counts are the same as from OSV.dev alone

### Requirement: UC-18 Chainguard advisories fixed in the installed version are not counted
A Chainguard advisory whose current record (UC-17) names a fixed version at or below the installed version of the package, compared with Alpine's version rules, SHALL NOT be counted for the Chainguard image. A vulnerability SHALL count only for the packages whose advisories are still open, and SHALL NOT count when none are left.

#### Scenario: UC-18.1 glibc fixed in 2.44-r8
- **WHEN** Chainguard's `node` is compared (as on 2026-10-11) and the current records of its `glibc-2.44` and `ld-linux` advisories have them fixed in `2.44-r8`, the installed version
- **THEN** none of those advisories' CVEs are counted, though OSV.dev's copies still list them as pending an upstream fix

#### Scenario: UC-18.2 Partly fixed vulnerability
- **WHEN** a CVE's advisory is fixed in the installed version for one package and still open for another
- **THEN** the CVE counts once, with the status from the advisory that is still open

#### Scenario: UC-18.3 Fix in a newer version
- **WHEN** an advisory's current record names a fixed version newer than the installed one
- **THEN** it counts as fix available
