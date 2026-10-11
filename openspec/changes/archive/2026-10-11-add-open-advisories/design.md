# Design

## Context

See proposal.md for the motivation. Relevant current state:

- `Service.CheckVulnerabilities` (service.go) reads an image's amd64 packages from its SBOM (`Registry.Packages`) and runs `analyzeVulns` against the Wolfi security database (vulns.go). Both the MCP tool `check_vulnerabilities` and `GET /api/images/{name}/vulnerabilities` call it. That endpoint feeds the Security tab's lookup, totals, fixes chart and table.
- The upstream comparison added an OSV.dev client (osv.go): `OSV.Query` per package with pagination, a 24-hour cache keyed by `OSVPackage`, at most 10 requests in flight, and `QueryAll`, which fails the whole set on the first error. `OSVVuln` reads only `id` and `affected` (package, `fixed` events, `ecosystem_specific.urgency` and `components[].latest_event_status`). `osvEcosystemForDistro` maps a package URL's namespace to `Wolfi` or `Chainguard`.
- Chainguard's OSV records (sampled live on 2026-10-11) list the package under both the `Chainguard` and `Wolfi` ecosystems. Each carries the tracked IDs in `upstream` (for example `["CVE-2026-89092", "GHSA-gwqp-9qgw-c5pv"]`), not in `aliases`. Each component has `component_name`, `component_type` (`apk`, `python`, `rust-crate`, …) and `latest_event_status`. States seen: `fixed` (with a `fixed` range event), `fix_not_planned`, `pending_upstream_fix` and `detection`. No false-positive state appeared among 400 recently modified advisories.
- `compareUpstream` counted by OSV record ID, which overcounted Chainguard advisories. The separate change `fix-upstream-cve-counting` fixes that: `OSVVuln` gains `Upstream` and `Aliases`, and a new `vulnKeys` names the vulnerabilities each record stands for.
- Chainguard also publishes each advisory as a file, `https://advisories.cgr.dev/chainguard/v3/osv/<id>.json` (anonymous, served as `application/json`, with the same schema as the OSV record; a missing ID answers 404 with an XML `NoSuchKey` body). OSV.dev serves *older copies* of some of them: on 2026-10-11 the feed had `CGA-2727-9j94-3x3p`, `CGA-4mgp-78rw-5337` and `CGA-84qm-pxw8-f58f` (glibc-2.44) fixed in `2.44-r8` (modified 2026-10-07T02:17Z, `latest_event_status: "fixed"`, range `[{introduced: 0}, {fixed: 2.44-r8}]`), while OSV.dev's `/v1/vulns/<id>` still had the 2026-10-06T22:08Z copy, pending an upstream fix with no fixed event. The first implementation therefore listed fixed advisories as open (see "Finding from the browser checks").
- In tests, `newFakeBackend` points OSV at an unreachable address, and `fakeOSV` (fake_test.go) serves `/v1/query` from records registered per `OSVPackage`.

## Goals / Non-Goals

**Goals:**
- One `Service` method for open advisories that the web API, the Security tab lookup and `check_vulnerabilities` all use.
- The Recorded fixes data and endpoint stay as fast as today: no OSV request on the `/vulnerabilities` path.
- Reuse the OSV client, its concurrency limit and its cache.

**Non-Goals:**
- Changing how the upstream comparison classifies or counts (left to the separate fix).
- Severity, arm64, or other tags than the Security tab's.

## Decisions

### 1. Two Service methods plus one that combines them

- `Service.OpenAdvisories(ctx, OpenAdvisoriesInput{Image, Tag, ID}) (*AdvisoryReport, error)` reads the packages and looks them up. It backs `GET /api/images/{name}/advisories`.
- `CheckVulnerabilities` keeps its current behavior for the web endpoint.
- A new `Service.SecurityReport` backs the MCP tool. It reads the packages once, runs `analyzeVulns` and the advisory lookup, and returns `VulnReport` with a new optional `open_advisories` field. If OSV fails, it sets `open_advisories_error` instead (OA-10.3).
- The package-to-report step is a function taking `[]Package`, so `SecurityReport` doesn't fetch the SBOM twice.

*Alternatives:* (a) Have `CheckVulnerabilities` always include advisories. Rejected: the fixes chart would wait on OSV, and an OSV outage would break the existing endpoint. (b) Build the combined report in the tool handler. Rejected: logic must stay in `Service`, per `openspec/config.yaml`.

### 2. Advisory data model (Go, mirrored in `web/src/api.ts`)

```
AdvisoryReport { image, tag, digest, checked_at, source, note,
                 total, counts{fix_available, pending_upstream_fix, confirmed, fix_not_planned, not_yet_assessed},
                 vulnerabilities[], not_covered[], status_from_osv[]?, id?, match?, lookup_note? }
OpenVulnerability { id, status, aliases[], packages[] }
AdvisoryPackage  { name, version, status, advisories[], fixed_version?, components[]{name, kind} }
```

- `match` is the matching vulnerability when `id` is set. `vulnerabilities` is still returned, so the UI lookup and the section can share one cached query per tag.
- `components` lists only non-`apk` components, so it is empty for advisories on the package itself.
- `kind` is mapped to a readable label in Go: `rust-crate` → "Rust crate", `python` → "Python package", `go-module` → "Go module", `npm` → "npm package". Any other value passes through unchanged.

### 3. Grouping key

Reuse `vulnKeys` from the comparison fix (`fix-upstream-cve-counting`). Don't write a second key function: both features must count the same way (OA-2, UC-16). It returns the distinct CVE IDs a record names (its own ID, `upstream`, `aliases`), else its distinct GHSA IDs, else its own ID, and `OSVVuln` already reads `upstream` and `aliases` after that fix.

A record can therefore have several keys. The report builder adds the record's package entry to each key's vulnerability, so an advisory naming two CVEs appears in both rows (OA-2.4). Its advisory ID is listed under each.

Known gap, shared with the comparison: two advisories that name neither a CVE nor a GHSA (only, say, a PYSEC ID) fall back to their own IDs and count twice. None appeared in the 2026-10-11 data.

An ID lookup (OA-9) matches case-insensitively against the key, every record ID and every `upstream`/`aliases` entry of the grouped records.

### 4. Status mapping

The mapping is a new function `advisoryStatus(v OSVVuln, pkg OSVPackage)`, separate from the comparison's three-way `classify`, reading the `affected` entry with the package's own name and ecosystem:
- A non-empty `fixed` event means **fix available**. OSV returns only records that affect the installed version, so the fix is newer.
- Otherwise `latest_event_status` of the component whose `component_type` is `apk` (or, failing that, the first component): `pending_upstream_fix`, `fix_not_planned`, `detection` → not yet assessed, and anything else → confirmed.

A vulnerability's overall status is the first that applies in OA-4's order. That puts confirmed states above unassessed ones and the actionable state first.

The status and the fixed version are read from the **current record** (decision 9), not from the OSV.dev copy. A record whose entry for the package has a fixed version at or below the installed version (`compareAPKVersions`) is not open and is left out (OA-16). A record with no entry for the package's own name and ecosystem is **kept as open** (decided with the user on 2026-10-11): it may be a real advisory, and leaving it out would hide it, which the "never overclaim security" rule guards against. Its status is taken from the OSV.dev copy's entry for the package when that has one (`withCurrentRecords` keeps the OSV.dev copy in that case), otherwise it is "confirmed" (OA-16.4).

### 5. Caching and `checked_at`

- `OSV` gets a per-ecosystem TTL: 1 hour for `Wolfi` and `Chainguard`, 24 hours otherwise. This also makes the comparison's Chainguard side fresher, which UC-10's "at most 24 hours" allows.
- `QueryAll` also returns the oldest fetch time among the results it used, and the feed client returns the oldest fetch time among the records it used; the older of the two becomes `checked_at`, so a cached answer reports its real age (OA-12.1).
- Feed records are cached for 1 hour per advisory ID, including a 404 (so a withdrawn advisory isn't requested again within the hour).
- The SBOM package list is read as `CheckVulnerabilities` does today.

### 9. Reading current records from Chainguard's advisory feed (decided 2026-10-11)

- A new client, `AdvisoryFeed` (in `advisory_feed.go`), fetches `<URL>/chainguard/v3/osv/<id>.json` anonymously and decodes it as an `OSVVuln` (same schema). Its fields `HTTP`, `URL`, `TTL`, `Now` and cache mirror `OSV`'s; `Service.Feed` holds it. The upstream comparison does not use it yet; a follow-up will reuse the client.
- `openAdvisories` runs in two steps: `QueryAllAt` on OSV.dev for each package (as before), then `FetchAll` on the feed for the distinct advisory IDs found. The steps run one after the other, so no more than 10 requests (`osvMaxInFlight`) are in flight at once across both (OA-12.4); each step uses the same worker pattern and cancels on the first error.
- **404 handling:** a 404 uses the OSV.dev copy for that advisory and lists its ID in `status_from_osv`, with a note in the report (OA-17.3). The feed is the source OSV.dev copies from, so a 404 means the advisory was withdrawn or hasn't been published to the bucket yet; failing the whole result would make one withdrawn advisory block the section for an hour, and dropping it could hide a real one. The OSV copy is the more cautious reading and is labelled as unconfirmed. *Rejected:* failing (too brittle), and dropping (can hide an advisory).
- Any other failure (network error, 5xx, a body that isn't JSON) fails the whole result as a `*SourceError` with source "Chainguard advisories", like OSV.dev (OA-13.4, OA-17.4); no partial result is shown.
- Records that aren't `CGA-` IDs aren't read from the feed (it only holds Chainguard's advisories); the OSV.dev copy is used as is.
- **Load:** one extra request per distinct advisory on a cache miss (20 for python `latest`, 64 for `latest-dev`, measured on 2026-10-11), each a small JSON file. The 1-hour cache is shared across tags and images, so `latest-dev` after `latest` reuses the advisories they share (OA-12.3).
- **Config:** `advisories.cgr.dev` is a Chainguard public, anonymous endpoint, so `openspec/config.yaml`'s rule "data comes only from Chainguard's public, anonymous endpoints" already covers it; no exception is added.
- *Alternatives considered:* (a) trust OSV.dev alone: rejected, it showed fixed advisories as open. (b) Cross-check with the Wolfi security database: rejected, it disagrees with the feed for some packages (see Risks). (c) Download `all.json`: rejected, it is only an ID index of about 1.2 million entries.

### 6. Web UI

- A new `OpenAdvisories` component (`web/src/components/OpenAdvisories.tsx`), rendered by `SecurityTab` above Recorded fixes. Its query key is `['advisories', name, tag]`, independent of `vulnSummary` (OA-13).
- Status labels, tones and order go in `web/src/styles.ts`, which keeps Vite fast refresh working.
- Layout: the source line with the check time, then a row of `Stat` tiles for the total and each non-zero status (zeros as text in the summary line). Then a list with one item per vulnerability: an ID link, a status badge and the packages with versions, statuses, components and advisory IDs.
- No chart: the list is already the clearest form, and every count is text (OA-14).
- The lookup card also queries `/advisories?id=` and shows an "Open advisory" block next to the Wolfi result.
- Loading text gives the expected time: "Checking N packages on OSV.dev, usually a few seconds". 76 packages took 1.6 s uncached, so no progress job is needed.
- The Recorded fixes note gains a pointer to the section (SF-7).

### 7. Wording

- Source line: "Known open advisories per Chainguard's published advisories, found via OSV.dev with their status from Chainguard's advisory feed, checked <time>."
- Zero: "No open advisories are published for this image's packages as of <time>." Never "no vulnerabilities".
- Not yet assessed: "Detected but not yet assessed by Chainguard; some may turn out not to apply."
- The `check_vulnerabilities` description, the `vulnNote`, the README's tool table and its Wolfi paragraph are updated to match.

### 8. Tests

- `newFakeBackend` keeps OSV unreachable, so tests that don't need it still fail fast if they call it.
- Advisory tests use a new helper that wires `fakeOSV` with Chainguard-shaped records (python `latest`'s glibc and python advisories, a two-advisories-one-CVE case, a `rust-crate` component, a `fixed` range and a `fix_not_planned` record).
- Existing `check_vulnerabilities` tool tests assert OA-10.3 (OSV unreachable → fixes still returned with `open_advisories_error`), so they keep passing without new fixtures.

### Dependencies

None added.

## Risks / Trade-offs

- [Chainguard changes its OSV fields or state names] → Unknown states map to "confirmed", which is never shown as fixed or harmless. Tests pin the known names.
- [False positives are published after all] → The grype cross-check (task 1) and OA-15 would show extra advisories that grype doesn't report. If found, filter that state and record it here before building the UI.
- [OSV.dev outage or rate limiting] → The section shows an error with a retry, the fixes section is unaffected, and MCP still returns the fixes (OA-10.3, OA-13).
- [Hourly freshness costs more OSV requests] → At most one request per package per hour per server, 10 at a time. A typical image has 30–80 packages, and the feed adds one small request per distinct advisory (20–64 for python), also cached for an hour.
- [The feed and OSV.dev disagree, or the feed is down] → The feed is the authority for status. A feed outage fails the section like an OSV.dev outage (OA-13.4); a 404 falls back to the OSV copy and says so (OA-17.3).
- [The Wolfi security database still disagrees for some packages] → After the feed fix, `wget` (CVE-2026-58470 to -58472, secdb fixed in `1.25.0-r15`, installed `1.25.0-r20`) and `uv` (CVE-2026-21895, secdb fixed in `0.9.27-r4`, installed `0.13.0-r0`) are open in the feed but recorded as fixed in the secdb. Both sources are Chainguard's and the section reports what the advisory feed says; this is for the user to raise with Chainguard, not to reconcile here.
- [Differences from the comparison's numbers until its fix lands] → The comparison's Chainguard counts will disagree with this section (20 vs 7 for python) until the separate fix is merged. Implement this change after it merges, reusing `vulnKeys`, so both agree.
- [grype tolerance] → OA-15's tolerance is set from the manual cross-check in task 1. Record it here (CVEs grype reports that OSV lacks, and why) before writing the live test.

## Grype cross-check (task 1.1, 2026-10-11)

Run by hand with `docker run anchore/grype` (database current on 2026-10-11) on `cgr.dev/chainguard/python:latest`, `python:latest-dev`, `cosign:latest` and `kubectl:latest` (free, Go-built), compared with OSV.dev results for the same SBOM packages (`Wolfi` ecosystem, grouped as OA-2).

| Image | OSV open advisories (by vulnerability) | grype OS-package CVEs | grype OS CVEs not in OSV |
|---|---|---|---|
| python `latest` | 7 (20 records): 4 pending upstream fix, 3 detection | 2 | none |
| python `latest-dev` | 26 (64 records): 21 pending upstream fix, 5 detection | 2 | none |
| cosign `latest` | 9 (24 records): 5 pending, 2 fix not planned, 1 detection, and 4 `CGA-` advisories that name no CVE (counted under their own IDs) | 0 | none |
| kubectl `latest` | 1 (2 records): pending upstream fix | 0 | none |

- **Matches:** grype's two OS-package findings (CVE-2025-15367 and CVE-2026-12345 on `python-3.15`) are both in OSV's not-yet-assessed set. The figures match the specs (OA-2.1: 20 records → 7; OA-5.1: 26 = 21 + 5).
- **Only OSV reports:** the glibc, binutils, wget, uv and pip CVEs. Grype's Chainguard data lists fixes, so it can't see advisories still pending an upstream fix. This is the gap the feature fills.
- **Only grype reports:** Go modules (cosign: `GO-2026-6225`, `GO-2026-5932`; kubectl: `GO-2025-3547`). They are not OS packages and are out of OA-15's scope.
- **False-positive states:** none. The states seen were `pending_upstream_fix`, `detection` and `fix_not_planned` (and `fixed` in the earlier sample); no state needs filtering.
- **Advisories with no CVE or GHSA:** cosign's four `CGA-` advisories name neither, so they count under their own IDs (the known gap in decision 3).

**OA-15 tolerance:** zero. Every CVE grype reports for a package of type `apk` must be an open advisory or a recorded fix not yet installed; Go modules and other non-OS matches are ignored. On `python:latest-dev` today that holds (2 of 2).

**Live tests (task 6, run 2026-10-11):** `TestLive/open_advisories` found `python` `latest-dev`'s 26 vulnerabilities, 21 waiting on an upstream fix and 5 not yet assessed, equal to the spec's figures. `TestLive/open_advisories_grype` (run through a `grype.cmd` shim around `docker run anchore/grype`) found the same 2 OS-package CVEs as the manual run, both open advisories.

## Finding from the browser checks (task 7.2, 2026-10-11): fixed advisories shown as open, resolved by decision 9

The first implementation read the status from OSV.dev's copies. The browser checks showed that the Wolfi security database also listed some "waiting on an upstream fix" vulnerabilities as fixed at or below the installed version: all 4 on `python` `latest` (CVE-2026-8674, -86805, -89092, -95818, all glibc-2.44 at 2.44-r8) and 8 of the 26 on `latest-dev`. Grype reported none of them. It looked like a backport, but the parent session verified the real cause: OSV.dev serves out-of-date copies of some Chainguard advisories (evidence in Context above). The user decided to keep finding advisories on OSV.dev and read each one's current status from Chainguard's feed.

With the feed (measured 2026-10-11): `python` `latest` goes from 7 to **2** vulnerabilities (CVE-2025-15367 and CVE-2026-12345, both not yet assessed, the same two grype reports), and `latest-dev` from 26 to **21** (17 waiting on an upstream fix, 4 not yet assessed). 16 of `latest`'s 20 OSV records and 25 of `latest-dev`'s 64 are fixed in the installed version. The grype cross-check above predates this; its conclusions (no false-positive state, zero tolerance for OS packages) hold, and now `latest`'s open advisories equal grype's findings.

## Open Questions

None. The OA-15 tolerance is zero for OS packages, and no state is filtered (see the grype cross-check). The 404 handling is decision 9.
