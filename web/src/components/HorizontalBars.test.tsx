import { render } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
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
})
