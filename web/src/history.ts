// Wording and arithmetic for the catalog History view. Pure functions, kept
// out of the component files so React fast refresh keeps working.

import type { HistoryEntry, HistoryPoint, HistoryReport } from './api'

const n = (value: number) => value.toLocaleString('en-US')

/** How many names a list shows before the rest go behind "Show all" (HV-8.3). */
export const NAMES_SHOWN = 10

/** The date a snapshot was taken: the entry's day, or the end of its range. */
export const entryEnd = (e: HistoryEntry) => e.to ?? e.date ?? ''

/** An entry's date as a heading: an exact day, or a range between two snapshots (HV-7). */
export function entryDate(e: HistoryEntry): string {
  if (e.date) return e.date
  if (e.kind === 'no_changes') return `${e.from} to ${e.to}`
  return `Between ${e.from} and ${e.to}`
}

/** One line for an entry, e.g. "Between 2026-10-08 and 2026-10-10: 5 added". */
export function entrySummary(e: HistoryEntry, searching: boolean): string {
  const when = entryDate(e)
  if (e.kind === 'first_capture') {
    const what = searching ? (e.total === 1 ? 'matching image' : 'matching images') : e.total === 1 ? 'image' : 'images'
    return `${when}: first capture, ${n(e.total)} ${what}`
  }
  if (e.kind === 'no_changes') return `${when}: no changes`
  return `${when}: ${changeCounts(e).join(', ')}`
}

/** The counts of an entry's changes, such as "5 added" or "15 not in the 2026-10-08 capture". */
export function changeCounts(e: HistoryEntry): string[] {
  const counts: string[] = []
  if (e.added.length) counts.push(`${n(e.added.length)} added`)
  if (e.back.length) counts.push(`${n(e.back.length)} back`)
  if (e.removed.length) counts.push(`${n(e.removed.length)} removed`)
  if (e.not_in_capture.length) counts.push(`${n(e.not_in_capture.length)} not in the ${entryEnd(e)} capture`)
  if (e.became_free.length) counts.push(`${n(e.became_free.length)} became free`)
  if (e.became_subscription.length) counts.push(`${n(e.became_subscription.length)} became subscription`)
  return counts
}

/** An entry's name lists, labeled; archive-era wording never says "removed" (HV-9). */
export function entryLists(e: HistoryEntry): { label: string; names: string[] }[] {
  return [
    { label: 'Added', names: e.added },
    { label: 'Back (listed again after a capture that missed them)', names: e.back },
    { label: 'Removed', names: e.removed },
    { label: `Not in the ${entryEnd(e)} capture`, names: e.not_in_capture },
    { label: 'Became free', names: e.became_free },
    { label: 'Became subscription', names: e.became_subscription },
  ].filter((l) => l.names.length > 0)
}

/** The summary tiles' numbers: images now, free now, and the change since the first point with a match. */
export function historyTiles(r: HistoryReport) {
  const first = r.points.find((p) => p.total > 0) ?? r.points[0]
  const delta = r.total_now - (first?.total ?? 0)
  return {
    // A search can first match after the first capture; the tile then counts from there.
    label: first === r.points[0] ? 'Since first capture' : 'Since first match',
    images: r.total_now,
    free: r.free_now,
    delta: `${delta < 0 ? '−' : '+'}${n(Math.abs(delta))}`,
    from: first ? `from ${n(first.total)} on ${first.date}` : '',
  }
}

/** Splits the timeline into entries from daily tracking and Internet Archive captures (HV-18). */
export function splitEntries(entries: HistoryEntry[]) {
  return { live: entries.filter((e) => !e.archive), archive: entries.filter((e) => e.archive) }
}

/**
 * Where the chart draws the data, in pixels, with room for axis labels. `width`
 * is only the default, used until the real width is measured (HV-24).
 */
export const chart = { width: 480, height: 240, left: 44, right: 12, top: 12, bottom: 28 }

const DAY_MS = 86_400_000

export type ChartRange = 'all' | 'daily'

/** Which count the chart plots: the total, or the free count on its own axis (HV-25). */
export type ChartSeries = 'total' | 'free'

/** Snapshots drawn within this many viewBox units of each other overlap visually (HV-20). */
export const NEIGHBOUR_UNITS = 6

/**
 * Positions the points on a time axis (x) and a linear axis (y). For the total,
 * "all" starts the axis at zero and "daily" keeps only daily snapshots and zooms
 * the axis around them (HV-22). The free series keeps the snapshots with a free
 * count, whatever the range, on an axis around them (HV-25). `indexes` maps each
 * drawn point to its index in `points`, and `value` reads the plotted count.
 */
export function chartLayout(points: HistoryPoint[], range: ChartRange = 'all', width: number = chart.width, series: ChartSeries = 'total') {
  const free = series === 'free'
  const value = (p: HistoryPoint) => (free ? (p.free ?? 0) : p.total)
  const keep = (p: HistoryPoint) => (free ? p.free !== undefined : range === 'all' || p.source === 'live')
  const indexes = points.flatMap((p, i) => (keep(p) ? [i] : []))
  const shown = indexes.map((i) => points[i])
  const times = shown.map((p) => Date.parse(p.date))
  const min = Math.min(...times)
  const max = Math.max(...times)
  const values = shown.map(value)
  const [lo, top] = range === 'all' && !free ? [0, niceMax(Math.max(1, ...values))] : niceDomain(Math.min(...values), Math.max(...values))
  const w = width - chart.left - chart.right
  const h = chart.height - chart.top - chart.bottom
  const x = (date: string) => chart.left + (max > min ? ((Date.parse(date) - min) / (max - min)) * w : w / 2)
  const y = (value: number) => chart.top + h - ((value - lo) / (top - lo)) * h
  const ticks = [lo, (lo + top) / 2, top].map((v) => ({ value: v, y: y(v) }))
  const years: { label: string; x: number }[] = []
  for (let year = new Date(min).getUTCFullYear() + 1; Date.UTC(year, 0, 1) <= max; year++) {
    years.push({ label: String(year), x: x(new Date(Date.UTC(year, 0, 1)).toISOString().slice(0, 10)) })
  }
  return { x, y, ticks, years, days: Math.round((max - min) / DAY_MS), shown, indexes, value, startsAtZero: lo === 0 }
}

/**
 * A y-axis [lo, hi] around the values with a tick in the middle: round numbers
 * that contain the data, with a flat series sitting mid-plot (HV-22.4).
 */
export function niceDomain(min: number, max: number): [number, number] {
  const center = (min + max) / 2
  const half = max > min ? ((max - min) / 2) * 1.1 : Math.max(1, Math.abs(center) * 0.003)
  let step = niceMax(half)
  for (;;) {
    let lo = Math.round((center - step) / step) * step
    if (lo < 0) lo = 0
    if (lo <= min && lo + 2 * step >= max) return [lo, lo + 2 * step]
    step = niceMax(step * 1.01)
  }
}

/** The index of the x position nearest `px`; `xs` must be sorted ascending. */
export function nearestIndex(xs: number[], px: number): number {
  let lo = 0
  let hi = xs.length - 1
  while (lo < hi) {
    const mid = (lo + hi) >> 1
    if (xs[mid] < px) lo = mid + 1
    else hi = mid
  }
  return lo > 0 && px - xs[lo - 1] <= xs[lo] - px ? lo - 1 : lo
}

/** The other indexes whose x lies within `within` units of the one at `i` (HV-20). */
export function neighbours(xs: number[], i: number, within = NEIGHBOUR_UNITS): number[] {
  return xs.flatMap((v, j) => (j !== i && Math.abs(v - xs[i]) <= within ? [j] : []))
}

/** A snapshot as parts: date, source, images, free images and the change (HV-19). */
export function pointParts(points: HistoryPoint[], i: number): string[] {
  const p = points[i]
  const prev = points[i - 1]
  let change = 'first capture'
  if (prev) {
    const d = p.total - prev.total
    change = d === 0 ? `no change since ${prev.date}` : `${d < 0 ? '−' : '+'}${n(Math.abs(d))} since ${prev.date}`
  }
  return [
    p.date,
    p.source === 'archive' ? 'Internet Archive capture' : 'Daily snapshot',
    `${n(p.total)} ${p.total === 1 ? 'image' : 'images'}`,
    p.free === undefined ? 'free unknown' : `${n(p.free)} free`,
    change,
  ]
}

/** The same as one line, for the live region (HV-21). */
export const pointText = (points: HistoryPoint[], i: number) => pointParts(points, i).join(' · ')

/** Rounds up to 1, 2 or 5 times a power of ten, so the axis ends on a round number. */
export function niceMax(value: number): number {
  const pow = 10 ** Math.floor(Math.log10(value))
  for (const m of [1, 2, 5, 10]) if (value <= m * pow) return m * pow
  return 10 * pow
}

/** Points as an SVG polyline's points attribute. */
export function polyline(points: { x: number; y: number }[]): string {
  return points.map((p) => `${p.x.toFixed(1)},${p.y.toFixed(1)}`).join(' ')
}
