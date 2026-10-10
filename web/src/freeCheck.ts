// Progress reporting for the catalog List view's first "Free only" check.

/** Images per batch of the check; about 2 seconds each when not cached. */
export const FREE_CHECK_BATCH = 250
/** Time per image assumed until a batch has returned (measured 2026-10-10). */
export const ASSUMED_MS_PER_IMAGE = 9
/** How long a check runs before the progress bar replaces the loading message. */
export const PROGRESS_DELAY_MS = 1000

/** Seconds needed to check the images not yet checked. */
export function secondsLeft(checked: number, total: number, msPerImage: number): number {
  return (Math.max(0, total - checked) * msPerImage) / 1000
}

/** The time left, rounded up to the next 5 seconds. */
export function timeLeftText(seconds: number): string {
  if (seconds < 5) return 'a few seconds left'
  return `about ${Math.ceil(seconds / 5) * 5} seconds left`
}

export function freeCheckLabel(checked: number, total: number, msPerImage: number): string {
  const n = (v: number) => v.toLocaleString('en-US')
  return `Checking free-tier status: ${n(checked)} of ${n(total)} images · ${timeLeftText(secondsLeft(checked, total, msPerImage))}`
}

/** The average measured time per image over the batches so far, or the assumed rate before any. */
export function averageMsPerImage(batches: { count: number; ms: number }[]): number {
  const images = batches.reduce((n, b) => n + b.count, 0)
  if (images === 0) return ASSUMED_MS_PER_IMAGE
  return batches.reduce((n, b) => n + b.ms, 0) / images
}
