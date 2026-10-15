import { useEffect, useRef, useState, type KeyboardEvent, type PointerEvent } from 'react'
import type { HistoryPoint } from '../api'
import { chart, chartLayout, nearestIndex, neighbours, pointParts, pointText, polyline, type ChartRange } from '../history'
import { historyChartColor as color } from '../styles'

const n = (value: number) => value.toLocaleString('en-US')

const CARD_WIDTH = 200
const LINE = 16

/**
 * Total and free images per snapshot on a time axis (HV-15). Plain SVG sized by
 * attributes, so it needs no inline styles. Internet Archive captures are
 * hollow points and daily snapshots filled ones. Pointing at or tapping the
 * chart, or stepping with the keyboard, shows the nearest snapshot in a card
 * drawn inside the SVG (HV-19 to HV-21). The SVG is hidden from assistive
 * technology; the live region and the table of every point are its text
 * equivalent. In the "daily" range only daily snapshots are drawn, on a zoomed
 * axis, and only the total (HV-22). `dailyOffered` lets the card suggest that
 * range (HV-20).
 */
export function GrowthChart({ points, range = 'all', dailyOffered = false }: { points: HistoryPoint[]; range?: ChartRange; dailyOffered?: boolean }) {
  const { x, y, ticks, years, shown, indexes, startsAtZero } = chartLayout(points, range)
  const [sel, setSel] = useState<number | null>(null)
  const [focused, setFocused] = useState(false)
  const [announced, setAnnounced] = useState('')
  const svgRef = useRef<SVGSVGElement>(null)
  const wrapRef = useRef<HTMLDivElement>(null)

  // A tap outside the chart hides the card (HV-19.4), as touch has no pointer leave.
  useEffect(() => {
    const away = (e: globalThis.PointerEvent) => {
      if (!wrapRef.current?.contains(e.target as Node)) setSel(null)
    }
    document.addEventListener('pointerdown', away)
    return () => document.removeEventListener('pointerdown', away)
  }, [])

  if (shown.length === 0) return null
  const daily = range === 'daily'
  const totalPts = shown.map((p) => ({ x: x(p.date), y: y(p.total) }))
  const freePts = daily ? [] : shown.filter((p) => p.free !== undefined).map((p) => ({ date: p.date, x: x(p.date), y: y(p.free ?? 0) }))
  const baseline = chart.height - chart.bottom
  const xs = totalPts.map((p) => p.x)

  function pick(e: PointerEvent) {
    const box = svgRef.current?.getBoundingClientRect()
    if (!box || box.width === 0) return
    setSel(nearestIndex(xs, ((e.clientX - box.left) * chart.width) / box.width))
  }

  function onKeyDown(e: KeyboardEvent) {
    const last = shown.length - 1
    let next: number
    switch (e.key) {
      case 'ArrowLeft':
        next = sel === null ? last : Math.max(0, sel - 1)
        break
      case 'ArrowRight':
        next = sel === null ? 0 : Math.min(last, sel + 1)
        break
      case 'Home':
        next = 0
        break
      case 'End':
        next = last
        break
      case 'Escape':
        e.preventDefault()
        setSel(null)
        return
      default:
        return
    }
    e.preventDefault()
    setSel(next)
    setAnnounced(pointText(points, indexes[next]))
  }

  return (
    <div className="space-y-3">
      {!startsAtZero && <p className="text-xs text-zinc-600 dark:text-zinc-400">Axis doesn't start at zero</p>}
      <div
        ref={wrapRef}
        role="group"
        tabIndex={0}
        aria-label="Growth chart: use the left and right arrow keys to move between snapshots"
        onKeyDown={onKeyDown}
        onFocus={() => setFocused(true)}
        onBlur={() => setFocused(false)}
        className="rounded-md focus-visible:outline-2 focus-visible:outline-indigo-600"
      >
        <svg ref={svgRef} viewBox={`0 0 ${chart.width} ${chart.height}`} className="h-auto w-full" aria-hidden="true" data-testid="growth-chart">
          {ticks.map((t) => (
            <g key={t.value}>
              <line x1={chart.left} x2={chart.width - chart.right} y1={t.y} y2={t.y} className={color.grid} />
              <text x={chart.left - 6} y={t.y + 4} textAnchor="end" fontSize="12" className={color.text}>
                {n(t.value)}
              </text>
            </g>
          ))}
          <text x={chart.left} y={chart.height - 8} fontSize="12" className={color.text}>
            {shown[0].date}
          </text>
          <text x={chart.width - chart.right} y={chart.height - 8} textAnchor="end" fontSize="12" className={color.text}>
            {shown[shown.length - 1].date}
          </text>
          {years.map((yr) => (
            <line key={yr.label} x1={yr.x} x2={yr.x} y1={baseline} y2={baseline + 4} className={color.grid} />
          ))}
          <polyline points={polyline(totalPts)} fill="none" strokeWidth="2" strokeLinejoin="round" className={color.total.line} />
          {freePts.length > 1 && <polyline points={polyline(freePts)} fill="none" strokeWidth="2" strokeLinejoin="round" className={color.free.line} />}
          {shown.map((p, i) => (
            <circle
              key={p.date}
              data-point={p.source}
              cx={totalPts[i].x}
              cy={totalPts[i].y}
              r="3.5"
              strokeWidth="2"
              className={`${color.total.point} ${p.source === 'archive' ? color.hollow : color.total.filled}`}
            />
          ))}
          {freePts.map((p) => (
            <circle key={p.date} data-point="free" cx={p.x} cy={p.y} r="3.5" strokeWidth="2" className={`${color.free.point} ${color.free.filled}`} />
          ))}
          {sel !== null && <Card points={points} shown={shown} indexes={indexes} totalPts={totalPts} sel={sel} range={range} dailyOffered={dailyOffered} xs={xs} />}
          <rect
            data-testid="chart-overlay"
            x={chart.left}
            y={chart.top}
            width={chart.width - chart.left - chart.right}
            height={baseline - chart.top}
            fill="transparent"
            onPointerMove={pick}
            onPointerDown={pick}
            onPointerLeave={(e) => {
              if (e.pointerType !== 'touch') setSel(null)
            }}
          />
        </svg>
        {focused && <p className="mt-1 text-xs text-zinc-600 dark:text-zinc-400">Use ← and → to move between snapshots, Home and End for the first and last, Escape to close.</p>}
        <p className="sr-only" aria-live="polite" data-testid="chart-live">
          {announced}
        </p>
      </div>
      {sel !== null && (
        <p data-testid="chart-selection" className="text-sm sm:hidden">
          {pointText(points, indexes[sel])}
          {selectionLines({ points, shown, indexes, xs, sel, range, dailyOffered }).extra.map((line, i) => (i === 0 ? ` · ${line}` : ` ${line}`)).join('')}
        </p>
      )}
      {daily && <p className="text-xs text-zinc-600 dark:text-zinc-400">Free images aren't plotted in this range; the tooltip and the table show them.</p>}

      <ul className="flex flex-wrap gap-x-4 gap-y-1 text-xs text-zinc-600 dark:text-zinc-400">
        <li className="flex items-center gap-1.5">
          <span className={`inline-block h-0.5 w-4 ${color.total.swatch}`} aria-hidden="true" /> Images
        </li>
        {freePts.length > 0 && (
          <li className="flex items-center gap-1.5">
            <span className={`inline-block h-0.5 w-4 ${color.free.swatch}`} aria-hidden="true" /> Free (from {freePts[0].date})
          </li>
        )}
        {!daily && (
          <li className="flex items-center gap-1.5">
            <span className="inline-block size-2.5 rounded-full border-2 border-indigo-500 dark:border-indigo-400" aria-hidden="true" /> Internet Archive capture
          </li>
        )}
        <li className="flex items-center gap-1.5">
          <span className={`inline-block size-2.5 rounded-full ${color.total.swatch}`} aria-hidden="true" /> Daily snapshot
        </li>
      </ul>

      <details className="text-sm">
        <summary className="cursor-pointer font-medium focus-visible:outline-2 focus-visible:outline-indigo-600">Table of snapshots</summary>
        <div className="mt-2 overflow-x-auto">
          <table className="w-full text-left text-xs tabular-nums">
            <caption className="sr-only">Images per snapshot</caption>
            <thead>
              <tr className="text-zinc-500 dark:text-zinc-400">
                <th scope="col" className="py-1 pr-3 font-medium">Date</th>
                <th scope="col" className="py-1 pr-3 font-medium">Source</th>
                <th scope="col" className="py-1 pr-3 text-right font-medium">Images</th>
                <th scope="col" className="py-1 text-right font-medium">Free</th>
              </tr>
            </thead>
            <tbody>
              {points.map((p) => (
                <tr key={p.date} className="border-t border-zinc-200 dark:border-zinc-800">
                  <th scope="row" className="whitespace-nowrap py-1 pr-3 font-normal">{p.date}</th>
                  <td className="py-1 pr-3">{p.source === 'archive' ? 'Internet Archive capture' : 'Daily snapshot'}</td>
                  <td className="py-1 pr-3 text-right">{n(p.total)}</td>
                  <td className="py-1 text-right">{p.free === undefined ? 'unknown' : n(p.free)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </details>
    </div>
  )
}

/**
 * The selected snapshot's text: `main` is its date, source, counts and change;
 * `extra` the note about snapshots drawn close to it and the suggestion of the
 * daily range (HV-20), in short lines for the card.
 */
function selectionLines({ points, shown, indexes, xs, sel, range, dailyOffered }: {
  points: HistoryPoint[]
  shown: HistoryPoint[]
  indexes: number[]
  xs: number[]
  sel: number
  range: ChartRange
  dailyOffered: boolean
}) {
  const parts = pointParts(points, indexes[sel])
  const main = [parts[0], parts[1], `${parts[2]} · ${parts[3]}`, parts[4]]
  const extra: string[] = []
  const near = neighbours(xs, sel)
  if (near.length > 0) {
    const dates = near.map((j) => shown[j].date).sort()
    extra.push(`${near.length} more ${near.length === 1 ? 'snapshot is' : 'snapshots are'} drawn close`)
    extra.push('to this one')
    extra.push(`(${dates[0]}${dates.length > 1 ? ` to ${dates[dates.length - 1]}` : ''})`)
    if (range === 'all' && dailyOffered && shown[sel].source === 'live') extra.push('Try "Since daily tracking"')
  }
  return { main, extra }
}

/** The guide line, the ringed point and the card for the selected snapshot (HV-19, HV-20). */
function Card({ points, shown, indexes, totalPts, sel, range, dailyOffered, xs }: {
  points: HistoryPoint[]
  shown: HistoryPoint[]
  indexes: number[]
  totalPts: { x: number; y: number }[]
  sel: number
  range: ChartRange
  dailyOffered: boolean
  xs: number[]
}) {
  const { main, extra } = selectionLines({ points, shown, indexes, xs, sel, range, dailyOffered })
  const lines = [...main, ...extra]
  const p = totalPts[sel]
  const height = lines.length * LINE + 10
  const left = p.x + 12 + CARD_WIDTH > chart.width - chart.right ? p.x - 12 - CARD_WIDTH : p.x + 12
  const top = Math.min(Math.max(p.y - height / 2, chart.top), chart.height - chart.bottom - height)
  return (
    <g pointerEvents="none">
      <line x1={p.x} x2={p.x} y1={chart.top} y2={chart.height - chart.bottom} strokeWidth="1" strokeDasharray="3 3" className={color.guide} />
      <circle cx={p.x} cy={p.y} r="6" fill="none" strokeWidth="2" className={color.total.point} />
      {/* Below sm the SVG is scaled down too far to read the card, so the text goes under the chart instead (HV-23). */}
      <g data-testid="chart-card" className="hidden sm:inline">
        <rect x={left} y={top} width={CARD_WIDTH} height={height} rx="6" strokeWidth="1" className={color.card} />
        {lines.map((line, i) => (
          <text key={i} x={left + 8} y={top + 1 + (i + 1) * LINE} fontSize="12" fontWeight={i === 0 ? 600 : 400} className={i < main.length ? color.cardText : color.cardMuted}>
            {line}
          </text>
        ))}
      </g>
    </g>
  )
}
