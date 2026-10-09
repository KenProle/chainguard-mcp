# 001: Variant comparison

**Status:** Approved (2026-10-08)

## Problem

Chainguard publishes most images in a minimal variant (`latest`: no shell, no package manager) and a development variant (`latest-dev`: shell, `apk`, build tools). The usual advice is to build in `-dev` and run in the minimal image, but the UI gives no sense of what that choice is worth.

The difference is large. For `python` on linux/amd64 (October 2026):

| | `latest` | `latest-dev` |
|---|---|---|
| Packages | 29 | 76 |
| Download size | 26.9 MB | 272.6 MB |
| Shell / `apk` | no / no | yes / yes |

`latest-dev` adds 47 packages (including `gcc`, `bash`, `git`, `make`) and removes or changes none.

## Users and stories

- **US-1:** As a developer migrating a Dockerfile, I want to see what the `-dev` variant adds over the minimal one, so I can decide which belongs in my build stage and which in my runtime stage.
- **US-2:** As a security reviewer, I want to see the size and package difference between variants, so I can justify using minimal images in production.

## Acceptance criteria

Criteria use Given / When / Then. "The pair" means the two tags being compared.

### Choosing what to compare

- **AC-1.1:** Given a free image, when the user opens the comparison, then the pair defaults to `latest` and `latest-dev` if both tags exist.
- **AC-1.2:** Given an image whose tags include another `X` / `X-dev` pair (e.g. `next` / `next-dev`), when the user selects that base tag, then the pair becomes `X` and `X-dev`.
- **AC-1.3:** Given the tag list, the user can choose any two different tags of the same image to compare (e.g. `node`'s `latest` and `latest-slim`).
- **AC-1.4:** Given an image with fewer than two tags, or no `-dev` pair and no other tag selected, then no chart is shown and the message "This image has no -dev variant to compare against" appears, with the tag selectors still available if there are two or more tags.
- **AC-1.5:** The selected pair and architecture are kept in the URL, so reloading or sharing the link shows the same comparison.

### What it shows

- **AC-1.6:** For each tag in the pair, the comparison shows the package count and the compressed download size for the selected architecture.
- **AC-1.7:** The size difference is shown both in absolute terms (e.g. "+245.7 MB") and as a ratio (e.g. "10.1× larger"). When the two sizes are within 1%, it says "about the same size" instead of a ratio.
- **AC-1.8:** The packages are split into three groups, each with a count: only in the first tag, only in the second tag, and in both but at different versions. Empty groups say "None".
- **AC-1.9:** Capability differences are called out explicitly: whether each tag has a shell and whether it has `apk`.
- **AC-1.10:** A visual size comparison (bar chart) shows the two download sizes on a shared scale starting at zero.
- **AC-1.11:** Given the architecture toggle (amd64 / arm64), when the user switches it, then every number, list and chart updates to that architecture.

### States and quality

- **AC-1.12:** Given a subscription-only image, the existing subscription message is shown and no comparison is attempted.
- **AC-1.13:** While either side is loading, a loading indicator is shown; if either side fails, its error is shown with a retry option and the other side's data is not presented as a comparison.
- **AC-1.14:** The chart has a text equivalent: the sizes, counts and ratio are available as text in the page, not only in the chart graphics.
- **AC-1.15:** The comparison is usable at 375 px width, in light and dark mode, and causes no Content Security Policy violations.

## Non-functional requirements

- Uses only data the API already provides (tags, details and packages for each tag). At most 5 API requests for a comparison (1 tags + 2 details + 2 packages).
- No new third-party chart library; any added dependency must be justified in `design.md`.

## Decisions

- **D1: Where it lives.** A new "Compare" tab on the image page, so the existing tabs stay focused. (Rejected: a section at the bottom of the Packages tab.)
- **D2: Which tags can be compared.** Any two tags, defaulting to the `-dev` pair (AC-1.1 to AC-1.3). (Rejected: only `X` vs `X-dev` pairs, which couldn't compare `latest` with `latest-slim`.)
- **D3: Size metric.** Compressed download size, which the API already returns. Uncompressed (on-disk) size would need extra registry requests and is out of scope.

## Out of scope

- Comparing two different images (e.g. `python` vs `node`), or a Chainguard image against an upstream one.
- File-level or layer-level differences.
- Package size per package (the SBOM doesn't include it).
