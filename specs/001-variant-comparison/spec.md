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

Each criterion is either a **scenario** (Given / when / then) or a **rule** in [EARS](https://alistairmavin.com/ears/) form: "The Compare tab shall…", "When *trigger*, the Compare tab shall…", "While *state*, …" or "If *unwanted condition*, then …". "The pair" means the two tags being compared.

### Choosing what to compare

- **AC-1.1:** **Given** a free image with both `latest` and `latest-dev` tags, **when** the user opens the Compare tab, **then** the pair is `latest` and `latest-dev`.
- **AC-1.2:** **Given** an image whose tags include another `X` / `X-dev` pair (e.g. `next` / `next-dev`), **when** the user selects `X` as the base tag, **then** the pair becomes `X` and `X-dev`.
- **AC-1.3:** The Compare tab **shall** let the user choose any two different tags of the same image as the pair (e.g. `node`'s `latest` and `latest-slim`).
- **AC-1.4:** **If** the image has fewer than two tags, or has no `-dev` pair and the user hasn't chosen another pair, **then** the Compare tab **shall** show "This image has no -dev variant to compare against" instead of a chart, and **shall** keep the tag selectors available when the image has two or more tags.
- **AC-1.5:** The Compare tab **shall** keep the pair and the architecture in the URL, so that reloading or sharing the link shows the same comparison.

### What it shows

- **AC-1.6:** The Compare tab **shall** show, for each tag in the pair, the package count and the compressed download size for the selected architecture.
- **AC-1.7:** The Compare tab **shall** show the size difference both as an absolute amount (e.g. "+245.7 MB") and as a ratio (e.g. "10.1× larger"). **If** the two sizes are within 1% of each other, **then** it **shall** say "about the same size" instead of a ratio.
- **AC-1.8:** The Compare tab **shall** split the packages into three groups with counts: only in the first tag, only in the second tag, and in both at different versions. **If** a group is empty, **then** it **shall** show "None".
- **AC-1.9:** The Compare tab **shall** state, for each tag, whether it has a shell and whether it has `apk`.
- **AC-1.10:** The Compare tab **shall** show a bar chart of the two download sizes on a shared scale starting at zero.
- **AC-1.11:** **Given** a comparison is shown, **when** the user switches the architecture (amd64 / arm64), **then** every number, list and chart shows that architecture's data.

### States and quality

- **AC-1.12:** **Given** a subscription-only image, **when** the user opens the Compare tab, **then** the existing subscription message is shown and no comparison is attempted.
- **AC-1.13:** **While** either tag's data is loading, the Compare tab **shall** show a loading indicator. **If** either tag's data fails to load, **then** it **shall** show that error with a retry option and **shall not** present the other tag's data as a comparison.
- **AC-1.14:** The Compare tab **shall** present the sizes, counts and ratio as text on the page, not only in the chart graphics.
- **AC-1.15:** The Compare tab **shall** be usable at 375 px width and in light and dark mode, and **shall** cause no Content Security Policy violations.

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
