export type Bar = {
  label: string
  value: number
  valueLabel: string
  /** Tailwind fill classes for the bar, e.g. "fill-rose-500 dark:fill-rose-400". Defaults to indigo. */
  colorClass?: string
  /** Accessible name of the bar's button when the chart is selectable. Defaults to "label, valueLabel". */
  accessibleLabel?: string
}

const defaultColor = 'fill-indigo-500 dark:fill-indigo-400'

/**
 * A horizontal bar chart on a linear scale from zero to the largest value.
 * Bars are SVG rects sized by attributes rather than styles, so they work under
 * the strict style-src CSP.
 *
 * Without onSelect the chart is decorative and hidden from assistive
 * technology: callers must show the same numbers as text. With onSelect each
 * row is a toggle button named by its accessibleLabel, and the selected row is
 * marked with aria-pressed and a ring.
 */
export function HorizontalBars({
  bars,
  selected,
  onSelect,
}: {
  bars: Bar[]
  selected?: string
  onSelect?: (label: string) => void
}) {
  const max = Math.max(0, ...bars.map((b) => b.value))
  const row = (b: Bar) => (
    <>
      <span className={`truncate font-mono text-xs text-zinc-600 dark:text-zinc-400 ${selected === b.label ? 'font-semibold text-zinc-900 dark:text-zinc-100' : ''}`}>
        {b.label}
      </span>
      <div className="flex min-w-0 items-center gap-2">
        <svg viewBox="0 0 100 10" preserveAspectRatio="none" className="h-5 min-w-0 flex-1" aria-hidden="true">
          <rect x="0" y="0" height="10" width={max > 0 ? (b.value / max) * 100 : 0} rx="1" className={b.colorClass ?? defaultColor} />
        </svg>
        <span className="w-20 shrink-0 text-right text-xs tabular-nums">{b.valueLabel}</span>
      </div>
    </>
  )
  // On phones the label sits above its bar, so the bar gets the full width.
  const grid = 'grid grid-cols-1 gap-1 text-sm sm:grid-cols-[minmax(0,8rem)_1fr] sm:items-center sm:gap-3'

  if (!onSelect) {
    return (
      <div aria-hidden="true" className="space-y-2">
        {bars.map((b) => (
          <div key={b.label} className={grid}>
            {row(b)}
          </div>
        ))}
      </div>
    )
  }
  return (
    <div className="space-y-1">
      {bars.map((b) => (
        <button
          key={b.label}
          type="button"
          aria-pressed={selected === b.label}
          aria-label={b.accessibleLabel ?? `${b.label}, ${b.valueLabel}`}
          onClick={() => onSelect(b.label)}
          className={`${grid} w-full rounded-md px-1 py-0.5 text-left hover:bg-zinc-100 focus-visible:outline-2 focus-visible:outline-indigo-600 dark:hover:bg-zinc-800 ${
            selected === b.label ? 'ring-2 ring-indigo-500 dark:ring-indigo-400' : ''
          }`}
        >
          {row(b)}
        </button>
      ))}
    </div>
  )
}
