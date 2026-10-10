import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { useEffect, useRef, useState } from 'react'
import { Link, useSearchParams } from 'react-router'
import { api } from '../api'
import { CatalogMap } from '../components/CatalogMap'
import { ErrorState, FreeBadge, Loading, Pagination } from '../components/ui'
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

/** One page of the image list, with free-tier status for each image. */
function CatalogList({ query, freeOnly, page, onPage }: { query: string; freeOnly: boolean; page: number; onPage: (page: number) => void }) {
  const { data, error, isPending, isFetching, refetch } = useQuery({
    queryKey: ['images', query, freeOnly, page],
    queryFn: ({ signal }) =>
      api.listImages({ query, freeOnly, limit: PAGE_SIZE, offset: (page - 1) * PAGE_SIZE }, signal),
    placeholderData: keepPreviousData,
  })

  const pages = data ? Math.max(1, Math.ceil(data.total / PAGE_SIZE)) : 1

  return (
    <>
      {isPending ? (
        <Loading
          label="Loading images…"
          hint={freeOnly && !query ? 'Checking every image for free-tier access. The first time can take about 30 seconds.' : undefined}
        />
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
