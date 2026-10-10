import { describe, expect, it } from 'vitest'
import { ASSUMED_MS_PER_IMAGE, averageMsPerImage, freeCheckLabel, secondsLeft, timeLeftText } from './freeCheck'

describe('free check helpers', () => {
  it('FF-4.1 estimates 20 seconds left after 1,000 of 3,171 images in 8.8 seconds', () => {
    const ms = averageMsPerImage([{ count: 1000, ms: 8800 }])
    expect(ms).toBeCloseTo(8.8)
    expect(secondsLeft(1000, 3171, ms)).toBeCloseTo(19.1, 1)
    expect(freeCheckLabel(1000, 3171, ms)).toMatch(/about 20 seconds left$/)
  })

  it('FF-4.2 says a few seconds left after 3,000 of 3,171 images in 26.4 seconds', () => {
    const ms = averageMsPerImage([{ count: 3000, ms: 26400 }])
    expect(freeCheckLabel(3000, 3171, ms)).toMatch(/a few seconds left$/)
  })

  it('FF-3.2 labels an unstarted check at the assumed rate', () => {
    expect(averageMsPerImage([])).toBe(ASSUMED_MS_PER_IMAGE)
    expect(freeCheckLabel(0, 3171, ASSUMED_MS_PER_IMAGE)).toBe('Checking free-tier status: 0 of 3,171 images · about 30 seconds left')
  })

  it('FF-3.1 labels 250 of 3,171 images', () => {
    expect(freeCheckLabel(250, 3171, ASSUMED_MS_PER_IMAGE)).toMatch(/^Checking free-tier status: 250 of 3,171 images ·/)
  })

  it('rounds exactly 5 seconds to "about 5 seconds" and anything under to "a few"', () => {
    expect(timeLeftText(5)).toBe('about 5 seconds left')
    expect(timeLeftText(4.99)).toBe('a few seconds left')
    expect(timeLeftText(5.01)).toBe('about 10 seconds left')
  })

  it('says a few seconds left when nothing remains', () => {
    expect(secondsLeft(3171, 3171, 9)).toBe(0)
    expect(freeCheckLabel(3171, 3171, 9)).toMatch(/a few seconds left$/)
  })
})
