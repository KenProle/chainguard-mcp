# Design

## Context

See `proposal.md` for why, and `specs/image-license-breakdown/spec.md` for the requirements (LB-1 to LB-14).

Current state of the web UI (`web/`):
- `PackagesTab.tsx` receives `name` and `tag` from `ImagePage` (which keeps the tag in the URL), holds the architecture and the text filter in local state, and fetches `['packages', name, tag, arch]` through TanStack Query. The text filter matches package name or origin. The table already prints each package's full license expression, `WITH` exception included, or "—" when there is none.
- The packages response (`ImagePackages` in `web/src/api.ts`) carries each package's `license`. The Go side (`sbom.go`) copies the SPDX `licenseDeclared` and leaves `license` empty for `NOASSERTION`, so "no license" arrives as a missing field.
- `HorizontalBars` (`web/src/components/HorizontalBars.tsx`), added by `add-variant-comparison` for this feature, draws `{ label, value, valueLabel }[]` as SVG rects sized by attributes on a zero-based linear scale, in one indigo color, inside an `aria-hidden` wrapper. The Compare tab is its only user.
- `web/src/compare.ts` is the model for pure logic: small exported functions with exact-number unit tests in `compare.test.ts`; the component stays thin.
- `web/src/test/fixtures/python.ts` holds real python responses captured on 2026-10-08, including `latest` and `latest-dev` packages on both architectures. Their licenses match the live API on the day this change was written (same `latest` amd64 digest `sha256:7d010bb2…`).

**No API changes.** The Go server, the web API and the MCP tools are untouched; the breakdown is computed from the existing packages response.

## Goals / Non-Goals

**Goals:**
- Expression parsing and categorization are pure functions, unit-tested directly against LB-1 to LB-6 with the real python licenses.
- `HorizontalBars` is extended, not forked, and the Compare tab's output doesn't change.

**Non-Goals:**
- A general SPDX parser: no validation, no `LicenseRef-` handling beyond treating it as Unrecognized, no license list download.
- Persisting the selected category across reloads.

## Decisions

### 1. Logic in a pure module, `web/src/licenses.ts`
Exports, each mapping to requirements:
- `type LicenseCategory = 'permissive' | 'weak' | 'strong' | 'none' | 'unrecognized'`, plus display names ("Permissive", "Weak copyleft", "Strong copyleft", "Not declared", "Unrecognized").
- `categorizeIdentifier(id)` → the fixed table (LB-2). Prefix rules (`BSD-`, `GPL-`, `LGPL-`, …) match by family, checked so `LGPL-`/`AGPL-` aren't caught by `GPL-`. The table is a short, commented array of `[pattern, category]` in this file, which is the "documented table in the code".
- `categorizeLicense(expression?)` → one category (LB-1, LB-3 to LB-6). Empty or missing → `none`. Otherwise tokenize on whitespace and parentheses and evaluate with a small recursive-descent parser in SPDX precedence: `or := and ('OR' and)*`, `and := term ('AND' term)*`, `term := '(' or ')' | id ['WITH' exception]`. `WITH` keeps the base identifier's category (LB-5). Combining:
  - `AND`: strong if any part is strong; else unrecognized if any part is unrecognized; else weak if any is weak; else permissive (LB-3).
  - `OR`: permissive if any part is permissive; else unrecognized if any part is unrecognized; else weak if any is weak; else strong (LB-4).
  - A malformed expression (unbalanced parentheses, a dangling operator) → `unrecognized`, never an exception.
- `licenseBreakdown(packages)` → `{ category, count }[]` for non-empty categories, sorted by count descending, ties broken by the fixed order Strong copyleft, Weak copyleft, Permissive, Unrecognized, Not declared, so the most restrictive comes first (LB-1, LB-7).

*Why:* the scenarios become fast unit tests with exact numbers (LB-1.1: 16 / 6 / 7 for the 29-package fixture; LB-1.2: 37 / 13 / 24 / 0 / 2 for 76), and the parser can be tested on single expressions. These counts were checked against the fixtures with a throwaway script while writing this change. *Alternatives:* splitting on ` AND `/` OR ` with regexes, rejected because `git`'s parenthesized expression (LB-6.1) needs real grouping; an SPDX parsing library (e.g. `spdx-expression-parse`), rejected because it throws on malformed expressions and on identifiers missing from its bundled license list, both of which we must categorize as Unrecognized instead, and a 40-line parser doesn't justify a dependency. **No new dependencies are added.**

### 2. Extend `HorizontalBars` with color and selection, both optional
New optional fields and props:
- `Bar.colorClass?: string`: a Tailwind fill class pair (e.g. `fill-rose-500 dark:fill-rose-400`) applied to the `rect`; defaults to today's indigo (LB-8).
- `Bar.accessibleLabel?: string`, and props `selected?: string` (a bar label) and `onSelect?: (label: string) => void`.

Without `onSelect` the component renders exactly as now: the `aria-hidden` wrapper the Compare tab relies on (VC-9.1's test checks it). With `onSelect`, each row becomes a `<button type="button" aria-pressed={selected === label} aria-label={accessibleLabel}>` containing the visible label, the SVG (itself `aria-hidden`) and the count text, so each bar is the focusable control LB-13 asks for and native buttons handle Enter and Space. The selected row gets a ring and bolder label (LB-9.1), so selection isn't shown by color alone.

*Why one component:* the chart and its controls stay aligned with no duplicated layout, and the Compare tab is untouched. *Alternative:* keep `HorizontalBars` decorative and render a separate row of category buttons above it. Rejected: two representations of the same five categories, and two places to keep in sync.

Category colors live in `web/src/styles.ts` as `licenseCategoryColor: Record<LicenseCategory, string>` (complete class strings, so Tailwind's scanner sees them, and kept out of component files for fast refresh). Proposed: Permissive emerald, Weak copyleft amber, Strong copyleft rose, Unrecognized violet, Not declared zinc, each with a lighter `dark:` variant. Colors are classes, not styles, so the CSP is unaffected (LB-14).

### 3. A `LicenseBreakdown` component inside the Packages tab's card
`web/src/components/LicenseBreakdown.tsx` takes `packages`, `selected` and `onSelect`, calls `licenseBreakdown`, and renders, above the text filter:
1. a heading "Licenses" and the bars, each labeled with its name, and its count as the value label ("16 packages") (LB-7, LB-8)
2. a "Show all" button, shown only while a category is selected (LB-9.3)
3. the note: "Based on the licenses declared in the image's SBOM. Informational only, not legal advice." (LB-12)

Accessible names are "Permissive, 16 packages" (singular "1 package"). It renders only once packages have loaded, inside the existing success branch, so loading, error and subscription states are the tab's existing ones and no new request is made (LB-7.3).

### 4. Selection state in `PackagesTab`, keyed to tag and architecture
`PackagesTab` holds `selection: { key: string; category: LicenseCategory } | null`, where `key` is `${tag}/${arch}`. The effective selection is `selection?.key === currentKey ? selection.category : null`, so changing the tag (a prop) or the architecture clears it on the next render without an effect (LB-11). Activating the selected category again sets it to `null` (LB-9.2).

The table filter becomes: text matches (name or origin, as today) **and** (no selection **or** `categorizeLicense(p.license) === selected`) (LB-10). "No packages match." still covers an empty result. Categories are computed once per package list with `useMemo`.

*Alternative:* category in the URL (`?license=strong`). Rejected for now: the architecture and the text filter aren't in the URL either, and a shared link with a category but a different architecture would be confusing. Easy to add later.

### 5. Test fixtures, not live data
Unit and component tests use `web/src/test/fixtures/python.ts`, which already has both tags on both architectures. `mockApi` already matches query parameters, so the tag and architecture switches (LB-11) need no test-harness changes. A hand-built package list covers cases python lacks: an empty license (LB-1.3), `AND`/`OR` with an unrecognized part (LB-3.4, LB-4.2) and malformed expressions. Test names start with their scenario ID (`LB-1.1 …`).

## Risks / Trade-offs

- **[The mapping is a judgment call]** e.g. `MPL-2.0` as weak copyleft, `blessing` as permissive, and `OLDAP-2.8`/`CC-BY-4.0` left Unrecognized. Identifiers are added when real images show them: `tomcat` (2026-10-09) added the X11-era and graphics-library licenses (`X11`, `XFree86-*`, `libpng-*`, `IJG`, `FTL`, `Bitstream-*`), all permissive notice licenses that had left 8 of its 47 packages Unrecognized. → The table is explicit and commented in one place, the UI says it's informational, and Unrecognized makes gaps visible instead of guessing.
- **[Deprecated or unusual identifiers]** `latest-dev` has `GPL-2.0` and `GPL-3.0` (deprecated, without `-only`) and space-padded parentheses. → Family prefixes cover deprecated GPL forms; the tokenizer splits on parentheses regardless of spacing.
- **[Live data drifts from the examples]** Chainguard rebuilds daily and its license expressions change. → Tests use the dated fixtures; the spec's examples are dated.
- **[Extending a shared component]** could change the Compare tab. → Both new props are optional, and `HorizontalBars`'s existing tests (including `aria-hidden` in VC-9.1) and `CompareTab.test.tsx` must pass unchanged.
- **[Web-only logic]** The MCP tools can't answer "how many copyleft packages?" the same way. → Accepted for this change (see proposal); the module is pure and small enough to port to Go if an MCP tool is wanted.

## Migration Plan

Frontend only, shipped in the embedded UI on the next build. Rollback is reverting the change; there's no data, API or configuration migration.
