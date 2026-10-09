# Proposal

## Why

"What licenses does this image ship?" is a common compliance question, and today the only answer is the License column in the Packages tab's table. For `python:latest` on linux/amd64 (October 2026) that column holds 11 distinct license expressions across 29 packages, several of them compound:

| Packages | Declared license (from the SBOM) |
|---|---|
| 9 | `MIT` |
| 3 | `MPL-2.0 AND MIT` |
| 3 | `GPL-3.0-or-later` |
| 3 | `LGPL-2.1-or-later` |
| 3 | `Apache-2.0` |
| 2 | `GPL-3.0-or-later WITH GCC-exception-3.1` |
| 2 | `PSF-2.0` |
| 1 | `GPL-3.0-or-later AND GPL-2.0-or-later AND … AND CC-PDDC` (11 parts, `libuuid` from `util-linux`) |
| 1 | `BSD-2-Clause AND GPL-2.0-only` |
| 1 | `BSD-2-Clause` |
| 1 | `blessing` (SQLite's public-domain dedication) |

`python:latest-dev` (76 packages) adds `OR` choices (`gmp`: `LGPL-3.0-or-later OR GPL-2.0-or-later`), a parenthesized expression (`git`: `GPL-2.0-or-later AND ( Artistic-1.0-Perl OR GPL-1.0-or-later )`) and identifiers outside the common families (`OLDAP-2.8`, `CC-BY-4.0`). Reading that table to spot copyleft obligations is slow and error-prone.

Users:
- A compliance reviewer wants to see how an image's packages break down by license type, to spot copyleft obligations quickly.
- A developer wants to click a license type and see exactly which packages have it, to check them.

## What Changes

- The web UI's Packages tab shows a **license breakdown** above the package table: one bar per license category (Permissive, Weak copyleft, Strong copyleft, Not declared, Unrecognized) with its package count.
- Each package lands in exactly one category, using a fixed mapping of license identifiers and documented rules for `AND`, `OR`, `WITH` and parentheses.
- Selecting a category filters the package table to that category's packages, combined with the existing text filter.
- A note says the breakdown is based on the licenses declared in the SBOM, is informational only and is not legal advice.

## Capabilities

### New Capabilities
- `image-license-breakdown`: categorizing an image's packages by declared license type in the web UI, and filtering the package list by category.

### Modified Capabilities
None. `image-variant-comparison` is unaffected; this change only reuses its bar chart component.

## Decisions

Made with the user on 2026-10-08:
- **Compound `AND` expressions:** the most restrictive part decides the category, so each package lands in exactly one category. *Rejected:* counting the package in every category it touches, which makes the totals exceed the package count.
- **`WITH` exceptions:** categorized by the base license; deciding what an exception permits is a legal judgment the tool shouldn't make. *Rejected:* treating runtime-library exceptions (e.g. `GCC-exception-3.1`) as weak copyleft.
- **Computed in the browser** from the existing packages response, with no new API endpoint or request, and the classification logic in a pure, unit-tested function separate from the chart.
- **`OR` with an unrecognized part:** mirrors the `AND` rule. The least restrictive part decides, but if any part is unrecognized and no part is permissive, the package is Unrecognized, since the unknown license might be the least restrictive choice.
- **Parentheses and precedence:** follow SPDX: `WITH` binds tightest, then `AND`, then `OR`, and parentheses group. `git` is Strong copyleft because its `AND` includes `GPL-2.0-or-later`, whatever the parenthesized choice resolves to.
- **The mapping covers the common families only.** `OLDAP-2.8` and `CC-BY-4.0` are not in it, so `python:latest-dev` shows 2 Unrecognized packages. *Alternative:* add them (and `Artistic-*`) to Permissive now; left out because classifying them is a judgment worth making deliberately, and the Unrecognized category exists for exactly this case. Adding identifiers later only edits the table.
- **Web UI only.** `openspec/config.yaml` puts shared logic in `Service` methods so the MCP tools and UI stay identical. This feature computes in the browser instead, as the Compare tab does: the MCP `get_image_packages` tool already returns each package's license, and no MCP tool categorizes licenses, so there is no second copy to drift. If an MCP tool for this is wanted later, the logic moves to Go then.

## Out of Scope

- Legal interpretation, license compatibility checks, or license texts.
- Validating that SBOM license expressions are well-formed SPDX (malformed parts count as Unrecognized).
- Licenses of files that aren't OS packages (e.g. vendored code inside a package).
- Any change to the MCP tools or the Go server.
- Keeping the selected category in the URL (the tag and tab already are; the architecture and text filter aren't, and the category follows them).

## Impact

- **Web UI:** the Packages tab gains the breakdown and category filter. `HorizontalBars` gains optional per-bar colors and an optional selectable mode; the Compare tab's use of it is unchanged. No API or Go changes; `design.md` confirms.
- **Dependencies:** none.
