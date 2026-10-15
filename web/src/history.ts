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

/** Where the chart draws the data: a 480×240 viewBox with room for axis labels. */
export const chart = { width: 480, height: 240, left: 44, right: 12, top: 12, bottom: 28 }

const DAY_MS = 86_400_000

/** Positions the points on a time axis (x) and a linear axis from zero (y). */
export function chartLayout(points: HistoryPoint[]) {
  const times = points.map((p) => Date.parse(p.date))
  const min = Math.min(...times)
  const max = Math.max(...times)
  const top = niceMax(Math.max(1, ...points.map((p) => p.total)))
  const w = chart.width - chart.left - chart.right
  const h = chart.height - chart.top - chart.bottom
  const x = (date: string) => chart.left + (max > min ? ((Date.parse(date) - min) / (max - min)) * w : w / 2)
  const y = (value: number) => chart.top + h - (value / top) * h
  const ticks = [0, top / 2, top].map((v) => ({ value: v, y: y(v) }))
  const years: { label: string; x: number }[] = []
  for (let year = new Date(min).getUTCFullYear() + 1; Date.UTC(year, 0, 1) <= max; year++) {
    years.push({ label: String(year), x: x(new Date(Date.UTC(year, 0, 1)).toISOString().slice(0, 10)) })
  }
  return { x, y, ticks, years, days: Math.round((max - min) / DAY_MS) }
}

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
