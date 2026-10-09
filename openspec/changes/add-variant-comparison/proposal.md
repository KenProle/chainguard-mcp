# Proposal

## Why

Chainguard publishes most images as a minimal variant (`latest`: no shell, no package manager) and a development variant (`latest-dev`). The advice is to build in `-dev` and run in the minimal image, but nothing in the UI shows what that choice is worth, and the difference is large. For `python` on linux/amd64 (October 2026), `latest` has 29 packages and is 26.9 MB; `latest-dev` has 76 packages and is 272.6 MB, adding 47 packages (including `gcc`, `bash`, `git`, `make`) and removing or changing none.

Users:
- A developer migrating a Dockerfile wants to see what `-dev` adds, to decide which variant belongs in the build stage and which in the runtime stage.
- A security reviewer wants the size and package difference between variants, to justify minimal images in production.

## What Changes

- A new **Compare** tab on the web UI's image page compares two tags of the same image: package counts, download sizes, a size chart, capability differences (shell, `apk`) and the packages that differ.
- The comparison defaults to the `-dev` pair and lets the user pick any two tags and either architecture; the selection is kept in the URL.

## Capabilities

### New Capabilities
- `image-variant-comparison`: comparing two tags of one image in the web UI, covering their sizes, package differences and capabilities.

### Modified Capabilities
None. No specs exist yet; this is the project's first OpenSpec change.

## Decisions

Made with the user on 2026-10-08:
- **Where it lives:** a new Compare tab on the image page, keeping the existing tabs focused. *Rejected:* a section at the bottom of the Packages tab.
- **Which tags:** any two tags of the image, defaulting to the `-dev` pair. *Rejected:* only `X` vs `X-dev` pairs, which can't compare `node`'s `latest` with `latest-slim`.
- **Size metric:** compressed download size, which the API already returns. Uncompressed size would need extra registry requests.

## Out of Scope

- Comparing two different images (e.g. `python` vs `node`), or a Chainguard image against an upstream one.
- File-level or layer-level differences, and per-package sizes (the SBOM doesn't include them).
- Any change to the MCP tools.

## Impact

- **Web UI:** a new tab on the image page. The comparison reuses existing API calls (tags, details and packages for each tag), so no API or Go changes are expected; `design.md` will confirm.
- **Dependencies:** none expected.
- **Migrated from** `specs/001-variant-comparison/spec.md`; its acceptance criteria AC-1.1 to AC-1.15 become the requirements in this change's spec.
