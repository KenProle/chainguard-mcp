import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes, useLocation, useParams } from 'react-router'
import { describe, expect, it, vi } from 'vitest'
import type { GroupBy } from '../api'
import { catalogGroups, catalogPage, familiesFor } from '../test/fixtures/catalog'
import { CatalogPage } from './CatalogPage'

const json = (body: unknown, status = 200) =>
  new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })

/**
 * Stubs fetch for the Map view. Families and groups answer at once. Image list
 * batches answer at once when auto is set; otherwise they wait until the test
 * calls answer(), so tests can see what happens between batches.
 */
function mockMapApi({ auto = true, fail = [] as number[], failed = [] as string[] } = {}) {
  const requests: URL[] = []
  const waiting: { url: URL; respond: (r: Response) => void }[] = []
  const batch = (url: URL) => {
    const offset = Number(url.searchParams.get('offset') ?? 0)
    if (fail.includes(offset)) return json({ error: 'upstream error', code: 'upstream_error' }, 502)
    return json(catalogPage(offset, Number(url.searchParams.get('limit')), failed, url.searchParams.get('query') ?? ''))
  }
  vi.stubGlobal(
    'fetch',
    vi.fn(async (input: RequestInfo | URL) => {
      const url = new URL(String(input), 'http://localhost')
      requests.push(url)
      const query = url.searchParams.get('query') ?? ''
      if (url.pathname === '/api/families') return json(familiesFor(query))
      if (url.pathname === '/api/groups') {
        return json(catalogGroups((url.searchParams.get('group_by') ?? 'variant') as GroupBy, query as '' | 'nginx'))
      }
      if (url.pathname === '/api/images') {
        if (auto) return batch(url)
        return new Promise<Response>((respond) => waiting.push({ url, respond }))
      }
      return json({ error: 'no mock', code: 'not_found' }, 404)
    }),
  )
  return {
    /** Image list requests as "query@offset", in the order they were made. */
    batches: () => requests.filter((u) => u.pathname === '/api/images').map((u) => `${u.searchParams.get('query') ?? ''}@${u.searchParams.get('offset')}`),
    waiting: () => waiting.length,
    /** Answers the oldest waiting batch. */
    answer: () => {
      const next = waiting.shift()
      if (!next) throw new Error('no batch is waiting')
      next.respond(batch(next.url))
    },
  }
}

function Location() {
  const { search } = useLocation()
  return <output data-testid="search">{search}</output>
}

function ImageStub() {
  return <h1>Image page for {useParams().name}</h1>
}

function renderCatalog(path = '/?view=map') {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={[path]}>
        <Routes>
          <Route
            path="/"
            element={
              <>
                <CatalogPage />
                <Location />
              </>
            }
          />
          <Route path="/images/:name" element={<ImageStub />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

const treemap = () => screen.findByRole('img', { name: /^Treemap of/ })
const unit = (name: string) => document.querySelector(`rect[data-image="${name}"]`)
const groupUnits = (label: string) => [...document.querySelectorAll(`g[data-group-label="${label}"] rect[data-image]`)]
// Rendering the whole catalog's 3,166 units is slow in jsdom, so allow time
// for every batch.
const statusLine = (text: string) => screen.findByText(text, {}, { timeout: 10_000 })
const allLoaded = () => statusLine('3,166 images in 1,751 families · 59 free')
const nginxLoaded = () => statusLine('31 images in 13 families · 1 free')
const familyRow = (name: string) =>
  screen.getAllByRole('row').find((row) => row.querySelector('td a')?.textContent === name) as HTMLElement

describe('Catalog map view', { timeout: 20_000 }, () => {
  it('CM-5.1 switches to the map, grouped by variant, and records it in the URL', async () => {
    mockMapApi()
    renderCatalog('/')
    const user = userEvent.setup()
    await user.click(await screen.findByRole('button', { name: 'Map' }))
    expect(screen.getByTestId('search')).toHaveTextContent('view=map')
    expect(await screen.findByRole('table', { name: 'Image families' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Map' })).toHaveAttribute('aria-pressed', 'true')
    expect(screen.getByRole('button', { name: 'Variant' })).toHaveAttribute('aria-pressed', 'true')
    expect(groupUnits('Base images')).toHaveLength(1730)
    expect(screen.queryByRole('checkbox', { name: 'Free only' })).not.toBeInTheDocument()
  })

  it('CM-5.2 opens a shared link with a grouping and a search', async () => {
    mockMapApi()
    renderCatalog('/?view=map&group=prefix&q=nginx')
    expect(screen.getByRole('searchbox', { name: 'Search images' })).toHaveValue('nginx')
    const table = await screen.findByRole('table', { name: 'Image families' })
    expect(within(table).getAllByRole('row')).toHaveLength(13 + 1)
    expect(screen.getByRole('button', { name: 'Name prefix' })).toHaveAttribute('aria-pressed', 'true')
    expect(groupUnits('ingress')).toHaveLength(9)
  })

  it('CM-5.3 keeps the list view as the default', async () => {
    mockMapApi()
    renderCatalog('/')
    expect(await screen.findByText('3,166 images')).toBeInTheDocument()
    expect(screen.getByRole('checkbox', { name: 'Free only' })).toBeInTheDocument()
    expect(screen.getByRole('navigation', { name: 'Pagination' })).toHaveTextContent('Page 1 of 64')
    expect(screen.queryByRole('img', { name: /^Treemap of/ })).not.toBeInTheDocument()
  })

  it('CM-5.4 switches the grouping without changing the table', async () => {
    mockMapApi()
    renderCatalog()
    await treemap()
    const rowsBefore = screen.getAllByRole('row').map((r) => r.textContent)
    fireEvent.pointerMove(unit('apko') as Element)
    expect(document.querySelector('[data-tooltip]')).toHaveTextContent('Base images')
    const user = userEvent.setup()
    await user.click(screen.getByRole('button', { name: 'Name prefix' }))
    expect(screen.getByTestId('search')).toHaveTextContent('group=prefix')
    await waitFor(() => expect(groupUnits('crossplane')).toHaveLength(414))
    expect(groupUnits('Base images')).toHaveLength(0)
    expect(document.querySelector('[data-tooltip]')).toBeNull()
    expect(screen.getAllByRole('row').map((r) => r.textContent)).toEqual(rowsBefore)
  })

  it('CM-6.1 draws one unit per image in its group', async () => {
    mockMapApi()
    renderCatalog()
    await treemap()
    expect(groupUnits('FIPS')).toHaveLength(1210)
    expect(groupUnits('FIPS').map((u) => u.getAttribute('data-image'))).toContain('nginx-fips')
    expect(document.querySelectorAll('rect[data-image]')).toHaveLength(3166)
  })

  it('CM-6.4 keeps every unit in place as statuses arrive', async () => {
    const api = mockMapApi({ auto: false })
    renderCatalog()
    await treemap()
    const place = (name: string) => ['x', 'y', 'width', 'height'].map((a) => unit(name)?.getAttribute(a)).join()
    const before = ['apko', 'nginx', 'nginx-fips'].map(place)
    expect(unit('apko')).toHaveClass('fill-zinc-200')
    await waitFor(() => expect(api.waiting()).toBe(1))
    api.answer()
    await statusLine('Checked 1,000 of 3,166 images')
    expect(unit('apko')).toHaveClass('fill-emerald-500')
    expect(['apko', 'nginx', 'nginx-fips'].map(place)).toEqual(before)
  })

  it('CM-7.1 colors nginx free and its variants subscription', async () => {
    mockMapApi()
    renderCatalog()
    await allLoaded()
    expect(unit('nginx')).toHaveClass('fill-emerald-500')
    for (const name of ['nginx-fips', 'nginx-iamguarded', 'nginx-iamguarded-fips']) expect(unit(name)).toHaveClass('fill-slate-300')
    expect(screen.getByText('Free: 59')).toBeInTheDocument()
    expect(screen.getByText('Subscription: 3,107')).toBeInTheDocument()
    expect(screen.queryByText(/^Not known/)).not.toBeInTheDocument()
  })

  it('CM-7.3 shows a failed check as not known', async () => {
    mockMapApi({ failed: ['apko'] })
    renderCatalog()
    await statusLine('3,166 images in 1,751 families · 58 free')
    expect(unit('apko')).toHaveClass('fill-zinc-200')
    expect(screen.getByText('Not known: 1')).toBeInTheDocument()
  })

  it('CM-8.1 draws the map before any status is known', async () => {
    mockMapApi({ auto: false })
    renderCatalog()
    await treemap()
    expect(screen.getByText('Checking free-tier status: 0 of 3,166 images')).toBeInTheDocument()
    expect(screen.getByText(/about 30 seconds/)).toBeInTheDocument()
    expect(document.querySelectorAll('rect[data-image].fill-zinc-200')).toHaveLength(3166)
    expect(screen.getByText('Not known: 3,166')).toBeInTheDocument()
  })

  it('CM-8.2 loads statuses in batches of 1,000, one at a time', async () => {
    const api = mockMapApi({ auto: false })
    renderCatalog()
    await waitFor(() => expect(api.waiting()).toBe(1))
    expect(api.batches()).toEqual(['@0'])

    api.answer()
    await statusLine('Checked 1,000 of 3,166 images')
    expect(unit('apko')).toHaveClass('fill-emerald-500')
    await waitFor(() => expect(api.batches()).toEqual(['@0', '@1000']))

    api.answer()
    await waitFor(() => expect(api.batches()).toEqual(['@0', '@1000', '@2000']))
    api.answer()
    await waitFor(() => expect(api.batches()).toEqual(['@0', '@1000', '@2000', '@3000']))
    api.answer()
    await allLoaded()
    expect(api.batches()).toHaveLength(4)
  })

  it('CM-8.4 keeps the map when a batch fails and retries only that batch', async () => {
    const api = mockMapApi({ fail: [1000] })
    renderCatalog()
    const alert = await screen.findByRole('alert')
    expect(alert).toHaveTextContent('upstream error')
    expect(unit('apko')).toHaveClass('fill-emerald-500')
    expect(screen.getByText('Checked 1,000 of 3,166 images')).toBeInTheDocument()
    expect(api.batches()).toEqual(['@0', '@1000'])

    const user = userEvent.setup()
    await user.click(within(alert).getByRole('button', { name: 'Try again' }))
    await waitFor(() => expect(api.batches()).toEqual(['@0', '@1000', '@1000']))
  })

  it('CM-8.5 abandons the old batches when the search changes', async () => {
    const api = mockMapApi({ auto: false })
    renderCatalog()
    await waitFor(() => expect(api.waiting()).toBe(1))
    api.answer()
    await waitFor(() => expect(api.batches()).toEqual(['@0', '@1000']))

    const user = userEvent.setup()
    await user.type(screen.getByRole('searchbox', { name: 'Search images' }), 'nginx')
    await waitFor(() => expect(api.batches()).toContain('nginx@0'))
    // The old search's waiting batch answers late; no further old batch follows.
    while (api.waiting() > 0) api.answer()
    await nginxLoaded()
    expect(api.batches().filter((b) => b.startsWith('@'))).toEqual(['@0', '@1000'])
  })

  it('CM-9.1 shows a tooltip at once when the pointer moves onto a unit', async () => {
    mockMapApi()
    renderCatalog()
    const map = await treemap()
    await allLoaded()
    expect(map.querySelector('title')).toBeNull()
    fireEvent.pointerMove(unit('nginx-fips') as Element)
    expect(map.querySelector('[data-tooltip]')).toHaveTextContent('nginx-fips · subscription · FIPS')
    fireEvent.pointerLeave(map)
    expect(map.querySelector('[data-tooltip]')).toBeNull()
  })

  it('CM-9.2 opens an image from its unit', async () => {
    mockMapApi()
    renderCatalog()
    await treemap()
    fireEvent.click(unit('nginx') as Element)
    expect(await screen.findByRole('heading', { name: 'Image page for nginx' })).toBeInTheDocument()
  })

  it('CM-10.1 lists a family with its images and statuses', async () => {
    mockMapApi()
    renderCatalog('/?view=map&q=nginx')
    await nginxLoaded()
    const row = familyRow('nginx')
    const cells = within(row).getAllByRole('cell')
    expect(cells[1]).toHaveTextContent('4')
    expect(cells[2]).toHaveTextContent('1')
    const items = within(cells[0]).getAllByRole('listitem')
    expect(items.map((li) => li.textContent)).toEqual([
      'nginx Free',
      'nginx-fips Subscription',
      'nginx-iamguarded Subscription',
      'nginx-iamguarded-fips Subscription',
    ])
    for (const li of items) {
      const link = within(li).getByRole('link')
      expect(link).toHaveAttribute('href', `/images/${link.textContent}`)
    }
  })

  it('CM-10.2 sorts the families with a free image first', async () => {
    mockMapApi()
    renderCatalog()
    await allLoaded()
    const user = userEvent.setup()
    await user.click(screen.getByRole('button', { name: /^Free/ }))
    expect(screen.getByTestId('search')).toHaveTextContent('sort=free')
    const names = screen.getAllByRole('row').slice(1, 4).map((r) => r.querySelector('td a')?.textContent)
    expect(names).toEqual(['go', 'haproxy', 'jdk'])
    expect(screen.getByRole('columnheader', { name: /^Free/ })).toHaveAttribute('aria-sort', 'descending')
  })

  it('CM-10.3 pages through the families', async () => {
    mockMapApi()
    renderCatalog()
    const nav = await screen.findByRole('navigation', { name: 'Pagination' })
    expect(nav).toHaveTextContent('Page 1 of 36')
    const user = userEvent.setup()
    await user.click(within(nav).getByRole('button', { name: 'Next' }))
    expect(screen.getByTestId('search')).toHaveTextContent('page=2')
    expect(nav).toHaveTextContent('Page 2 of 36')
  })

  it('CM-10.4 shows no free count until a family is checked', async () => {
    mockMapApi({ auto: false })
    renderCatalog('/?view=map&q=nginx')
    await treemap()
    const cells = within(familyRow('nginx')).getAllByRole('cell')
    expect(cells[2]).toHaveTextContent('—')
    expect(within(cells[0]).queryByText('Free')).not.toBeInTheDocument()
  })

  it('CM-11.1 shows a stacked bar per group for phones', async () => {
    mockMapApi()
    renderCatalog()
    await allLoaded()
    const phone = document.querySelector('.sm\\:hidden') as HTMLElement
    const rows = [...phone.querySelectorAll('div[aria-hidden="true"] > div')]
    expect(rows.map((r) => r.firstElementChild?.textContent)).toEqual([
      'Base images',
      'FIPS',
      'IAM-guarded',
      'IAM-guarded FIPS',
      'Other variants (7 kinds)',
    ])
    expect(rows.map((r) => r.lastElementChild?.textContent?.match(/\d+ free/)?.[0])).toEqual([
      '58 free',
      '0 free',
      '0 free',
      '0 free',
      '1 free',
    ])
    const base = [...rows[0].querySelectorAll('rect')].map((r) => Number(r.getAttribute('width')))
    expect(base[0]).toBeCloseTo((58 / 1730) * 100, 6)
    expect(base[1]).toBeCloseTo((1672 / 1730) * 100, 6)
    expect(document.querySelector('.hidden.sm\\:block svg[role="img"]')).not.toBeNull()
  })

  it('CM-12.1 names the treemap with a summary', async () => {
    mockMapApi()
    renderCatalog()
    await allLoaded()
    expect(
      screen.getByRole('img', {
        name: 'Treemap of 3,166 images in 5 variant groups: 59 free. The family table below lists every family.',
      }),
    ).toBeInTheDocument()
  })

  it('CM-12.2 reaches the switches and table by keyboard without stopping in the treemap', async () => {
    mockMapApi()
    renderCatalog('/?view=map&q=nginx')
    const map = await treemap()
    await nginxLoaded()
    expect(map.querySelectorAll('a, [tabindex], button')).toHaveLength(0)

    const user = userEvent.setup()
    const target = within(familyRow('nginx')).getAllByRole('link')[0]
    const focused = new Set<Element | null>()
    for (let i = 0; i < 40 && document.activeElement !== target; i++) {
      await user.tab()
      focused.add(document.activeElement)
      expect(map.contains(document.activeElement)).toBe(false)
    }
    expect(target).toHaveFocus()
    expect(focused).toContain(screen.getByRole('button', { name: 'Name prefix' }))
    await user.keyboard('{Enter}')
    expect(await screen.findByRole('heading', { name: 'Image page for nginx' })).toBeInTheDocument()
  })

  it('CM-17.3 shortens a long name with an ellipsis and shows it in full on hover', async () => {
    mockMapApi()
    renderCatalog('/?view=map&group=prefix')
    const map = await treemap()
    await waitFor(() => expect(groupUnits('crossplane')).toHaveLength(414))
    // Every header band has text in it.
    const headers = [...map.querySelectorAll('[data-header]')]
    expect(headers.length).toBeGreaterThan(0)
    for (const h of headers) expect(h.querySelector('text')?.textContent).toBeTruthy()
    const kubernetes = map.querySelector('g[data-group-label="kubernetes"] [data-header]') as Element
    expect(kubernetes.querySelector('text')?.textContent).toMatch(/^kube[a-z]*…$/)
    fireEvent.pointerMove(kubernetes.querySelector('rect') as Element)
    expect(map.querySelector('[data-tooltip]')).toHaveTextContent('kubernetes: 51 images')
  })

  it('CM-18.1 draws a name prefix as one block within its group', async () => {
    mockMapApi()
    renderCatalog()
    await treemap()
    const units = groupUnits('Base images')
    const names = units.map((u) => u.getAttribute('data-image') ?? '')
    // crossplane's 220 images are the largest block, so they come first.
    const cross = names.flatMap((n, i) => (n === 'crossplane' || n.startsWith('crossplane-') ? [i] : []))
    expect(cross).toHaveLength(220)
    expect(cross[0]).toBe(0)
    expect(cross.at(-1)).toBe(219)
    // No other unit's center lies inside the crossplane block's bounds.
    const box = (u: Element) => ['x', 'y', 'width', 'height'].map((a) => Number(u.getAttribute(a)))
    const boxes = cross.map((i) => box(units[i]))
    const [x0, y0] = [Math.min(...boxes.map((b) => b[0])), Math.min(...boxes.map((b) => b[1]))]
    const [x1, y1] = [Math.max(...boxes.map((b) => b[0] + b[2])), Math.max(...boxes.map((b) => b[1] + b[3]))]
    units.forEach((u, i) => {
      if (cross.includes(i)) return
      const [x, y, w, h] = box(u)
      const inside = x + w / 2 > x0 && x + w / 2 < x1 && y + h / 2 > y0 && y + h / 2 < y1
      expect(inside).toBe(false)
    })
  })

  it('CM-18.4 still names the group when hovering a unit in a block', async () => {
    mockMapApi()
    renderCatalog('/?view=map&group=prefix')
    const map = await treemap()
    await waitFor(() => expect(groupUnits('crossplane')).toHaveLength(414))
    fireEvent.pointerMove(map.querySelector('rect[data-image^="flux-"]') as Element)
    expect(map.querySelector('[data-tooltip]')).toHaveTextContent('flux-fips · subscription · Other (690 prefixes)')
  })

  it('CM-17.1 labels the large groups', async () => {
    mockMapApi()
    renderCatalog()
    const map = await treemap()
    const headers = [...map.querySelectorAll('text')].map((t) => t.textContent)
    expect(headers).toContain('Base images: 1,730')
    expect(headers).toContain('FIPS: 1,210')
  })

  it('CM-17.1 gives every prefix group a header', async () => {
    mockMapApi()
    renderCatalog('/?view=map&group=prefix')
    const map = await treemap()
    await waitFor(() => expect(groupUnits('crossplane')).toHaveLength(414))
    const groups = [...map.querySelectorAll('g[data-group-label]')]
    expect(groups).toHaveLength(11)
    for (const g of groups) expect(g.querySelector('[data-header] text')?.textContent).toBeTruthy()
    expect(map.querySelector('g[data-group-label="cert"] [data-header]')).toHaveTextContent('cert: 37')
  })
})
