# Design

## Context

`Service.compareUpstream` (`compare.go`) makes one `OSV.QueryAll` call for both sides' lookups. It then runs `summarize(subset(results, …))` for each side, which uses `classifyAll` → `classify` (`osv.go`). `classify` returns fix available for any `fixed` event in the package's own entry, else won't fix for Debian's urgencies, else no fix yet. On the Chainguard side this reads OSV.dev's copy of each CGA record, and OSV.dev's query matches against that copy. So an advisory that Chainguard has since fixed in the installed version is still returned, still has no `fixed` event in the stale copy, and counts as no fix yet.

`add-open-advisories` added everything needed to correct this (see `openspec/specs/image-open-advisories/spec.md`, OA-16 and OA-17):
- `AdvisoryFeed.FetchAll` (`advisory_feed.go`) reads current records, caches each for 1 hour (404s too), allows 10 requests in flight, and fails the whole set as a `*SourceError` "Chainguard advisories".
- `withCurrentRecords` (`advisories.go`) swaps in each current record when it has an entry for the package, and returns the IDs the feed lacks.
- `openState` decides whether a record is open for the installed version, using Alpine version rules.
- `advisoryStatus` maps a record to fix available, pending, confirmed, fix not planned or not assessed.

`Service.openAdvisories` runs these steps inline, and the comparison has to run the same ones.

## Goals / Non-Goals

**Goals:**
- The Chainguard side's counts follow the same rules as the Security tab's open advisories, so the two can't disagree for the same image and architecture.
- One implementation of "found on OSV.dev, status from the feed" shared by both features.

**Non-Goals:**
- Changing the upstream side, `vulnKeys`, the API types or the web UI.
- Making the progress bar cover the feed step (see Risks).

## Decisions

### 1. Share the feed step as a Service helper

Move the part of `openAdvisories` that collects the distinct `CGA-` IDs from OSV results, calls `Feed.FetchAll`, and applies `withCurrentRecords` into `(s *Service) currentAdvisories(ctx, results) (current map[OSVPackage][]OSVVuln, fromOSV []string, checked time.Time, err error)`. `openAdvisories` and `compareUpstream` both call it. `openAdvisories` keeps its behavior and its tests unchanged.

*Alternative:* copy the loop into `compare.go`. Rejected because CLAUDE.md asks for one source of logic, and the copies would drift.

### 2. Run the feed only on the Chainguard side's results, after the shared OSV step

`compareUpstream` keeps its single `QueryAll` call with progress. It then passes only `subset(results, cgLookups)` to `currentAdvisories`. The OSV step finishes before the feed step starts, and each step limits itself to 10 requests in flight, so no more than 10 are ever in flight across both, as UC-10 requires. Debian and Alpine records never reach the feed (UC-17.4), and the ID check means only `CGA-` IDs are requested anyway.

### 3. Drop fixed records with `openState`, only on the Chainguard side

After the current records are swapped in, keep a Chainguard record for a package only if `openState(v, pkg)` reports it open (UC-18). A record with no entry for the package keeps OSV.dev's copy and stays open, as `openState` already does (UC-17.3, matching OA-16.4). This filter is not applied upstream. Debian versions don't follow Alpine's rules, and OSV.dev already matches Debian records against the installed version.

### 4. `classify` branches on the Chainguard ecosystems

For a package in ecosystem `Wolfi` or `Chainguard`, `classify` maps `advisoryStatus(v, pkg)` as follows:
- `fix_available` → fix available
- `fix_not_planned` → won't fix
- anything else → no fix yet

It uses `openState`'s fixed version, so a `fixed` event from an earlier, already-closed range doesn't count (UC-5.4, UC-5.5, UC-18.3). Other ecosystems keep today's code. `classifyAll`, `summarize` and the "best status wins" ranking stay as they are, so UC-18.2 (a CVE partly fixed) works as before: the fixed record has been removed and the open one decides.

*Alternative:* pass a classifier into `summarize` for each side. Rejected because the ecosystem already identifies the side, and a branch is smaller.

### 5. A missing feed record becomes a note

When `fromOSV` isn't empty, append to `Notes`: "Chainguard's advisory feed has no record of CGA-…, so OSV.dev's copy was used for its status, which couldn't be confirmed." This uses the same wording as the open-advisories report. No new field is added, so `web/src/api.ts` and the UI stay unchanged; the UI already shows `notes` (UpstreamComparison.tsx).

### 6. Wording

`comparisonSource` becomes: "Known vulnerabilities per each distribution's published advisories, via OSV.dev, with the status of Chainguard's advisories from Chainguard's advisory feed. Each image is graded by its own maintainers' advisories, and the counts are known vulnerabilities only, not a guarantee about either image." (UC-6.3). The `compare_with_upstream` description and the README get the same addition. Nothing calls an image secure.

### 7. Test fixtures

`newFakeUpstreamService` gets a fake feed (`newFakeFeed`) holding the current records of its two CGA advisories, still pending. The existing UC counts don't change, and `newFakeBackend`'s feed stays unreachable for every other test. New tests add records to that feed:
- a stale OSV copy that the feed has fixed in the installed version
- a fix in a newer version
- `fix_not_planned`
- a 404
- a 500
- a record without the package's entry

The web job test also checks that a feed failure gives 502 `source_failed` "Chainguard advisories".

No API shape changes: `GET /api/compare-upstream` returns the same fields, with different Chainguard counts, `source` and `notes`. No dependencies are added.

## Risks / Trade-offs

- [The progress bar reaches 100% before the feed step runs] → With nothing cached, about 20 feed requests at 10 at a time take well under a second (`add-open-advisories` measured the OSV step for 76 packages at 1.6 s), and the job only reports `done` once everything has finished. Extending the bar's total partway through would make it jump backwards.
- [The Chainguard side's OSV and feed data are cached for an hour, the upstream side's for 24 hours] → This was already the case for the OSV lookups since `add-open-advisories`. UC-10.1 and UC-10.2 now state it.
- [A comparison now fails when the feed is down, though OSV.dev works] → Counting stale statuses would overstate the Chainguard side, so the comparison fails with a retry, as UC-11 already requires for OSV.dev.
- [`fix_not_planned` → won't fix is a new mapping] → It is listed in the proposal for the user to confirm. If it's rejected, `classify` maps it to no fix yet and UC-5.4 is dropped before implementation.

## Live check (2026-10-11)

Run through the built server (`GET /api/compare-upstream` and the Alternatives page), amd64:

| Compared | Upstream known vulnerabilities (fix / no fix / won't fix) | Chainguard side | Chainguard total before this change |
|---|---|---|---|
| `node:20` vs `node` | 833 (295 / 325 / 213), unchanged | 1, no fix yet (CVE-2026-93750) | 6 |
| `python:3` vs `python` | 481 (0 / 280 / 201), unchanged | 2, no fix yet | 7 |
| `busybox:latest` vs `busybox` | not checked (no OS packages) | 0 | 5 |

These match UC-4.3, UC-5.2 and UC-15.1. The Chainguard totals equal the open-advisory totals for the same images (live subtest `upstream_chainguard_matches_open_advisories`). No advisory was missing from the feed. The page shows the new source text, and the console has no errors.
