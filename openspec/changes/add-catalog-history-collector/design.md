# Design

## Context

See `proposal.md` for why, and `specs/catalog-history/spec.md` for the requirements (CH-1 to CH-10).

Current state:
- `Catalog.Images` (`catalog.go`) fetches and parses the `images.chainguard.dev` sitemap, keeping `/directory/image/<name>/overview` URLs, with a one-hour cache. `Registry.PublicStatus` (`registry.go`) checks many images' free-tier status, 8 at a time with per-image 24-hour caching, and leaves out images whose check fails. Together they're what `list_images` with `free_only` uses, so a live snapshot can reuse them unchanged (CH-7).
- The binary has three flags (`-http`, `-version`, `-sbom-dir`); all code is one `main` package, so a separate `cmd/` program couldn't import this code. New modes are therefore flags on the existing binary.
- `main` is protected by a ruleset requiring pull requests and the five CI checks; only the admin role can bypass it. A workflow's `GITHUB_TOKEN` can't, so the history can't be committed to `main` (CH-10). The ruleset covers only the default branch.
- CI (`ci.yml`) runs on pushes to `main` only, so commits to another branch don't trigger it.
- Measured on 2026-10-10: the sitemap lists 3,171 images (59 free); a full free-tier check takes about 28 seconds; the Internet Archive's CDX index lists 23 captures of the sitemap with status 200, of which 18 are `application/xml` and 5 are HTML pages, covering 17 months from 2024-04 to 2026-10.

**No API changes.** The MCP tools, the web API and the web UI are untouched; the follow-up change reads the history.

## Goals / Non-Goals

**Goals:**
- Start a gap-free daily history as soon as possible, with name history back to 2024.
- A file format that the follow-up change (and anyone else) can read without this code: plain JSON lines.
- Snapshots that are safe to rerun and that refuse obviously broken input instead of recording it.

**Non-Goals:**
- Reading, serving or displaying the history (follow-up change `add-catalog-history-view`).
- Free-tier history before the first live snapshot.

## Decisions

### 1. Snapshot logic in `history.go`, pure where possible
- Types: `HistoryLine` with JSON fields `date`, `source` (`archive` | `live`), `total`, `free` (`*int`, null when unknown), `added`, `removed`, `became_free`, `became_subscription`, `unchecked`, plus `images` (only on the first line, CH-2) and `free_images` (only on the first live line, CH-2). Empty lists are written as `[]`, not omitted, so readers don't need special cases.
- Pure functions, unit-tested in `unit_test.go`:
  - `replayHistory(lines) (images, free set, hasLive bool, lastDate)`: rebuilds the latest state from the file (CH-2.1).
  - `diffSnapshot(prev state, names, free map, checked set) HistoryLine`: the change lists of CH-3 and CH-4.
  - `checkPlausible(prevTotal, line) error`: the CH-6 rules (no images, or more than 5% removed).
- `recordHistory(ctx, svc, path, now)`: reads the file, skips if the last line has today's UTC date (CH-5), fetches names with `Catalog.Images` and statuses with `Registry.PublicStatus` (CH-7), diffs, checks plausibility, and appends one line. The append writes the whole line in one `Write` to a file opened with `O_APPEND`, so a crash can't leave a half line followed by a good one.

*Alternative:* a `Service` method. Rejected for now: CLAUDE.md puts logic in `Service` so the MCP tools and web UI share it, and neither front end uses recording. The follow-up's reading logic will be a `Service` method.

### 2. Two flags on the binary
- `-record-history <file>`: take a live snapshot (decision 1), print "added line for 2026-10-11: 3,173 images, +2 −0" or "skipped: already recorded today", exit 0; exit 1 with the error otherwise (CH-6, CH-9.2).
- `-backfill-history <file>`: decision 3; exit 1 if the file has lines (CH-8.3).
Both run and exit without starting the MCP server. `main.go` handles them after `flag.Parse`, like `-version`.

### 3. Backfill from the Internet Archive
- List captures with the CDX API: `https://web.archive.org/cdx/search/cdx?url=images.chainguard.dev/sitemap.xml&output=json&fl=timestamp,mimetype&filter=statuscode:200`.
- Group by month (`timestamp[:6]`); in each month try captures in time order, fetching `https://web.archive.org/web/<timestamp>id_/https://images.chainguard.dev/sitemap.xml` (the `id_` form returns the original bytes, not the Archive's HTML wrapper), until one parses as a sitemap with at least one image page (CH-8.2).
- Parse with the same code as `Catalog`: move the decoding in `Catalog.Images` into `parseSitemap(io.Reader) ([]string, error)` and use it in both places, so the archive and the live snapshot agree on what counts as an image.
- Pause 2 seconds between Archive requests (CH-8); about 20 requests in all.
- The first line holds every name (CH-2); each later line diffs with the previous one. Lines are dated by the capture's date.
- The base URLs are fields on a small `Archive` struct so tests can point it at a fake server.

### 4. The `data` branch and its file
- An orphan branch `data` holding only `catalog-history.jsonl` and a short `README.md` that describes the format and links to this spec. It's created once, from the backfill run locally (task 5.2), and pushed with the user's approval.
- *Alternative:* a `data/` folder on `main`. Rejected: blocked by the ruleset, and each daily commit would run the full CI.

### 5. The scheduled workflow, `.github/workflows/history.yml`
```
on:  schedule (daily, 06:17 UTC: off the hour, as GitHub recommends)
     workflow_dispatch
permissions: contents: write
concurrency: { group: catalog-history, cancel-in-progress: false }
job snapshot (ubuntu-latest):
  checkout main                      -> builds the snapshot command
  setup-go (go-version-file: go.mod)
  go build -o chainguard-mcp .
  checkout ref data into ./history   -> the file to append to
  ./chainguard-mcp -record-history history/catalog-history.jsonl
  in ./history: commit and push only if the file changed
```
A refused snapshot exits 1, which fails the job before the commit step (CH-9.2). The commit uses the `github-actions[bot]` identity and a message like `snapshot 2026-10-11: 3,173 images (+2 −0), 59 free`. The web UI build isn't needed, so the job doesn't install Node.

### 6. The config exception
Extend the "Data comes only from Chainguard's public, anonymous endpoints" bullet in `openspec/config.yaml`'s `context`: the exception is this project's catalog history file (published on its `data` branch, readable by the server in the follow-up) and the Internet Archive copies of Chainguard's sitemap used to backfill it. It goes in the `context` block scalar, so it needs no quoting; CI checks the config still parses.

### 7. Tests
- `unit_test.go`: `replayHistory`, `diffSnapshot` and `checkPlausible` against small hand-written histories for CH-1 to CH-6, named by scenario ID.
- A new `history_test.go`: `recordHistory` against `newFakeService` (CH-5, CH-7.1: the snapshot's total and free images match `ListImages` with `FreeOnly`), and the backfill against a fake Archive server serving a CDX list, two sitemap captures and one HTML capture (CH-8).
- `live_test.go` (`CHAINGUARD_LIVE=1`): the backfill's capture listing finds at least 17 months since 2024-04, a stable property rather than an exact count.

## Risks / Trade-offs

- **[GitHub disables scheduled workflows in public repositories after 60 days without repository activity.]** Pushes to `data` may not count. → The README on `data` says so; if the job is disabled, re-enabling it is one click, and the gap shows as a missing date range. The follow-up change can surface "last snapshot" age in the UI so a stall is visible.
- **[The free-tier check depends on Chainguard's token endpoint.]** A partial outage makes many checks fail. → Failed checks keep the previous status and are listed as unchecked (CH-4), so an outage can't masquerade as mass free/subscription changes.
- **[A broken or truncated sitemap.]** → CH-6 refuses a snapshot that loses more than 5% of images, and the job fails visibly.
- **[Archive precision.]** Before the first live snapshot, an image's first appearance is only known to within a month or more. → Lines carry `source: archive`, so readers can say "between" two dates.
- **[The token endpoint load from GitHub's IPs.]** About 3,200 requests a day, sequential with 8 workers. → Same as one first "Free only" check; once a day.
- **[History rewritten by mistake.]** Someone could force-push `data`. → Task 5.3 offers the user a ruleset for `data` that blocks deletion and force-pushes while still allowing the workflow's normal pushes.

## Migration Plan

Additive. Create `data` from the backfill, merge the code and workflow to `main`, then trigger the workflow once by hand to take the first live snapshot. Rollback: disable the workflow; the history file stays as it is.
