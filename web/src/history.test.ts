import { describe, expect, it } from 'vitest'
import type { HistoryReport } from './api'
import { chart, chartLayout, historyTiles, neighbours, nearestIndex, niceDomain, niceMax, pointText } from './history'
import fixture from './test/fixtures/historyReport.json'

const report = fixture as unknown as HistoryReport

describe('history helpers', () => {
  it('HV-14.1 computes the tiles from the first point', () => {
    expect(historyTiles(report)).toEqual({ label: 'Since first capture', images: 3171, free: 59, delta: '+2,496', from: 'from 675 on 2024-04-19' })
  })

  it('HV-14.4 counts a search from its first match', () => {
    const late = { ...report, points: report.points.map((p) => ({ ...p, total: p.date >= '2026-10-10' ? 1 : 0 })), total_now: 1 }
    expect(historyTiles(late)).toMatchObject({ label: 'Since first match', delta: '+0', from: 'from 1 on 2026-10-10' })
  })

  it('HV-15.1 positions points on a time axis from zero', () => {
    const { x, y } = chartLayout(report.points)
    expect(x('2024-04-19')).toBeLessThan(x('2026-10-08'))
    expect(x('2026-10-10') - x('2026-10-08')).toBeLessThan(x('2026-10-08') - x('2026-05-10'))
    expect(y(0)).toBeGreaterThan(y(3171))
  })

  it('rounds the axis maximum up to a round number', () => {
    expect([niceMax(1), niceMax(3171), niceMax(4001), niceMax(675)]).toEqual([1, 5000, 5000, 1000])
  })

  const idx = (date: string) => report.points.findIndex((p) => p.date === date)

  it('HV-19.1 words an Internet Archive capture', () => {
    expect(pointText(report.points, idx('2026-10-08'))).toBe('2026-10-08 · Internet Archive capture · 3,166 images · free unknown · +769 since 2026-05-10')
  })

  it('HV-19.2 words the first capture', () => {
    expect(pointText(report.points, 0)).toBe('2024-04-19 · Internet Archive capture · 675 images · free unknown · first capture')
  })

  it('HV-19.3 words a daily snapshot with no change', () => {
    expect(pointText(report.points, idx('2026-10-12'))).toBe('2026-10-12 · Daily snapshot · 3,171 images · 59 free · no change since 2026-10-11')
  })

  it('HV-20.1 finds the 5 snapshots drawn close to 2026-10-14 in All time', () => {
    const { x, shown } = chartLayout(report.points, 'all')
    const xs = shown.map((p) => x(p.date))
    expect(neighbours(xs, idx('2026-10-14')).map((j) => shown[j].date)).toEqual(['2026-10-08', '2026-10-10', '2026-10-11', '2026-10-12', '2026-10-13'])
  })

  it('HV-20.2 finds no neighbours for 2025-05-04', () => {
    const { x, shown } = chartLayout(report.points, 'all')
    expect(neighbours(shown.map((p) => x(p.date)), idx('2025-05-04'))).toEqual([])
  })

  it.each([480, 935, 309])('HV-24 lays the plot out from left to width - right at %i units', (width) => {
    const { x, shown } = chartLayout(report.points, 'all', width)
    expect(x(shown[0].date)).toBe(chart.left)
    expect(x(shown[shown.length - 1].date)).toBeCloseTo(width - chart.right, 9)
  })

  it('HV-24 defaults to 480 units', () => {
    const a = chartLayout(report.points, 'all')
    const b = chartLayout(report.points, 'all', 480)
    expect(a.shown.map((p) => a.x(p.date))).toEqual(b.shown.map((p) => b.x(p.date)))
  })

  it('HV-20 still finds the same 5 close neighbours of 2026-10-14 at 935 units (the axis spans years)', () => {
    const { x, shown } = chartLayout(report.points, 'all', 935)
    const xs = shown.map((p) => x(p.date))
    expect(neighbours(xs, idx('2026-10-14')).map((j) => shown[j].date)).toEqual(['2026-10-08', '2026-10-10', '2026-10-11', '2026-10-12', '2026-10-13'])
  })

  it('HV-22.4 puts a flat series mid-plot with distinct ticks', () => {
    const [lo, hi] = niceDomain(3171, 3171)
    expect([lo, hi]).toEqual([3160, 3180])
    const { y, ticks, startsAtZero, shown } = chartLayout(report.points, 'daily')
    expect(shown).toHaveLength(5)
    expect(new Set(ticks.map((t) => t.value)).size).toBe(3)
    expect(ticks[1].value).toBe(3170)
    expect(Math.abs(y(3171) - ticks[1].y)).toBeLessThan(20)
    expect(startsAtZero).toBe(false)
  })

  it.each(['all', 'daily'] as const)('HV-25.1 lays the free series out on its own axis in the %s range', (range) => {
    const { shown, value, ticks, y, startsAtZero } = chartLayout(report.points, range, 480, 'free')
    expect(shown.map((p) => p.date)).toEqual(['2026-10-10', '2026-10-11', '2026-10-12', '2026-10-13', '2026-10-14'])
    expect(shown.map(value)).toEqual([59, 59, 59, 59, 59])
    expect(ticks.map((t) => t.value)).toEqual([58, 59, 60])
    expect(y(59)).toBeCloseTo(ticks[1].y, 9)
    expect(startsAtZero).toBe(false)
  })

  it('niceDomain contains growing data and stays at or above zero', () => {
    const [lo, hi] = niceDomain(3166, 3171)
    expect(lo).toBeLessThanOrEqual(3166)
    expect(hi).toBeGreaterThanOrEqual(3171)
    expect(niceDomain(0, 0)[0]).toBe(0)
  })

  it('All time still starts at zero', () => {
    const { ticks, startsAtZero } = chartLayout(report.points, 'all')
    expect(ticks[0].value).toBe(0)
    expect(startsAtZero).toBe(true)
  })

  it('nearestIndex snaps to the closest x', () => {
    const xs = [10, 20, 40]
    expect([nearestIndex(xs, 0), nearestIndex(xs, 14), nearestIndex(xs, 16), nearestIndex(xs, 31), nearestIndex(xs, 99)]).toEqual([0, 0, 1, 2, 2])
  })
})
