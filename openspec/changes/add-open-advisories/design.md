# Design

## Context

See proposal.md for the motivation. Relevant current state:

- `Service.CheckVulnerabilities` (service.go) reads an image's amd64 packages from its SBOM (`Registry.Packages`) and runs `analyzeVulns` against the Wolfi security database (vulns.go). Both the MCP tool `check_vulnerabilities` and `GET /api/images/{name}/vulnerabilities` call it. That endpoint feeds the Security tab's lookup, totals, fixes chart and table.
- The upstream comparison added an OSV.dev client (osv.go): `OSV.Query` per package with pagination, a 24-hour cache keyed by `OSVPackage`, at most 10 requests in flight, and `QueryAll`, which fails the whole set on the first error. `OSVVuln` reads only `id` and `affected` (package, `fixed` events, `ecosystem_specific.urgency` and `components[].latest_event_status`). `osvEcosystemForDistro` maps a package URL's namespace to `Wolfi` or `Chainguard`.
- Chainguard's OSV records (sampled live on 2026-10-11) list the package under both the `Chainguard` and `Wolfi` ecosystems. Each carries the tracked IDs in `upstream` (for example `["CVE-2026-89092", "GHSA-gwqp-9qgw-c5pv"]`), not in `aliases`. Each component has `component_name`, `component_type` (`apk`, `python`, `rust-crate`, …) and `latest_event_status`. States seen: `fixed` (with a `fixed` range event), `fix_not_planned`, `pending_upstream_fix` and `detection`. No false-positive state appeared among 400 recently modified advisories.
- `compareUpstream` counts by OSV record ID, which overcounts Chainguard advisories. That is being fixed in a separate session (`fix-upstream-comparison`), which will add a canonical vulnerability key.
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
                 vulnerabilities[], not_covered[], id?, match? }
OpenVulnerability { id, status, aliases[], packages[] }
AdvisoryPackage  { name, version, status, advisories[], fixed_version?, components[]{name, kind} }
```

- `match` is the matching vulnerability when `id` is set. `vulnerabilities` is still returned, so the UI lookup and the section can share one cached query per tag.
- `components` lists only non-`apk` components, so it is empty for advisories on the package itself.
- `kind` is mapped to a readable label in Go: `rust-crate` → "Rust crate", `python` → "Python package", `go-module` → "Go module", `npm` → "npm package". Any other value passes through unchanged.

### 3. Grouping key

The key is the first `CVE-` entry in `upstream` or `aliases`, then the first entry of `upstream`, then the record's `id` (OA-2). Read `upstream` and `aliases` into `OSVVuln`. If the separate comparison fix has landed by then, reuse its key function rather than writing a second one. Otherwise write it here, and the comparison fix rebases onto it.

An ID lookup (OA-9) matches case-insensitively against the key, every record ID and every `upstream`/`aliases` entry of the grouped records.

### 4. Status mapping

The mapping is a new function `advisoryStatus(v OSVVuln, pkg OSVPackage)`, separate from the comparison's three-way `classify`, reading the `affected` entry with the package's own name and ecosystem:
- A non-empty `fixed` event means **fix available**. OSV returns only records that affect the installed version, so the fix is newer.
- Otherwise `latest_event_status` of the component whose `component_type` is `apk` (or, failing that, the first component): `pending_upstream_fix`, `fix_not_planned`, `detection` → not yet assessed, and anything else → confirmed.

A vulnerability's overall status is the first that applies in OA-4's order. That puts confirmed states above unassessed ones and the actionable state first.

### 5. Caching and `checked_at`

- `OSV` gets a per-ecosystem TTL: 1 hour for `Wolfi` and `Chainguard`, 24 hours otherwise. This also makes the comparison's Chainguard side fresher, which UC-10's "at most 24 hours" allows.
- `QueryAll` also returns the oldest fetch time among the results it used. That becomes `checked_at`, so a cached answer reports its real age (OA-12.1).
- The SBOM package list is read as `CheckVulnerabilities` does today.

### 6. Web UI

- A new `OpenAdvisories` component (`web/src/components/OpenAdvisories.tsx`), rendered by `SecurityTab` above Recorded fixes. Its query key is `['advisories', name, tag]`, independent of `vulnSummary` (OA-13).
- Status labels, tones and order go in `web/src/styles.ts`, which keeps Vite fast refresh working.
- Layout: the source line with the check time, then a row of `Stat` tiles for the total and each non-zero status (zeros as text in the summary line). Then a list with one item per vulnerability: an ID link, a status badge and the packages with versions, statuses, components and advisory IDs.
- No chart: the list is already the clearest form, and every count is text (OA-14).
- The lookup card also queries `/advisories?id=` and shows an "Open advisory" block next to the Wolfi result.
- Loading text gives the expected time: "Checking N packages on OSV.dev, usually a few seconds". 76 packages took 1.6 s uncached, so no progress job is needed.
- The Recorded fixes note gains a pointer to the section (SF-7).

### 7. Wording

- Source line: "Known open advisories per Chainguard's published advisories, via OSV.dev, checked <time>."
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
- [Hourly freshness costs more OSV requests] → At most one request per package per hour per server, 10 at a time. A typical image has 30–80 packages.
- [Differences from the comparison's numbers until its fix lands] → The comparison's Chainguard counts will disagree with this section (20 vs 7 for python) until the separate fix is merged. Implement this change after it, or reuse its key, so both agree.
- [grype tolerance] → OA-15's tolerance is set from the manual cross-check in task 1. Record it here (CVEs grype reports that OSV lacks, and why) before writing the live test.

## Open Questions

- The OA-15 tolerance and any states to filter, both settled by task 1's grype run before the UI work starts.
