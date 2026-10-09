import { describe, expect, it } from 'vitest'
import { formatBytes } from './styles'

describe('formatBytes', () => {
  it('shows small sizes in bytes', () => {
    expect(formatBytes(0)).toBe('0 B')
    expect(formatBytes(999)).toBe('999 B')
  })

  it('uses decimal kilobytes', () => {
    expect(formatBytes(1000)).toBe('1.0 KB')
    expect(formatBytes(1536)).toBe('1.5 KB')
  })

  it('uses decimal megabytes with one decimal place', () => {
    expect(formatBytes(26_882_525)).toBe('26.9 MB')
    expect(formatBytes(272_631_719)).toBe('272.6 MB')
    expect(formatBytes(999_950)).toBe('1.0 MB')
  })

  it('uses decimal gigabytes', () => {
    expect(formatBytes(1_500_000_000)).toBe('1.5 GB')
  })
})
