// Shared Tailwind class strings and formatting helpers. Kept out of the
// component files so React fast refresh keeps working during development.

import type { LicenseCategory } from './licenses'

/** Bar colors for the license breakdown's categories, readable in light and dark mode. */
export const licenseCategoryColor: Record<LicenseCategory, string> = {
  permissive: 'fill-emerald-500 dark:fill-emerald-400',
  weak: 'fill-amber-500 dark:fill-amber-400',
  strong: 'fill-rose-500 dark:fill-rose-400',
  unrecognized: 'fill-violet-500 dark:fill-violet-400',
  none: 'fill-zinc-400 dark:fill-zinc-500',
}

/**
 * Colors for the fixes chart's segments: fill for the bars, swatch for the
 * legend. Fixes included are indigo rather than green so the chart doesn't
 * read as "safe"; fixes not yet installed match the red pending-fix box.
 */
export const fixSegmentColor: Record<'fixed' | 'notAffected' | 'pending', { fill: string; swatch: string }> = {
  fixed: { fill: 'fill-indigo-500 dark:fill-indigo-400', swatch: 'bg-indigo-500 dark:bg-indigo-400' },
  notAffected: { fill: 'fill-zinc-300 dark:fill-zinc-600', swatch: 'bg-zinc-300 dark:bg-zinc-600' },
  pending: { fill: 'fill-rose-500 dark:fill-rose-400', swatch: 'bg-rose-500 dark:bg-rose-400' },
}

export const inputClass =
  'w-full rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm shadow-sm placeholder:text-zinc-400 focus:border-indigo-500 focus:outline-none focus:ring-2 focus:ring-indigo-500/30 dark:border-zinc-700 dark:bg-zinc-900'

export const buttonClass =
  'inline-flex items-center justify-center rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white shadow-sm hover:bg-indigo-500 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-indigo-600 disabled:opacity-50'

/** Formats a byte count in decimal units (1 MB = 1,000,000 bytes), as Docker does. */
export function formatBytes(n: number): string {
  if (n < 1000) return `${n} B`
  const units = ['KB', 'MB', 'GB']
  let v = n / 1000
  let i = 0
  // Round first, so 999,950 bytes moves up to "1.0 MB" rather than "1000.0 KB".
  while (Math.round(v * 10) / 10 >= 1000 && i < units.length - 1) {
    v /= 1000
    i++
  }
  return `${v.toFixed(1)} ${units[i]}`
}
