# Proposal

## Why

The Alternatives page tells a user which Chainguard image replaces the one they run, but not what the switch is worth. The most useful answer is about known vulnerabilities, and it is large. Measured on 2026-10-11 for Docker Hub's `node:20` (Debian 12, amd64, last built 2026-04-22; Node.js 20 reached end of life on 2026-04-30) against `cgr.dev/chainguard/node:latest`:

- `node:20` installs 393 Debian packages; Chainguard's `node` installs 28.
- An independent scan of `node:20` with grype found 817 known CVEs in its OS packages, about half of them with a fix that Debian has released but the image doesn't include; OSV.dev, queried by package, found all 817 plus 16 more, once Linux kernel advisories (which don't apply inside a container) are excluded.
- For Chainguard's `node`, OSV.dev lists 22 advisories, all waiting on an upstream fix, and none with a fix available.

## What Changes

- On the Alternatives page, after the recommended image, a **comparison with the upstream image the user entered** (as typed: `node:20` is compared as `node:20`): OS package counts, download sizes, build dates, and known vulnerabilities on each side, split into fix available, no fix yet, and (where the distribution says so) won't fix, plus an end-of-life note when the upstream version is out of support.
- The same comparison as a new **MCP tool**, `compare_with_upstream`.
- **Three new data sources**, approved by the user: Docker Hub's registry (the upstream image's SBOM attestation, size and build date), OSV.dev (known vulnerabilities by package and version, for Debian, Alpine and Ubuntu as well as Wolfi and Chainguard), and endoflife.date (support end dates). The exceptions are recorded in `openspec/config.yaml`.

## Capabilities

### New Capabilities
- `upstream-comparison`: comparing an upstream container image with its Chainguard replacement by size, packages, known vulnerabilities and support status, in the web UI and through MCP.

### Modified Capabilities
None. The Alternatives page predates the living specs and has no spec of its own; this change adds behavior below its existing result without changing it.

## Decisions

Made with the user in `/opsx:explore` on 2026-10-10 and 2026-10-11:
- **Compare known vulnerabilities, not just size** (extending idea 5 in `docs/visualization-ideas.md`). *Rejected:* size only (misses the main value).
- **Compare the image exactly as the user typed it**, stale or not, and show its build date and an end-of-life note when it applies. *Rejected:* substituting a "fair" maintained tag (the app would have to guess, and it would no longer describe the user's situation).
- **Docker Hub, OSV.dev and endoflife.date** may be used as data sources. *Rejected:* bundling a scanner such as grype (a large dependency with its own database of several hundred MB) and showing no end-of-life note (staleness without the reason).
- **Lead with fixes that exist but aren't in the image, and with package counts**, not one combined total, and say that each side is graded by its own maintainers' advisories. *Rejected:* a single headline number (dominated by issues no distribution has fixed, or chooses not to fix).
- **Grype as a cross-check, not a feature**: run by hand while exploring (results above and in `design.md`) and kept as an optional live test.

## Out of Scope

- Registries other than Docker Hub (`ghcr.io`, `quay.io`, …), and upstream images without an SBOM attestation, which get a clear "no SBOM published" state instead of a guess (on 2026-10-11, `bitnami/redis` and `grafana/grafana` had none).
- Language packages (npm, PyPI, …) in either image; their count is shown but they aren't compared.
- Severity ratings: Debian leaves most urgencies unassigned, so the first version splits only by fix status.
- Open vulnerabilities on Chainguard images outside this comparison, such as on the Security tab (idea 7).
- Comparing against a different upstream tag than the one entered.

## Impact

- **Go:** a Docker Hub registry client (reusing the existing registry code with Docker Hub's token endpoint), an OSV client, an endoflife.date client, a `Service` method with caching, a web API endpoint and an MCP tool.
- **Web UI:** a comparison section on the Alternatives page, with progress while it loads (a first comparison takes about 20 seconds) and a text equivalent for any chart.
- **Planning:** three exceptions added to `openspec/config.yaml`.
- **External load per uncached comparison:** about 5 requests to Docker Hub (anonymous limit: 100 an hour per IP), one request per OS package to OSV.dev (about 550 for `node:20`), and one to endoflife.date.
- **Dependencies:** none.
