import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { HorizontalBars } from './HorizontalBars'

const widths = (container: HTMLElement) => [...container.querySelectorAll('rect')].map((r) => Number(r.getAttribute('width')))

describe('HorizontalBars', () => {
  it('VC-9.1 draws a value ten times another ten times as wide', () => {
    const { container } = render(
      <HorizontalBars
        bars={[
          { label: 'latest', value: 10, valueLabel: '10 B' },
          { label: 'latest-dev', value: 100, valueLabel: '100 B' },
        ]}
      />,
    )
    const [small, large] = widths(container)
    expect(large).toBe(small * 10)
    expect(container.firstElementChild).toHaveAttribute('aria-hidden', 'true')
  })

  it('draws a zero value as a zero-width bar', () => {
    const { container } = render(
      <HorizontalBars
        bars={[
          { label: 'empty', value: 0, valueLabel: '0 B' },
          { label: 'full', value: 5, valueLabel: '5 B' },
        ]}
      />,
    )
    expect(widths(container)).toEqual([0, 100])
  })

  it('LB-7.2 draws a value twice another twice as wide', () => {
    const { container } = render(
      <HorizontalBars
        bars={[
          { label: 'Weak copyleft', value: 6, valueLabel: '6 packages' },
          { label: 'Permissive', value: 12, valueLabel: '12 packages' },
        ]}
      />,
    )
    const [small, large] = widths(container)
    expect(large).toBe(small * 2)
  })

  it("applies each bar's color class, defaulting to indigo", () => {
    const { container } = render(
      <HorizontalBars
        bars={[
          { label: 'a', value: 1, valueLabel: '1', colorClass: 'fill-rose-500' },
          { label: 'b', value: 1, valueLabel: '1' },
        ]}
      />,
    )
    const [a, b] = container.querySelectorAll('rect')
    expect(a).toHaveClass('fill-rose-500')
    expect(b).toHaveClass('fill-indigo-500')
  })

  it('SF-2.1 draws segments end to end on the shared scale', () => {
    const { container } = render(
      <HorizontalBars
        bars={[
          {
            label: 'openssl-4.0',
            value: 153,
            valueLabel: '147 fixed',
            segments: [
              { value: 147, colorClass: 'fill-indigo-500' },
              { value: 6, colorClass: 'fill-zinc-300' },
            ],
          },
        ]}
      />,
    )
    const [fixed, notAffected] = container.querySelectorAll('rect')
    expect(Number(fixed.getAttribute('x'))).toBe(0)
    expect(Number(fixed.getAttribute('width'))).toBeCloseTo((147 / 153) * 100)
    expect(Number(notAffected.getAttribute('x'))).toBeCloseTo((147 / 153) * 100)
    expect(Number(notAffected.getAttribute('width'))).toBeCloseTo((6 / 153) * 100)
  })

  it('SF-2.2 draws a 1-unit bar 1/153 as wide as a 153-unit bar', () => {
    const { container } = render(
      <HorizontalBars
        bars={[
          { label: 'openssl-4.0', value: 153, valueLabel: '147 fixed', segments: [{ value: 153, colorClass: 'a' }] },
          { label: 'gcc', value: 1, valueLabel: '1 fixed', segments: [{ value: 1, colorClass: 'a' }] },
        ]}
      />,
    )
    const [large, small] = widths(container)
    expect(large).toBe(100)
    expect(small).toBeCloseTo(100 / 153)
  })

  it('SF-2.3 draws three segments in order with their colors', () => {
    const { container } = render(
      <HorizontalBars
        bars={[
          {
            label: 'zlib',
            value: 13,
            valueLabel: '10 fixed',
            segments: [
              { value: 10, colorClass: 'fill-indigo-500' },
              { value: 2, colorClass: 'fill-zinc-300' },
              { value: 1, colorClass: 'fill-rose-500' },
            ],
          },
        ]}
      />,
    )
    const rects = [...container.querySelectorAll('rect')]
    expect(rects.map((r) => r.getAttribute('class'))).toEqual(['fill-indigo-500', 'fill-zinc-300', 'fill-rose-500'])
    expect(widths(container).map((w) => Math.round(w * 13))).toEqual([1000, 200, 100])
  })

  it('SF-4.2 draws no rect for a zero-value segment', () => {
    const { container } = render(
      <HorizontalBars
        bars={[
          {
            label: 'sqlite',
            value: 4,
            valueLabel: '0 fixed',
            segments: [
              { value: 0, colorClass: 'fill-indigo-500' },
              { value: 4, colorClass: 'fill-zinc-300' },
              { value: 0, colorClass: 'fill-rose-500' },
            ],
          },
        ]}
      />,
    )
    const rects = container.querySelectorAll('rect')
    expect(rects).toHaveLength(1)
    expect(rects[0]).toHaveClass('fill-zinc-300')
    expect(rects[0]).toHaveAttribute('x', '0')
  })

  it('renders no controls without onSelect', () => {
    render(<HorizontalBars bars={[{ label: 'a', value: 1, valueLabel: '1' }]} />)
    expect(screen.queryByRole('button')).toBeNull()
  })

  it('renders each bar as a toggle button with onSelect', async () => {
    const onSelect = vi.fn()
    render(
      <HorizontalBars
        bars={[
          { label: 'Strong copyleft', value: 7, valueLabel: '7 packages', accessibleLabel: 'Strong copyleft, 7 packages' },
          { label: 'Weak copyleft', value: 6, valueLabel: '6 packages' },
        ]}
        selected="Weak copyleft"
        onSelect={onSelect}
      />,
    )
    const strong = screen.getByRole('button', { name: 'Strong copyleft, 7 packages' })
    expect(strong).toHaveAttribute('aria-pressed', 'false')
    expect(screen.getByRole('button', { name: 'Weak copyleft, 6 packages' })).toHaveAttribute('aria-pressed', 'true')
    await userEvent.click(strong)
    expect(onSelect).toHaveBeenCalledWith('Strong copyleft')
  })
})
