import { describe, expect, it } from 'vitest'
import { fixesChart, fixesWithoutRecordsText } from './fixes'
import { pythonVulnsLatest, pythonVulnsLatestDev, pythonVulnsWithPending } from './test/fixtures/pythonVulns'

const names = (rows: { package: string }[]) => rows.map((r) => r.package)

describe('python vulnerability fixtures', () => {
  it('have totals equal to the sums of their packages', () => {
    for (const report of [pythonVulnsLatest, pythonVulnsLatestDev]) {
      const packages = report.packages ?? []
      expect(packages.reduce((n, p) => n + p.fixed_count, 0)).toBe(report.total_fixed)
      expect(packages.reduce((n, p) => n + p.not_affected_count, 0)).toBe(report.total_not_affected)
    }
    expect([pythonVulnsLatest.total_fixed, pythonVulnsLatest.total_not_affected]).toEqual([336, 51])
    expect([pythonVulnsLatestDev.total_fixed, pythonVulnsLatestDev.total_not_affected]).toEqual([546, 74])
  })
})

describe('fixesChart', () => {
  it('SF-1.1 keeps the 12 python latest packages with records', () => {
    const { rows, withoutRecords } = fixesChart(pythonVulnsLatest.packages)
    expect(rows).toHaveLength(12)
    expect(withoutRecords).toBe(9)
    expect(names(rows)).not.toContain('bzip2')
    expect(names(rows)).not.toContain('readline')
    expect(names(rows)).not.toContain('zstd')
  })

  it('SF-1.2 keeps a package with only never-affected records', () => {
    const { rows } = fixesChart(pythonVulnsLatest.packages)
    expect(rows.find((r) => r.package === 'sqlite')).toEqual({ package: 'sqlite', fixed: 0, notAffected: 4, pending: 0 })
  })

  it('SF-1.3 keeps the 25 python latest-dev packages with records', () => {
    const { rows, withoutRecords } = fixesChart(pythonVulnsLatestDev.packages)
    expect(rows).toHaveLength(25)
    expect(withoutRecords).toBe(31)
  })

  it('SF-3.1 orders python latest by fixes, then never affected, then name', () => {
    expect(names(fixesChart(pythonVulnsLatest.packages).rows)).toEqual([
      'openssl-4.0',
      'python-3.14',
      'expat',
      'py3-pip',
      'glibc-2.44',
      'zlib',
      'gcc',
      'sqlite',
      'util-linux',
      'brotli',
      'ncurses',
      'xz',
    ])
  })

  it('SF-3.2 puts a package with a fix not yet installed first', () => {
    const { rows } = fixesChart(pythonVulnsWithPending.packages)
    expect(names(rows).slice(0, 2)).toEqual(['zlib', 'openssl-4.0'])
    expect(rows[0]).toEqual({ package: 'zlib', fixed: 10, notAffected: 2, pending: 1 })
  })

  it('handles a missing package list', () => {
    expect(fixesChart(undefined)).toEqual({ rows: [], withoutRecords: 0 })
  })
})

describe('fixesWithoutRecordsText', () => {
  it('SF-7.1 counts the packages without records', () => {
    expect(fixesWithoutRecordsText(9, 21)).toBe('9 more source packages have no records in the Wolfi security database.')
    expect(fixesWithoutRecordsText(1, 21)).toBe('1 more source package has no records in the Wolfi security database.')
    expect(fixesWithoutRecordsText(0, 21)).toBeUndefined()
  })

  it('SF-7.2 says when no package has records', () => {
    const allZero = pythonVulnsLatest.packages?.map((p) => ({ ...p, fixed_count: 0, not_affected_count: 0 }))
    const { rows, withoutRecords } = fixesChart(allZero)
    expect(rows).toEqual([])
    expect(fixesWithoutRecordsText(withoutRecords, 21)).toBe(
      "None of this image's 21 source packages have records in the Wolfi security database.",
    )
    expect(fixesWithoutRecordsText(1, 1)).toBe("None of this image's 1 source package has records in the Wolfi security database.")
  })

  it('SF-7.2 never claims packages are unaffected', () => {
    for (const [n, total] of [
      [0, 21],
      [1, 21],
      [9, 21],
      [21, 21],
    ]) {
      expect(fixesWithoutRecordsText(n, total) ?? '').not.toMatch(/no vulnerabilities|secure|unaffected/i)
    }
  })
})
