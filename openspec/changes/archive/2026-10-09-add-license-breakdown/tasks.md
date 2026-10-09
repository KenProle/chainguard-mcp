# Tasks

Test names start with the scenario ID they cover (e.g. `LB-1.1 …`). Run web commands from `web/` (Node on `PATH` as described in CLAUDE.md).

## 1. License categorization (`web/src/licenses.ts`)

- [x] 1.1 Implement `LicenseCategory`, the category display names and `categorizeIdentifier(id)` with the commented mapping table (design decision 1; LB-2). Verify with unit tests in `web/src/licenses.test.ts`: `LB-2.1` (`PSF-2.0`, `blessing`, `LGPL-2.1-or-later`, `GPL-3.0-or-later` → Permissive, Permissive, Weak copyleft, Strong copyleft), `LB-2.2` (`OLDAP-2.8` → Unrecognized), every identifier in `python:latest`'s fixture is recognized, and `LGPL-*`/`AGPL-*` aren't caught by the `GPL-` rule.
- [x] 1.2 Implement `categorizeLicense(expression?)`: tokenizer, recursive-descent parser with SPDX precedence, and the `AND`/`OR`/`WITH` combining rules (LB-1, LB-3, LB-4, LB-5, LB-6). Verify with unit tests: `LB-1.3` (missing or empty → Not declared), `LB-3.1` (`BSD-2-Clause AND GPL-2.0-only` → Strong), `LB-3.2` (`MPL-2.0 AND MIT` → Weak), `LB-3.3` (`libuuid`'s 11-part expression → Strong), `LB-3.4` (Permissive `AND` unknown → Unrecognized), `LB-4.1` (`LGPL-3.0-or-later OR GPL-2.0-or-later` → Weak), `LB-4.2` (Strong `OR` unknown → Unrecognized), `LB-5.1` (`GPL-3.0-or-later WITH GCC-exception-3.1` → Strong), `LB-6.1` (`git`'s parenthesized expression → Strong), `LB-6.2` (`MIT OR GPL-2.0-only AND LGPL-2.1-or-later` → Permissive), and malformed input (`MIT AND`, `( MIT`) → Unrecognized without throwing.
- [x] 1.3 Implement `licenseBreakdown(packages)`: non-empty categories, largest first, ties in the fixed restrictiveness order (LB-1, LB-7). Verify with unit tests against the python fixtures: `LB-1.1` (`latest` amd64 → Permissive 16, Strong 7, Weak 6, in that order), `LB-1.2` (`latest-dev` amd64 → Permissive 37, Strong 24, Weak 13, Unrecognized 2, summing to 76), and a tie ordered by restrictiveness.

## 2. Chart component

- [x] 2.1 Extend `HorizontalBars` with optional `colorClass` and `accessibleLabel` per bar, and optional `selected` / `onSelect` props that render each row as an `aria-pressed` button (design decision 2; LB-8, LB-9, LB-13). Add `licenseCategoryColor` to `web/src/styles.ts`. Verify with component tests in `HorizontalBars.test.tsx`: the existing VC-9.1 and zero-width tests pass unchanged (no `onSelect` → `aria-hidden` wrapper, no buttons); `LB-7.2` (a value twice another draws a bar twice as wide); with `onSelect`, each row is a button named by `accessibleLabel`, `aria-pressed` reflects `selected`, and clicking calls `onSelect` with the label; a bar's `rect` carries its `colorClass`. Verify `CompareTab.test.tsx` passes unchanged.

## 3. Packages tab

- [x] 3.1 Build `LicenseBreakdown` in `web/src/components/` (heading, bars with category colors and "N packages" labels, "Show all" while a category is selected, the not-legal-advice note) and render it in `PackagesTab` above the text filter (design decision 3; LB-7, LB-8, LB-12, LB-13). Verify with page tests in `web/src/pages/PackagesTab.test.tsx` using the python fixtures: `LB-7.1` (bars for Permissive 16, Strong copyleft 7, Weak copyleft 6 in that order, none for Not declared or Unrecognized), `LB-7.3` (only the packages request is made), `LB-8.1` (category names and counts present as text), `LB-12.1` (the note mentions the SBOM and "not legal advice"), `LB-13.1` (a button named "Strong copyleft, 7 packages" with `aria-pressed="false"`).
- [x] 3.2 Add the category selection keyed to tag and architecture, and combine it with the text filter in the table (design decision 4; LB-9, LB-10, LB-11, LB-13). Verify with page tests in `PackagesTab.test.tsx`: `LB-9.1` (selecting Strong copyleft shows exactly its 7 packages, including `libgcc`, `readline` and `libzstd1`, and marks it pressed), `LB-9.2` (activating it again shows all 29), `LB-9.3` ("Show all" shows all 29 and hides itself), `LB-10.1` (Strong copyleft plus `gcc` shows `libgcc` and `libstdc++` only), `LB-11.1` (switching to arm64 clears the selection and shows arm64 counts), `LB-11.2` (`latest-dev` counts sum to 76), `LB-13.2` (focusing a category and pressing Enter, then Space, toggles it), and `LB-5.1` (the `libgcc` row shows the full `WITH` expression).
- [x] 3.3 Document the license breakdown in the README's Web UI section, including the categories, how compound expressions are categorized and that it's not legal advice. Verify: the README describes the feature as it behaves.

## 4. Integration verification

- [x] 4.1 Run every check: `gofmt -l .`, `go vet ./...`, `go test ./...`, and in `web/` `npm run lint`, `npm run typecheck`, `npm test`, `npm run build`, plus `openspec validate --all --strict`. Verify: all pass, and `git diff --stat` shows no Go changes.
- [x] 4.2 Exercise the feature in the running app (build the UI, run `go run . -http 127.0.0.1:8080`) (LB-1 to LB-14). Verify in the browser: `python` `latest` shows the breakdown with live counts adding up to the package total; selecting a category filters the table and works with the text filter; switching architecture and tag recalculates and clears the selection; `python` `latest-dev` shows Unrecognized packages; keyboard Tab/Enter/Space work; the Compare tab's size chart looks as before; `loki-fips` still shows the subscription message. For `LB-14.1`: at 375 px wide in dark mode nothing overflows and the console shows no Content Security Policy violations. If the browser pane can't take screenshots, ask the user to confirm the layout visually.
- [x] 4.3 Push and confirm CI passes on the implementing commit, as `config.yaml`'s archive guidance requires. Verify: all CI jobs succeed.

## 5. X11-era and graphics-library licenses

- [x] 5.1 Add `X11`, `XFree86-*`, `libpng-*`, `IJG`, `FTL` and `Bitstream-*` to the Permissive rows of the mapping table in `web/src/licenses.ts` (LB-2), and add `web/src/test/fixtures/tomcat.ts`, tomcat's amd64 packages captured on 2026-10-09. Verify with unit tests in `licenses.test.ts`: `LB-2.3` (the six expressions from `libx11`, `libxi`, `libpng`, `libjpeg-turbo`, `freetype` and `ttf-dejavu` are Permissive, and the tomcat fixture counts Permissive 28, Weak copyleft 11, Strong copyleft 8), and every identifier in the tomcat fixture is recognized.
- [x] 5.2 Run every check as in 4.1, open `tomcat`'s Packages tab in the running app and confirm it shows no Unrecognized packages, then push and confirm CI passes on that commit. Verify: all checks and CI jobs succeed.

## Workflow follow-up

- Archive with `/opsx:archive` once 5.2 passes; this creates the living spec `openspec/specs/image-license-breakdown/spec.md`. Check its Purpose and requirement IDs survived the merge.
