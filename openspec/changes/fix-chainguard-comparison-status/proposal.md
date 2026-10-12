# Proposal

## Why

The upstream comparison counts the Chainguard image's known vulnerabilities from OSV.dev's copies of Chainguard's advisories. Those copies can be days older than Chainguard's own advisory feed. On 2026-10-11 OSV.dev still listed glibc advisories such as `CGA-2727-9j94-3x3p` as pending an upstream fix, but the feed (record modified 2026-10-07) has them fixed in `2.44-r8`, the version installed. So the comparison counts fixed advisories against the Chainguard image. `add-open-advisories` already fixed this for the Security tab by reading each advisory's status from the feed, and this change does the same for the comparison. Measured live on 2026-10-11 (amd64, `latest`):

| Chainguard image | OS packages | Records on OSV.dev | Open per the feed | Known vulnerabilities now | With this change |
|---|---|---|---|---|---|
| `node` | 28 | 22 | 2 | 6 | **1** (CVE-2026-93750 in `npm-12`, pending an upstream fix) |
| `python` | 29 | 20 | 4 | 7 | **2** (CVE-2025-15367 and CVE-2026-12345, not yet assessed) |
| `busybox` | 9 | 20 | 0 | 5 | **0** |

The feed had a record for every advisory OSV.dev listed. Both open python CVEs are the same ones the Security tab reports (OA-10.1).

## What Changes

- **Chainguard side only:** each Chainguard advisory found on OSV.dev is read again from Chainguard's advisory feed. Its fix status and fixed version come from that current record's entry for the installed package.
- Advisories that the current record has fixed at or below the installed version are no longer counted. A vulnerability counts only while at least one of its advisories is still open.
- Classification on the Chainguard side follows the current record: **fix available** when it names a fixed version newer than the one installed, **won't fix** when Chainguard has marked it `fix_not_planned`, and **no fix yet** otherwise (pending an upstream fix, not yet assessed, or confirmed).
- If the feed has no record of an advisory (a 404), OSV.dev's copy is used, and the comparison adds a note naming that advisory and saying its status couldn't be confirmed. Any other feed failure fails the comparison as a source failure naming "Chainguard advisories", like an OSV.dev failure.
- The source wording (the API's `source`, the MCP tool's description and the README) says that Chainguard's statuses come from its advisory feed.
- The upstream (Debian/Alpine) side is unchanged: it still reads only OSV.dev.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `upstream-comparison`: UC-5 (classification on the Chainguard side, including won't fix), UC-6 (source wording), UC-10 (the feed's cache and the shared limit on requests in flight), UC-11 (feed failures), and the figures in UC-4.3, UC-5.2 and UC-15.1, re-measured on 2026-10-11. New UC-17 (status from Chainguard's advisory feed) and UC-18 (advisories already fixed in the installed version aren't counted).

## Decisions

Made with the user on 2026-10-11, while building `add-open-advisories`:
- **Read each Chainguard advisory's status from Chainguard's own feed, and use OSV.dev only to find which advisories apply.** *Rejected:* trusting OSV.dev's copies, which counted fixed glibc advisories; and reconciling with the Wolfi security database, which disagrees with the feed for some packages (see `add-open-advisories`' design.md).
- **Apply it to the comparison as a separate follow-up change**, reusing the feed client and its rules (OA-16, OA-17). *Rejected:* widening `add-open-advisories`, which was already in review.
- **Leave the upstream side on OSV.dev.** Debian's and Alpine's advisories come from their own trackers through OSV.dev, and none of the stale copies we found were theirs.

Proposed here, for the user to confirm on review:
- **`fix_not_planned` counts as won't fix.** UC-5 defines won't fix as "the distribution marks it as one it won't fix", which is what this state means. On 2026-10-11 none of the three images above had one. *Alternative:* keep it as no fix yet, as now, which would make the Chainguard side never show won't fix.
- **Report a missing feed record in `notes` only**, not in a new field, so the API types and the web UI don't change. *Alternative:* an `unconfirmed_status` list on the Chainguard side, like the open-advisories report's `status_from_osv`.

## Out of Scope

- The upstream side's data and counting.
- Showing the Chainguard side's individual advisories or CVEs in the comparison. The comparison still shows only counts.
- Web UI changes. The source text and notes already come from the API.
- Severity.

## Impact

- **Go:** `compare.go` (the Chainguard side's results go through the feed before counting), `osv.go` or `advisories.go` (a classification of current Chainguard records for the comparison), the `compare_with_upstream` description in `tools.go`, and test fixtures: `newFakeUpstreamService` gets a fake feed.
- **API:** no new fields or endpoints. The Chainguard side's counts, `source` and `notes` change. A feed failure gives 502 `source_failed` with source "Chainguard advisories".
- **External load:** one feed request per distinct Chainguard advisory found (22 for `node`, 20 for `python`), cached for 1 hour, and no more than 10 requests in flight to OSV.dev and the feed together.
- **Docs:** the README's comparison section and data-source table, and CLAUDE.md's notes on the comparison and open advisories ("The upstream comparison does not use the feed yet").
- **Dependencies:** none.
