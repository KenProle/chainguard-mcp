# 002: License breakdown

**Status:** Approved (2026-10-08)

## Problem

"What licenses does this image ship?" is a common compliance question. Today the answer is a license column in a package table, which for `python:latest` holds 11 distinct license expressions across 29 packages, several of them compound:

| Packages | Declared license (from the SBOM) |
|---|---|
| 9 | `MIT` |
| 3 | `MPL-2.0 AND MIT` |
| 3 | `GPL-3.0-or-later` |
| 3 | `LGPL-2.1-or-later` |
| 3 | `Apache-2.0` |
| 2 | `GPL-3.0-or-later WITH GCC-exception-3.1` |
| 2 | `PSF-2.0` |
| 1 | `GPL-3.0-or-later AND GPL-2.0-or-later AND … AND CC-PDDC` (13 parts, `util-linux`) |
| 1 | `BSD-2-Clause AND GPL-2.0-only` |
| 1 | `BSD-2-Clause` |
| 1 | `blessing` (SQLite's public-domain dedication) |

## Users and stories

- **US-1:** As a compliance reviewer, I want to see how an image's packages break down by license type, so I can spot copyleft obligations quickly.
- **US-2:** As a developer, I want to click a license type and see exactly which packages have it, so I can check them.

## Acceptance criteria

Each criterion is either a **scenario** (Given / when / then) or a **rule** in [EARS](https://alistairmavin.com/ears/) form: "The license breakdown shall…", "When *trigger*, the license breakdown shall…", "While *state*, …" or "If *unwanted condition*, then …".

### Categories

- **AC-2.1:** The license breakdown **shall** place every package in exactly one category: **Permissive**, **Weak copyleft**, **Strong copyleft**, **Not declared** (no license in the SBOM), or **Unrecognized** (a license identifier not in the mapping).
- **AC-2.2:** The license breakdown **shall** map license identifiers to categories using a fixed, documented table in the code that covers at least every identifier in the table above. Permissive includes `MIT`, `Apache-2.0`, `BSD-*`, `PSF-2.0`, `ISC`, `Zlib`, `blessing` and `CC-PDDC`; weak copyleft includes `LGPL-*`, `MPL-*` and `EPL-*`; strong copyleft includes `GPL-*` and `AGPL-*`.
- **AC-2.3:** **When** a package's license is an `AND` expression (all licenses apply), the license breakdown **shall** use the most restrictive category among its parts: strong copyleft > weak copyleft > permissive. **If** any part is unrecognized and no part is strong copyleft, **then** it **shall** categorize the package as Unrecognized.
- **AC-2.4:** **When** a package's license is an `OR` expression (a choice of licenses), the license breakdown **shall** use the least restrictive category among its parts.
- **AC-2.5:** **When** a package's license has a `WITH` exception (e.g. `GPL-3.0-or-later WITH GCC-exception-3.1`), the license breakdown **shall** categorize the package by the base license, and the UI **shall** show the exception wherever the expression is displayed.
- **AC-2.6:** **Given** `python:latest` on amd64 with the licenses in the table above, **when** the breakdown is calculated, **then** the counts are Permissive 16, Weak copyleft 6, Strong copyleft 7, Not declared 0, Unrecognized 0. (This example is a required test case.)
- **AC-2.7:** The license breakdown's category counts **shall** add up to the image's total package count.

### Display

- **AC-2.8:** The Packages tab **shall** show the license breakdown above the package table, as one horizontal bar per non-empty category labeled with its package count, sorted largest first, on a shared scale starting at zero.
- **AC-2.9:** The license breakdown **shall** use consistent category colors that work in light and dark mode, and **shall** print each category's name and count as text, so color isn't the only cue.
- **AC-2.10:** **Given** the breakdown is shown, **when** the user activates a category (click, or Enter/Space when focused), **then** the package table shows only that category's packages and the category is visibly marked as selected. **Given** a category is selected, **when** the user activates it again or the "Show all" control, **then** the package table shows all packages again.
- **AC-2.11:** **While** a category is selected, the package table **shall** apply both the category filter and the existing text filter.
- **AC-2.12:** **When** the tag or architecture changes, the license breakdown **shall** recalculate from the new package list and clear any category selection.
- **AC-2.13:** The license breakdown **shall** show a note that it's based on the licenses declared in the image's SBOM, is informational only, and is not legal advice.

### Quality

- **AC-2.14:** The license breakdown **shall** render each category bar as a keyboard-focusable control whose accessible name includes the category and count (e.g. "Strong copyleft, 7 packages").
- **AC-2.15:** The license breakdown **shall** be usable at 375 px width and in light and dark mode, and **shall** cause no Content Security Policy violations.

## Non-functional requirements

- Computed in the browser from the existing packages response; no new API endpoint or request.
- Classification logic is a pure function with unit tests, separate from the chart component.

## Decisions

- **D1: Compound `AND` expressions.** Most restrictive part (AC-2.3): conservative, and each package lands in exactly one category. (Rejected: counting the package in every category it touches, which makes totals exceed the package count.)
- **D2: `WITH` exceptions.** Categorized by the base license (AC-2.5); deciding what an exception permits is a legal judgment the tool shouldn't make. (Rejected: treating runtime-library exceptions as weak copyleft.)

## Out of scope

- Legal interpretation, license compatibility checks, or license texts.
- Validating that SBOM license expressions are well-formed SPDX (malformed parts count as Unrecognized).
- Licenses of files that aren't OS packages (e.g. vendored code inside a package).
