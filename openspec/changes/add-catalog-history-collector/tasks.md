# Tasks

Commands are for Git Bash on Windows as in CLAUDE.md. Test names start with the scenario ID they cover (e.g. `CH-6.1 …`). Design decisions are numbered as in `design.md`.

## 1. Project rule

- [x] 1.1 Add the exception from design decision 6 to the "Data comes only from Chainguard's public, anonymous endpoints" bullet of `context` in `openspec/config.yaml`. Verify: `openspec validate --all --strict` passes, and `openspec instructions proposal --change add-catalog-history-collector --json` shows the new wording in its `context` (proving the config still parses).

## 2. History logic (`history.go`)

- [x] 2.1 Add `HistoryLine` and `replayHistory` (design decision 1; CH-1, CH-2). Verify with tests in `unit_test.go`: `CH-2.1` (replaying a three-line history rebuilds each snapshot's image list, and its length equals that line's total), and reading a file with a malformed line returns an error naming the line number.
- [x] 2.2 Add `diffSnapshot` (CH-1, CH-2, CH-3, CH-4). Verify with unit tests: `CH-1.1` (after an archive line of 3,166 names, a live snapshot with the five extra names from the spec produces exactly that added list, total 3,171, free 59), `CH-1.2` (archive lines have null free and empty status lists), `CH-2.2` (the first live snapshot lists 59 `free_images` and an empty `became_free`), `CH-3.1`, `CH-3.2`, `CH-3.3` and `CH-4.1`; and every list is sorted.
- [x] 2.3 Add `checkPlausible` (CH-6). Verify with unit tests: `CH-6.1` (1,500 after 3,171 → error naming both counts), `CH-6.2` (15 removed of 3,171 → accepted), an empty sitemap → error, and exactly 5% removed → accepted.

## 3. Live snapshots

- [x] 3.1 Move the sitemap decoding in `Catalog.Images` into `parseSitemap(io.Reader)` (design decision 3) without changing behavior. Verify: the existing `go test ./...` passes unchanged.
- [x] 3.2 Add `recordHistory` and the `-record-history` flag (design decisions 1 and 2; CH-5, CH-7). Verify with tests in a new `history_test.go` using `newFakeService`: `CH-7.1` (the snapshot's total and free images equal `ListImages` with `FreeOnly`), `CH-5.1` (a second run with the same date leaves the file byte-for-byte unchanged and reports "skipped"), and a refused snapshot leaves the file unchanged and returns an error, and the flag then exits with status 1 (for `CH-9.2`); and `go run . -record-history <temp file>` against the real endpoints, run by hand once, appends one plausible line (about 3,171 images, about 59 free).

## 4. Backfill

- [x] 4.1 Add the `Archive` client and the `-backfill-history` flag (design decision 3; CH-8). Verify with tests in `history_test.go` against a fake Archive server (a CDX listing, two sitemap captures in different months and one HTML capture): `CH-8.2` (the HTML capture is skipped and the month's next capture used), `CH-8.3` (a non-empty file is left unchanged with an error), and the lines' dates come from the capture timestamps, the first line lists every name and the second diffs from it.
- [x] 4.2 Add a live test to `live_test.go` (skipped unless `CHAINGUARD_LIVE=1`): the Archive's capture listing yields at least 17 months with a sitemap since 2024-04 (`CH-8.1` as a stable property). Verify: `CHAINGUARD_LIVE=1 go test -run TestLive -v ./...` passes.

## 5. Publishing

- [x] 5.1 Add `.github/workflows/history.yml` as in design decision 5 (CH-9, CH-10). Verify: the workflow file passes `actionlint` if available (or a careful read against design decision 5), uses the same `actions/checkout` and `actions/setup-go` versions as `ci.yml`, has `contents: write` only in this workflow, and commits only when the history file changed. For `CH-9.2`: the snapshot step runs before the commit step with no `continue-on-error`, so the exit code 1 tested in 3.2 fails the job and nothing is pushed.
- [x] 5.2 Run the backfill locally into a new file, check it (17 archive lines, first 675 images, last 3,166, CH-8.1), then create the orphan `data` branch with that file and a `README.md` describing the format (field by field, as in CH-1 and CH-2), the daily job, and the 60-day inactivity caveat (design, Risks). Ask the user before pushing `data`. Verify: `git show data:catalog-history.jsonl | wc -l` prints 17 and the branch has no other history.
- [x] 5.3 Ask the user whether to add a ruleset for `data` that blocks deletion and force-pushes but allows normal pushes (design, Risks). Changing repository settings is the user's decision; record the answer here. Verify: the answer is recorded, and if a ruleset was added, the workflow's push in 6.2 still succeeds. Answer (2026-10-10): yes; added ruleset "Protect data history" (id 24859001) blocking deletion and force-pushes on `data`, no bypass actors.
- [x] 5.4 Document the history: a "Catalog history" section in the README (what is recorded, where, how often, and that free-tier status starts on the first live day), and a "Catalog history" bullet in CLAUDE.md's data sources (the file on `data`, the Archive backfill, the two flags). Verify: both describe the feature as built.

## 6. Integration verification

- [x] 6.1 Run every check: `gofmt -l .` (only files changed by this change matter; Windows line endings make unchanged files appear), `go vet ./...`, `go test ./...`, `npm --prefix web run lint`, `npm --prefix web run typecheck`, `npm --prefix web test`, `npm --prefix web run build` and `openspec validate --all --strict`. Verify: all pass, and `git diff --stat` shows no change under `web/`.
- [x] 6.2 Ask the user before committing and pushing the code to `main`. Then confirm CI passes on the implementing commit, trigger the History workflow by hand (`gh workflow run history.yml`), and check: it succeeds, `data` gets exactly one new commit touching only `catalog-history.jsonl` (CH-10.1), whose new line is the first live snapshot with today's date, about 3,171 images, about 59 `free_images` and an empty `became_free` (CH-1.1, CH-2.2). Trigger it a second time: it succeeds without a new commit (CH-5.1). Verify: as described.
- [ ] 6.3 The next day, confirm the scheduled run added one more line on its own (CH-9.1). Verify: `data` has a commit from the scheduled run dated that day.

## Workflow follow-up

- Archive with `/opsx:archive` once 6.3 passes; this creates the living spec `openspec/specs/catalog-history/spec.md`.
- Then plan the follow-up change `add-catalog-history-view` from idea #6 in `docs/visualization-ideas.md`.
