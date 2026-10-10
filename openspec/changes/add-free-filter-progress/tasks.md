# Tasks

Test names start with the scenario ID they cover (e.g. `FF-4.1 …`). Run web commands from `web/`, with Node on `PATH` as CLAUDE.md describes. Design decisions are numbered as in `design.md`.

## 1. Project rule

- [ ] 1.1 Add the 3-second rule to the Web UI bullet of `context` in `openspec/config.yaml` (design decision 6): a wait that can take more than about 3 seconds shows determinate progress or the expected time, never only "Loading". Verify: `openspec validate --all --strict` passes, and `openspec instructions proposal --change add-free-filter-progress --json` shows the new rule in its `context` (proving the config still parses).

## 2. Progress logic (`web/src/freeCheck.ts`)

- [ ] 2.1 Add the constants and helpers from design decision 1: `FREE_CHECK_BATCH`, `ASSUMED_MS_PER_IMAGE`, `PROGRESS_DELAY_MS`, `secondsLeft`, `timeLeftText`, `freeCheckLabel` and `averageMsPerImage` (FF-3, FF-4). Verify with unit tests in `web/src/freeCheck.test.ts`: `FF-4.1` (1,000 of 3,171 checked in 8,800 ms → "about 20 seconds left"), `FF-4.2` (3,000 of 3,171 in 26,400 ms → "a few seconds left"), `FF-3.2` (0 of 3,171 at the assumed 9 ms → "Checking free-tier status: 0 of 3,171 images · about 30 seconds left"), `FF-3.1` (the label for 250 of 3,171 begins "Checking free-tier status: 250 of 3,171 images ·"), plus exactly 5 seconds and 0 remaining.

## 3. Progress bar (`web/src/components/ProgressBar.tsx`)

- [ ] 3.1 Build `ProgressBar` as in design decision 5 (FF-3, FF-8), with its fill and track classes in `web/src/styles.ts`, and no new dependencies. Verify with component tests in `web/src/components/ProgressBar.test.tsx`: `FF-3.1` (250 of 3,171 → the fill `rect`'s `width` attribute is about 7.88), `FF-3.3` (role `progressbar` with `aria-valuenow` 1,000, `aria-valuemax` 3,171 and `aria-valuetext` equal to the visible label), a zero value draws a zero-width fill, and the label is inside an `aria-live="polite"` element.

## 4. Free-only check in the List view

- [ ] 4.1 Add a reusable page-test setup to a new `web/src/pages/FreeFilter.test.tsx`: `mockApi` responses for a 3,171-image catalog, one per `offset` with `limit: '250'` (generated in a loop, image names like `image-0001`), plus a `free_only: 'true'` response for 59 free images. Verify: a first test, `FF-1.2 makes no check when "Free only" is off`, passes: with "Free only" unticked, exactly one `/api/images` request is made, with no `free_only` and a limit of 50, and no progress bar is shown.
- [ ] 4.2 Implement the batched check in `CatalogList` (`web/src/pages/CatalogPage.tsx`) as in design decisions 2 and 3, with each batch measuring its own duration (FF-1, FF-2, FF-6). Verify with page tests: `FF-2.1` (13 requests with limit 250 at offsets 0 to 3,000, each made only after the previous returned, then one `free_only=true` request), `FF-1.1` (the list then says "59 images"), `FF-2.2` (search `python` with 11 matches → one batch, then the free-only request), `FF-6.1` (unticking after 3 batches → no further batch requests, unfiltered list shown) and `FF-6.2` (changing the search mid-check → no further batches for the empty search; the check restarts for `python`).
- [ ] 4.3 Show the bar after one second and keep `Loading` until then (design decision 4; FF-3, FF-5), and remove the old 30-second hint from `Loading`. Verify with page tests using fake timers: `FF-5.1` (all batches return at once → no `progressbar` is ever rendered, and the free list appears), `FF-3.2` (a pending first batch → after 1 second the bar appears, empty, labeled "… 0 of 3,171 images · about 30 seconds left") and `FF-3.1` (after the first batch, the bar shows 250 of 3,171).
- [ ] 4.4 Handle a failed batch (design decision 3; FF-7). Verify with a page test: `FF-7.1` (the batch at offset 1,000 fails after 4 returned → the error and a retry button are shown with the bar still at 1,000 of 3,171; retrying requests offset 1,000 and onward, and never offsets 0 to 750 again).
- [ ] 4.5 Update the README's catalog description: "Free only" shows a progress bar with the time left during the first check (about 30 seconds), and the results are cached for 24 hours. Verify: the README describes the behavior as built, and the existing `pages.test.tsx` and `CatalogMap.test.tsx` tests still pass.

## 5. Integration verification

- [ ] 5.1 Run every check: `gofmt -l .`, `go vet ./...` and `go test ./...` (which should show no Go changes were needed), and in `web/` `npm run lint`, `npm run typecheck`, `npm test` and `npm run build`, plus `openspec validate --all --strict`. Verify: all pass.
- [ ] 5.2 Exercise the feature in the running app: build the UI and start a **fresh** Go server (`go run . -http 127.0.0.1:8080`) so the free-tier cache is empty. Verify in the browser: ticking "Free only" shows "Loading images…" briefly, then the bar filling in about 13 steps with a falling time estimate, then the free images (about 59); unticking and ticking again shows no bar (cached); at 375 px wide in dark mode the bar and label fit without horizontal scrolling and the console shows no Content Security Policy violations (FF-8.1). If the browser pane can't take screenshots, ask the user to confirm the layout visually.
- [ ] 5.3 Ask the user before committing and pushing, then push to `main` and confirm CI passes on the implementing commit, as `config.yaml`'s archive guidance requires. Verify: all five required CI jobs succeed.

## Workflow follow-up

- Before archiving, have the change reviewed on the current model (or run `/code-review`), checking the code against FF-1 to FF-8, especially timer cleanup and the stale-parameters pitfall CLAUDE.md describes for `CatalogPage`.
- Archive with `/opsx:archive` once CI passes; this creates the living spec `openspec/specs/catalog-free-filter/spec.md`.
- Possible follow-up change: move the Map view's status line (CM-8) onto `ProgressBar`.
