export type Bar = { label: string; value: number; valueLabel: string }

/**
 * A horizontal bar chart on a linear scale from zero to the largest value.
 * Bars are SVG rects sized by attributes rather than styles, so they work under
 * the strict style-src CSP. Hidden from assistive technology: callers must show
 * the same numbers as text.
 */
export function HorizontalBars({ bars }: { bars: Bar[] }) {
  const max = Math.max(0, ...bars.map((b) => b.value))
  return (
    <div aria-hidden="true" className="space-y-2">
      {bars.map((b) => (
        <div key={b.label} className="grid grid-cols-[minmax(0,8rem)_1fr] items-center gap-3 text-sm">
          <span className="truncate font-mono text-xs text-zinc-600 dark:text-zinc-400">{b.label}</span>
          <div className="flex min-w-0 items-center gap-2">
            <svg viewBox="0 0 100 10" preserveAspectRatio="none" className="h-5 min-w-0 flex-1">
              <rect x="0" y="0" height="10" width={max > 0 ? (b.value / max) * 100 : 0} rx="1" className="fill-indigo-500 dark:fill-indigo-400" />
            </svg>
            <span className="w-20 shrink-0 text-right text-xs tabular-nums">{b.valueLabel}</span>
          </div>
        </div>
      ))}
    </div>
  )
}
