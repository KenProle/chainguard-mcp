# Spec Delta

## Purpose

Lets users compare two tags of the same Chainguard image, typically the minimal and `-dev` variants, to see the difference in size, packages and capabilities before choosing one.

## ADDED Requirements

### Requirement: Default comparison pair
The Compare tab SHALL open on a `-dev` pair: `latest` and `latest-dev` when both tags exist, otherwise the first base tag `X` that has an `X-dev` counterpart.

#### Scenario: Image with latest and latest-dev
- **WHEN** the user opens the Compare tab for a free image whose tags include `latest` and `latest-dev`
- **THEN** the comparison shows `latest` and `latest-dev`

#### Scenario: Selecting another base tag
- **WHEN** the user selects `next` as the base tag of an image whose tags include `next` and `next-dev`
- **THEN** the comparison shows `next` and `next-dev`

### Requirement: Choosing any two tags
The Compare tab SHALL let the user choose any two different tags of the same image as the pair.

#### Scenario: Comparing latest with latest-slim
- **WHEN** the user chooses `latest` and `latest-slim` for the `node` image
- **THEN** the comparison shows `latest` and `latest-slim`

#### Scenario: Same tag on both sides
- **WHEN** the user tries to choose the same tag for both sides
- **THEN** the Compare tab does not offer that combination

### Requirement: No variant to compare
If the image has fewer than two tags, or has no `-dev` pair and the user hasn't chosen another pair, the Compare tab SHALL show "This image has no -dev variant to compare against" instead of a chart, keeping the tag selectors when there are two or more tags.

#### Scenario: Image with no -dev tag
- **WHEN** the user opens the Compare tab for `static`, whose tags are `latest`, `latest-glibc`, `latest-glibc-tzdata` and `latest-tzdata`
- **THEN** the message "This image has no -dev variant to compare against" is shown with no chart
- **AND** the tag selectors are available

#### Scenario: Image with a single tag
- **WHEN** the user opens the Compare tab for an image with only one tag
- **THEN** the message is shown and no tag selectors are offered

### Requirement: Shareable comparison
The Compare tab SHALL keep the compared tags and the architecture in the URL, so reloading or sharing the link shows the same comparison.

#### Scenario: Reloading a comparison
- **WHEN** the user compares `latest` with `latest-slim` on arm64 and reloads the page
- **THEN** the same tags and architecture are shown

### Requirement: Per-tag summary
The Compare tab SHALL show, for each compared tag, its package count and its compressed download size for the selected architecture.

#### Scenario: python latest and latest-dev on amd64
- **WHEN** the user compares `python` `latest` with `latest-dev` on amd64
- **THEN** `latest` shows 29 packages and its download size
- **AND** `latest-dev` shows 76 packages and its download size

### Requirement: Size difference
The Compare tab SHALL state the size difference both as an absolute amount and as a ratio of the larger to the smaller size.

#### Scenario: Large difference
- **WHEN** the two tags' download sizes are 26.9 MB and 272.6 MB
- **THEN** the difference is shown as "+245.7 MB" and "10.1× larger"

#### Scenario: Nearly equal sizes
- **WHEN** the two sizes are within 1% of each other
- **THEN** the Compare tab says "about the same size" instead of a ratio

### Requirement: Package differences
The Compare tab SHALL list, each with a count, the packages only in the first tag, the packages only in the second tag, and the packages in both at different versions.

#### Scenario: Packages added by latest-dev
- **WHEN** the user compares `python` `latest` with `latest-dev`
- **THEN** "Only in latest-dev" lists 47 packages, including `gcc`, `bash`, `git` and `make`

#### Scenario: Empty group
- **WHEN** one of the three groups has no packages
- **THEN** that group shows "None"

### Requirement: Capability differences
The Compare tab SHALL state, for each compared tag, whether it has a shell and whether it has the `apk` package manager.

#### Scenario: Minimal and dev variants
- **WHEN** the user compares `python` `latest` with `latest-dev`
- **THEN** `latest` is shown with no shell and no `apk`
- **AND** `latest-dev` is shown with a shell and `apk`

### Requirement: Size chart
The Compare tab SHALL show a bar chart of the two download sizes on a shared scale that starts at zero.

#### Scenario: Bars are proportional
- **WHEN** one tag's download size is ten times the other's
- **THEN** its bar is ten times as long

### Requirement: Architecture selection
The Compare tab SHALL let the user switch between the amd64 and arm64 architectures, and every number, list and chart SHALL show the selected architecture's data.

#### Scenario: Switching to arm64
- **WHEN** the user switches a comparison from amd64 to arm64
- **THEN** the sizes, package counts, package lists and chart show arm64 data

### Requirement: Subscription-only images
For a subscription-only image, the Compare tab SHALL show the existing subscription message and SHALL NOT attempt a comparison.

#### Scenario: Paid image
- **WHEN** the user opens the Compare tab for `loki-fips`
- **THEN** the subscription message is shown and no comparison is attempted

### Requirement: Loading and error states
While either tag's data is loading, the Compare tab SHALL show a loading indicator. If either tag's data fails to load, it SHALL show that error with a retry option and SHALL NOT present the other tag's data as a comparison.

#### Scenario: One side fails
- **WHEN** the data for one compared tag fails to load and the other succeeds
- **THEN** the error and a retry option are shown
- **AND** no partial comparison is shown

#### Scenario: Still loading
- **WHEN** either tag's data is still loading
- **THEN** a loading indicator is shown

### Requirement: Text equivalent
The Compare tab SHALL present the sizes, package counts and ratio as text on the page, not only in the chart graphics.

#### Scenario: Screen reader user
- **WHEN** the comparison is read without the chart graphics
- **THEN** both sizes, both package counts and the ratio are available as text

### Requirement: Layout, theme and security policy
The Compare tab SHALL be usable at 375 px width and in light and dark mode, and SHALL cause no Content Security Policy violations.

#### Scenario: Phone width in dark mode
- **WHEN** the comparison is viewed at 375 px wide in dark mode
- **THEN** nothing overflows horizontally, all text is readable, and the browser console shows no Content Security Policy violations
