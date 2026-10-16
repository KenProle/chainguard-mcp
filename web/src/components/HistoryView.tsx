import { useQuery } from '@tanstack/react-query'
import { api, ApiError, type HistoryEntry } from '../api'
import type { ChartRange, ChartSeries } from '../history'
import { NAMES_SHOWN, entryLists, entrySummary, historyTiles, splitEntries } from '../history'
import { segmentButton, segmentGroup, statTileClass } from '../styles'
import { GrowthChart } from './GrowthChart'
import { Card, ErrorState, Loading } from './ui'

const n = (value: number) => value.toLocaleString('en-US')

/**
 * How the catalog has changed over time, from the project's catalog history
 * (HV-14): the age of the data, summary tiles, a growth chart and a timeline
 * of changes, all following the catalog's search.
 */
export function HistoryView({ query, range = 'all', onRange, series = 'total', onSeries }: {
  query: string
  range?: ChartRange
  onRange?: (range: ChartRange) => void
  series?: ChartSeries
  onSeries?: (series: ChartSeries) => void
}) {
  const { data, error, isPending, refetch } = useQuery({
    queryKey: ['history', query],
    queryFn: ({ signal }) => api.history(query, signal),
  })
  if (isPending) return <Loading label="Loading catalog history…" />
  if (error instanceof ApiError && error.code === 'history_unavailable') {
    return (
      <Card>
        <h2 className="font-semibold">The catalog history couldn't be loaded</h2>
        <p className="mt-1 text-sm text-zinc-600 dark:text-zinc-400">
          This server has no copy of the history and couldn't download one. The rest of the catalog works as usual.
        </p>
      </Card>
    )
  }
  if (error) return <ErrorState error={error} onRetry={() => refetch()} />

  const tiles = historyTiles(data)
  const searching = query !== ''
  const { live, archive } = splitEntries(data.entries)
  // "Since daily tracking" needs at least 2 daily snapshots; without them `range=daily` is ignored (HV-22.3).
  const dailyOffered = data.points.filter((p) => p.source === 'live').length >= 2
  const effective: ChartRange = range === 'daily' && dailyOffered ? 'daily' : 'all'
  // "Free images" needs at least 2 snapshots with a free count; without them `series=free` is ignored (HV-25.3).
  const freeOffered = data.points.filter((p) => p.free !== undefined).length >= 2
  const shownSeries: ChartSeries = series === 'free' && freeOffered ? 'free' : 'total'
  const firstDate = data.points[0]?.date ?? ''
  const unchanged = data.entries.length > 0 && data.entries.every((e) => e.kind !== 'changes')
  return (
    <div className="space-y-5">
      <div className="space-y-2 text-sm text-zinc-600 dark:text-zinc-400">
        <p>
          Last snapshot: {data.last_snapshot} ·{' '}
          {data.copy_source === 'build' ? 'embedded in this build' : 'fetched from the project’s data branch'}
        </p>
        {data.stalled && data.note && (
          <p role="status" className="rounded-lg border border-amber-300 bg-amber-50 p-3 text-amber-900 dark:border-amber-500/40 dark:bg-amber-500/10 dark:text-amber-200">
            {data.note}
          </p>
        )}
      </div>

      <dl className="grid grid-cols-1 gap-3 sm:grid-cols-3">
        <Tile label="Images" value={n(tiles.images)} />
        {tiles.free !== undefined && <Tile label="Free" value={n(tiles.free)} />}
        <Tile label={tiles.label} value={tiles.delta} note={tiles.from} />
      </dl>

      <Card>
        <h2 className="mb-3 font-semibold">{searching ? `Images matching “${query}”` : 'Images in the catalog'}</h2>
        <div className="mb-3 flex flex-wrap gap-2">
          {freeOffered && (
            <div role="group" aria-label="Chart series" className={segmentGroup}>
              <button type="button" aria-pressed={shownSeries === 'total'} onClick={() => onSeries?.('total')} className={segmentButton(shownSeries === 'total')}>
                All images
              </button>
              <button type="button" aria-pressed={shownSeries === 'free'} onClick={() => onSeries?.('free')} className={segmentButton(shownSeries === 'free')}>
                Free images
              </button>
            </div>
          )}
          {/* Free counts exist only for daily snapshots, so the range has nothing to change there (HV-25). */}
          {shownSeries === 'total' && (
            <div role="group" aria-label="Chart range" className={segmentGroup}>
              <button type="button" aria-pressed={effective === 'all'} onClick={() => onRange?.('all')} className={segmentButton(effective === 'all')}>
                All time
              </button>
              {dailyOffered && (
                <button type="button" aria-pressed={effective === 'daily'} onClick={() => onRange?.('daily')} className={segmentButton(effective === 'daily')}>
                  Since daily tracking
                </button>
              )}
            </div>
          )}
        </div>
        <GrowthChart
          key={`${effective}|${shownSeries}|${query}|${data.last_snapshot}`}
          points={data.points}
          range={effective}
          series={shownSeries}
          dailyOffered={dailyOffered}
        />
      </Card>

      <section aria-labelledby="timeline-heading" className="space-y-3">
        <h2 id="timeline-heading" className="font-semibold">
          Changes
        </h2>
        <p className="text-xs text-zinc-600 dark:text-zinc-400">{data.dates_note}</p>
        {data.entries.length === 0 ? (
          <p className="text-sm text-zinc-600 dark:text-zinc-400">No images match.</p>
        ) : (
          <>
            {unchanged && (
              <p className="text-sm">No changes are recorded for matching images since the first capture, {firstDate}.</p>
            )}
            {live.length > 0 && <Timeline entries={live} searching={searching} />}
            {archive.length > 0 && (
              <details className="rounded-xl border border-zinc-200 dark:border-zinc-800">
                <summary className="cursor-pointer px-4 py-3 text-sm font-medium focus-visible:outline-2 focus-visible:outline-indigo-600">
                  Before daily tracking (Internet Archive captures) · {archive.length} {archive.length === 1 ? 'entry' : 'entries'}
                </summary>
                <div className="border-t border-zinc-200 px-4 py-3 dark:border-zinc-800">
                  <Timeline entries={archive} searching={searching} />
                </div>
              </details>
            )}
          </>
        )}
      </section>
    </div>
  )
}

function Tile({ label, value, note }: { label: string; value: string; note?: string }) {
  return (
    <div className={statTileClass}>
      <dt className="text-xs text-zinc-600 dark:text-zinc-400">{label}</dt>
      <dd className="text-2xl font-semibold tabular-nums">{value}</dd>
      {note && <dd className="text-xs text-zinc-600 dark:text-zinc-400">{note}</dd>}
    </div>
  )
}

function Timeline({ entries, searching }: { entries: HistoryEntry[]; searching: boolean }) {
  return (
    <ol className="space-y-3">
      {entries.map((e) => (
        <li key={`${e.date ?? e.from}-${e.to ?? ''}-${e.kind}`} className="text-sm">
          <p className="font-medium">{entrySummary(e, searching)}</p>
          {e.free_first_recorded !== undefined && (
            <p className="text-xs text-zinc-600 dark:text-zinc-400">
              Free-tier tracking starts here: {n(e.free_first_recorded)} free {e.free_first_recorded === 1 ? 'image' : 'images'} recorded.
            </p>
          )}
          {entryLists(e).map((l) => (
            <NameList key={l.label} label={l.label} names={l.names} />
          ))}
        </li>
      ))}
    </ol>
  )
}

/** A labeled list of image names: the first few, and the rest behind a disclosure (HV-8.3). */
function NameList({ label, names }: { label: string; names: string[] }) {
  const shown = names.slice(0, NAMES_SHOWN)
  const rest = names.slice(NAMES_SHOWN)
  return (
    <div className="mt-1">
      <p className="text-xs text-zinc-600 dark:text-zinc-400">
        {label} ({n(names.length)})
      </p>
      <ul className="flex flex-wrap gap-x-3 gap-y-0.5 font-mono text-xs">
        {shown.map((name) => (
          <li key={name} className="break-all">
            {name}
          </li>
        ))}
      </ul>
      {rest.length > 0 && (
        <details className="mt-1">
          <summary className="cursor-pointer text-xs text-indigo-600 focus-visible:outline-2 focus-visible:outline-indigo-600 dark:text-indigo-400">
            Show {n(rest.length)} more
          </summary>
          <ul className="mt-1 flex flex-wrap gap-x-3 gap-y-0.5 font-mono text-xs">
            {rest.map((name) => (
              <li key={name} className="break-all">
                {name}
              </li>
            ))}
          </ul>
        </details>
      )}
    </div>
  )
}
