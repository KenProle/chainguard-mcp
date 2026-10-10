import { describe, expect, it } from 'vitest'
import { layoutGroups, type Rect, squarify } from './treemap'
import catalogFamilies from './test/fixtures/catalogFamilies.json'
import { catalogGroups } from './test/fixtures/catalog'

const area: Rect = { x: 0, y: 0, w: 1000, h: 600 }
const sizes = catalogFamilies.families.map((f) => f.images.length)
const eps = 1e-6

const areaOf = (r: Rect) => r.w * r.h
const ratio = (r: Rect) => Math.max(r.w / r.h, r.h / r.w)

/** Asserts the rects stay inside outer, don't overlap and cover it. */
function expectTiling(rects: Rect[], outer: Rect) {
  for (const r of rects) {
    expect(r.w).toBeGreaterThan(0)
    expect(r.h).toBeGreaterThan(0)
    expect(r.x).toBeGreaterThanOrEqual(outer.x - eps)
    expect(r.y).toBeGreaterThanOrEqual(outer.y - eps)
    expect(r.x + r.w).toBeLessThanOrEqual(outer.x + outer.w + eps)
    expect(r.y + r.h).toBeLessThanOrEqual(outer.y + outer.h + eps)
  }
  for (let i = 0; i < rects.length; i++) {
    const a = rects[i]
    for (let j = i + 1; j < rects.length; j++) {
      const b = rects[j]
      const overlapW = Math.min(a.x + a.w, b.x + b.w) - Math.max(a.x, b.x)
      const overlapH = Math.min(a.y + a.h, b.y + b.h) - Math.max(a.y, b.y)
      if (overlapW > eps && overlapH > eps) throw new Error(`rects ${i} and ${j} overlap`)
    }
  }
  const total = rects.reduce((n, r) => n + areaOf(r), 0)
  expect(total).toBeCloseTo(areaOf(outer), 6)
}

describe('squarify', () => {
  it('lays out the real catalog: 1,751 families of 3,166 images', () => {
    expect(sizes).toHaveLength(1751)
    expect(sizes.reduce((a, b) => a + b, 0)).toBe(3166)
  })

  it('CM-6.1 gives each family an area proportional to its image count', () => {
    const rects = squarify(sizes, area)
    expect(rects).toHaveLength(sizes.length)
    const nginx = catalogFamilies.families.findIndex((f) => f.name === 'nginx')
    expect(catalogFamilies.families[0].name).toBe('go')
    expect(areaOf(rects[0]) / areaOf(area)).toBeCloseTo(6 / 3166, 12)
    expect(areaOf(rects[nginx]) / areaOf(area)).toBeCloseTo(4 / 3166, 12)
    rects.forEach((r, i) => {
      expect(Math.abs(areaOf(r) - (sizes[i] / 3166) * areaOf(area)) / areaOf(r)).toBeLessThan(eps)
    })
  })

  it('CM-6.2 splits a family into equal units that tile it', () => {
    const rects = squarify(sizes, area)
    // Every family size in the catalog: 1, 2, 3, 4 and 6.
    for (const count of new Set(sizes)) {
      const family = rects[sizes.indexOf(count)]
      const units = squarify(Array<number>(count).fill(1), family)
      expect(units).toHaveLength(count)
      for (const u of units) expect(areaOf(u)).toBeCloseTo(areaOf(family) / count, 9)
      expectTiling(units, family)
    }
  })

  it('CM-6.3 tiles the drawing area with the families', () => {
    expectTiling(squarify(sizes, area), area)
  })

  it('keeps family rectangles under 2:1 and units under 3:1 for the real catalog', () => {
    const rects = squarify(sizes, area)
    expect(Math.max(...rects.map(ratio))).toBeLessThan(2)
    const unitRatios = rects.flatMap((r, i) => squarify(Array<number>(sizes[i]).fill(1), r).map(ratio))
    expect(Math.max(...unitRatios)).toBeLessThan(3)
  })

  it('returns nothing for no values', () => {
    expect(squarify([], area)).toEqual([])
  })
})

describe('layoutGroups', () => {
  const variant = catalogGroups('variant')
  const blocks = variant.groups.map((g) => g.blocks.map((b) => b.count))
  const opts = { gap: 3, header: 22, blockGap: 2 }
  const layout = layoutGroups(blocks, area, opts)
  /** The space a group's blocks fill: its inset rectangle below the header. */
  const body = (i: number): Rect => {
    const { rect, header } = layout[i]
    const top = header ? header.y + header.h : rect.y + opts.gap / 2
    return { x: rect.x + opts.gap / 2, y: top, w: rect.w - opts.gap, h: rect.y + rect.h - opts.gap / 2 - top }
  }
  /** The units of block j of group i. */
  const blockUnits = (i: number, j: number) => {
    const start = blocks[i].slice(0, j).reduce((a, b) => a + b, 0)
    return layout[i].units.slice(start, start + blocks[i][j])
  }

  it('lays out the 2026-10-09 variant groups', () => {
    expect(variant.groups.map((g) => `${g.label}:${g.images.length}:${g.blocks.length}`)).toEqual([
      'Base images:1730:696',
      'FIPS:1210:460',
      'IAM-guarded:117:67',
      'IAM-guarded FIPS:95:50',
      'Other variants:14:8',
    ])
  })

  it('CM-6.1 gives each group an area proportional to its image count', () => {
    expect(areaOf(layout[0].rect) / areaOf(area)).toBeCloseTo(1730 / 3166, 12)
    expect(areaOf(layout[1].rect) / areaOf(area)).toBeCloseTo(1210 / 3166, 12)
    expect(layout[1].units).toHaveLength(1210)
  })

  it('CM-6.2 tiles each group body with its blocks', () => {
    layout.forEach((g, i) => expectTiling(g.blocks, body(i)))
  })

  it('CM-6.3 tiles the drawing area with the groups', () => {
    expectTiling(
      layout.map((g) => g.rect),
      area,
    )
  })

  it('CM-6.4 depends only on the group and block sizes', () => {
    expect(layoutGroups(blocks, area, opts)).toEqual(layout)
  })

  it('CM-18.4 sizes blocks by image count and fills each with equal units', () => {
    layout.forEach((g, i) => {
      const total = blocks[i].reduce((a, b) => a + b, 0)
      g.blocks.forEach((b, j) => {
        expect(areaOf(b) / areaOf(body(i))).toBeCloseTo(blocks[i][j] / total, 9)
        const space = inset(b)
        const units = blockUnits(i, j)
        for (const u of units) expect(areaOf(u)).toBeCloseTo(areaOf(space) / units.length, 6)
        expectTiling(units, space)
      })
    })
  })

  it('CM-18.2 leaves a single-block group without inner gaps', () => {
    const [one] = layoutGroups([[414]], area, opts)
    expect(one.blocks).toHaveLength(1)
    expectTiling(one.units, one.blocks[0])
  })

  it('CM-17.1 gives the large groups headers', () => {
    expect(layout[0].header).toBeDefined()
    expect(layout[1].header).toBeDefined()
    expect(layout[0].header?.h).toBe(22)
  })

  it('CM-17.2 leaves out the header of a group too small for one', () => {
    const [big, small] = layoutGroups([[2900], [100]], area, opts)
    expect(big.header).toBeDefined()
    expect(small.header).toBeUndefined()
    expectTiling(small.units, { x: small.rect.x + 1.5, y: small.rect.y + 1.5, w: small.rect.w - 3, h: small.rect.h - 3 })
  })

  it('keeps the units of a group thinner than the gap', () => {
    const [, sliver] = layoutGroups([[3000], [6]], area, opts)
    expect(sliver.rect.w).toBeLessThan(opts.gap)
    for (const u of sliver.units) expect(areaOf(u)).toBeGreaterThan(0)
  })

  /** A block's space for units: inset by half the block gap, capped like layoutGroups. */
  function inset(r: Rect): Rect {
    const d = Math.min(opts.blockGap / 2, r.w / 4, r.h / 4)
    return { x: r.x + d, y: r.y + d, w: r.w - 2 * d, h: r.h - 2 * d }
  }
})
