import { useId } from 'react'
import { progressBarColor } from '../styles'

/**
 * A determinate progress bar with a text label. The bar is an SVG whose fill
 * is sized by an attribute rather than a style, so it works under the strict
 * style-src CSP. The label sits beside the progressbar element, not inside it:
 * a progressbar's children are presentational, so a live region inside it may
 * never be announced. The label also names the bar.
 */
export function ProgressBar({ value, max, label }: { value: number; max: number; label: string }) {
  const labelId = useId()
  const width = max > 0 ? Math.min(100, Math.max(0, (value / max) * 100)) : 0
  return (
    <div className="space-y-2">
      <div
        role="progressbar"
        aria-labelledby={labelId}
        aria-valuemin={0}
        aria-valuemax={max}
        aria-valuenow={value}
        aria-valuetext={label}
      >
        <svg viewBox="0 0 100 4" preserveAspectRatio="none" className="block h-2 w-full" aria-hidden="true">
          <rect x="0" y="0" width="100" height="4" rx="1" className={progressBarColor.track} />
          <rect x="0" y="0" width={width} height="4" rx="1" className={progressBarColor.fill} />
        </svg>
      </div>
      <p id={labelId} aria-live="polite" className="text-sm text-zinc-600 dark:text-zinc-400">
        {label}
      </p>
    </div>
  )
}
