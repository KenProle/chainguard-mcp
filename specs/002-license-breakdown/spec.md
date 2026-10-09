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

### Categories

- **AC-2.1:** Every package is placed in exactly one category: **Permissive**, **Weak copyleft**, **Strong copyleft**, **Not declared** (no license in the SBOM), or **Unrecognized** (a license identifier not in the mapping).
- **AC-2.2:** License identifiers map to categories through a fixed, documented table in the code that covers at least every identifier in the table above. Permissive includes `MIT`, `Apache-2.0`, `BSD-*`, `PSF-2.0`, `ISC`, `Zlib`, `blessing` and `CC-PDDC`; weak copyleft includes `LGPL-*`, `MPL-*` and `EPL-*`; strong copyleft includes `GPL-*` and `AGPL-*`.
- **AC-2.3:** For `AND` expressions (all licenses apply), the package takes the most restrictive category among its parts: strong copyleft > weak copyleft > permissive. If any part is unrecognized and no part is strong copyleft, the package is Unrecognized.
- **AC-2.4:** For `OR` expressions (a choice of licenses), the package takes the least restrictive category among its parts.
- **AC-2.5:** For `WITH` exceptions (e.g. `GPL-3.0-or-later WITH GCC-exception-3.1`), the package is categorized by the base license, and the exception is shown wherever the expression is displayed.
- **AC-2.6:** Given `python:latest` on amd64 with the licenses in the table above, the categories are: Permissive 16, Weak copyleft 6, Strong copyleft 7, Not declared 0, Unrecognized 0. (This example is a required test case.)
- **AC-2.7:** The category counts always add up to the image's total package count.

### Display

- **AC-2.8:** The Packages tab shows the breakdown above the package table: one horizontal bar per non-empty category, with its package count, sorted by count (largest first), on a shared scale starting at zero.
- **AC-2.9:** Categories use consistent colors that also work in dark mode, and the category name and count are printed as text, not conveyed by color alone.
- **AC-2.10:** Given the breakdown, when the user activates a category (click, or Enter/Space when focused), then the package table shows only that category's packages and the selection is visibly marked; activating it again, or a "Show all" control, clears the filter.
- **AC-2.11:** The category filter combines with the existing text filter (both apply).
- **AC-2.12:** Given the tag or architecture changes, the breakdown recalculates from the new package list and any category filter is cleared.
- **AC-2.13:** A short note under the breakdown says it's based on the licenses declared in the image's SBOM, is informational only, and is not legal advice.

### Quality

- **AC-2.14:** Category bars are keyboard-focusable controls with accessible names that include the category and count (e.g. "Strong copyleft, 7 packages").
- **AC-2.15:** Usable at 375 px width, in light and dark mode, with no Content Security Policy violations.

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
