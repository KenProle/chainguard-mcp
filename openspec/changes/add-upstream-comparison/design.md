# Design

## Context

See `proposal.md` for why and `specs/upstream-comparison/spec.md` for the requirements (UC-1 to UC-14).

Current state:
- `Service.FindAlternatives` (`alternatives.go`) maps a reference to a recommended Chainguard image. The Alternatives page (`web/src/pages/AlternativesPage.tsx`) shows that result; it has no living spec.
- `Registry` (`registry.go`) talks to `cgr.dev`: anonymous tokens from `BaseURL + /token`, then `Manifest`, `Blob` and `PlatformManifest`. `sbom.go` reads the SPDX attestation for a platform manifest and `parseSBOMPackages` turns it into `Package` values (name, version, origin, distro). `Service.ImageDetails` gives sizes and the build date for Chainguard images.
- The catalog's free-filter work added a CSP-safe `ProgressBar` (`web/src/components/ProgressBar.tsx`).

Findings from `/opsx:explore` on 2026-10-11 (the evidence behind several decisions):
- **Docker Hub:** anonymous tokens come from `https://auth.docker.io/token?service=registry.docker.io&scope=repository:<repo>:pull`, and the registry is `https://registry-1.docker.io`. Official images have an image index whose `attestation-manifest` entries (annotation `vnd.docker.reference.digest` naming the platform manifest) hold an SPDX layer (`in-toto.io/predicate-type: https://spdx.dev/Document`); `node:20`'s is 16.1 MB. `node:20-alpine`, `python:3.12-slim` and `nginx:latest` have them; `bitnami/redis` and `grafana/grafana` don't. Anonymous access is limited to 100 requests an hour per IP (`ratelimit-remaining` header).
- **The SBOM lists binary and source packages as separate entries**, linked by `GENERATED_FROM` (`libssl3` → `openssl`): `node:20` has 547 Debian entries, 393 installed binaries and 154 source-only entries. Purls carry the release (`os_name=debian&os_version=12`). The only binary from source `linux` is `linux-libc-dev`.
- **OSV.dev:** `POST /v1/query` with `{package:{name,ecosystem},version}` returns full vulnerability records, including per-ecosystem `affected` ranges with `fixed` events and Debian's `urgency`; `POST /v1/querybatch` returns only IDs (and gave a different total, 1,833, so it isn't used for counting). Debian records are keyed by source package. One query per package for `node:20` took 13.5 s at 10 concurrent requests. Chainguard records (`CGA-…`) carry a status such as `pending_upstream_fix` in `affected[].ecosystem_specific.components[].latest_event_status`.
- **Cross-check with grype 0.120.1** (database built 2026-10-11), scanning the real `node:20` image: 817 distinct CVEs in Debian packages (Debian 12.13). OSV by source package: 2,447 IDs, of which 1,614 were for `linux`; without `linux`, 833, including **all 817 of grype's** plus 16 more. Grype reports 0 for Chainguard's `node`, because its Wolfi data, like the app's security database, only lists fixes; OSV lists 22 advisories pending upstream fixes.
- **endoflife.date:** `GET https://endoflife.date/api/<product>.json` returns cycles with `eol` dates (Node.js 20: 2026-04-30); it tracks 483 products, and some names differ from image names (`golang` redirects to `go`).

**API changes:** a new endpoint and MCP tool (decision 6); no existing endpoint changes.

## Goals / Non-Goals

**Goals:**
- Numbers that agree with an independent scanner (UC-13) and are worded fairly (UC-6).
- One `Service` method behind both the web UI and MCP.
- Fast repeat comparisons and bounded load on the three services.

**Non-Goals:**
- Registries other than Docker Hub, language packages, and severity ratings (proposal, Out of Scope).
- Changing the Alternatives result itself or the Security tab.

## Decisions

### 1. Reuse `Registry` for Docker Hub
Give `Registry` a configurable token URL and service, and create a second instance for Docker Hub (`BaseURL` `https://registry-1.docker.io`, token URL `https://auth.docker.io/token`, service `registry.docker.io`, namespace from the reference: `library` for single names). `Manifest`, `Blob`, `PlatformManifest` and the SBOM attestation code then work unchanged; `SBOMDocument` learns Docker's attestation-index layout (attestation manifests referenced by `vnd.docker.reference.digest`) next to Chainguard's `.att` tag. `Registry.Ref` keeps producing `cgr.dev` references only for the Chainguard instance. A 429 maps to a new sentinel `ErrRateLimited` (UC-11.1). *Alternative:* a separate Docker Hub client. Rejected: it would duplicate manifest, blob and SBOM handling.

### 2. Upstream package list from the SBOM's relationships
Parse SPDX packages with `pkg:deb/…`, `pkg:apk/…` or Ubuntu purls; follow `GENERATED_FROM` to find each binary's source. The package count is the number of binaries (UC-3); the OSV lookups use the **distinct (source name, source version, ecosystem)** set, which is how Debian keys its advisories; entries with no `GENERATED_FROM` are looked up under their own name. Skip source `linux` (UC-4). The ecosystem comes from the purl: `os_name=debian&os_version=12` → `Debian:12`, Alpine `…?distro=alpine-3.20` → `Alpine:v3.20`, Ubuntu → `Ubuntu:<version>`. Count `pkg:npm`, `pkg:pypi` and other language purls only for the "not compared" note.

### 3. OSV client and classification, as pure functions
- `osvQuery(ctx, pkg)` calls `POST /v1/query` (following `next_page_token` until done), through a cache keyed by ecosystem, name and version with a 24-hour expiry (UC-10), with at most 10 requests in flight.
- `classify(vuln, pkg) FixStatus` is pure: take the `affected` entry for the package's own name **and ecosystem** (UC-5.3); `fixed` event present → fix available; Debian `urgency` "unimportant" or "end-of-life" → won't fix; a Chainguard component status of `pending_upstream_fix` or anything else → no fix yet.
- `summarize(results) Counts` is pure: deduplicates by vulnerability ID across the image, with fix available winning over the other two (UC-4.2, UC-5).
These are unit-tested against fixtures captured from the `node:20` and Chainguard `node` responses of 2026-10-11.

### 4. Chainguard side
Use the existing `Registry.Packages` for the recommended image's `latest` tag and architecture; query OSV with ecosystem `Wolfi` or `Chainguard` from each package's `distro` (UC-4), not only Wolfi, so subscription-only packages are covered when an image has them. Sizes and build date come from `ImageDetails`.

### 5. endoflife.date
A small mapping table next to `imageAliases` (`node` → `nodejs`, `postgres` → `postgresql`, `golang` → `go`, `python`, `nginx`, `redis`, …). The cycle is the tag's leading version (`20-alpine` → `20`, `3.12-slim` → `3.12`); tags without one (`latest`, `lts`) get no note (UC-8.2). Only a passed `eol` date produces a note. Cached 24 hours; a failure only omits the note (UC-11).

### 6. `Service.CompareUpstream` as a cached background job, polled by the UI
A first comparison takes about 20 seconds (16 MB SBOM plus about 550 OSV lookups), so the web UI needs progress (UC-9), which a single blocking request can't give.
- `Service.CompareUpstream(ctx, ref, arch) (*UpstreamComparison, error)` blocks until done; the MCP tool `compare_with_upstream` calls it (UC-12).
- The web API `GET /api/compare-upstream?image=<ref>&arch=<arch>` starts the same work as a background job keyed by reference and architecture (or joins a running one) and returns immediately: `{"status":"running","checked":210,"total":550}` while it runs, or `{"status":"done", ...comparison}` when finished, or the error. The UI polls about once a second and shows `ProgressBar`. Finished results stay cached (decision 3); a job left unpolled for 5 minutes is dropped.
- *Alternatives:* server-sent events (another streaming mechanism to test and secure, for little gain over polling), and client-driven batches like the free filter (would move counting logic into the UI, against the one-source-of-logic rule).

### 7. Output and UI
`UpstreamComparison` carries, per side: reference, digest, platform, build date, compressed size, OS package count, language packages not compared, and counts (total, fix available, no fix yet, won't fix), plus `sbom_published`, the end-of-life note, `source` ("known vulnerabilities per each distribution's published advisories, via OSV.dev") and `checked_at`. `web/src/api.ts` mirrors it. On the Alternatives page a "Compare with <ref>" section below the result shows a one-sentence summary (UC-6.1), a table with one column per image, the end-of-life note, and the source line; bars for the fix-status split are optional and `aria-hidden`, since the table holds the numbers (UC-14).

### 8. Config exceptions
Add Docker Hub (upstream images' SBOM attestations, sizes and build dates), OSV.dev and endoflife.date to the exceptions in `openspec/config.yaml`'s data-source bullet, next to the catalog history exception. CI checks the config still parses.

## Risks / Trade-offs

- **[OSV and grype may drift apart over time.]** → The UC-13 live test, run before releases, catches it.
- **[Each side is graded by its own maintainers.]** Debian's and Chainguard's triage differ (Chainguard files pending advisories; grype ignores them). → The source line says so, and the split shows fix status instead of one total.
- **[Docker Hub's rate limit is per IP.]** A shared server could hit 100 an hour. → Digest caching, at most 10 Docker Hub requests per comparison, and a clear rate-limit message.
- **[Load on OSV.dev.]** About 550 requests per uncached comparison. → 10 at a time, a 24-hour cache, and lookups by distinct source package.
- **[endoflife.date mapping mistakes.]** → Only confident mappings show a note; unknown names show none.
- **[Background jobs in a stateless server.]** → In-memory only, bounded, dropped when unpolled; a restart just recomputes.

## Migration Plan

Additive. Rollback is reverting the change; caches are in memory.

## Changes from review (2026-10-11)

- **Unchecked images (UC-15).** Review found that `busybox:latest` (an SBOM with no distribution packages) compared as "0 known vulnerabilities" against Chainguard's 20, and that RPM packages (Fedora, Amazon Linux, AlmaLinux publish SBOMs) were skipped silently, which would read the same way. Each side now has `packages_checked` and `unchecked_reason` (`no_sbom`, `no_os_packages`, `unsupported_distribution`); `parseUpstreamPackages` records unsupported distributions in `Unsupported` instead of failing or skipping, and any of them leaves the side unchecked, since a partial check would undercount. Unchecked sides carry no counts, and the UI and notes say that nothing checked is not the same as nothing found.
- **Known limitation of `classify`.** Any `fixed` event in the package's own advisory counts as fix available. An advisory fixed and later reintroduced without a fix would be miscounted for versions in the second range; handling it needs ecosystem-aware version comparison (Debian, Alpine, Ubuntu). It's rare, and the UC-13 cross-check would surface a material drift.
- **Polling in a background tab.** Review also found the page stopped polling while its tab was hidden (TanStack Query's default), so a user who switched tabs during the ~20-second first comparison would come back to a stalled progress bar, and after 5 minutes the server drops the unpolled job. The query now sets `refetchIntervalInBackground: true`; polling still stops when the comparison is done or the section is left.
