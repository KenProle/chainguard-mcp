import { describe, expect, it } from 'vitest'
import { scrollLeftToReveal } from './scroll'

describe('scrollLeftToReveal', () => {
  it('leaves a fully visible item alone', () => {
    expect(scrollLeftToReveal(0, 300, 100, 80)).toBe(0)
  })

  it('scrolls right just far enough to show an item cut off at the end', () => {
    // The Compare tab at 375 px: starts at 290, 90 wide, in a 311 px view.
    expect(scrollLeftToReveal(0, 311, 290, 90)).toBe(69)
  })

  it('scrolls left to show an item before the visible area', () => {
    expect(scrollLeftToReveal(200, 300, 50, 80)).toBe(50)
  })

  it('aligns the item start when it is wider than the view', () => {
    expect(scrollLeftToReveal(0, 100, 150, 400)).toBe(150)
  })
})
