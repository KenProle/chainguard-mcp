# Tasks

Commands are for Git Bash on Windows as in CLAUDE.md. Test names start with the scenario ID they cover. Figures come from Wolfi's x86_64 index and dfc's mapping file of 2026-10-19; see design.md for the approach.

## 1. Check the repository first

- [ ] 1.1 Check which repository the free `-dev` images install from. Run `docker run --rm --entrypoint cat cgr.dev/chainguard/python:latest-dev /etc/apk/repositories`, and the same for `node:latest-dev`. Then run `apk add --no-cache build-base postgresql-18-dev` in one of them. Verify: the repository is Wolfi's (`packages.wolfi.dev` or the identical `apk.cgr.dev/chainguard`), and the install succeeds. If it's another repository, stop and revisit design decision 2 with the user before going on.

## 2. dfc snapshot and strict reader

- [ ] 2.1 Add `mappings/` with dfc's `builtin-mappings.yaml` (verbatim), `dfc-source.json` (commit and date) and `LICENSE-dfc`, all at one upstream commit. Embed them with `//go:embed mappings` (PM-15). Verify: `go build` succeeds and the files match upstream at that commit.
- [ ] 2.2 Write the strict reader (design decision 6), returning the Debian mappings, with `[]` read as an empty list (PM-15, PM-4.5). Verify with unit tests:
  - `PM-15.2`: the embedded file reads, and has a non-empty Debian section with `build-essential` → `build-base`
  - error cases (a tab, a quoted value, a 6-space line, an unknown top-level key) each name their line number
- [ ] 2.3 Add the `-refresh-mappings <file>` run mode (design decision 7, PM-16). Verify:
  - `PM-16.1` and `PM-16.2` tests against an `httptest` server standing in for GitHub: a new commit is written with added and removed keys printed; a malformed file is refused, and the existing file is byte-for-byte unchanged
  - a manual `go run . -refresh-mappings mappings/dfc-builtin-mappings.yaml` reports no changes

## 3. Package index

- [ ] 3.1 Write `PackageIndex` (design decisions 1 and 2): streamed parse to newest-build `pkgInfo` plus `provides`, a 6-hour TTL and a stale copy on failure (PM-10, PM-11). Add the trimmed fixture `testdata/apkindex-2026-10-19.tar.gz` (generated with `UPDATE_FIXTURES=1`), serve it from `fakeChainguard`, and wire `newFakeService`. Verify with tests:
  - `PM-10.1`: one download for two calls
  - `PM-10.2`: a failed refresh serves the old copy and its fetch time
  - `PM-11.1`: no copy gives a `*SourceError` naming the Wolfi package index
  - parsing the fixture gives `libssl3` with origin `openssl`, and `pc:libpq` → the `postgresql-*-dev` streams

## 4. Mapping

- [ ] 4.1 Write `Service.MapPackages` with the input, output and validation types (design decision 5), and the exact and dfc steps, including the `-dev` rule (PM-1 to PM-5, PM-9). Verify with unit tests against the fixture:
  - `PM-1.1`, `PM-1.2`
  - `PM-2.1`
  - `PM-3.1`
  - `PM-4.1` to `PM-4.5`
  - `PM-5.1`, `PM-5.2`
  - `PM-9.1` to `PM-9.3`
- [ ] 4.2 Add the rules and candidate choice (design decisions 3 and 4) (PM-6 to PM-8). Verify with unit tests:
  - `PM-6.1` to `PM-6.4`
  - `PM-7.1` to `PM-7.3`
  - `PM-8.1`, `PM-3.2`
  - `PM-2.2`: every package in every result for the 59-name prototype list is in the fixture
- [ ] 4.3 Document the data source in CLAUDE.md, in the style of the existing bullets: the index facts, the dfc snapshot and refresh, the `-dev` rule and the stream choice. Add the two sources to `openspec/config.yaml`'s context (design decision 9). Verify: both files mention `packages.wolfi.dev`; `openspec validate --all --strict` passes.

## 5. MCP tool and prompt

- [ ] 5.1 Register `map_packages` in `tools.go`, read-only, with the description PM-12 requires. Change `migratePrompt` step 4 to call it and to list `candidates` and `unknown` results for checking (PM-12, PM-14). Verify with tool tests through `mcpClient`: `PM-12.1`, `PM-12.2` (via `callTool`), and `PM-14.1` (the prompt text names `map_packages`). The existing prompt test still passes.
- [ ] 5.2 Add `map_packages` to the README's Tools section and an example question. Verify: both are present.

## 6. API endpoint

- [ ] 6.1 Add the `/api/packages/map` entry to `apiRoutes`, with `names` required and an example the fake backend answers (PM-13). Add its row to the README's JSON API table. Verify with `web_test.go` tests: `PM-13.1` to `PM-13.4`; `TestAPIIndex` and the WA-2 and WA-4 tests still pass.
- [ ] 6.2 Add the README's `-refresh-mappings` line under Run and a "Third-party data" note crediting dfc and its Apache-2.0 license, with no implied affiliation (PM-15). Verify: the README has both, and the license file is linked.

## 7. Live check and verification

- [ ] 7.1 Add the `TestLive/package_mapping` subtest. It asserts `build-essential` → `build-base` and that `libpq-dev` gives a `postgresql-*-dev`, and prints the drift report (PM-17). Verify: `CHAINGUARD_LIVE=1 go test -run 'TestLive/package_mapping' -v ./...` passes and prints the snapshot's age, the missing targets and the corrected mappings.
- [ ] 7.2 Run `gofmt -l . && go vet ./...`, `go test ./...`, the web lint, typecheck and tests (unchanged UI, as a guard), and `openspec validate --all --strict`. Verify: all pass.
- [ ] 7.3 In the running app (`-http 127.0.0.1:8080`), request `/api/packages/map?names=curl,build-essential,libssl-dev,libpq-dev,python3,dnsutils` and check `/api/` lists the endpoint. Over stdio, from Claude Code, get the `migrate_dockerfile` prompt for a Dockerfile with `apt-get install -y build-essential libpq-dev`. Verify: the JSON matches the spec's examples against the live index, and the assistant calls `map_packages` and lists `libpq-dev` as needing a check.
- [ ] 7.4 Measure the improvement. Migrate three real Dockerfiles that install apt packages (one Python, one Node, one with build dependencies) with the prompt, once with `map_packages` hidden and once with it available. Record in the change folder (`measurement.md`) how many package names were wrong each way. Verify: the file exists with both counts per Dockerfile.

## Workflow follow-up

- Review the change before archiving, and confirm CI passed on the implementing commit.
- Archive with `/opsx:archive`, which merges `package-mapping` into the living specs.
- Mark Gap 1 done in any backlog note, and pick up Gap 3 (Tier 1 review checks) next.
