import { act, fireEvent, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useLocation } from 'react-router'
import { describe, expect, it, vi } from 'vitest'
import type { HistoryReport } from '../api'
import { chartLayout } from '../history'
import { CatalogPage } from '../pages/CatalogPage'
import fixture from '../test/fixtures/historyReport.json'
import { mockApi, renderAt } from '../test/render'

const full = fixture as unknown as HistoryReport

function Url() {
  return <output data-testid="url">{useLocation().search}</output>
}

function renderChart(search = '', report: HistoryReport = full) {
  mockApi({ '/api/history': { body: report } })
  renderAt(
    `/?view=history${search}`,
    '/',
    <>
      <CatalogPage />
      <Url />
    </>,
  )
}

/** Gives the SVG a 480-unit-wide box, so clientX equals the viewBox x. */
async function chartWithBox() {
  const svg = await screen.findByTestId('growth-chart')
  vi.spyOn(svg, 'getBoundingClientRect').mockReturnValue({ left: 0, top: 0, width: 480, height: 240, right: 480, bottom: 240, x: 0, y: 0, toJSON: () => ({}) })
  return svg
}

const at = (date: string, range: 'all' | 'daily' = 'all') => ({ clientX: chartLayout(full.points, range).x(date), clientY: 100 })
const card = () => screen.queryByTestId('chart-card')
const overlay = () => screen.getByTestId('chart-overlay')
const drawn = (svg: HTMLElement) => svg.querySelectorAll('circle[data-point]:not([data-point="free"])')

describe('Growth chart pointer', () => {
  it('HV-19.1 shows the card for the nearest snapshot', async () => {
    renderChart()
    await chartWithBox()
    fireEvent.pointerMove(overlay(), at('2026-10-08'))
    expect(card()).toHaveTextContent('2026-10-08')
    expect(card()).toHaveTextContent('Internet Archive capture')
    expect(card()).toHaveTextContent('3,166 images · free unknown')
    expect(card()).toHaveTextContent('+769 since 2026-05-10')
  })

  it('HV-19.4 hides the card when the pointer leaves', async () => {
    renderChart()
    await chartWithBox()
    fireEvent.pointerMove(overlay(), at('2026-10-08'))
    fireEvent.pointerLeave(overlay())
    expect(card()).not.toBeInTheDocument()
  })

  it('HV-19.4 a tap selects, and a tap outside the chart hides the card', async () => {
    renderChart()
    await chartWithBox()
    fireEvent.pointerDown(overlay(), { ...at('2025-05-04'), pointerType: 'touch' })
    expect(card()).toHaveTextContent('2025-05-04')
    fireEvent.pointerLeave(overlay(), { pointerType: 'touch' })
    expect(card()).toBeInTheDocument()
    fireEvent.pointerDown(document.body)
    expect(card()).not.toBeInTheDocument()
  })

  it('HV-20.1 says the daily snapshots are drawn close together and suggests the zoomed range', async () => {
    renderChart()
    await chartWithBox()
    fireEvent.pointerMove(overlay(), at('2026-10-14'))
    expect(card()).toHaveTextContent('5 more snapshots are drawn close')
    expect(card()).toHaveTextContent('(2026-10-08 to 2026-10-13)')
    expect(card()).toHaveTextContent('Try "Since daily tracking"')
  })

  it('HV-20.2 mentions no neighbours for a spread-out snapshot', async () => {
    renderChart()
    await chartWithBox()
    fireEvent.pointerMove(overlay(), at('2025-05-04'))
    expect(card()).toBeInTheDocument()
    expect(card()).not.toHaveTextContent('drawn close')
  })
})

describe('Growth chart keyboard', () => {
  it('HV-21.1 steps with the keyboard, announces each snapshot, and Escape hides the card', async () => {
    renderChart()
    await chartWithBox()
    const group = screen.getByRole('group', { name: /Growth chart/ })
    fireEvent.keyDown(group, { key: 'End' })
    fireEvent.keyDown(group, { key: 'ArrowLeft' })
    expect(card()).toHaveTextContent('2026-10-13')
    expect(screen.getByTestId('chart-live')).toHaveTextContent('2026-10-13 · Daily snapshot · 3,171 images · 59 free · no change since 2026-10-12')
    fireEvent.keyDown(group, { key: 'Home' })
    expect(card()).toHaveTextContent('first capture')
    fireEvent.keyDown(group, { key: 'Escape' })
    expect(card()).not.toBeInTheDocument()
  })

  it('HV-21.2 shows a hint on focus, and the SVG stays hidden with its table', async () => {
    renderChart()
    await chartWithBox()
    expect(screen.queryByText(/to move between snapshots, Home/)).not.toBeInTheDocument()
    act(() => screen.getByRole('group', { name: /Growth chart/ }).focus())
    expect(screen.getByText(/to move between snapshots, Home/)).toBeInTheDocument()
    expect(screen.getByTestId('growth-chart')).toHaveAttribute('aria-hidden', 'true')
    expect(document.querySelectorAll('table tbody tr')).toHaveLength(22)
  })

  it('HV-21.1 pointer moves do not write to the live region', async () => {
    renderChart()
    await chartWithBox()
    fireEvent.pointerMove(overlay(), at('2026-10-08'))
    expect(screen.getByTestId('chart-live')).toBeEmptyDOMElement()
  })
})

describe('Growth chart range', () => {
  it('HV-22.1 zooms to the daily snapshots and HV-22.2 goes back', async () => {
    const user = userEvent.setup()
    renderChart()
    await user.click(await screen.findByRole('button', { name: 'Since daily tracking' }))
    expect(screen.getByTestId('url')).toHaveTextContent('range=daily')
    const svg = screen.getByTestId('growth-chart')
    expect(drawn(svg)).toHaveLength(5)
    expect(svg.querySelector('circle[data-point="free"]')).toBeNull()
    expect(svg).toHaveTextContent('2026-10-10')
    expect(screen.getByText("Axis doesn't start at zero")).toBeInTheDocument()
    expect(screen.getByText(/Free images aren't plotted in this range/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Since daily tracking' })).toHaveAttribute('aria-pressed', 'true')

    await user.click(screen.getByRole('button', { name: 'All time' }))
    expect(screen.getByTestId('url')).not.toHaveTextContent('range')
    expect(drawn(screen.getByTestId('growth-chart'))).toHaveLength(22)
    expect(screen.queryByText("Axis doesn't start at zero")).not.toBeInTheDocument()
  })

  it('HV-19.3 shows a daily snapshot in the zoomed range', async () => {
    renderChart('&range=daily')
    await chartWithBox()
    fireEvent.pointerMove(overlay(), at('2026-10-12', 'daily'))
    expect(card()).toHaveTextContent('Daily snapshot')
    expect(card()).toHaveTextContent('3,171 images · 59 free')
    expect(card()).toHaveTextContent('no change since 2026-10-11')
  })

  it('HV-22.3 offers only All time with fewer than 2 daily snapshots and ignores range=daily', async () => {
    const one = { ...full, points: full.points.filter((p) => p.source === 'archive' || p.date === '2026-10-10') }
    renderChart('&range=daily', one)
    expect(await screen.findByRole('button', { name: 'All time' })).toHaveAttribute('aria-pressed', 'true')
    expect(screen.queryByRole('button', { name: 'Since daily tracking' })).not.toBeInTheDocument()
    expect(drawn(screen.getByTestId('growth-chart'))).toHaveLength(18)
  })

  it('HV-22.1 resets the selection when the range changes', async () => {
    const user = userEvent.setup()
    renderChart()
    await chartWithBox()
    fireEvent.pointerMove(overlay(), at('2026-10-14'))
    expect(card()).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Since daily tracking' }))
    expect(card()).not.toBeInTheDocument()
  })
})

describe('Growth chart on narrow screens', () => {
  it('HV-23.1 shows the selected snapshot as text below the chart and hides the card on narrow screens', async () => {
    renderChart()
    const svg = await chartWithBox()
    fireEvent.pointerMove(overlay(), at('2026-10-08'))
    const line = screen.getByTestId('chart-selection')
    expect(line).toHaveTextContent('2026-10-08 · Internet Archive capture · 3,166 images · free unknown · +769 since 2026-05-10')
    // jsdom has no media queries: the classes carry the behavior, as Tailwind turns them into CSS.
    expect(line).toHaveClass('text-sm', 'sm:hidden')
    expect(card()).toHaveClass('hidden', 'sm:inline')
    expect(svg.querySelector('line[stroke-dasharray]')).not.toBeNull()
    expect(svg.querySelector('circle[r="6"]')).not.toBeNull()
  })

  it('HV-23.1 adds the nearby-snapshot note and the suggestion to the text', async () => {
    renderChart()
    await chartWithBox()
    fireEvent.pointerMove(overlay(), at('2026-10-14'))
    expect(screen.getByTestId('chart-selection')).toHaveTextContent('5 more snapshots are drawn close to this one (2026-10-08 to 2026-10-13) Try "Since daily tracking"')
  })
})
