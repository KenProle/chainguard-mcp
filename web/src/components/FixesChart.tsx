import type { PackageFixes } from '../api'
import { fixesChart, fixesWithoutRecordsText } from '../fixes'
import { fixSegmentColor } from '../styles'
import { HorizontalBars } from './HorizontalBars'

const legend = [
  { key: 'fixed', text: 'Fixes included' },
  { key: 'notAffected', text: 'Never affected' },
  { key: 'pending', text: 'Not yet installed' },
] as const

/**
 * A stacked bar per source package with security records: fixes included,
 * never affected and fixes not yet installed. The chart and its legend are
 * hidden from assistive technology; the Security tab's table and pending-fix
 * list carry the same numbers as text.
 */
export function FixesChart({ packages }: { packages?: PackageFixes[] }) {
  if (!packages?.length) return null
  const { rows, withoutRecords } = fixesChart(packages)
  const note = fixesWithoutRecordsText(withoutRecords, packages.length)

  return (
    <section aria-labelledby="fixes-chart-heading" className="mb-4 space-y-2">
      <h3 id="fixes-chart-heading" className="text-sm font-medium">
        Fixes per source package
      </h3>
      {rows.length > 0 && (
        <>
          <ul aria-hidden="true" className="flex flex-wrap gap-x-4 gap-y-1 text-xs text-zinc-600 dark:text-zinc-400">
            {legend.map(({ key, text }) => (
              <li key={key} className="flex items-center gap-1.5">
                <span className={`inline-block size-3 rounded-sm ${fixSegmentColor[key].swatch}`} />
                {text}
              </li>
            ))}
          </ul>
          <HorizontalBars
            bars={rows.map((r) => ({
              label: r.package,
              value: r.fixed + r.notAffected + r.pending,
              valueLabel: `${r.fixed} fixed`,
              segments: [
                { value: r.fixed, colorClass: fixSegmentColor.fixed.fill },
                { value: r.notAffected, colorClass: fixSegmentColor.notAffected.fill },
                { value: r.pending, colorClass: fixSegmentColor.pending.fill },
              ],
            }))}
          />
        </>
      )}
      {note && <p className="text-xs text-zinc-500">{note}</p>}
    </section>
  )
}
