# Spec Delta

## Purpose

Shows how a Chainguard image's packages break down by declared license type (permissive, weak copyleft, strong copyleft), so users can spot copyleft obligations and find the packages behind them.

LB-1 to LB-6 define how packages are categorized, LB-7 to LB-12 how the breakdown is displayed, and LB-13 to LB-14 its accessibility, layout and security policy.

## ADDED Requirements

### Requirement: LB-1 One category per package
The license breakdown SHALL place every package in exactly one category: Permissive, Weak copyleft, Strong copyleft, Not declared (no license in the SBOM) or Unrecognized (a license identifier not in the mapping), so the category counts add up to the image's package count.

#### Scenario: LB-1.1 python latest on amd64
- **WHEN** the breakdown is calculated for `python` `latest` on amd64 (29 packages, with the licenses captured on 2026-10-08)
- **THEN** the counts are Permissive 16, Weak copyleft 6, Strong copyleft 7, Not declared 0 and Unrecognized 0

#### Scenario: LB-1.2 python latest-dev on amd64
- **WHEN** the breakdown is calculated for `python` `latest-dev` on amd64 (76 packages, captured on 2026-10-08)
- **THEN** the counts are Permissive 37, Weak copyleft 13, Strong copyleft 24, Not declared 0 and Unrecognized 2
- **AND** they add up to 76

#### Scenario: LB-1.3 Package with no declared license
- **WHEN** a package's SBOM entry declares no license (`NOASSERTION`, which the packages response omits)
- **THEN** the package is counted as Not declared

### Requirement: LB-2 License identifier mapping
The license breakdown SHALL map license identifiers to categories with a fixed table that covers every identifier in `python` and `tomcat` `latest`. Permissive SHALL include `MIT`, `X11`, `XFree86-*`, `Apache-*`, `BSD-*`, `PSF-*`, `ISC`, `Zlib`, `libpng-*`, `IJG`, `FTL`, `Bitstream-*`, `blessing` and `CC-PDDC`; Weak copyleft `LGPL-*`, `MPL-*` and `EPL-*`; Strong copyleft `GPL-*` and `AGPL-*`. Any other identifier SHALL be Unrecognized.

#### Scenario: LB-2.1 Common identifiers
- **WHEN** packages are licensed `PSF-2.0` (`python-3.14`), `blessing` (`sqlite-libs`), `LGPL-2.1-or-later` (`glibc-2.44`) and `GPL-3.0-or-later` (`readline`)
- **THEN** they are categorized Permissive, Permissive, Weak copyleft and Strong copyleft

#### Scenario: LB-2.2 Identifier outside the mapping
- **WHEN** a package is licensed `OLDAP-2.8` (`libldap-2.7` in `python:latest-dev`)
- **THEN** it is categorized Unrecognized

#### Scenario: LB-2.3 X11-era and graphics library licenses
- **WHEN** the breakdown is calculated for `tomcat` `latest` on amd64 (47 packages, captured on 2026-10-09), whose packages include `XFree86-1.1` (`libx11`), `MIT AND X11` (`libxi`), `libpng-2.0` (`libpng`), `BSD-3-Clause AND IJG AND Zlib` (`libjpeg-turbo`), `FTL OR GPL-2.0-or-later` (`freetype`) and `Bitstream-Vera` (`ttf-dejavu`)
- **THEN** those packages are categorized Permissive
- **AND** the counts are Permissive 28, Weak copyleft 11, Strong copyleft 8, Not declared 0 and Unrecognized 0

### Requirement: LB-3 AND expressions
When a package's license is an `AND` expression (all licenses apply), the license breakdown SHALL use the most restrictive category among its parts (Strong copyleft over Weak copyleft over Permissive). If any part is Unrecognized and no part is Strong copyleft, the package SHALL be Unrecognized.

#### Scenario: LB-3.1 Permissive and strong copyleft
- **WHEN** a package is licensed `BSD-2-Clause AND GPL-2.0-only` (`libzstd1`)
- **THEN** it is categorized Strong copyleft

#### Scenario: LB-3.2 Permissive and weak copyleft
- **WHEN** a package is licensed `MPL-2.0 AND MIT` (`zlib`)
- **THEN** it is categorized Weak copyleft

#### Scenario: LB-3.3 Long expression
- **WHEN** a package is licensed with `util-linux`'s 11-part `AND` expression, which includes `GPL-3.0-or-later`, `LGPL-2.1-or-later`, `MIT` and `CC-PDDC` (`libuuid`)
- **THEN** it is categorized Strong copyleft

#### Scenario: LB-3.4 Unrecognized part without strong copyleft
- **WHEN** a package's `AND` expression combines a Permissive identifier with an identifier outside the mapping
- **THEN** it is categorized Unrecognized

### Requirement: LB-4 OR expressions
When a package's license is an `OR` expression (a choice of licenses), the license breakdown SHALL use the least restrictive category among its parts. If any part is Unrecognized and no part is Permissive, the package SHALL be Unrecognized.

#### Scenario: LB-4.1 Choice of weak or strong copyleft
- **WHEN** a package is licensed `LGPL-3.0-or-later OR GPL-2.0-or-later` (`gmp` in `python:latest-dev`)
- **THEN** it is categorized Weak copyleft

#### Scenario: LB-4.2 Unrecognized choice without a permissive one
- **WHEN** a package's `OR` expression has one Strong copyleft part and one part whose identifier is outside the mapping
- **THEN** it is categorized Unrecognized

### Requirement: LB-5 WITH exceptions
When a license has a `WITH` exception, the license breakdown SHALL categorize it by the base license, and every place the UI displays the expression SHALL show the exception.

#### Scenario: LB-5.1 GCC runtime library exception
- **WHEN** a package is licensed `GPL-3.0-or-later WITH GCC-exception-3.1` (`libgcc`)
- **THEN** it is categorized Strong copyleft
- **AND** its row in the package table shows `GPL-3.0-or-later WITH GCC-exception-3.1`

### Requirement: LB-6 Grouping and precedence
The license breakdown SHALL evaluate license expressions with SPDX precedence: `WITH` binds tightest, then `AND`, then `OR`, and parentheses group sub-expressions.

#### Scenario: LB-6.1 Parenthesized choice inside AND
- **WHEN** a package is licensed `GPL-2.0-or-later AND ( Artistic-1.0-Perl OR GPL-1.0-or-later )` (`git` in `python:latest-dev`)
- **THEN** it is categorized Strong copyleft

#### Scenario: LB-6.2 AND binds tighter than OR
- **WHEN** a package is licensed `MIT OR GPL-2.0-only AND LGPL-2.1-or-later`
- **THEN** it is evaluated as `MIT OR (GPL-2.0-only AND LGPL-2.1-or-later)` and categorized Permissive

### Requirement: LB-7 Breakdown chart on the Packages tab
The Packages tab SHALL show the license breakdown above the package table, computed from the packages already loaded, as one horizontal bar per non-empty category, labeled with its package count, sorted largest first, on a shared scale starting at zero.

#### Scenario: LB-7.1 python latest
- **WHEN** the user opens the Packages tab for `python` `latest` on amd64
- **THEN** bars are shown for Permissive (16), Strong copyleft (7) and Weak copyleft (6), in that order
- **AND** no bars are shown for Not declared or Unrecognized

#### Scenario: LB-7.2 Bars are proportional
- **WHEN** one category has twice as many packages as another
- **THEN** its bar is twice as long

#### Scenario: LB-7.3 No extra requests
- **WHEN** the breakdown is shown
- **THEN** no API request is made beyond the packages request the tab already makes

### Requirement: LB-8 Category colors and text labels
The license breakdown SHALL give each category a consistent color that works in light and dark mode, and SHALL print each category's name and count as text, so color is not the only cue.

#### Scenario: LB-8.1 Reading without color
- **WHEN** the breakdown for `python` `latest` is read without color
- **THEN** the text "Permissive", "Strong copyleft" and "Weak copyleft" and their counts are present

### Requirement: LB-9 Filtering by category
The license breakdown SHALL let the user select one category at a time to filter the package table to that category's packages, mark the selected category visibly, and clear the selection when the category is activated again or the "Show all" control is used.

#### Scenario: LB-9.1 Selecting a category
- **WHEN** the user activates Strong copyleft for `python` `latest`
- **THEN** the package table shows only its 7 packages, including `libgcc`, `readline` and `libzstd1`
- **AND** Strong copyleft is marked as selected

#### Scenario: LB-9.2 Activating the selected category again
- **WHEN** a category is selected and the user activates it again
- **THEN** the package table shows all packages and no category is selected

#### Scenario: LB-9.3 Show all
- **WHEN** a category is selected and the user activates "Show all"
- **THEN** the package table shows all packages and no category is selected

### Requirement: LB-10 Category and text filters combined
While a category is selected, the package table SHALL show only packages that match both the category and the existing text filter.

#### Scenario: LB-10.1 Strong copyleft and "gcc"
- **WHEN** Strong copyleft is selected for `python` `latest` and the user types `gcc` in the filter
- **THEN** the package table shows `libgcc` and `libstdc++` only

### Requirement: LB-11 Recalculation on tag or architecture change
When the tag or architecture changes, the license breakdown SHALL recalculate from the new package list and SHALL clear any category selection.

#### Scenario: LB-11.1 Switching architecture
- **WHEN** a category is selected and the user switches from amd64 to arm64
- **THEN** the breakdown shows the arm64 packages' counts and no category is selected

#### Scenario: LB-11.2 Switching tag
- **WHEN** the user switches `python` from `latest` to `latest-dev` on amd64
- **THEN** the breakdown's counts add up to 76

### Requirement: LB-12 Informational note
The license breakdown SHALL state that it is based on the licenses declared in the image's SBOM, is informational only, and is not legal advice.

#### Scenario: LB-12.1 Note is shown
- **WHEN** the breakdown is shown
- **THEN** the note mentions the SBOM's declared licenses and says it is not legal advice

### Requirement: LB-13 Keyboard and screen reader access
The license breakdown SHALL render each category as a keyboard-focusable control that can be activated with Enter or Space, whose accessible name includes the category and count, and which exposes whether it is selected.

#### Scenario: LB-13.1 Accessible name
- **WHEN** a screen reader focuses the Strong copyleft control for `python` `latest`
- **THEN** its accessible name is "Strong copyleft, 7 packages" and it is announced as not pressed

#### Scenario: LB-13.2 Keyboard activation
- **WHEN** the user focuses a category with Tab and presses Enter or Space
- **THEN** the category is selected, as with a click

### Requirement: LB-14 Layout, theme and security policy
The license breakdown SHALL be usable at 375 px width and in light and dark mode, and SHALL cause no Content Security Policy violations.

#### Scenario: LB-14.1 Phone width in dark mode
- **WHEN** the Packages tab for `python` `latest-dev` is viewed at 375 px wide in dark mode
- **THEN** nothing overflows horizontally, all category names and counts are readable, and the browser console shows no Content Security Policy violations
