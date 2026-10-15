# Design

## Context

See proposal.md for the motivation and specs/catalog-history-view/spec.md (HV-1 to HV-16) for the behavior. Relevant current state:

- `history.go` already parses and replays the file:
  - `HistoryLine` holds one snapshot. Its `Images` and `FreeImages` fields appear only on the first line and the first live line.
  - `parseHistory` reads the lines.
  - `replayHistory` rebuilds the latest state and fails when a line's `total` doesn't match, which is how a damaged file is detected.
  - Collection runs outside `Service` (`-record-history`). Nothing reads the file at runtime.
- The file lives only on the `data` branch, at `https://raw.githubusercontent.com/KenProle/chainguard-mcp/data/catalog-history.jsonl`. On 2026-10-14 it had 22 lines (96 KB). It grows by about 150 bytes a day.
- `openspec/config.yaml` already allows this file as a data source.
- `go:embed` can only embed files in the source tree. `webui.go` solves the same problem for the UI with a committed `web/dist/.gitkeep` and a build step that fills the folder.
- The catalog page switches views through `view` in the query string (`list` by default, `map`), with segmented buttons from `web/src/styles.ts`.

## Goals / Non-Goals

**Goals:**
- One `Service` method computes everything (points, timeline entries, first appearances, data age) for the API and MCP.
- Release binaries and Docker images work offline with the history as of their build, and running servers stay current without a rebuild.
- The UI uses plain SVG and native HTML (`<details>`) under the existing CSP.

**Non-Goals:**
- Changing `HistoryLine`, the file format or the collector.
- Caching derived results across copies. Recomputing from 22 lines and about 3,200 names per request is cheap, under a millisecond.

## Decisions

### 1. Embedding: a committed placeholder folder filled by the build

- `history_embed.go` embeds `//go:embed all:historydata`. The folder holds a committed `historydata/.gitkeep`, and `historydata/catalog-history.jsonl` is gitignored.
- With only `.gitkeep` present, there is no embedded copy (HV-3.2).
- `.goreleaser.yaml` gets a `before` hook that downloads the file into `historydata/` (`curl -fsSL`). If the download fails, the release fails, because a release must embed the history.
- The `Dockerfile` downloads it in the web stage, because `cgr.dev/chainguard/go:latest` has neither `curl` nor `wget` and `cgr.dev/chainguard/node:latest-dev` has `wget` (checked on 2026-10-14).
  - The web stage runs `mkdir -p historydata && wget -q -O historydata/catalog-history.jsonl <url> || rm -f historydata/catalog-history.jsonl`.
  - The Go stage copies the folder with `COPY --from=web`.
  - A failed download leaves the folder empty, so an offline `docker build` still works and the image just has no embedded copy.
- CI's Docker smoke test also requests `/api/history` and expects 200.

*Alternatives:*
- **Commit a copy to `main` periodically.** Rejected: it needs a manual or bot commit to a protected branch, which is what the `data` branch exists to avoid.
- **A `-fetch-history` flag reusing the runtime fetcher.** Rejected for now: `curl` already exists in both build environments, and the runtime validates any copy anyway. An invalid embedded file is treated as no embedded copy, with a log line.

### 2. `HistorySource`: the copy in use, refreshed at most hourly

`HistorySource` (`history_view.go`) holds:
- the embedded bytes
- `URL`, `HTTP`, `TTL` (1 hour) and `Timeout` (5 seconds)
- a `Now` clock for tests
- the copy in use: its lines, the replayed data from decision 3, where it came from (`build` or `data_branch`) and when it was fetched

`current(ctx)` returns the copy in use. When the last fetch attempt is older than the TTL, it fetches first:
- **Only one fetch at a time:** a mutex and an "in flight" flag make concurrent callers use the current copy.
- **The timeout bounds the wait** (HV-2.2), independent of the request context.
- **Validation:** a fetched copy must parse, replay, and not end before the copy in use (HV-1). Failures and rejections are logged and the copy in use is kept.
- **Failed fetches count as attempts,** so an offline server tries once an hour rather than on every request.
- **No copy at all** gives `ErrHistoryUnavailable`, which `writeError` maps to 503 `history_unavailable`.

### 3. Derived data: one replay per copy

When a copy is installed, one pass over the lines builds:
- each line's full image set as a sorted slice. 22 slices of up to 3,171 names take about 2 MB, which is fine. Searches then count matching names per point without replaying.
- per line: `added`, split into new images and images back after being missed (archive lines only, using a set of every name seen so far), plus `removed`, `became_free` and `became_subscription`
- `firstSeen[name]`: the index of the first line listing the image (HV-10)

**Dates (HV-7):** a line's changes get an exact date when the previous line's date is the day before. Otherwise they span `from` (the previous line's date) to `to`. The first line is the first capture.

**Timeline (HV-8):**
- Lines are walked newest first, after filtering names by the query.
- Consecutive lines whose filtered changes are empty merge into one `no_changes` entry, `from` the earliest of them `to` the latest.
- Archive lines put missing images in `not_in_capture` and returning ones in `back`. Live lines use `removed` (HV-9).
- The first live line's entry adds `free_first_recorded: 59`.
- With a query, lines before the first one listing a matching image are dropped, so no "no changes" or zero-image first-capture entry is emitted for them (HV-17).
- Each entry carries `archive` (its line is an Internet Archive capture, or it's the first capture). The UI uses it to group the archive era (HV-18), so the grouping needs no date logic in the browser.

### 4. Output types (Go, mirrored in `web/src/api.ts`)

```
HistoryReport {
  last_snapshot, copy_source ("build" | "data_branch"), fetched_at?, days_old, stalled, note,
  query?, since?,
  points:  [{date, source ("archive" | "live"), total, free?}],
  entries: [{kind ("first_capture" | "changes" | "no_changes"), date?, from?, to?, archive,
             total, added[], back[], removed[], not_in_capture[], became_free[],
             became_subscription[], free_first_recorded?}],
  totals_at_since?: {date, total, free?}, total_now, free_now?
}
CatalogSince { earliest_capture: bool, date?, from?, to?, text }
```

- **Full lists:** `entries` carry the complete name lists; the UI shows 10 and puts the rest in a `<details>` (HV-8.3).
- **Since:** `since` keeps entries whose `to` (or `date`) is after it, so an entry spanning `since` is kept with its range. `totals_at_since` is the last point on or before `since` (HV-12.1).
- **Stalled:** `stalled` is true when the last snapshot is more than 2 days before today's UTC date. `note` then carries the stalled wording (HV-4.2).

### 5. Service, API and MCP

- **`Service.History`** (`*HistorySource`) is set up by `NewService`.
- **`Service.CatalogHistory(ctx, HistoryInput{Query, Since})`** validates `since` (`time.DateOnly`, an `*InputError` otherwise) and builds the report.
- **`Service.ImageDetails`** adds `CatalogSince` from the copy in use. If no copy exists or the image isn't listed, it uses "not yet recorded" (HV-10.4) or leaves the field out (HV-13.2), and never fails the details call.
- **Web API:** `apiRoutes` gets `GET /api/history`, plus a README API-table row.
- **MCP:** `tools.go` registers `catalog_changes`. Its description says archive-era dates are ranges between captures and that the result reports its own age.

These are API changes: a new endpoint, a new error code, a new MCP tool and a new optional field on image details.

### 6. Web UI

- **Catalog page:** `CatalogPage` gets a third segmented button, `History` (`view=history`). In this view the pagination, grouping and free-only controls are hidden; free status before 2026-10-10 is unknown, so a free filter would mislead. The search box stays.
- **`HistoryView`** (`web/src/components/HistoryView.tsx`) queries `['history', query]` and renders:
  - the age line ("Last snapshot: 2026-10-14", plus the stalled note)
  - three tiles (images now, free now, change since the first point), in the same style as the Security tab's stat tiles
  - `GrowthChart`
  - the timeline: live-era entries first, then the archive-era entries inside a `<details>` titled "Before daily tracking (Internet Archive captures)" with their count. Each name list shows 10 names and the rest in a nested `<details>`.
- **`GrowthChart`:** plain SVG.
  - Axes and shapes: a time-scaled x axis and a linear y axis from 0, a polyline for the total and one for free from the first live point. Archive points are hollow circles and live points are filled.
  - Text equivalent: a table of every point (HV-15.2). The SVG is `aria-hidden`.
  - Layout: sized by `viewBox` and attributes only, as `HorizontalBars` is, so it needs no inline styles.
- **Image page:** the header shows "In the catalog: <text>" from `details.catalog_since` (HV-16.1).
- **Shared code:** formatting helpers and the entry wording live in `web/src/styles.ts`, or a sibling helper module without components, so Vite fast refresh keeps working.

### 7. Tests

- **The real history as a fixture:** `testdata/catalog-history-2026-10-14.jsonl` is a copy of the `data` branch file, so the spec's figures (675, 3,171, 204 to 1,315 for `fips`, `verdaccio`, `argocd-fips-repo-server`) are asserted exactly.
- **Fake data-branch server:** an `httptest` server serving a configurable body and status, with a request counter, tests HV-1 and HV-2 (newer, offline, damaged, slow, hourly).
- **Fake backends:**
  - `newFakeBackend` gives `HistorySource` the real fixture as its embedded copy and an unreachable URL, so existing tests stay offline and image details gain a `catalog_since`.
  - A separate helper builds a source with no embedded copy (HV-3.2).
- **Web fixture:** `web/src/test/fixtures/historyReport.json` is generated from the Go report for the fixture (`UPDATE_FIXTURES=1`, as `TestCatalogFamiliesFixture` does), so page tests and Go agree.

### Dependencies

None added.

## Risks / Trade-offs

- [The repository is renamed or forked] → The URL is one constant next to the embed; a fork keeps working offline from its embedded copy.
- [`raw.githubusercontent.com` caches for about 5 minutes, or is unreachable] → Freshness is daily anyway; failures fall back to the copy in use (HV-1.2) and are retried after an hour.
- [The collector stalls] → `stalled` and the note make it visible (HV-4.2) rather than silently showing old data as current.
- [An archive capture misses images] → Wording says "not in the capture" and "back" (HV-9), and ranges replace exact dates (HV-7).
- [The first live line's 5 additions are relative to the 2026-10-08 capture] → They read as a range (HV-7.1), not as added on 2026-10-10.
- [Image-set memory] → About 2 MB for 22 lines today, growing by one set per day (roughly 35 MB a year at 3,200 names). If this grows too large, store only the change lists and replay per query. Note it here and revisit after a few months.
- [Development builds show no history offline] → The 503 state is explicit (HV-3.2). Building with the file locally is one `curl` away, documented in the README.

## Migration Plan

None needed. The feature is additive. Rolling back means removing the view; the `data` branch and collector are untouched.
