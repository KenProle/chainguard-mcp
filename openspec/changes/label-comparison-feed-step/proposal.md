# Proposal

## Why

The upstream comparison's progress bar (UC-9) counts the OSV.dev lookups, e.g. "221 of 221 packages checked" for `node:20`. Since UC-17, the comparison then reads each Chainguard advisory's current status from Chainguard's advisory feed. That step isn't counted, so the bar sits full for a second or two before the result appears, and looks stuck.

## What Changes

- The comparison job's status gains a `step` while it runs: `packages` during the OSV.dev lookups, then `advisories` while Chainguard's advisory feed is read.
- During the `advisories` step, the progress bar stays full and its label reads "Checking the current status of Chainguard's advisories…".

## Capabilities

### New Capabilities
None.

### Modified Capabilities
- `upstream-comparison`: UC-9 also names the advisory-feed step. Its new scenario is UC-9.2.

## Decisions

Made with the user on 2026-10-15:
- **Label the second step.** *Rejected:* adding the feed lookups to the progress total, which would make the total jump partway through.

## Out of Scope

- Counting individual feed lookups.
- Any change to what's compared or counted.
- Changes to the blocking `CompareUpstream` and the `compare_with_upstream` tool, which report no progress.

## Impact

- **Go:**
  - `compare.go`'s progress callback also carries the step.
  - `compare_job.go` stores the step and returns it as `step` in the job status. This is an added JSON field on `GET /api/compare-upstream`.
- **Web UI:** `web/src/api.ts` gains the optional `step` field, and `UpstreamComparison.tsx`'s progress label uses it.
- **Dependencies:** none.
