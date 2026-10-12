## MODIFIED Requirements

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

### Requirement: UC-15 Unchecked images never read as zero
When an upstream image's SBOM lists no packages from a supported distribution, or lists packages from a distribution with no vulnerability data here (such as RPM-based Fedora), the comparison SHALL show that its known vulnerabilities could not be checked and why, with no package or vulnerability counts for it, and SHALL NOT present it as having 0 known vulnerabilities. Size and build date SHALL still be shown.

#### Scenario: UC-15.1 A static image
- **WHEN** `busybox:latest`, whose SBOM lists no distribution packages (as on 2026-10-11), is compared with Chainguard's `busybox`, whose 20 advisories on OSV.dev are all fixed in the installed versions per Chainguard's advisory feed
- **THEN** the upstream side says its SBOM lists no distribution packages so its known vulnerabilities can't be checked, that this is not the same as having none, and shows no counts, while the Chainguard side shows 0 known vulnerabilities with the source and date

#### Scenario: UC-15.2 An unsupported distribution
- **WHEN** an upstream image's SBOM lists RPM packages from Fedora 40
- **THEN** the upstream side says its packages come from a distribution with no vulnerability data here, names it, and shows no counts; no package is silently left out

## ADDED Requirements

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
