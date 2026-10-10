// Squarified treemap layout (Bruls, Huizing and van Wijk, 2000): rectangles
// whose areas are proportional to their values, kept close to square.

export type Rect = { x: number; y: number; w: number; h: number }

/** The worst aspect ratio in a row of areas laid along a side of this length. */
function worstRatio(row: number[], side: number): number {
  const sum = row.reduce((a, b) => a + b, 0)
  const max = Math.max(...row)
  const min = Math.min(...row)
  return Math.max((side * side * max) / (sum * sum), (sum * sum) / (side * side * min))
}

/**
 * Lays out values in area, returning one rectangle per value in input order.
 * Values must be positive and sorted from largest to smallest. The rectangles
 * don't overlap and together cover area exactly.
 */
export function squarify(values: number[], area: Rect): Rect[] {
  const total = values.reduce((a, b) => a + b, 0)
  if (values.length === 0 || total <= 0) return []
  const scale = (area.w * area.h) / total
  const out: Rect[] = []
  let free = { ...area }
  let i = 0
  while (i < values.length) {
    // Grow the row along the free space's shorter side while that doesn't make
    // its worst rectangle less square.
    const side = Math.min(free.w, free.h)
    const row = [values[i] * scale]
    let j = i + 1
    while (j < values.length) {
      const next = [...row, values[j] * scale]
      if (worstRatio(next, side) > worstRatio(row, side)) break
      row.push(values[j] * scale)
      j++
    }
    const sum = row.reduce((a, b) => a + b, 0)
    const last = j === values.length
    if (free.w >= free.h) {
      // A column at the left edge; the last row takes all remaining width.
      const w = last ? free.w : sum / free.h
      let y = free.y
      row.forEach((a, k) => {
        const h = k === row.length - 1 ? free.y + free.h - y : a / w
        out.push({ x: free.x, y, w, h })
        y += h
      })
      free = { x: free.x + w, y: free.y, w: free.w - w, h: free.h }
    } else {
      // A row at the top edge.
      const h = last ? free.h : sum / free.w
      let x = free.x
      row.forEach((a, k) => {
        const w = k === row.length - 1 ? free.x + free.w - x : a / h
        out.push({ x, y: free.y, w, h })
        x += w
      })
      free = { x: free.x, y: free.y + h, w: free.w, h: free.h - h }
    }
    i = j
  }
  return out
}

export type GroupLayout = {
  /** The group's share of the area, before the gap is taken off. */
  rect: Rect
  /** The header band, when the group has room for one. */
  header?: Rect
  /** Each block's share of the group's body, before the block gap is taken off. */
  blocks: Rect[]
  /** One equal unit per image within its block, block by block. */
  units: Rect[]
}

/** Shrinks r by d on every side, never by more than a quarter of its size. */
function inset(r: Rect, d: number): Rect {
  const i = Math.min(d, r.w / 4, r.h / 4)
  return { x: r.x + i, y: r.y + i, w: r.w - 2 * i, h: r.h - 2 * i }
}

/**
 * Lays out groups of blocks of equal units. Each group is given as its block
 * sizes (largest first); groups are squarified by their totals (largest
 * first) into area and inset by half the gap so neighbors are separated. A
 * header band comes off the top of groups at least two headers tall and
 * minHeaderWidth wide. The blocks are squarified into the rest and, when a
 * group has more than one, inset by half the block gap; then each block's
 * units are squarified into it.
 */
export function layoutGroups(
  groups: number[][],
  area: Rect,
  { gap, header, blockGap = 0, minHeaderWidth = 40 }: { gap: number; header: number; blockGap?: number; minHeaderWidth?: number },
): GroupLayout[] {
  const totals = groups.map((blocks) => blocks.reduce((a, b) => a + b, 0))
  return squarify(totals, area).map((rect, i) => {
    const inner = inset(rect, gap / 2)
    const hasHeader = inner.h >= 2 * header && inner.w >= minHeaderWidth
    const body = hasHeader ? { x: inner.x, y: inner.y + header, w: inner.w, h: inner.h - header } : inner
    const blocks = squarify(groups[i], body)
    const nested = groups[i].length > 1
    return {
      rect,
      header: hasHeader ? { x: inner.x, y: inner.y, w: inner.w, h: header } : undefined,
      blocks,
      units: blocks.flatMap((b, j) => squarify(Array<number>(groups[i][j]).fill(1), nested ? inset(b, blockGap / 2) : b)),
    }
  })
}
