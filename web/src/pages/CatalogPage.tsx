import { keepPreviousData, useInfiniteQuery, useQuery } from '@tanstack/react-query'
import { useEffect, useRef, useState } from 'react'
import { Link, useSearchParams } from 'react-router'
import { api } from '../api'
import { CatalogMap } from '../components/CatalogMap'
import { ProgressBar } from '../components/ProgressBar'
import { ErrorState, FreeBadge, Loading, Pagination } from '../components/ui'
import { FREE_CHECK_BATCH, PROGRESS_DELAY_MS, averageMsPerImage, freeCheckLabel } from '../freeCheck'
import { familySorts, type FamilySort } from '../catalogMap'
import type { GroupBy } from '../api'
import { inputClass, segmentButton, segmentGroup } from '../styles'

const PAGE_SIZE = 50


export function CatalogPage() {
  const [params, setParams] = useSearchParams()
  const query = params.get('q') ?? ''
  const freeOnly = params.get('free') === '1'
  const page = Math.max(1, Number(params.get('page')) || 1)
  const view = params.get('view') === 'map' ? 'map' : 'list'
  const sortParam = params.get('sort')
  const sort: FamilySort = familySorts.find((s) => s === sortParam) ?? 'images'
  const groupBy: GroupBy = params.get('group') === 'prefix' ? 'prefix' : 'variant'

  // The search box updates the URL after a short pause, so the URL (and the
  // query) always reflects the current search and can be shared. If the URL
  // changes some other way (back button, a link), the box follows it.
  const [draft, setDraft] = useState(query)
  const [syncedQuery, setSyncedQuery] = useState(query)
  if (query !== syncedQuery) {
    setSyncedQuery(query)
    setDraft(query)
  }
  const debounce = useRef<ReturnType<typeof setTimeout>>(undefined)
  useEffect(() => () => clearTimeout(debounce.current), [])

  function onSearchChange(value: string) {
    setDraft(value)
    clearTimeout(debounce.current)
    debounce.current = setTimeout(() => update({ q: value.trim(), page: '' }), 300)
  }

  // React Router's functional setParams receives the params from the render
  // that created the callback, so a debounced update would undo changes made
  // in between (like ticking "Free only"). Track the latest params instead.
  const latestParams = useRef(params)
  useEffect(() => {
    latestParams.current = params
  }, [params])

  function update(changes: Record<string, string>) {
    const next = new URLSearchParams(latestParams.current)
    for (const [k, v] of Object.entries(changes)) {
      if (v) next.set(k, v)
      else next.delete(k)
    }
    latestParams.current = next
    setParams(next, { replace: true })
  }

  return (
    <div className="space-y-5">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">Image catalog</h1>
        <p className="mt-1 text-sm text-zinc-600 dark:text-zinc-400">
          Search Chainguard's container images. Free images can be inspected in detail.
        </p>
      </div>

      <div role="group" aria-label="View" className={segmentGroup}>
        <button type="button" aria-pressed={view === 'list'} onClick={() => update({ view: '', page: '' })} className={segmentButton(view === 'list')}>
          List
        </button>
        <button type="button" aria-pressed={view === 'map'} onClick={() => update({ view: 'map', page: '' })} className={segmentButton(view === 'map')}>
          Map
        </button>
      </div>

      <div className="flex flex-col gap-3 sm:flex-row sm:items-center">
        <label className="flex-1">
          <span className="sr-only">Search images</span>
          <input
            type="search"
            value={draft}
            onChange={(e) => onSearchChange(e.target.value)}
            placeholder="Search images, e.g. python, nginx, fips"
            className={inputClass}
          />
        </label>
        {view === 'list' && (
          <label className="flex items-center gap-2 text-sm">
            <input
              type="checkbox"
              checked={freeOnly}
              onChange={(e) => update({ free: e.target.checked ? '1' : '', page: '' })}
              className="size-4 rounded border-zinc-300 accent-indigo-600"
            />
            Free only
          </label>
        )}
      </div>

      {view === 'map' ? (
        <CatalogMap
          query={query}
          groupBy={groupBy}
          onGroupBy={(g) => update({ group: g === 'variant' ? '' : g })}
          sort={sort}
          page={page}
          onSort={(s) => update({ sort: s === 'images' ? '' : s, page: '' })}
          onPage={(p) => update({ page: String(p) })}
        />
      ) : (
        <CatalogList query={query} freeOnly={freeOnly} page={page} onPage={(p) => update({ page: String(p) })} />
      )}
    </div>
  )
}

/**
 * One page of the image list, with free-tier status for each image. With
 * "Free only" on, the statuses are first checked in batches, one at a time, so
 * a progress bar can show how far the check is; the server then answers the
 * filtered list from its cache.
 */
function CatalogList({ query, freeOnly, page, onPage }: { query: string; freeOnly: boolean; page: number; onPage: (page: number) => void }) {
  const check = useInfiniteQuery({
    queryKey: ['freeCheck', query],
    enabled: freeOnly,
    queryFn: async ({ pageParam, signal }) => {
      const started = performance.now()
      const list = await api.listImages({ query, limit: FREE_CHECK_BATCH, offset: pageParam }, signal)
      return { ...list, ms: performance.now() - started }
    },
    initialPageParam: 0,
    getNextPageParam: (last) => (last.offset + last.count < last.total ? last.offset + last.count : undefined),
  })
  const { hasNextPage, isFetching: checkFetching, isFetchNextPageError, fetchNextPage } = check
  // The page count is a dependency because a fast answer can skip the render
  // where isFetching is true, leaving the other values unchanged. fetchNextPage
  // ignores `enabled`, so unticking "Free only" must stop it here.
  const loadedBatches = check.data?.pages.length ?? 0
  useEffect(() => {
    if (freeOnly && hasNextPage && !checkFetching && !isFetchNextPageError) void fetchNextPage({ cancelRefetch: false })
  }, [freeOnly, hasNextPage, checkFetching, isFetchNextPageError, fetchNextPage, loadedBatches])
  const checkDone = check.data !== undefined && !hasNextPage
  const checking = freeOnly && !checkDone

  // The first batch can't say how many images match, so until it returns the
  // family list (catalog only, no status checks) supplies the total.
  const families = useQuery({
    queryKey: ['families', query],
    queryFn: ({ signal }) => api.families(query, signal),
    enabled: checking,
  })

  // The bar appears only if the check is still running after a second.
  const [barFor, setBarFor] = useState<string | null>(null)
  useEffect(() => {
    if (!checking) return
    const timer = setTimeout(() => setBarFor(query), PROGRESS_DELAY_MS)
    return () => {
      clearTimeout(timer)
      setBarFor(null)
    }
  }, [checking, query])

  const { data, error, isPending, isFetching, refetch } = useQuery({
    queryKey: ['images', query, freeOnly, page],
    queryFn: ({ signal }) =>
      api.listImages({ query, freeOnly, limit: PAGE_SIZE, offset: (page - 1) * PAGE_SIZE }, signal),
    placeholderData: keepPreviousData,
    enabled: !freeOnly || checkDone,
  })

  const batches = check.data?.pages ?? []
  const checked = batches.reduce((n, b) => n + b.count, 0)
  const total = batches[0]?.total ?? families.data?.total
  const showBar = total !== undefined && (barFor === query || check.error !== null)

  const pages = data ? Math.max(1, Math.ceil(data.total / PAGE_SIZE)) : 1

  return (
    <>
      {checking ? (
        <div className="space-y-3">
          {showBar && <ProgressBar value={checked} max={total} label={freeCheckLabel(checked, total, averageMsPerImage(batches))} />}
          {check.error ? (
            <ErrorState error={check.error} onRetry={() => (loadedBatches > 0 ? void fetchNextPage() : void check.refetch())} />
          ) : (
            !showBar && <Loading label="Loading images…" />
          )}
        </div>
      ) : isPending ? (
        <Loading label="Loading images…" />
      ) : error ? (
        <ErrorState error={error} onRetry={() => refetch()} />
      ) : (
        <>
          <p className="text-sm text-zinc-600 dark:text-zinc-400" aria-live="polite">
            {data.total === 0 ? 'No images match.' : `${data.total.toLocaleString()} image${data.total === 1 ? '' : 's'}`}
            {isFetching && ' · updating…'}
          </p>
          {data.images.length > 0 && (
            <ul className="divide-y divide-zinc-200 overflow-hidden rounded-xl border border-zinc-200 bg-white dark:divide-zinc-800 dark:border-zinc-800 dark:bg-zinc-900">
              {data.images.map((img) => (
                <li key={img.name}>
                  <Link
                    to={`/images/${img.name}`}
                    className="flex items-center justify-between gap-3 px-4 py-3 hover:bg-zinc-50 focus-visible:bg-zinc-50 focus-visible:outline-none dark:hover:bg-zinc-800/60 dark:focus-visible:bg-zinc-800/60"
                  >
                    <span className="min-w-0">
                      <span className="block truncate font-medium">{img.name}</span>{' '}
                      <span className="block truncate font-mono text-xs text-zinc-500 dark:text-zinc-400">{img.reference}</span>
                    </span>{' '}
                    <FreeBadge free={img.free} />
                  </Link>
                </li>
              ))}
            </ul>
          )}
          {pages > 1 && <Pagination page={page} pages={pages} onPage={onPage} />}
        </>
      )}
    </>
  )
}
