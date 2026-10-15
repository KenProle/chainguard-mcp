import type { HistoryPoint } from '../api'
import { chart, chartLayout, polyline } from '../history'
import { historyChartColor as color } from '../styles'

const n = (value: number) => value.toLocaleString('en-US')

/**
 * Total and free images per snapshot on a time axis (HV-15). Plain SVG sized by
 * attributes, so it needs no inline styles. Internet Archive captures are
 * hollow points and daily snapshots filled ones. The SVG is hidden from
 * assistive technology; the table of every point is its text equivalent.
 */
export function GrowthChart({ points }: { points: HistoryPoint[] }) {
  if (points.length === 0) return null
  const { x, y, ticks, years } = chartLayout(points)
  const totalPts = points.map((p) => ({ x: x(p.date), y: y(p.total) }))
  const freePts = points.filter((p) => p.free !== undefined).map((p) => ({ date: p.date, x: x(p.date), y: y(p.free ?? 0) }))
  const baseline = chart.height - chart.bottom
  return (
    <div className="space-y-3">
      <svg viewBox={`0 0 ${chart.width} ${chart.height}`} className="h-auto w-full" aria-hidden="true" data-testid="growth-chart">
        {ticks.map((t) => (
          <g key={t.value}>
            <line x1={chart.left} x2={chart.width - chart.right} y1={t.y} y2={t.y} className={color.grid} />
            <text x={chart.left - 6} y={t.y + 4} textAnchor="end" fontSize="12" className={color.text}>
              {n(t.value)}
            </text>
          </g>
        ))}
        <text x={chart.left} y={chart.height - 8} fontSize="12" className={color.text}>
          {points[0].date}
        </text>
        <text x={chart.width - chart.right} y={chart.height - 8} textAnchor="end" fontSize="12" className={color.text}>
          {points[points.length - 1].date}
        </text>
        {years.map((yr) => (
          <line key={yr.label} x1={yr.x} x2={yr.x} y1={baseline} y2={baseline + 4} className={color.grid} />
        ))}
        <polyline points={polyline(totalPts)} fill="none" strokeWidth="2" strokeLinejoin="round" className={color.total.line} />
        {freePts.length > 1 && <polyline points={polyline(freePts)} fill="none" strokeWidth="2" strokeLinejoin="round" className={color.free.line} />}
        {points.map((p, i) => (
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
      </svg>

      <ul className="flex flex-wrap gap-x-4 gap-y-1 text-xs text-zinc-600 dark:text-zinc-400">
        <li className="flex items-center gap-1.5">
          <span className={`inline-block h-0.5 w-4 ${color.total.swatch}`} aria-hidden="true" /> Images
        </li>
        {freePts.length > 0 && (
          <li className="flex items-center gap-1.5">
            <span className={`inline-block h-0.5 w-4 ${color.free.swatch}`} aria-hidden="true" /> Free (from {freePts[0].date})
          </li>
        )}
        <li className="flex items-center gap-1.5">
          <span className="inline-block size-2.5 rounded-full border-2 border-indigo-500 dark:border-indigo-400" aria-hidden="true" /> Internet Archive capture
        </li>
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
