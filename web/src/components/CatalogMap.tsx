import { keepPreviousData, useInfiniteQuery, useQuery } from '@tanstack/react-query'
import { type KeyboardEvent, useCallback, useEffect, useMemo, useRef } from 'react'
import { api, type GroupBy } from '../api'
import {
  blockBars,
  type FamilySort,
  type FreeStatus,
  familiesInGroup,
  familyRows,
  groupLabel,
  mapStatusText,
  sortFamilyRows,
  statusCounts,
  statusesFrom,
  treemapSummary,
  zoomedSummary,
  zoomLabel,
} from '../catalogMap'
import { freeStatusColor, segmentButton, segmentGroup } from '../styles'
import { CatalogTreemap } from './CatalogTreemap'
import { FamilyTable } from './FamilyTable'
import { HorizontalBars } from './HorizontalBars'
import { Card, ErrorState, Loading, Pagination } from './ui'

const PAGE_SIZE = 50
/** Images per free-tier status request: the image list's largest page. */
const BATCH_SIZE = 1000

const groupings: { value: GroupBy; label: string }[] = [
  { value: 'variant', label: 'Variant' },
  { value: 'prefix', label: 'Name prefix' },
]

/** Where a zoom started, so zooming out can return focus to the same kind of control. */
type ZoomSource = 'treemap' | 'bar'

/** Focus to move once the zoom in the URL has been drawn. */
type PendingFocus = { to: 'back' } | { to: 'group'; label: string; from: ZoomSource }

/** Whether the treemap rather than the phone bars is showing (Tailwind's sm breakpoint). */
const wide = () => (typeof window.matchMedia === 'function' ? window.matchMedia('(min-width: 640px)').matches : true)

const legend: { status: FreeStatus; label: string }[] = [
  { status: 'free', label: 'Free' },
  { status: 'subscription', label: 'Subscription' },
  { status: 'unknown', label: 'Not known' },
]

/**
 * The catalog page's Map view: a treemap of the images grouped by variant or
 * name prefix (a bar per group on phones) and a sortable family table. The map is drawn as soon as the
 * families load; free-tier statuses then arrive from the image list in
 * batches, one at a time, and color it in. A group's header or bar zooms into
 * it; the zoom, kept in the URL by the caller, narrows the map, legend and
 * table to that group.
 */
export function CatalogMap({
  query,
  groupBy,
  onGroupBy,
  zoom,
  onZoom,
  sort,
  page,
  onSort,
  onPage,
}: {
  query: string
  groupBy: GroupBy
  onGroupBy: (groupBy: GroupBy) => void
  /** The label of the group zoomed into, or empty. */
  zoom: string
  onZoom: (label: string, options?: { replace?: boolean }) => void
  sort: FamilySort
  page: number
  onSort: (sort: FamilySort) => void
  onPage: (page: number) => void
}) {
  const families = useQuery({
    queryKey: ['families', query],
    queryFn: ({ signal }) => api.families(query, signal),
  })
  // Keep the old groups on screen while another grouping loads.
  const groups = useQuery({
    queryKey: ['groups', query, groupBy],
    queryFn: ({ signal }) => api.groups(query, groupBy, signal),
    placeholderData: keepPreviousData,
  })

  // Each batch starts after the previous one finishes, so checking every image
  // costs Chainguard's token endpoint no more than "Free only" does.
  const status = useInfiniteQuery({
    queryKey: ['freeStatus', query],
    queryFn: ({ pageParam, signal }) => api.listImages({ query, limit: BATCH_SIZE, offset: pageParam }, signal),
    initialPageParam: 0,
    getNextPageParam: (last) => (last.offset + last.count < last.total ? last.offset + last.count : undefined),
  })
  const { hasNextPage, isFetching, isFetchNextPageError, fetchNextPage } = status
  // The page count is a dependency because a fast answer can skip the render
  // where isFetching is true, leaving the other values unchanged.
  const loadedPages = status.data?.pages.length ?? 0
  useEffect(() => {
    if (hasNextPage && !isFetching && !isFetchNextPageError) void fetchNextPage({ cancelRefetch: false })
  }, [hasNextPage, isFetching, isFetchNextPageError, fetchNextPage, loadedPages])

  const statuses = useMemo(() => statusesFrom(status.data?.pages ?? []), [status.data])
  // A zoom that names no group of the current search and grouping is ignored,
  // and dropped from the URL once that grouping has loaded.
  const zoomed = zoom ? groups.data?.groups.find((g) => g.label === zoom) : undefined
  const groupsCurrent = groups.data !== undefined && !groups.isPlaceholderData && groups.data.group_by === groupBy
  useEffect(() => {
    if (zoom && groupsCurrent && !zoomed) onZoom('', { replace: true })
  }, [zoom, groupsCurrent, zoomed, onZoom])

  const familyList = families.data?.families
  const shownFamilies = useMemo(
    () => (familyList && zoomed ? familiesInGroup(familyList, zoomed.images) : familyList),
    [familyList, zoomed],
  )
  const rows = useMemo(() => (shownFamilies ? sortFamilyRows(familyRows(shownFamilies, statuses), sort) : []), [shownFamilies, statuses, sort])

  // Focus moves only on zooms the user makes here, never on ones from the URL.
  const container = useRef<HTMLDivElement>(null)
  const pendingFocus = useRef<PendingFocus | null>(null)
  const zoomSource = useRef<ZoomSource | null>(null)
  useEffect(() => {
    const pending = pendingFocus.current
    if (!pending || !container.current) return
    const target =
      pending.to === 'back'
        ? container.current.querySelector<HTMLElement>('[data-zoom-back]')
        : [...container.current.querySelectorAll<HTMLElement>(pending.from === 'bar' ? '[data-bar]' : '[data-zoom-group]')].find(
            (el) => el.getAttribute(pending.from === 'bar' ? 'data-bar' : 'data-zoom-group') === pending.label,
          )
    // The target appears once the new zoom has been drawn.
    if (!target) return
    pendingFocus.current = null
    target.focus()
  })

  // The caller's onZoom changes every render; a stable zoomIn keeps the
  // memoized treemap from redrawing when the table is sorted or paged.
  const latestOnZoom = useRef(onZoom)
  useEffect(() => {
    latestOnZoom.current = onZoom
  })
  const zoomIn = useCallback((label: string, from: ZoomSource) => {
    zoomSource.current = from
    pendingFocus.current = { to: 'back' }
    latestOnZoom.current(label)
  }, [])
  const zoomInTreemap = useCallback((label: string) => zoomIn(label, 'treemap'), [zoomIn])

  function zoomOut() {
    if (!zoomed || !groups.data) return
    const from = zoomSource.current ?? (wide() ? 'treemap' : 'bar')
    // Bars are found by their shown label, which names how many groups Other merges.
    const label = from === 'bar' ? groupLabel(zoomed, groups.data.group_by) : zoomed.label
    pendingFocus.current = { to: 'group', label, from }
    zoomSource.current = null
    onZoom('')
  }

  function onKeyDown(e: KeyboardEvent) {
    if (e.key === 'Escape' && zoomed) {
      e.preventDefault()
      zoomOut()
    }
  }

  if (families.isPending || groups.isPending) return <Loading label="Loading image families…" />
  if (families.error) return <ErrorState error={families.error} onRetry={() => families.refetch()} />
  if (groups.error) return <ErrorState error={groups.error} onRetry={() => groups.refetch()} />
  const { total } = families.data
  if (total === 0) return <p className="text-sm text-zinc-600 dark:text-zinc-400">No images match.</p>

  const checked = status.data?.pages.reduce((n, p) => n + p.count, 0) ?? 0
  const done = status.data !== undefined && !hasNextPage
  const counts = statusCounts(families.data.families, statuses)
  // The legend counts what the map shows; the status line, the whole check.
  const legendCounts = zoomed ? statusCounts([zoomed], statuses) : counts
  // Labels follow the grouping the shown data was made with, not the one loading.
  const shownGroupBy = groups.data.group_by
  const zoomedLabel = zoomed && groupLabel(zoomed, shownGroupBy)
  const pages = Math.max(1, Math.ceil(rows.length / PAGE_SIZE))
  const current = Math.min(page, pages)

  return (
    <div className="space-y-4">
      <div role="group" aria-label="Group by" className="flex flex-wrap items-center gap-2 text-sm text-zinc-600 dark:text-zinc-400">
        <span aria-hidden="true">Group by</span>
        <span className={segmentGroup}>
          {groupings.map((g) => (
            <button
              key={g.value}
              type="button"
              aria-pressed={groupBy === g.value}
              onClick={() => onGroupBy(g.value)}
              className={segmentButton(groupBy === g.value)}
            >
              {g.label}
            </button>
          ))}
        </span>
      </div>
      <div className="space-y-2">
        <p className="text-sm text-zinc-600 dark:text-zinc-400" aria-live="polite">
          {mapStatusText(done ? total : checked, total, families.data.families.length, counts.free)}
          {!done && <span className="block text-xs">The first check can take about 30 seconds; colors fill in as it goes.</span>}
        </p>
        <ul className="flex flex-wrap gap-x-4 gap-y-1 text-xs text-zinc-600 dark:text-zinc-400">
          {legend
            .filter((l) => l.status !== 'unknown' || legendCounts.unknown > 0)
            .map((l) => (
              <li key={l.status} className="flex items-center gap-1.5">
                <span aria-hidden="true" className={`inline-block size-3 rounded-sm ${freeStatusColor[l.status].swatch}`} />
                {l.label}: {legendCounts[l.status].toLocaleString('en-US')}
              </li>
            ))}
        </ul>
      </div>

      {status.error && (
        <ErrorState error={status.error} onRetry={() => (status.data ? fetchNextPage() : status.refetch())} />
      )}

      <div ref={container} onKeyDown={onKeyDown} className="space-y-2">
        {zoomed && (
          <nav aria-label="Map zoom" className="text-sm">
            <ol className="flex flex-wrap items-center gap-1.5">
              <li>
                <button
                  type="button"
                  data-zoom-back=""
                  aria-label={`All groups, zoomed into ${zoomedLabel}`}
                  onClick={zoomOut}
                  className="rounded font-medium text-indigo-700 hover:underline focus-visible:outline-2 focus-visible:outline-indigo-600 dark:text-indigo-300"
                >
                  All groups
                </button>
              </li>
              <li aria-hidden="true" className="text-zinc-500">
                ›
              </li>
              <li aria-current="location" className="min-w-0 font-medium wrap-break-word">
                {zoomedLabel}
              </li>
            </ol>
          </nav>
        )}
        <Card className="p-2 sm:p-2">
          <div className="hidden sm:block">
            <CatalogTreemap
              groups={groups.data.groups}
              groupBy={shownGroupBy}
              zoomed={zoomed}
              statuses={statuses}
              summary={
                zoomed
                  ? zoomedSummary(zoomedLabel ?? '', zoomed.images.length, zoomed.blocks.length, done ? legendCounts.free : null)
                  : treemapSummary(total, groups.data.groups.length, shownGroupBy, done ? counts.free : null)
              }
              onZoom={zoomInTreemap}
            />
          </div>
          <div className="p-2 sm:hidden">
            {zoomed ? (
              <HorizontalBars
                bars={blockBars(zoomed).map((b) => {
                  const c = statusCounts([b], statuses)
                  return {
                    label: b.label,
                    value: b.images.length,
                    valueLabel: `${c.free.toLocaleString('en-US')} free`,
                    segments: legend.map((l) => ({ value: c[l.status], colorClass: freeStatusColor[l.status].fill })),
                  }
                })}
              />
            ) : (
              <HorizontalBars
                bars={groups.data.groups.map((g) => {
                  const c = statusCounts([g], statuses)
                  const label = groupLabel(g, shownGroupBy)
                  return {
                    label,
                    value: g.images.length,
                    valueLabel: `${c.free.toLocaleString('en-US')} free`,
                    accessibleLabel: zoomLabel(label, g.images.length, c.free),
                    segments: legend.map((l) => ({ value: c[l.status], colorClass: freeStatusColor[l.status].fill })),
                  }
                })}
                pressable={false}
                onSelect={(label) => {
                  const group = groups.data.groups.find((g) => groupLabel(g, shownGroupBy) === label)
                  if (group) zoomIn(group.label, 'bar')
                }}
              />
            )}
          </div>
        </Card>
      </div>

      <FamilyTable rows={rows.slice((current - 1) * PAGE_SIZE, current * PAGE_SIZE)} sort={sort} onSort={onSort} />
      {pages > 1 && <Pagination page={current} pages={pages} onPage={onPage} />}
    </div>
  )
}
