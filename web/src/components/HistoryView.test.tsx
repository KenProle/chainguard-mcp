import { fireEvent, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import type { HistoryEntry, HistoryReport } from '../api'
import { CatalogPage } from '../pages/CatalogPage'
import { ImagePage } from '../pages/ImagePage'
import fixture from '../test/fixtures/historyReport.json'
import { mockApi, renderAt } from '../test/render'

const full = fixture as unknown as HistoryReport

const entry = (e: Partial<HistoryEntry>): HistoryEntry => ({
  kind: 'changes',
  archive: false,
  total: 0,
  added: [],
  back: [],
  removed: [],
  not_in_capture: [],
  became_free: [],
  became_subscription: [],
  ...e,
})

/** The report for a search matching one image added between 2026-10-08 and 2026-10-10 (HV-6.2, HV-17.1). */
const verdaccio: HistoryReport = {
  ...full,
  query: 'verdaccio',
  points: full.points.map((p) => ({ ...p, total: p.date >= '2026-10-10' ? 1 : 0, free: p.free === undefined ? undefined : 0 })),
  entries: [
    entry({ kind: 'no_changes', from: '2026-10-11', to: '2026-10-14', total: 1 }),
    entry({ from: '2026-10-08', to: '2026-10-10', total: 1, added: ['verdaccio'] }),
  ],
  total_now: 1,
  free_now: 0,
}

/** The element holding a summary tile's figure. */
const tile = (label: string) => within(document.querySelector('dl')!).getByText(label).nextElementSibling

function renderHistory(search = '', report: HistoryReport = full) {
  const fetchMock = mockApi({ '/api/history': { body: report } })
  renderAt(`/?view=history${search}`, '/', <CatalogPage />)
  return fetchMock
}

describe('History view', () => {
  it('HV-14.1 shows the age, the tiles, the chart and the timeline', async () => {
    renderHistory()
    expect(await screen.findByText(/Last snapshot: 2026-10-14/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'History' })).toHaveAttribute('aria-pressed', 'true')
    expect(tile('Images')).toHaveTextContent('3,171')
    expect(tile('Free')).toHaveTextContent('59')
    const since = screen.getByText('Since first capture')
    expect(since.nextElementSibling).toHaveTextContent('+2,496')
    expect(since.parentElement).toHaveTextContent('from 675 on 2024-04-19')
    expect(screen.getByTestId('growth-chart')).toBeInTheDocument()
    expect(screen.getByText('2026-10-11 to 2026-10-14: no changes')).toBeInTheDocument()
    expect(screen.queryByText(/stalled/)).not.toBeInTheDocument()
  })

  it('HV-14.1 opens from the catalog page and asks the API', async () => {
    const fetchMock = mockApi({ '/api/history': { body: full }, '/api/images': { body: { total: 0, count: 0, offset: 0, images: [] } } })
    renderAt('/', '/', <CatalogPage />)
    await userEvent.click(screen.getByRole('button', { name: 'History' }))
    await screen.findByText(/Last snapshot/)
    expect(fetchMock.mock.calls.map((c) => String(c[0]))).toContain('/api/history')
  })

  it('HV-14.2 follows the search', async () => {
    const fetchMock = mockApi({ '/api/history': [{ query: { query: 'verdaccio' }, body: verdaccio }, { body: full }] })
    renderAt('/?view=history&q=verdaccio', '/', <CatalogPage />)
    expect(await screen.findByText('Between 2026-10-08 and 2026-10-10: 1 added')).toBeInTheDocument()
    expect(screen.getByText('verdaccio')).toBeInTheDocument()
    expect(tile('Images')).toHaveTextContent('1')
    expect(String(fetchMock.mock.calls[0][0])).toContain('query=verdaccio')
  })

  it('HV-14.4 counts a search from its first match', async () => {
    renderHistory('&q=verdaccio', verdaccio)
    await screen.findByText('Between 2026-10-08 and 2026-10-10: 1 added')
    const since = screen.getByText('Since first match')
    expect(since.nextElementSibling).toHaveTextContent('+0')
    expect(since.parentElement).toHaveTextContent('from 1 on 2026-10-10')
  })

  it('HV-14.3 says when matching images never changed', async () => {
    const traefik: HistoryReport = {
      ...full,
      query: 'traefik',
      entries: [
        entry({ kind: 'no_changes', from: '2026-10-10', to: '2026-10-14', total: 2 }),
        entry({ kind: 'no_changes', archive: true, from: '2024-05-10', to: '2026-10-08', total: 2 }),
        entry({ kind: 'first_capture', archive: true, date: '2024-04-19', total: 2 }),
      ],
    }
    renderHistory('&q=traefik', traefik)
    expect(await screen.findByText(/No changes are recorded for matching images since the first capture, 2024-04-19/)).toBeInTheDocument()
  })

  it('HV-17.1 starts a search at its first match', async () => {
    renderHistory('&q=verdaccio', verdaccio)
    await screen.findByText('Between 2026-10-08 and 2026-10-10: 1 added')
    const section = screen.getByRole('region', { name: 'Changes' })
    expect(section.querySelectorAll('ol > li')).toHaveLength(2)
    expect(screen.queryByText(/first capture,/)).not.toBeInTheDocument()
    expect(screen.queryByText(/Before daily tracking/)).not.toBeInTheDocument()
  })

  it('HV-17.2 ends a search matching the first capture with it', async () => {
    const fips: HistoryReport = {
      ...full,
      query: 'fips',
      entries: [full.entries[0], entry({ kind: 'first_capture', archive: true, date: '2024-04-19', total: 204 })],
    }
    renderHistory('&q=fips', fips)
    fireEvent.click(await screen.findByText(/Before daily tracking/))
    expect(screen.getByText('2024-04-19: first capture, 204 matching images')).toBeInTheDocument()
  })

  it('HV-18.1 shows live entries and collapses the archive era', async () => {
    renderHistory()
    await screen.findByText('2026-10-11 to 2026-10-14: no changes')
    expect(screen.getByText('Between 2026-10-08 and 2026-10-10: 5 added')).toBeVisible()
    const summary = screen.getByText(/Before daily tracking \(Internet Archive captures\) · 17 entries/)
    const details = summary.closest('details')!
    expect(details).not.toHaveAttribute('open')
    // A summary is a native disclosure widget, so Enter and Space open it too.
    expect(summary.tagName).toBe('SUMMARY')
    fireEvent.click(summary)
    expect(details).toHaveAttribute('open')
    expect(within(details).getByText('2024-04-19: first capture, 675 images')).toBeInTheDocument()
  })

  it('HV-9.1 words archive-era removals as not in the capture', async () => {
    renderHistory()
    await screen.findByText(/Before daily tracking/)
    expect(screen.getByText(/Between 2026-05-10 and 2026-10-08: 781 added, 3 back, 15 not in the 2026-10-08 capture/)).toBeInTheDocument()
    expect(screen.getByText('Not in the 2026-10-08 capture (15)')).toBeInTheDocument()
    expect(screen.queryByText(/^Removed/)).not.toBeInTheDocument()
  })

  it('HV-8.3 shows 10 names and expands to all of them', async () => {
    renderHistory()
    await screen.findByText(/Before daily tracking/)
    const list = screen.getByText('Added (781)').parentElement!
    expect(list.querySelectorAll(':scope > ul > li')).toHaveLength(10)
    fireEvent.click(within(list).getByText('Show 771 more'))
    expect(list.querySelectorAll('li')).toHaveLength(781)
  })

  it('HV-4.2 says when the collector may have stalled', async () => {
    renderHistory('', {
      ...full,
      days_old: 3,
      stalled: true,
      note: 'The last snapshot is 3 days old; the daily collector may have stalled, so newer changes may be missing.',
    })
    expect(await screen.findByText(/3 days old; the daily collector may have stalled/)).toBeInTheDocument()
  })

  it('HV-3.2 explains when the history could not be loaded', async () => {
    mockApi({ '/api/history': { status: 503, body: { error: "the catalog history couldn't be loaded", code: 'history_unavailable' } } })
    renderAt('/?view=history', '/', <CatalogPage />)
    expect(await screen.findByText("The catalog history couldn't be loaded")).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'List' })).toBeInTheDocument()
  })
})

describe('Growth chart', () => {
  it('HV-15.1 draws no free point before the first live snapshot', async () => {
    renderHistory()
    const chart = await screen.findByTestId('growth-chart')
    expect(chart.querySelectorAll('circle[data-point="archive"]')).toHaveLength(17)
    expect(chart.querySelectorAll('circle[data-point="live"]')).toHaveLength(5)
    expect(chart.querySelectorAll('circle[data-point="free"]')).toHaveLength(5)
    expect(screen.getByText(/Free \(from 2026-10-10\)/)).toBeInTheDocument()
    const rows = Array.from(document.querySelectorAll('tbody tr'))
    const live = rows.find((r) => r.textContent?.startsWith('2026-10-10'))!
    expect(live).toHaveTextContent('59')
  })

  it('HV-15.2 has a table of every snapshot and hides the chart from assistive technology', async () => {
    renderHistory()
    const chart = await screen.findByTestId('growth-chart')
    expect(chart).toHaveAttribute('aria-hidden', 'true')
    const rows = document.querySelectorAll('table tbody tr')
    expect(rows).toHaveLength(22)
    expect(rows[0]).toHaveTextContent('2024-04-19')
    expect(rows[0]).toHaveTextContent('Internet Archive capture')
    expect(rows[0]).toHaveTextContent('675')
    expect(rows[0]).toHaveTextContent('unknown')
  })
})

describe('Image page', () => {
  const tags = { '/api/images/chainctl/tags': { body: { image: 'chainctl', reference: 'cgr.dev/chainguard/chainctl', tags: ['latest'] } } }
  const details = (extra: object) => ({
    body: {
      image: 'chainctl',
      tag: 'latest',
      reference: 'cgr.dev/chainguard/chainctl:latest',
      pinned_reference: 'cgr.dev/chainguard/chainctl:latest@sha256:abc',
      digest: 'sha256:abc',
      platforms: [],
      user: '65532',
      runs_as_root: false,
      config_platform: 'linux/amd64',
      ...extra,
    },
  })

  it('HV-16.1 shows when the image first appeared', async () => {
    mockApi({
      ...tags,
      '/api/images/chainctl/details': details({
        catalog_since: { earliest_capture: false, from: '2026-02-05', to: '2026-05-10', text: 'added between 2026-02-05 and 2026-05-10' },
      }),
    })
    renderAt('/images/chainctl', '/images/:name', <ImagePage />)
    expect(await screen.findByText('In the catalog: added between 2026-02-05 and 2026-05-10')).toBeInTheDocument()
  })

  it('HV-13.2 shows nothing when the details have no catalog_since', async () => {
    mockApi({ ...tags, '/api/images/chainctl/details': details({}) })
    renderAt('/images/chainctl', '/images/:name', <ImagePage />)
    await screen.findByText('cgr.dev/chainguard/chainctl:latest@sha256:abc')
    await waitFor(() => expect(screen.queryByText(/In the catalog/)).not.toBeInTheDocument())
  })
})
