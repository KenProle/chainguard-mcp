# Tasks

Commands are for Git Bash on Windows as in CLAUDE.md. Test names start with the scenario ID they cover (e.g. `UC-5.3 …`). Design decisions are numbered as in `design.md`. The full `node:20` SBOM and OSV responses are about 30 MB, too large for fixtures: unit tests use small fixtures trimmed from the real 2026-10-11 data to exercise each rule, and the spec's exact figures are checked in the live run of task 8.2.

## 1. Project rule

- [ ] 1.1 Add Docker Hub, OSV.dev and endoflife.date to the data-source exceptions in `openspec/config.yaml` (design decision 8). Verify: `openspec validate --all --strict` passes and `openspec instructions proposal --change add-upstream-comparison --json` shows the new wording in its `context`.

## 2. Docker Hub

- [ ] 2.1 Make `Registry`'s token URL and service configurable, add a Docker Hub instance and `ErrRateLimited` for 429 responses (design decision 1; UC-2, UC-11). Verify: existing Go tests pass unchanged, and new tests against a fake Docker Hub in `fake_test.go` (token endpoint with `service=registry.docker.io`, an image index, a 429 response) show `library/` is added to single names and `UC-11.1` (429 → `ErrRateLimited`).
- [ ] 2.2 Read Docker's attestation-index SBOM layout in `SBOMDocument` (design decision 1; UC-2). Verify with fake-backend tests: `UC-2.1` (the SPDX layer of the amd64 attestation manifest is read), `UC-2.2` (an index without attestation manifests reports no SBOM, without error), and the Chainguard `.att` path still works (existing tests pass).

## 3. Package lists

- [ ] 3.1 Parse upstream packages from SPDX (design decision 2; UC-3, UC-4): binaries via `GENERATED_FROM`, lookups by distinct source package, ecosystem from the purl, `linux` skipped, language packages counted separately. Verify with unit tests on a fixture trimmed from the real `node:20` SBOM (keeping `libssl3`/`openssl`, `libc6`/`glibc`, `linux-libc-dev`/`linux` and a few npm entries): binary count, source set, `Debian:12` ecosystem, `linux` excluded, npm counted; plus an Alpine purl → `Alpine:v3.20`.
- [ ] 3.2 Pick the OSV ecosystem for Chainguard packages from their `distro` (`wolfi` → `Wolfi`, `chainguard` → `Chainguard`) (design decision 4). Verify with a unit test covering both.

## 4. OSV

- [ ] 4.1 Add the OSV client: `POST /v1/query` with pagination, a 24-hour cache by ecosystem, name and version, and at most 10 requests in flight (design decision 3; UC-10). Verify with tests against a fake OSV server: a paginated response is fully read; a second lookup within 24 hours makes no request (`UC-10.1`, client level); concurrency never exceeds 10.
- [ ] 4.2 Add `classify` and `summarize` (design decision 3; UC-4, UC-5). Verify with unit tests on records captured on 2026-10-11: `UC-5.3` (a Debian record fixed only for Debian 13 is no fix yet for Debian 12), a Debian 12 fix newer than the installed version → fix available, Debian "unimportant" → won't fix, a Chainguard `pending_upstream_fix` record → no fix yet (`UC-5.2` rule), and `UC-4.2` (one ID on two packages counts once; fix available wins).

## 5. End of life

- [ ] 5.1 Add the endoflife.date client and mapping table (design decision 5; UC-8). Verify with tests against a fake server: `UC-8.1` (`node:20` and `node:20-alpine` → Node.js 20, end of life 2026-04-30), `UC-8.2` (`node:latest` and an unknown image → no note), a cycle not yet past its end of life → no note, and a failed lookup → no note and no error.

## 6. Service, API and MCP

- [ ] 6.1 Add `Service.CompareUpstream` and the `UpstreamComparison` output (design decisions 4, 6 and 7; UC-1, UC-2, UC-6, UC-7, UC-10, UC-11). Verify with tests on the fake backends: `UC-1.1` (the pair and platform), `UC-1.2` (no alternative → error the UI can recognize), `UC-2.3` (`ghcr.io/…` → "only Docker Hub images" error), `UC-2.2` end to end, `UC-7.1` rule (sizes and build dates from both sides), `UC-11.2` (some OSV failures → error, no counts), `UC-10.1` (a repeat comparison makes no OSV or SBOM requests), and the `source` and `checked_at` fields are set (`UC-6`).
- [ ] 6.2 Add `GET /api/compare-upstream` as a background job polled by the UI (design decision 6), and add it to the API index. Verify in `web_test.go`: the first call returns `running` with `checked`/`total`, a later call returns `done` with the same comparison as `Service.CompareUpstream`, concurrent calls share one job, errors map to the existing codes plus a rate-limit code, and the endpoint index lists it.
- [ ] 6.3 Add the MCP tool `compare_with_upstream` (UC-12). Verify in `tools_test.go`: `UC-12.1` (its counts equal the web API's for the same reference on the fake backends), and its description states the source and that counts are known vulnerabilities, not a guarantee.

## 7. Web UI

- [ ] 7.1 Mirror the output in `web/src/api.ts` and add the comparison section to the Alternatives page (design decision 7; UC-6, UC-7, UC-8, UC-14). Verify with page tests: `UC-6.1` (summary sentence with fix-available and package counts), `UC-6.2` (zero reads "0 known vulnerabilities" with source and date, and the page never contains "vulnerability-free" or "secure"), the end-of-life note, the "no SBOM published" state, and the amd64/arm64 choice.
- [ ] 7.2 Poll the job and show `ProgressBar` (UC-9). Verify with page tests: `UC-9.1` (running responses show "N of M packages checked", then the comparison), and polling stops when done or when the page is left.
- [ ] 7.3 Show errors with a retry (UC-11). Verify with page tests: `UC-11.1` rate-limit message, `UC-11.2` OSV failure with retry and no counts, and `UC-2.3` other-registry message.
- [ ] 7.4 Document the comparison in the README (what is compared, sources, wording, Docker Hub only, OS packages only) and in CLAUDE.md's data sources (Docker Hub, OSV.dev and endoflife.date, the kernel exclusion and binary/source rule). Verify: both describe the feature as built.

## 8. Accuracy

- [ ] 8.1 Add the live test for UC-13 to `live_test.go`: skipped unless `CHAINGUARD_LIVE=1` and when `grype` isn't on `PATH`; scans `docker.io/library/node:20` with grype, excludes the kernel, and checks at least 95% of its OS-package CVEs are in the comparison (`UC-13.1`). Verify: with grype on `PATH`, `CHAINGUARD_LIVE=1 go test -run 'TestLive/upstream' -v ./...` passes; without it, the test reports skipped.
- [ ] 8.2 Run the comparison live for `node:20` and record the figures in this task (`UC-3.1`, `UC-4.1`, `UC-5.1`, `UC-5.2`, `UC-6.1`, `UC-7.1`), comparing with the spec's 2026-10-11 values; differences from daily drift are expected and noted, rule differences are bugs. Verify: the figures are recorded here and any difference is explained.

## 9. Integration verification

- [ ] 9.1 Run every check: `gofmt -l .` (only changed files matter on Windows), `go vet ./...`, `go test ./...`, `npm --prefix web run lint`, `npm --prefix web run typecheck`, `npm --prefix web test`, `npm --prefix web run build` and `openspec validate --all --strict`. Verify: all pass.
- [ ] 9.2 Exercise the feature in the running app (build the UI, run `go run . -http 127.0.0.1:8080`). Verify in the browser: `node:20` shows progress, then the comparison with the end-of-life note; a repeat is instant; `bitnami/redis` shows "no SBOM published"; `ghcr.io/…` shows the Docker Hub message; and `UC-14.1` (375 px wide in dark mode: no horizontal overflow, readable counts, no Content Security Policy violations).
- [ ] 9.3 Ask the user before committing and pushing to `main`, then confirm CI passes on the implementing commit. Verify: all CI jobs succeed.

## Workflow follow-up

- Review the change on the stronger model before archiving, as with the last two changes.
- Archive with `/opsx:archive` once CI passes; this creates the living spec `openspec/specs/upstream-comparison/spec.md`.
- Update idea 5's status in `docs/visualization-ideas.md`; idea 7 can reuse the OSV client from this change.
