# image-security-fixes Specification

## Purpose

Shows how a Chainguard image's recorded security fixes are spread across its source packages, and makes any fix a newer package version has but the image lacks stand out. It never implies that an image or package is free of vulnerabilities.

SF-1 to SF-4 define what the chart shows, SF-5 to SF-7 how it is labeled and worded, and SF-8 to SF-9 its data loading, layout and security policy.

## Requirements

### Requirement: SF-1 One bar per source package with records
The Security tab SHALL show a fixes chart with one bar per source package that has at least one record in the Wolfi security database (a fix included, a never-affected record or a fix not yet installed), and SHALL leave out source packages with no records.

#### Scenario: SF-1.1 python latest
- **WHEN** the Security tab is opened for `python` `latest` (21 source packages, captured on 2026-10-09)
- **THEN** the chart has 12 bars
- **AND** there are no bars for the 9 source packages without records, such as `bzip2`, `readline` and `zstd`

#### Scenario: SF-1.2 Only never-affected records
- **WHEN** a source package has no fixes included but has never-affected records, as `sqlite` in `python` `latest` has (0 fixed, 4 never affected)
- **THEN** it has a bar

#### Scenario: SF-1.3 python latest-dev
- **WHEN** the Security tab is opened for `python` `latest-dev` (56 source packages, captured on 2026-10-09)
- **THEN** the chart has 25 bars

### Requirement: SF-2 Stacked segments on a shared scale
Each bar SHALL be split into up to three segments, in this order: fixes included, never affected, and fixes not yet installed. Segment lengths SHALL be proportional to their counts on a linear scale shared by all bars, starting at zero, on which the package with the most records fills the full width.

#### Scenario: SF-2.1 Largest package
- **WHEN** the chart is shown for `python` `latest`
- **THEN** `openssl-4.0` (147 fixed, 6 never affected) fills the full width
- **AND** its fixes-included segment is 147/153 of that width and its never-affected segment 6/153

#### Scenario: SF-2.2 Small package on the same scale
- **WHEN** the chart is shown for `python` `latest`
- **THEN** `gcc`'s bar (1 fixed) is 1/153 of the full width

#### Scenario: SF-2.3 Fixes not yet installed
- **WHEN** a source package has 10 fixes included, 2 never-affected records and 1 fix not yet installed (`python` `latest`'s `zlib`, modified to add one pending fix)
- **THEN** its bar has three segments of 10, 2 and 1 units, in that order

### Requirement: SF-3 Sort order
The chart SHALL list source packages with fixes not yet installed first, then sort by fixes included, then by never-affected records (each descending), then by name.

#### Scenario: SF-3.1 python latest
- **WHEN** the chart is shown for `python` `latest`
- **THEN** the bars are in the order `openssl-4.0`, `python-3.14`, `expat`, `py3-pip`, `glibc-2.44`, `zlib`, `gcc`, `sqlite`, `util-linux`, `brotli`, `ncurses`, `xz`

#### Scenario: SF-3.2 A fix not yet installed comes first
- **WHEN** `python` `latest`'s `zlib` is modified to have 1 fix not yet installed
- **THEN** `zlib` is the first bar, followed by `openssl-4.0`

### Requirement: SF-4 Fixes not yet installed are highlighted
The chart SHALL draw fixes not yet installed in a color reserved for them, and the Security tab SHALL also name each package with fixes not yet installed, and those fixes' IDs, as text.

#### Scenario: SF-4.1 Package with a fix not yet installed
- **WHEN** `python` `latest`'s `zlib` (installed version `1.3.2.1_rc20260917-r0`) is modified to have the fix not yet installed `CVE-2026-0001`
- **THEN** its bar ends with a segment in the not-yet-installed color
- **AND** the tab shows the text `zlib 1.3.2.1_rc20260917-r0: CVE-2026-0001`

#### Scenario: SF-4.2 No fixes not yet installed
- **WHEN** no source package has a fix not yet installed, as in `python` `latest` on 2026-10-09
- **THEN** no bar has a not-yet-installed segment

### Requirement: SF-5 Labels and legend
Each bar SHALL be labeled with its source package name and its fixes-included count as text, and the chart SHALL have a legend naming the three segments, so color is not the only cue.

#### Scenario: SF-5.1 Labels for python latest
- **WHEN** the chart is shown for `python` `latest`
- **THEN** the `openssl-4.0` bar is labeled "147 fixed" and the `sqlite` bar "0 fixed"
- **AND** the legend reads "Fixes included", "Never affected" and "Not yet installed"

### Requirement: SF-6 Text equivalent
The Security tab SHALL keep a table listing every source package (with or without records) with its installed version, fixes included and never-affected count, as the chart's text equivalent, and the chart itself SHALL be hidden from assistive technology.

#### Scenario: SF-6.1 Table alongside the chart
- **WHEN** the Security tab is opened for `python` `latest`
- **THEN** the table lists all 21 source packages, including `openssl-4.0` with 147 and 6 and `readline` with 0 and 0
- **AND** the chart is hidden from assistive technology

### Requirement: SF-7 No claim of being vulnerability-free
The fixes chart and its surrounding text SHALL describe what the Wolfi security database records, and SHALL NOT state or imply that an image or package has no vulnerabilities, is secure or is unaffected. Packages without bars SHALL be described as having no records in the Wolfi security database. The tab SHALL keep its note that the data doesn't list unfixed vulnerabilities.

#### Scenario: SF-7.1 Packages without records
- **WHEN** the chart is shown for `python` `latest`
- **THEN** a line reads "9 more source packages have no records in the Wolfi security database."

#### Scenario: SF-7.2 No package has records
- **WHEN** an image's source packages all have no records
- **THEN** no chart is drawn and the text says none of its source packages have records in the Wolfi security database
- **AND** no text on the tab says the image has no vulnerabilities or is secure

#### Scenario: SF-7.3 Note stays
- **WHEN** the chart is shown
- **THEN** the tab says the data doesn't list unfixed vulnerabilities and isn't a full scan

### Requirement: SF-8 Data, tag changes and unavailable images
The fixes chart SHALL be computed from the per-package summary the Security tab already loads, with no additional API request. It SHALL recalculate when the tag changes, and SHALL NOT be shown while the summary is loading or when it fails, including for subscription-only images.

#### Scenario: SF-8.1 No extra requests
- **WHEN** the chart is shown
- **THEN** no API request is made beyond the vulnerability summary request the tab already makes

#### Scenario: SF-8.2 Switching tag
- **WHEN** the user switches `python` from `latest` to `latest-dev`
- **THEN** the chart shows the 25 bars of `latest-dev`, starting with `openssl-4.0`, `python-3.14` and `curl`

#### Scenario: SF-8.3 Subscription-only image
- **WHEN** the Security tab is opened for a subscription-only image such as `loki-fips`
- **THEN** the existing subscription message is shown and no chart is drawn

### Requirement: SF-9 Layout, theme and security policy
The fixes chart SHALL be usable at 375 px width and in light and dark mode, and SHALL cause no Content Security Policy violations.

#### Scenario: SF-9.1 Phone width in dark mode
- **WHEN** the Security tab for `python` `latest-dev` is viewed at 375 px wide in dark mode
- **THEN** nothing overflows horizontally, every package name, count and legend entry is readable, and the browser console shows no Content Security Policy violations
