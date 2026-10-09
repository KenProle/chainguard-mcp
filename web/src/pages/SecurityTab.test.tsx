import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import type { VulnReport } from '../api'
import { pythonTags } from '../test/fixtures/python'
import { pythonVulnsLatest, pythonVulnsLatestDev, pythonVulnsWithPending } from '../test/fixtures/pythonVulns'
import { mockApi, renderAt } from '../test/render'
import { ImagePage } from './ImagePage'

function mockPython(latest: VulnReport = pythonVulnsLatest) {
  return mockApi({
    '/api/images/python/tags': { body: pythonTags },
    '/api/images/python/vulnerabilities': [
      { query: { tag: 'latest' }, body: latest },
      { query: { tag: 'latest-dev' }, body: pythonVulnsLatestDev },
    ],
  })
}

const renderSecurity = () => renderAt('/images/python?tab=security', '/images/:name', <ImagePage />)

const chart = async () => within(await screen.findByRole('region', { name: 'Fixes per source package' }))
/** The bar rows of the (aria-hidden) chart. */
const barRows = async () => {
  const region = await screen.findByRole('region', { name: 'Fixes per source package' })
  return [...(region.querySelector('div[aria-hidden="true"]')?.children ?? [])]
}
const barLabels = async () => (await barRows()).map((row) => row.firstElementChild?.textContent)

describe('SecurityTab fixes chart', () => {
  it('SF-1.1 shows a bar per source package with records', async () => {
    mockPython()
    renderSecurity()
    const labels = await barLabels()
    expect(labels).toHaveLength(12)
    for (const name of ['bzip2', 'readline', 'zstd']) expect(labels).not.toContain(name)
  })

  it('SF-3.1 orders the bars by fixes included', async () => {
    mockPython()
    renderSecurity()
    expect(await barLabels()).toEqual([
      'openssl-4.0',
      'python-3.14',
      'expat',
      'py3-pip',
      'glibc-2.44',
      'zlib',
      'gcc',
      'sqlite',
      'util-linux',
      'brotli',
      'ncurses',
      'xz',
    ])
  })

  it('SF-4.1 highlights a fix not yet installed and names it as text', async () => {
    mockPython(pythonVulnsWithPending)
    renderSecurity()
    const [zlib] = await barRows()
    expect(zlib.firstElementChild).toHaveTextContent('zlib')
    const rects = zlib.querySelectorAll('rect')
    expect(rects[rects.length - 1]).toHaveClass('fill-rose-500')
    expect(screen.getByText('zlib 1.3.2.1_rc20260917-r0: CVE-2026-0001')).toBeInTheDocument()
  })

  it('SF-5.1 labels each bar with its fixes and shows a legend', async () => {
    mockPython()
    renderSecurity()
    const rows = await barRows()
    const valueOf = (name: string) => rows.find((r) => r.firstElementChild?.textContent === name)?.lastElementChild?.textContent
    expect(valueOf('openssl-4.0')).toBe('147 fixed')
    expect(valueOf('sqlite')).toBe('0 fixed')
    const section = await chart()
    for (const text of ['Fixes included', 'Never affected', 'Not yet installed']) {
      expect(section.getByText(text)).toBeInTheDocument()
    }
  })

  it('SF-6.1 keeps the table of every source package and hides the chart from assistive technology', async () => {
    mockPython()
    renderSecurity()
    const [wrapper] = (await barRows()).map((row) => row.parentElement)
    expect(wrapper).toHaveAttribute('aria-hidden', 'true')
    const body = screen.getByRole('table').querySelector('tbody')!
    const rows = within(body).getAllByRole('row')
    expect(rows).toHaveLength(21)
    const cells = (name: string) =>
      within(rows.find((r) => within(r).getAllByRole('cell')[0].textContent === name)!)
        .getAllByRole('cell')
        .map((c) => c.textContent)
    expect(cells('openssl-4.0').slice(2)).toEqual(['147', '6'])
    expect(cells('readline').slice(2)).toEqual(['0', '0'])
  })

  it('SF-7.1 counts the source packages without records', async () => {
    mockPython()
    renderSecurity()
    expect((await chart()).getByText('9 more source packages have no records in the Wolfi security database.')).toBeInTheDocument()
  })

  it('SF-7.2 says when no package has records, without claiming the image is unaffected', async () => {
    mockPython({
      ...pythonVulnsLatest,
      packages: pythonVulnsLatest.packages?.map((p) => ({ ...p, fixed_count: 0, not_affected_count: 0 })),
      total_fixed: 0,
      total_not_affected: 0,
    })
    renderSecurity()
    const section = await chart()
    expect(section.getByText("None of this image's 21 source packages have records in the Wolfi security database.")).toBeInTheDocument()
    expect(await barRows()).toHaveLength(0)
    expect(section.queryByText('Fixes included')).toBeNull()
    expect(document.body.textContent).not.toMatch(/no vulnerabilities|is secure|vulnerability-free/i)
  })

  it('SF-7.3 keeps the note that the data is not a full scan', async () => {
    mockPython()
    renderSecurity()
    await chart()
    expect(screen.getByText(/doesn't list unfixed vulnerabilities, so it isn't a full scan/)).toBeInTheDocument()
  })

  it('SF-8.1 makes no request beyond the vulnerability summary', async () => {
    const fetchMock = mockPython()
    renderSecurity()
    await chart()
    const paths = fetchMock.mock.calls.map(([url]) => new URL(String(url), 'http://localhost').pathname)
    expect(new Set(paths)).toEqual(new Set(['/api/images/python/tags', '/api/images/python/vulnerabilities']))
    expect(paths.filter((p) => p.endsWith('/vulnerabilities'))).toHaveLength(1)
  })

  it('SF-8.2 recalculates when the tag changes', async () => {
    mockPython()
    renderSecurity()
    await chart()
    await userEvent.selectOptions(screen.getByRole('combobox', { name: 'Tag' }), 'latest-dev')
    await screen.findByText('31 more source packages have no records in the Wolfi security database.')
    const labels = await barLabels()
    expect(labels).toHaveLength(25)
    expect(labels.slice(0, 3)).toEqual(['openssl-4.0', 'python-3.14', 'curl'])
  })

  it('SF-8.3 shows the subscription message and no chart for a subscription-only image', async () => {
    mockApi({
      '/api/images/loki-fips/tags': { status: 403, body: { error: 'not public', code: 'not_public' } },
    })
    renderAt('/images/loki-fips?tab=security', '/images/:name', <ImagePage />)
    expect(await screen.findByText('This image requires a Chainguard subscription')).toBeInTheDocument()
    expect(screen.queryByRole('region', { name: 'Fixes per source package' })).toBeNull()
  })
})
