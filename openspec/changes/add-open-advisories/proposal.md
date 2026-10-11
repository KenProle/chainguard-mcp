# Proposal

## Why

The Security tab and `check_vulnerabilities` can only show *recorded fixes*: the Wolfi security database lists what each package version fixed, never what is still open. Users want to know what is still open. Chainguard publishes advisories for its own packages, in OSV format, and the OSV.dev client built for the upstream comparison can already read them. Measured live on 2026-10-11 for `python` (amd64):

- `latest`: 29 packages, 20 OSV records, **7 distinct CVEs**. 4 are waiting on an upstream fix (glibc's CVE-2026-89092, -8674, -86805 and -95818) and 3 are detected but not yet assessed by Chainguard (glibc's CVE-2026-97399, and python's CVE-2026-12345 and CVE-2025-15367). None has a released fix.
- `latest-dev`: 76 packages, 64 records, **26 distinct CVEs** (21 waiting on upstream, 5 not yet assessed), 14 of them in `binutils`. Advisories also name software built into a package: `uv`'s Rust crate `rsa` (CVE-2026-21895) and the `setuptools` copy bundled inside `py3.15-pip-base` (CVE-2025-47273).
- Looking up all 76 packages with 10 requests at a time took 1.6 s, with nothing cached.

## What Changes

- An **Open advisories** section at the top of the Security tab: the known open advisories for the image's installed packages, per Chainguard's published advisories via OSV.dev, with when they were checked. It shows one total, split by status (fix available, waiting on an upstream fix, confirmed, fix not planned, not yet assessed), and one row per CVE with the affected packages, any compiled-in component, and a link to the CVE.
- The **vulnerability lookup** on the Security tab, and `check_vulnerabilities` with an `id`, also report whether that vulnerability is an open advisory for the image.
- **`check_vulnerabilities` without an `id`** also returns the open advisories. Its description and note stop saying the tool only knows about fixes.
- A new web API endpoint, `GET /api/images/{name}/advisories`.
- The existing fixes chart and table stay as they are. The Security tab's note that the Wolfi data "doesn't list unfixed vulnerabilities" now applies to the Recorded fixes section, not the whole tab.
- `openspec/config.yaml` lists OSV.dev as a source for open advisories on Chainguard images, not only for the upstream comparison.

## Capabilities

### New Capabilities
- `image-open-advisories`: known open advisories for a Chainguard image's installed packages: how they are counted, their statuses and wording, in the web UI and through MCP.

### Modified Capabilities
- `image-security-fixes`: SF-7 is reworded so that its "doesn't list unfixed vulnerabilities" note covers the Recorded fixes section, now that the tab also shows open advisories. The chart itself (SF-1 to SF-6, SF-8, SF-9) is unchanged.

## Decisions

Made with the user in `/opsx:explore` on 2026-10-11:
- **A separate section on the Security tab; the fixes chart stays as it is.** *Rejected:* a merged chart of fixed, never-affected and open advisories, which would mix two sources of different ages (Wolfi database cached for 6 hours vs. Chainguard advisories updated hourly) and different units (fixes per package vs. CVEs per image).
- **Folded into `check_vulnerabilities`**, including looking up a single ID. *Rejected:* a separate MCP tool, which assistants would have to know to call. The existing description says the tool "is not a full scan", which sends assistants to grype rather than to another tool.
- **One total, split by status, with unassessed detections included.** *Rejected:* counting only confirmed advisories in the headline, which would hide some from the number people read first.

Proposed in exploration and kept here (the user didn't object):
- **Count each CVE once per image.** Chainguard files one advisory per package per CVE, sometimes two, so python `latest`'s 20 records are 7 CVEs.
- **Query OSV.dev per package**, reusing the comparison's client, and cache Chainguard advisories for 1 hour. *Rejected:* Chainguard's `advisories.cgr.dev/chainguard/v3/osv/all.json`, which turned out to be only an index of 1.2 million advisory IDs (102 MB, served without compression), so per-package queries would still be needed.
- **No severity in the first version.** About half the records carry a CVSS vector (v3 or v4) and none carry a score. Each CVE links to its record instead.

## Out of Scope

- Severity ratings and sorting by severity.
- Tags and architectures other than those the Security tab already uses (the selected tag, amd64).
- Changes to the upstream comparison. The bug that made it count advisory IDs instead of CVEs is fixed in the separate change `fix-upstream-cve-counting`, whose `vulnKeys` rule this change reuses.
- Packages outside the Wolfi and Chainguard ecosystems (none in free-tier images today); they are listed as not covered.
- Subscription-only images, which keep the existing subscription message.
- Bundling a scanner such as grype. It stays a manual cross-check during implementation.

## Impact

- **Go:** a new `Service` method for open advisories, a `CheckVulnerabilities` that also returns them for MCP, an `upstream` field read from OSV records, a status mapping for Chainguard advisories, a 1-hour cache for Wolfi and Chainguard lookups (within UC-10's "at most 24 hours"), a new `apiRoutes` entry and a README row.
- **Web UI:** an Open advisories section, an open-advisory answer in the lookup, and `web/src/api.ts` types.
- **External load:** one OSV.dev request per installed package on a cache miss (29 for python `latest`, 76 for `latest-dev`), at most 10 at a time.
- **Planning:** OSV.dev's entry in `openspec/config.yaml` is widened, and `docs/visualization-ideas.md` idea 7 gets its status and corrections.
- **Dependencies:** none.
