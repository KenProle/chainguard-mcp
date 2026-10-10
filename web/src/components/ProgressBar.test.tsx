import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { ProgressBar } from './ProgressBar'

const fill = (container: HTMLElement) => container.querySelectorAll('rect')[1]

describe('ProgressBar', () => {
  it('FF-3.1 fills 250 of 3,171 to about 7.88', () => {
    const { container } = render(<ProgressBar value={250} max={3171} label="x" />)
    expect(Number(fill(container).getAttribute('width'))).toBeCloseTo(7.884, 2)
  })

  it('FF-3.3 exposes the value, maximum and text to assistive technology', () => {
    const label = 'Checking free-tier status: 1,000 of 3,171 images · about 20 seconds left'
    render(<ProgressBar value={1000} max={3171} label={label} />)
    const bar = screen.getByRole('progressbar')
    expect(bar).toHaveAttribute('aria-valuenow', '1000')
    expect(bar).toHaveAttribute('aria-valuemax', '3171')
    expect(bar).toHaveAttribute('aria-valuetext', label)
  })

  it('draws a zero value as a zero-width fill', () => {
    const { container } = render(<ProgressBar value={0} max={3171} label="x" />)
    expect(fill(container).getAttribute('width')).toBe('0')
  })

  it('FF-3 announces the label politely, from outside the progressbar', () => {
    render(<ProgressBar value={1} max={2} label="Half way" />)
    const label = screen.getByText('Half way')
    expect(label.closest('[aria-live]')).toHaveAttribute('aria-live', 'polite')
    // A progressbar's children are presentational, so a live region inside it may not be announced.
    expect(label.closest('[role="progressbar"]')).toBeNull()
  })

  it('FF-3.3 is named by its label', () => {
    render(<ProgressBar value={1} max={2} label="Half way" />)
    expect(screen.getByRole('progressbar', { name: 'Half way' })).toBeInTheDocument()
  })
})
