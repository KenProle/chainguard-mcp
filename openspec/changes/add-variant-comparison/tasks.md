# Tasks

Test names start with the scenario ID they cover (e.g. `VC-6.1 …`). Run web commands from `web/`.

## 1. Decimal sizes

- [x] 1.1 Switch `formatBytes` in `web/src/styles.ts` to decimal units (1 MB = 1,000,000 bytes) with one decimal place (design decision 7; supports VC-5, VC-6). Add `web/src/styles.test.ts` covering bytes, KB, MB and GB (26,882,525 bytes → "26.9 MB"), and update the existing Overview test in `pages.test.tsx` from "30 MB" to the decimal value. Verify: `npm test` passes.

## 2. Comparison logic (`web/src/compare.ts`)

- [x] 2.1 Implement `defaultPair(tags)` (VC-1, VC-3). Verify with unit tests in `web/src/compare.test.ts`: `VC-1.1` (`latest` + `latest-dev` → that pair), `VC-1.2` (with only `next` / `next-dev` → that pair), `VC-3.1` (`static`'s four tags → `null`), `VC-3.2` (a single tag → `null`).
- [x] 2.2 Implement `diffPackages(a, b)` matching by package name (VC-7). Verify with unit tests: `VC-7.1` (python fixture: 47 only in `latest-dev`, including `gcc`, `bash`, `git` and `make`; none only in `latest`; none changed), `VC-7.2` (empty groups are empty), and a version-changed case.
- [x] 2.3 Implement `sizeDifference(a, b)` (VC-6). Verify with unit tests: `VC-6.1` (26,882,525 vs 272,631,719 bytes → "+245.7 MB", "10.1×", second is larger), `VC-6.2` (sizes within 1% → nearly equal), and reversed order reporting the first as larger.

## 3. Test fixtures

- [x] 3.1 Extend `mockApi` in `web/src/test/render.tsx` to optionally match query parameters, so `tag=latest` and `tag=latest-dev` can return different data (design decision 8). Verify: the existing page tests pass unchanged, and a new test shows two responses for one path selected by query.
- [x] 3.2 Add python fixtures in `web/src/test/fixtures/` captured from the live API on 2026-10-08: tags; details for `latest` and `latest-dev`; packages for both tags on amd64 and arm64. Note the capture date in the fixture file. Verify: a sanity test confirms 29 and 76 packages on amd64 and the sizes used in VC-6.1.

## 4. Size chart

- [x] 4.1 Build `HorizontalBars` in `web/src/components/` (design decision 4; VC-9): SVG bars on a linear scale starting at zero, taking `{ label, value, valueLabel }[]`, marked `aria-hidden`, with no new dependencies. Verify with component tests: `VC-9.1` (a value ten times another gets a bar ten times as wide, read from the `rect` width attributes), and a zero value draws a zero-width bar.

## 5. Compare tab

- [x] 5.1 Add a "Compare" tab to `ImagePage.tsx`, and hide the page-level tag selector while it's active (design decision 3). Verify with a test: the tab appears, activates via `?tab=compare`, and the page's Tag selector isn't shown on it.
- [x] 5.2 Implement pair selection held in the URL (`a`, `b`, `arch`) (VC-1, VC-2, VC-3, VC-4). Verify with tests: `VC-1.1` (default `latest` vs `latest-dev`), `VC-1.2` (choosing `next` as the first tag sets the second to `next-dev`), `VC-2.1` (`node` `latest` vs `latest-slim`), `VC-2.2` (the second selector never offers the first's tag), `VC-3.1` (`static` shows the no-variant message, with selectors), `VC-3.2` (a single-tag image shows the message without selectors), `VC-4.1` (`?a=latest&b=latest-slim&arch=arm64` loads that comparison, and changing a selector updates the URL), plus invalid or duplicate URL tags falling back to the default pair.
- [x] 5.3 Render the summary sentence, summary table, size chart and the three package groups (VC-5, VC-6, VC-7, VC-8, VC-9, VC-13). Verify with tests using the python fixtures: `VC-5.1` (29 and 76 packages with both sizes), `VC-6.1` (sentence reads "latest-dev is 10.1× larger than latest (+245.7 MB)"), `VC-7.1` ("Only in latest-dev (47)" listing `gcc`), `VC-7.2` (empty groups say "None"), `VC-8.1` (shell and `apk` per tag), `VC-13.1` (sizes, counts and ratio present as page text).
- [x] 5.4 Implement the architecture toggle (VC-10). Verify with a test: `VC-10.1` (switching to arm64 requests both tags' packages with `arch=arm64`, shows the arm64 sizes, and makes no new details requests).
- [x] 5.5 Implement loading and error states (VC-11, VC-12). Verify with tests: `VC-12.1` (one packages request fails → its error and a retry are shown, and no comparison), `VC-12.2` (pending → loading indicator), `VC-11.1` (a subscription-only image shows the subscription message and makes no details or packages requests), and a tag missing the selected architecture shows "<tag> has no arm64 variant".
- [x] 5.6 Document the Compare tab in the README's Web UI section, and the decimal size units. Verify: the README describes the tab as it behaves.

## 6. Integration verification

- [x] 6.1 Run every check: `gofmt -l .`, `go vet ./...`, `go test ./...`, and in `web/` `npm run lint`, `npm run typecheck`, `npm test`, `npm run build`, plus `openspec validate --all --strict`. Verify: all pass, and `go test` shows no Go changes were needed.
- [x] 6.2 Exercise the feature in the running app (build the UI, run `go run . -http 127.0.0.1:8080`) (VC-1 to VC-14). Verify in the browser: python compares `latest` vs `latest-dev` by default with live counts and sizes; switching to arm64 updates everything; `node` compares `latest` with `latest-slim`; `static` shows the no-variant message; `loki-fips` shows the subscription message; a reload keeps the comparison; the Overview tab shows decimal sizes. For `VC-14.1`: at 375 px wide in dark mode nothing overflows and the console shows no Content Security Policy violations. If the browser pane can't take screenshots, ask the user to confirm the layout visually.
- [x] 6.3 Push and confirm CI passes on the implementing commit, as `config.yaml`'s archive guidance requires. Verify: all CI jobs succeed.

## Workflow follow-up

- Archive with `/opsx:archive` once 6.3 passes; this creates the living spec `openspec/specs/image-variant-comparison/spec.md`. Check its Purpose and requirement IDs survived the merge.
- Review the OpenSpec trial, then migrate `specs/002-license-breakdown` to OpenSpec, reusing `HorizontalBars`.
