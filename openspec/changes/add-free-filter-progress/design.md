# Design

## Context

See `proposal.md` for why, and `specs/catalog-free-filter/spec.md` for the requirements (FF-1 to FF-8).

Current state:
- `CatalogList` in `web/src/pages/CatalogPage.tsx` makes one `api.listImages({ query, freeOnly, limit: 50, offset })` request. With `freeOnly`, the server (`Service.ListImages`) calls `Registry.PublicStatus` for every matching image (8 at a time) before answering, so the request takes about 30 seconds the first time; the page shows `<Loading label="Loading images…" hint=…>` until then.
- Without `freeOnly`, the server checks only the returned page's images and caches each result for 24 hours in `Registry.IsPublic`. So requesting the unfiltered list page by page warms the same cache that `free_only` reads.
- The Map view already does this (`web/src/components/CatalogMap.tsx`): a `useInfiniteQuery` keyed `['freeStatus', query]` fetches the list in batches of 1,000, one at a time, driven by a `useEffect` that calls `fetchNextPage` (with `loadedPages` as a dependency, and a guard on `isFetchNextPageError`). This change follows the same pattern.

**No API changes.** The Go server, the web API and the MCP tools are untouched.

## Goals / Non-Goals

**Goals:**
- Real, determinate progress for the first free-only check, with a time estimate, and nothing extra when the statuses are cached.
- Logic that can be unit-tested with exact numbers (the FF-4 scenarios) without rendering.
- A progress bar component other waits can reuse, built on the same CSP-safe approach as `HorizontalBars`.

**Non-Goals:**
- Sharing the batch queries with the Map view. Their batch sizes differ, so they use separate query keys; the second view's batches answer from the server's cache in milliseconds anyway.
- Changing the Map view's status line (CM-8).

## Decisions

### 1. Pure helpers in `web/src/freeCheck.ts`
- `FREE_CHECK_BATCH = 250`, `ASSUMED_MS_PER_IMAGE = 9`, `PROGRESS_DELAY_MS = 1000`.
- `secondsLeft(checked, total, msPerImage)`: `(total - checked) × msPerImage / 1000`.
- `timeLeftText(seconds)`: rounds up to the next 5 seconds → `"about 20 seconds left"`; under 5 → `"a few seconds left"`.
- `freeCheckLabel(checked, total, msPerImage)`: `"Checking free-tier status: 250 of 3,171 images · about 25 seconds left"`, with `en-US` number formatting as in `catalogMap.ts`.
- `averageMsPerImage(batches)`: the sum of the batches' measured durations divided by the images they checked, or `ASSUMED_MS_PER_IMAGE` when none has returned.

FF-4.1 and FF-4.2 become unit tests on these. *Alternative:* computing inside the component. Rejected: the exact-number scenarios are awkward to test through rendering.

### 2. Time per image from the batches themselves
Each batch's `queryFn` measures its own duration (`performance.now()` around `api.listImages`) and returns it with the page, e.g. `{ ...page, ms }`. The average uses only those durations. Time spent waiting between batches, in an error state before a retry (FF-7), or in a background tab, is therefore not counted, and no start-time ref has to be reset when the search changes. *Alternative:* `elapsed = now − startedAt`. Rejected: errors and retries distort it, and the start has to be tracked per search.

### 3. The check in `CatalogList`
- A `useInfiniteQuery` keyed `['freeCheck', query]`, enabled only while `freeOnly` is on, with `limit: FREE_CHECK_BATCH`, `getNextPageParam` as in `CatalogMap`, and the same "fetch the next batch when the last one finished" effect (FF-2). Unticking disables it, so no further batch is requested (FF-6.1). A new search is a new key, and the old key's remaining batches are never requested (FF-6.2).
- The existing list query keeps its key and gets `enabled: !freeOnly || checkDone`, where `checkDone` means the check has data and no next page. It runs once, after the check, and is then answered from the server's cache (FF-2.1). With "Free only" off, nothing changes (FF-1.2).
- Errors: when a batch fails, show `ErrorState` with a retry that calls `fetchNextPage()` if some batches have returned, otherwise `refetch()`, so a retry resumes rather than restarting (FF-7.1). The bar stays on screen above the error.

### 4. Delaying the bar by one second (FF-5)
A `useEffect` starts a `PROGRESS_DELAY_MS` timer when a check starts (`freeOnly` on and the check not done) and sets `showProgress`. The effect is keyed on `freeOnly` and `query`, and its cleanup clears the timer, so a fast or abandoned check never shows the bar. Until then, and whenever the bar isn't shown, the existing `<Loading label="Loading images…" />` stays, without the 30-second hint, since the bar now covers the long case.

### 5. `ProgressBar` component (`web/src/components/ProgressBar.tsx`)
`<ProgressBar value max label />` renders:
- a `div` with `role="progressbar"`, `aria-valuemin={0}`, `aria-valuemax`, `aria-valuenow` and `aria-valuetext={label}` (FF-3.3);
- an SVG bar, `viewBox="0 0 100 4"` with `preserveAspectRatio="none"`, made of a full-width track `rect` and a filled `rect` whose `width` attribute is `value / max × 100`. SVG attributes aren't styles, so this is unaffected by the strict `style-src` policy (FF-8), as in `HorizontalBars`;
- the label as text below the bar, in an element with `aria-live="polite"` (FF-3).

Track and fill colors are Tailwind fill classes with dark variants, kept in `web/src/styles.ts` (per CLAUDE.md, not in the component file). *Alternatives:* the native `<progress>` element (styling its fill needs vendor pseudo-elements and differs across browsers), and a `div` with an inline `style` width (blocked by the CSP).

### 6. The 3-second rule in `openspec/config.yaml`
Add to the Web UI bullet of `context`: a wait that can take more than about 3 seconds shows determinate progress or the expected time, never only "Loading". It's added in the `context` block, which is a YAML block scalar, so it needs no quoting; check afterwards that the config still parses (CI does too).

### 7. Tests
- Unit tests in `web/src/freeCheck.test.ts` for the helpers: FF-4.1, FF-4.2, and FF-3.2's "about 30 seconds left" at the assumed rate.
- Component tests in `web/src/components/ProgressBar.test.tsx`: the fill `rect` width is proportional (250/3,171 → about 7.88) and the ARIA attributes (FF-3.3).
- Page tests in a new `web/src/pages/FreeFilter.test.tsx`, using `mockApi` with one response per `offset` (generated, for a 3,171-image catalog) plus a `free_only: 'true'` response, and `{ pending: true }` to hold a batch. For the one-second delay, use Vitest fake timers (`vi.useFakeTimers({ shouldAdvanceTime: true })`, then `vi.advanceTimersByTime(1000)`), with `userEvent.setup({ advanceTimers: vi.advanceTimersByTime })`.
- Test names start with their scenario IDs, e.g. `FF-6.1 stops after "Free only" is unticked`.

## Risks / Trade-offs

- **[The rate varies]** One measured batch took 3.2 s instead of 2.2 s. → The estimate is recomputed from the actual average after every batch, is rounded up to 5 seconds, and says "about".
- **[Cache expiry mid-check]** The server caches each status for 24 hours, so an expiry between the check and the free-only request is possible but rare; that request would then do a smaller check of its own and still return correct results, just more slowly.
- **[Two views, two checks]** Opening the Map after the List (or the reverse) runs its own batches. → They answer from the server's cache in milliseconds, so the cost is a few fast requests.
- **[13 requests instead of 1]** Each batch is a separate HTTP request. → They're sequential and the total number of token checks is unchanged, so Chainguard's token endpoint sees no extra load.

## Migration Plan

Frontend only, shipped in the embedded UI on the next build. Rollback is reverting the change; there's no data, API or configuration migration.
