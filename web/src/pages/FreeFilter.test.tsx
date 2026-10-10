import { act, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { mockApi, renderAt } from '../test/render'
import { CatalogPage } from './CatalogPage'

const TOTAL = 3171
const BATCH = 250

const image = (name: string, free: boolean) => ({ name, reference: `cgr.dev/chainguard/${name}`, free })

/** One batch of the whole catalog (image-0001 and on, 250 to a batch). */
function batchResponse(offset: number) {
  const count = Math.min(BATCH, TOTAL - offset)
  return {
    query: { limit: String(BATCH), offset: String(offset) },
    body: {
      total: TOTAL,
      count,
      offset,
      images: Array.from({ length: count }, (_, i) => image(`image-${String(offset + i + 1).padStart(4, '0')}`, false)),
    },
  }
}

const freeImages = Array.from({ length: 59 }, (_, i) => image(`free-${String(i + 1).padStart(2, '0')}`, true))

type Entry = { query?: Record<string, string>; pending?: boolean; status?: number; body?: unknown }

/**
 * Responses for a 3,171-image catalog: a batch per offset, the free-only list,
 * the plain first page, the family total, and an 11-image `python` search.
 * overrides replaces the batch at an offset.
 */
function catalogApi(overrides: Record<number, Partial<Entry>> = {}) {
  const batches: Entry[] = []
  for (let offset = 0; offset < TOTAL; offset += BATCH) batches.push({ ...batchResponse(offset), ...overrides[offset] })
  const python = Array.from({ length: 11 }, (_, i) => image(`python-${i}`, i === 0))
  const images: Entry[] = [
    { query: { query: 'python', limit: String(BATCH) }, body: { total: 11, count: 11, offset: 0, images: python } },
    { query: { query: 'python', free_only: 'true' }, body: { total: 1, count: 1, offset: 0, images: python.slice(0, 1) } },
    { query: { free_only: 'true' }, body: { total: 59, count: 50, offset: 0, images: freeImages.slice(0, 50) } },
    ...batches,
    { query: { limit: '50' }, body: { total: TOTAL, count: 1, offset: 0, images: [image('image-0001', false)] } },
  ]
  const fetchMock = mockApi({
    '/api/images': images,
    '/api/families': { body: { total: TOTAL, families: [] } },
  })
  return { fetchMock, batches }
}

/** The /api/images requests made so far, parsed. */
const imageCalls = (fetchMock: ReturnType<typeof mockApi>) =>
  fetchMock.mock.calls
    .map(([url]) => new URL(String(url), 'http://localhost'))
    .filter((u) => u.pathname === '/api/images')
    .map((u) => ({
      query: u.searchParams.get('query'),
      limit: u.searchParams.get('limit'),
      offset: Number(u.searchParams.get('offset')),
      free: u.searchParams.get('free_only'),
    }))

const batchCalls = (fetchMock: ReturnType<typeof mockApi>) => imageCalls(fetchMock).filter((c) => c.limit === String(BATCH))

describe('Free only filter', () => {
  afterEach(() => vi.useRealTimers())

  it('FF-1.2 makes no check when "Free only" is off', async () => {
    const { fetchMock } = catalogApi()
    renderAt('/', '/', <CatalogPage />)
    await screen.findByText(`${TOTAL.toLocaleString()} images`)

    const calls = imageCalls(fetchMock)
    expect(calls).toHaveLength(1)
    expect(calls[0]).toMatchObject({ limit: '50', free: null })
    expect(screen.queryByRole('progressbar')).not.toBeInTheDocument()
  })

  it('FF-2.1 checks 13 batches in order, then requests the free list; FF-1.1 shows 59 free images', async () => {
    const { fetchMock } = catalogApi()
    renderAt('/?free=1', '/', <CatalogPage />)

    expect(await screen.findByText('59 images')).toBeInTheDocument()
    const calls = imageCalls(fetchMock)
    expect(calls.map((c) => c.offset)).toEqual([...Array.from({ length: 13 }, (_, i) => i * BATCH), 0])
    expect(calls.slice(0, 13).every((c) => c.limit === String(BATCH) && c.free === null)).toBe(true)
    expect(calls[13]).toMatchObject({ free: 'true', limit: '50' })
    expect(screen.getAllByText('Free')).toHaveLength(50)
  })

  it('FF-2.2 checks one batch for a search with 11 matches, then the free list', async () => {
    const { fetchMock } = catalogApi()
    renderAt('/?free=1&q=python', '/', <CatalogPage />)

    expect(await screen.findByText('1 image')).toBeInTheDocument()
    const calls = imageCalls(fetchMock)
    expect(calls).toHaveLength(2)
    expect(calls[0]).toMatchObject({ query: 'python', limit: String(BATCH), offset: 0, free: null })
    expect(calls[1]).toMatchObject({ query: 'python', free: 'true' })
  })

  it('FF-5.1 shows no progress bar when the check is fast', async () => {
    catalogApi()
    let seen = false
    const observer = new MutationObserver(() => {
      if (document.querySelector('[role="progressbar"]')) seen = true
    })
    observer.observe(document.body, { childList: true, subtree: true })
    renderAt('/?free=1', '/', <CatalogPage />)

    expect(await screen.findByText('59 images')).toBeInTheDocument()
    observer.disconnect()
    expect(seen).toBe(false)
  })

  it('FF-3.2 shows an empty bar after a second while no batch has returned', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true })
    catalogApi({ 0: { pending: true } })
    renderAt('/?free=1', '/', <CatalogPage />)

    expect(await screen.findByText('Loading images…')).toBeInTheDocument()
    expect(screen.queryByRole('progressbar')).not.toBeInTheDocument()
    await act(async () => {
      await vi.advanceTimersByTimeAsync(1000)
    })
    const bar = await screen.findByRole('progressbar')
    expect(bar).toHaveAttribute('aria-valuenow', '0')
    expect(bar).toHaveAttribute('aria-valuetext', 'Checking free-tier status: 0 of 3,171 images · about 30 seconds left')
    expect(screen.queryByText('Loading images…')).not.toBeInTheDocument()
  })

  it('FF-3.1 shows the bar at 250 of 3,171 after the first batch', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true })
    catalogApi({ 250: { pending: true } })
    renderAt('/?free=1', '/', <CatalogPage />)

    await act(async () => {
      await vi.advanceTimersByTimeAsync(1000)
    })
    const bar = await screen.findByRole('progressbar')
    expect(bar).toHaveAttribute('aria-valuenow', '250')
    expect(bar).toHaveAttribute('aria-valuemax', String(TOTAL))
    expect(bar.getAttribute('aria-valuetext')).toMatch(/^Checking free-tier status: 250 of 3,171 images ·/)
  })

  it('FF-6.1 stops after "Free only" is unticked', async () => {
    const { fetchMock } = catalogApi({ 750: { pending: true } })
    renderAt('/?free=1', '/', <CatalogPage />)
    await expect.poll(() => batchCalls(fetchMock).length).toBe(4)

    await userEvent.setup().click(screen.getByRole('checkbox', { name: 'Free only' }))
    expect(await screen.findByText(`${TOTAL.toLocaleString()} images`)).toBeInTheDocument()
    await new Promise((r) => setTimeout(r, 100))
    expect(batchCalls(fetchMock).map((c) => c.offset)).toEqual([0, 250, 500, 750])
  })

  it('FF-6.2 restarts the check for a changed search', async () => {
    const { fetchMock } = catalogApi({ 500: { pending: true } })
    renderAt('/?free=1', '/', <CatalogPage />)
    await expect.poll(() => batchCalls(fetchMock).length).toBe(3)

    await userEvent.setup().type(screen.getByRole('searchbox', { name: 'Search images' }), 'python')
    expect(await screen.findByText('1 image')).toBeInTheDocument()
    const calls = batchCalls(fetchMock)
    expect(calls.filter((c) => c.query === null).map((c) => c.offset)).toEqual([0, 250, 500])
    expect(calls.filter((c) => c.query === 'python')).toHaveLength(1)
  })

  it('FF-7.1 keeps progress on a failed batch and retry resumes', async () => {
    const { fetchMock, batches } = catalogApi({ 1000: { status: 502, body: { error: 'upstream failed', code: 'upstream' } } })
    renderAt('/?free=1', '/', <CatalogPage />)

    const alert = await screen.findByRole('alert')
    expect(within(alert).getByText('upstream failed')).toBeInTheDocument()
    expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '1000')
    expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuemax', String(TOTAL))

    Object.assign(batches[4], { status: 200, body: batchResponse(1000).body })
    fetchMock.mockClear()
    await userEvent.setup().click(screen.getByRole('button', { name: 'Try again' }))

    expect(await screen.findByText('59 images')).toBeInTheDocument()
    expect(batchCalls(fetchMock).map((c) => c.offset)).toEqual([1000, 1250, 1500, 1750, 2000, 2250, 2500, 2750, 3000])
  })
})
