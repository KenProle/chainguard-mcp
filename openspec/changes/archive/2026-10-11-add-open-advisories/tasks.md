# Tasks

Commands are for Git Bash on Windows as in CLAUDE.md. Test names start with the scenario ID they cover (e.g. `OA-2.2 …`). Design decisions are numbered as in `design.md`. Unit tests use small Chainguard-shaped OSV records trimmed from the 2026-10-11 data; the spec's exact live figures are checked in task 6.2. Start after the separate change `fix-upstream-cve-counting` has merged, and reuse its `vulnKeys` (design decision 3).

## 1. Cross-check and project rules

- [x] 1.1 Run grype by hand (Windows binary, or `docker run anchore/grype`) on `cgr.dev/chainguard/python:latest`, `python:latest-dev` and one free Go-built image (pick one with `list_images` `free_only`), and compare its OS-package CVEs with the OSV results for the same packages. Record in `design.md`: matches, CVEs only one side reports and why, whether any false-positive state is published, and the tolerance for OA-15. Verify: `design.md` has the comparison and the Open Questions entry is resolved. If a state must be filtered or the gap is large, stop and revise the specs with the user before task 2.
- [x] 1.2 Widen OSV.dev's entry in `openspec/config.yaml` to cover open advisories on Chainguard images. Verify: `openspec validate --all --strict` passes and `openspec instructions proposal --change add-open-advisories --json` shows the new wording in its `context`.
- [x] 1.3 Update idea 7 in `docs/visualization-ideas.md`: its status (in progress, `add-open-advisories`, prefix `OA`) and the corrections from exploration (python `latest` has 2 open CVEs; OSV.dev's 20 records are 7 CVEs, 5 already fixed per the feed; `all.json` is an ID index). Also add `OA` (`image-open-advisories`) to CLAUDE.md's list of prefixes. Verify: both files read correctly.

## 2. OSV client

- [x] 2.1 Read the `fixed` version and `components[].component_name`/`component_type` into `OSVVuln` (`upstream` and `aliases` come with the comparison fix) (design decisions 2 and 3). Verify: a unit test decodes a captured Chainguard record (`CGA-2727-9j94-3x3p`) and a Debian record, and the existing UC tests still pass.
- [x] 2.2 Give `OSV` a per-ecosystem cache TTL (1 hour for `Wolfi` and `Chainguard`, 24 hours otherwise), and make `QueryAll` report the oldest fetch time it used (design decision 5; OA-12). Verify with fake-OSV tests using a controllable clock: `OA-12.1` (a repeat within an hour makes no request and keeps the first time), `OA-12.2` (after an hour it queries again), and a Debian package still cached at 23 hours (UC-10).

## 3. Advisory analysis

- [x] 3.1 Add `advisoryStatus` and the report builder, grouping by the comparison fix's `vulnKeys`, from `[]Package` plus OSV results: one entry per vulnerability with its packages, statuses, advisory IDs, fixed versions and non-`apk` components with readable kinds; totals; not-covered packages; ordering (design decisions 2–4; OA-1, OA-2, OA-3, OA-4, OA-5, OA-7). Verify with unit tests: `OA-1.3`, `OA-2.1` (python `latest-dev`'s 64 OSV records, 25 fixed → 21), `OA-2.2`, `OA-2.3`, `OA-2.4`, `OA-3.1`, `OA-3.2`, `OA-3.3`, `OA-4.1`, `OA-4.2`, `OA-5.2`, `OA-7.1` and `OA-7.2` (rule level), and an unknown state → confirmed.
- [x] 3.2 Add ID matching for lookups (design decision 3; OA-9). Verify with unit tests: `OA-9.1`, `OA-9.2` (match by advisory ID), a GHSA match, case-insensitive input, and `OA-9.3` (no match → `match` empty, wording without "unaffected").

## 4. Service, API and MCP

- [x] 4.1 Add a test helper that wires `fakeOSV` with Chainguard-shaped records for the fake `python` image, keeping `newFakeBackend`'s OSV unreachable (design decision 8). Verify: the existing Go suite still passes.
- [x] 4.2 Add `Service.OpenAdvisories` and `GET /api/images/{name}/advisories` with an `apiRoutes` entry and a README API-table row (design decision 1; OA-1, OA-11, OA-13). Verify in `web_test.go`: `OA-1.1` (rule level on the fake image), `OA-11.1`, `OA-11.2`, 502 `source_failed` when the fake OSV fails, `TestAPIIndex` passes, and `/vulnerabilities` makes no OSV request.
- [x] 4.3 Add `Service.SecurityReport`, switch `check_vulnerabilities` to it, and update its description, `vulnNote`, the README's tool table and Wolfi paragraph, and CLAUDE.md's vulnerability data-source notes (design decisions 1 and 7; OA-8, OA-10). Verify in `tools_test.go`: `OA-10.1` (fixes plus advisory totals on the fake image), `OA-10.2`, `OA-10.3` (OSV unreachable → fixes still returned with `open_advisories_error`), an `id` call returns both the Wolfi matches and the open-advisory match, and the SBOM is fetched once per call.

## 5. Web UI

- [x] 5.1 Mirror the new types in `web/src/api.ts`, and add status labels, tones and order to `web/src/styles.ts` (design decisions 2 and 6). Verify: `npm --prefix web run typecheck` passes.
- [x] 5.2 Add the `OpenAdvisories` component and render it above Recorded fixes in `SecurityTab`, with its own query, loading text with the expected time, an error with a retry, and the subscription message (design decisions 6 and 7; OA-6, OA-7, OA-8, OA-13). Verify with page tests: `OA-6.1`, `OA-6.2`, `OA-7.1`, `OA-7.2`, `OA-8.1`, `OA-8.2` (zero total; the tab never contains "vulnerability-free", "no vulnerabilities" or "secure"), `OA-8.3`, `OA-13.1`, `OA-13.2` and `OA-13.3`.
- [x] 5.3 Show the open-advisory answer in the lookup card and point the Recorded fixes note to the new section (OA-9, SF-7). Verify with page tests: `OA-9.1` and `OA-9.3` in the UI, `SF-7.3`, and the existing SF tests still pass.

## 6. Live checks

- [x] 6.1 Add the OA-15 live test (skipped unless `CHAINGUARD_LIVE=1`, and skipped without grype) using the tolerance from task 1.1. Verify: `CHAINGUARD_LIVE=1 go test -run 'TestLive' -v ./...` runs it and it passes, and it skips cleanly without grype.
- [x] 6.2 Add a live subtest that asserts stable properties for `python` `latest-dev`: every package is looked up in `Wolfi`, the status counts add up to the total, every vulnerability has at least one package, and `checked_at` is recent. Compare the exact figures against the spec's (26 = 21 + 5 on 2026-10-11) and note today's figures in the change. Verify: the live subtest passes.

## 8. Chainguard's advisory feed (added 2026-10-11, decision 9)

- [x] 8.1 Add `AdvisoryFeed` (`advisory_feed.go`): `Fetch` and `FetchAll` for `<URL>/chainguard/v3/osv/<id>.json`, decoding an `OSVVuln`, cached 1 hour per ID (a 404 too), at most 10 requests in flight, the oldest fetch time returned, a 404 reported as `ErrNotFound`, any other failure a `*SourceError` with source "Chainguard advisories"; and `Service.Feed`, unreachable in `newFakeBackend` like OSV. Verify with unit tests against a fake feed and a controllable clock: `OA-12.1`/`OA-12.2` for the feed, `OA-12.3` (a record shared between two tags is read once), `OA-12.4` (never more than 10 in flight), `OA-17.3` (404), `OA-17.4` (5xx and an invalid body fail the whole set).
- [x] 8.2 In `openAdvisories`, read each distinct `CGA-` advisory's current record after the OSV step, use it for status, fixed version and components, leave out advisories fixed at or below the installed version (one with no entry for the package stays open, as confirmed or with OSV.dev's status), list 404s in `status_from_osv` with a note, and make `checked_at` the older of the two sources (design decisions 4, 5 and 9; OA-16, OA-17). Verify with unit tests on a captured feed fixture (`testdata/`) of the python `latest-dev` advisories next to the stale OSV copies: `OA-16.1` (`CGA-2727-9j94-3x3p`, `glibc-2.44` `2.44-r8`), `OA-16.2`, `OA-16.3`, `OA-16.4`, `OA-17.1`, `OA-17.2`, and the figures of `OA-2.1`, `OA-3.1`, `OA-5.1`, `OA-6.2` and `OA-10.1`.
- [x] 8.3 Wire a fake feed into `newFakeAdvisoryService` (with a stale OSV copy, a fixed advisory and a 404 case) and update the API and MCP tests. Verify: `OA-9.4` (a CVE whose OSV copy is stale is not an open advisory), `OA-13.4` and `OA-17.4` (502 `source_failed` with source "Chainguard advisories"), `OA-10.3` also for a feed failure, `OA-17.3` through the API, and the SBOM is still read once per call.
- [x] 8.4 Update the web types, wording and tests: `status_from_osv` and the note in `web/src/api.ts`, the source line (OA-8.1) from the API, and the fixtures and page tests for the new figures (`OA-6.1`, `OA-6.2`, `OA-9.1`, `OA-9.3`, `OA-13.3`, and the OA-8.2 zero-total wording). Verify: lint, typecheck and `npm --prefix web test` pass.
- [x] 8.5 Update the live tests: the OA-15 grype test and the stable-properties subtest still pass with the feed, and a new live subtest asserts that `CGA-2727-9j94-3x3p` is not an open advisory for `python` `latest` while the feed has it fixed in the installed version (skipped with a message if the feed's record changed). Verify: `CHAINGUARD_LIVE=1 go test -run 'TestLive' -v ./...` passes.
- [x] 8.6 Update the README (data table and Wolfi paragraph) and CLAUDE.md's open-advisories note for the feed. Verify: both read correctly, and `TestAPIIndex` still passes.

## 7. Verification

- [x] 7.1 Run `gofmt -l . && go vet ./...`, `go test ./...`, `npm --prefix web run lint && npm --prefix web run typecheck && npm --prefix web test`, and `openspec validate --all --strict`. Verify: all pass.
- [x] 7.2 Build the UI and the server, run with `-http 127.0.0.1:8080`, and exercise the feature in the browser: python `latest` and `latest-dev` (counts, rows, `uv`'s Rust crate, check time), switching tag, looking up `CVE-2026-89092` (no longer an open advisory, OA-9.4) and a GHSA ID of an advisory that is still open, `loki-fips`, and 375 px in dark mode with no CSP violations in the console (`OA-14.1`). Call `check_vulnerabilities` for `python` through the built binary's MCP endpoint. Verify: screenshots and the MCP output match the specs' new figures (2 for `latest`, 21 for `latest-dev`). Redo after section 8.

## Workflow follow-up

- Review the change before archiving, and archive only after CI passes on the implementing commit.
- When archived, set idea 7's status in `docs/visualization-ideas.md` to Done.
