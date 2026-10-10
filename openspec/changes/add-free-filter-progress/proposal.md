# Proposal

## Why

Ticking "Free only" on the catalog's List view makes the server check every image's free-tier status before it answers. The first time, that takes about 30 seconds (measured on 2026-10-10: 3,171 images, 59 free, in 13 batches of 250 taking about 2.2 s each). All the page shows meanwhile is "Loading images…" with a hint, so the user can't tell whether anything is happening or how long is left. A plain loading message is fine for a few seconds; for a 30-second wait it isn't.

## What Changes

- When "Free only" is on and the statuses aren't cached yet, the List view checks them first in batches of 250 images (one request at a time, through the existing image list endpoint), and shows a **progress bar** that fills as each batch returns, labeled with the count checked and an estimate of the time left. When the check finishes, it shows the free images as today.
- If the statuses are already cached (the server keeps them for 24 hours), the batches return in milliseconds and no progress bar appears.
- A project-wide rule is added to `openspec/config.yaml`: any wait that can take more than about 3 seconds shows progress or the expected time, never only "Loading".

## Capabilities

### New Capabilities
- `catalog-free-filter`: the catalog List view's "Free only" filter, including how the first free-tier check reports its progress.

### Modified Capabilities
None. `catalog-map` (CM-5.3) says the List view keeps "Free only" and pagination, which still holds; the Map view's own batched status line (CM-8) is unchanged.

## Decisions

Made with the user on 2026-10-10:
- **Show real, determinate progress** rather than "Loading": a bar that fills, with how much is left. *Rejected:* a spinner or indeterminate bar (doesn't say how long is left), and a bar driven by a timer alone (would show progress that isn't real).
- **A 3-second rule for the whole UI**, recorded in `openspec/config.yaml` so every future change is checked against it, not just this one.
- **Build it with a cheaper model (Sonnet 5.5)** from this plan, with a review on the current model before archiving. This affects how the work is done, not what is built.

Made in this plan (open to change at review):
- **Batches of 250**, not the Map view's 1,000: about 13 steady steps of ~2 s instead of 4 jumps of ~8 s, for the same number of checks. *Rejected:* a server-side progress endpoint (needs Go and API changes, and batching already gives real progress).

## Out of Scope

- The Map view: it keeps its batches of 1,000 and its text status line (CM-8), which already reports progress. Moving it to the shared progress bar is a possible follow-up.
- Any Go, API or MCP change. The `list_images` MCP tool's `free_only` still answers in one call.
- Speeding up the check itself (more parallel token requests would put more load on Chainguard's token endpoint).
- Other waits in the UI (image details, SBOMs): the new config rule applies to future changes; existing pages aren't audited in this change.

## Impact

- **Web UI:** `web/src/pages/CatalogPage.tsx` (List view), a new progress bar component in `web/src/components/`, pure progress helpers with unit tests, page tests, and the README's catalog description.
- **Planning:** one rule added to `openspec/config.yaml`.
- **API, Go server, MCP tools:** no changes. **Dependencies:** none.
