import { describe, expect, it } from 'vitest'
import type { HistoryReport } from './api'
import { chartLayout, historyTiles, niceMax } from './history'
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
})
