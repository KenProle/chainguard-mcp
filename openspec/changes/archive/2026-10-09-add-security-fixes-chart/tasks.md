# Tasks

Test names start with the scenario ID they cover (e.g. `SF-1.1 …`). Run web commands from `web/` (Node on `PATH` as described in CLAUDE.md).

## 1. Fixes data (`web/src/fixes.ts`)

- [x] 1.1 Add `web/src/test/fixtures/pythonVulns.ts` with the `python` `latest` and `latest-dev` vulnerability summaries captured on 2026-10-09 (design decision 4). Verify: the fixture's `total_fixed` and `total_not_affected` (336 / 51 and 546 / 74) equal the sums of its packages' counts, checked by a unit test in `web/src/fixes.test.ts`.
- [x] 1.2 Implement `FixRow`, `fixesChart(packages)` and `fixesWithoutRecordsText(withoutRecords, total)` (design decision 1; SF-1, SF-3, SF-7). Verify with unit tests in `fixes.test.ts`: `SF-1.1` (`latest` → 12 rows, `withoutRecords` 9, no row for `bzip2`, `readline` or `zstd`), `SF-1.2` (`sqlite` has a row with 0 fixed and 4 never affected), `SF-1.3` (`latest-dev` → 25 rows), `SF-3.1` (the full `latest` order, `openssl-4.0` … `xz`), `SF-3.2` (`zlib` with one pending fix comes first, then `openssl-4.0`), `SF-7.1` ("9 more source packages have no records in the Wolfi security database." and the singular form), `SF-7.2` (all-zero input → no rows and "None of this image's 21 source packages have records in the Wolfi security database."), and no text from either function contains "no vulnerabilities", "secure" or "unaffected".

## 2. Stacked bars

- [x] 2.1 Extend `HorizontalBars` with optional `segments`, drawn as consecutive `rect`s on the shared scale, skipping zero-value segments; add `fixSegmentColor` to `web/src/styles.ts` (design decision 2; SF-2, SF-4). Verify with component tests in `HorizontalBars.test.tsx`: the existing VC-9.1, zero-width, LB-7.2, color and selectable tests pass unchanged; `SF-2.1` (segments 147 and 6 next to no larger bar: widths ≈ 96.08 and 3.92, the second starting where the first ends); `SF-2.2` (a 1-unit bar beside a 153-unit bar is 1/153 as wide); `SF-2.3` (segments 10, 2, 1 draw three `rect`s in that order with their color classes); `SF-4.2` (a zero-value segment draws no `rect`). Verify `CompareTab.test.tsx` and `PackagesTab.test.tsx` pass unchanged.

## 3. Security tab

- [x] 3.1 Build `FixesChart` in `web/src/components/` (heading, legend, stacked bars with "N fixed" labels, no-records line) and render it in `SecurityTab`'s success branch between the pending-fix box and the table (design decision 3; SF-1 to SF-8). Verify with page tests in a new `web/src/pages/SecurityTab.test.tsx` using the python fixture through `ImagePage`: `SF-1.1` (12 bars, by their labels), `SF-3.1` (bar labels in the SF-3.1 order), `SF-4.1` (with `zlib`'s pending fix: a `rose` segment on `zlib`'s bar and the text `zlib 1.3.2.1_rc20260917-r0: CVE-2026-0001`), `SF-5.1` ("147 fixed", "0 fixed" and the three legend entries), `SF-6.1` (the table has 21 body rows including `readline` and the chart's wrapper is `aria-hidden`), `SF-7.1` (the no-records line), `SF-7.2` (an all-zero summary shows the "None of…" line, no bars, and no text matching `/no vulnerabilities|is secure|vulnerability-free/i`), `SF-7.3` (the "doesn't list unfixed vulnerabilities" note), `SF-8.1` (only the tags and summary requests are made), `SF-8.2` (choosing `latest-dev` in the tag selector shows 25 bars starting `openssl-4.0`, `python-3.14`, `curl`), `SF-8.3` (`loki-fips` with a `not_public` tags response shows the subscription message and no chart).
- [x] 3.2 Update the README's Web UI section: the Security bullet describes the fixes chart, its three segments, that packages without records are counted, not charted, and that the data records fixes, not open vulnerabilities. Verify: the README describes the feature as it behaves.

## 4. Integration verification

- [x] 4.1 Run every check: `gofmt -l .`, `go vet ./...`, `go test ./...`, and in `web/` `npm run lint`, `npm run typecheck`, `npm test`, `npm run build`, plus `openspec validate --all --strict`. Verify: all pass, and `git diff --stat` shows no Go changes.
- [x] 4.2 Exercise the feature in the running app (build the UI, run `go run . -http 127.0.0.1:8080`) (SF-1 to SF-9). Verify in the browser: `python` `latest` shows bars whose counts match the table, in the SF-3 order, with the no-records line; switching to `latest-dev` redraws the chart; another image (e.g. `tomcat`) looks right; the Compare tab and the license breakdown look as before; `loki-fips` still shows the subscription message. For `SF-9.1`: at 375 px wide in dark mode nothing overflows and the console shows no Content Security Policy violations. If the browser pane can't take screenshots, ask the user to confirm the layout visually.
- [x] 4.3 Push and confirm CI passes on the implementing commit, as `config.yaml`'s archive guidance requires. Verify: all CI jobs succeed.

## Workflow follow-up

- Archive with `/opsx:archive` once 4.3 passes; this creates the living spec `openspec/specs/image-security-fixes/spec.md`. Check its Purpose and requirement IDs survived the merge.
- Mark idea #3 Done in `docs/visualization-ideas.md`, naming the capability and its `SF` prefix, and add `SF` to the capability prefixes listed in CLAUDE.md.
