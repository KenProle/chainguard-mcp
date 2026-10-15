import { useEffect, useState, type RefObject } from 'react'

/** The narrowest the growth chart is drawn (HV-24). */
export const MIN_CHART_WIDTH = 280

/**
 * The rendered width of an element in whole pixels (at least MIN_CHART_WIDTH),
 * kept current by a ResizeObserver; `undefined` before the first measurement or
 * where ResizeObserver doesn't exist.
 */
export function useElementWidth(ref: RefObject<HTMLElement | null>): number | undefined {
  const [width, setWidth] = useState<number>()
  useEffect(() => {
    const el = ref.current
    if (!el || typeof ResizeObserver === 'undefined') return
    const observer = new ResizeObserver((entries) => {
      const entry = entries[entries.length - 1]
      if (entry) setWidth(Math.max(MIN_CHART_WIDTH, Math.round(entry.contentRect.width)))
    })
    observer.observe(el)
    return () => observer.disconnect()
  }, [ref])
  return width
}
