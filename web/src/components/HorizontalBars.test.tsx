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
