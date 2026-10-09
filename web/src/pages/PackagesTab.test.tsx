import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { pythonPackages, pythonTags } from '../test/fixtures/python'
import { mockApi, renderAt } from '../test/render'
import { ImagePage } from './ImagePage'

function mockPython() {
  return mockApi({
    '/api/images/python/tags': { body: pythonTags },
    '/api/images/python/packages': (['latest', 'latest-dev'] as const).flatMap((tag) =>
      (['amd64', 'arm64'] as const).map((arch) => ({ query: { tag, arch }, body: pythonPackages[tag][arch] })),
    ),
  })
}

const renderPackages = (tag = 'latest') => renderAt(`/images/python?tab=packages&tag=${tag}`, '/images/:name', <ImagePage />)

const licenses = async () => within(await screen.findByRole('region', { name: 'Licenses' }))
const categoryButtons = async () => (await licenses()).getAllByRole('button').filter((b) => b.hasAttribute('aria-pressed'))
const shownPackages = () =>
  within(screen.getByRole('table'))
    .getAllByRole('row')
    .slice(1)
    .map((row) => within(row).getAllByRole('cell')[0].textContent)
const total = async () => (await categoryButtons()).reduce((n, b) => n + Number(/(\d+) package/.exec(b.getAttribute('aria-label') ?? '')?.[1]), 0)

describe('PackagesTab license breakdown', () => {
  it('LB-7.1 shows non-empty categories largest first', async () => {
    mockPython()
    renderPackages()
    expect((await categoryButtons()).map((b) => b.getAttribute('aria-label'))).toEqual([
      'Permissive, 16 packages',
      'Strong copyleft, 7 packages',
      'Weak copyleft, 6 packages',
    ])
  })

  it('LB-7.3 makes no request beyond the packages request', async () => {
    const fetchMock = mockPython()
    renderPackages()
    await categoryButtons()
    const paths = fetchMock.mock.calls.map(([url]) => new URL(String(url), 'http://localhost').pathname)
    expect(new Set(paths)).toEqual(new Set(['/api/images/python/tags', '/api/images/python/packages']))
    expect(paths.filter((p) => p.endsWith('/packages'))).toHaveLength(1)
  })

  it('LB-8.1 prints category names and counts as text', async () => {
    mockPython()
    renderPackages()
    const section = await licenses()
    for (const text of ['Permissive', '16 packages', 'Strong copyleft', '7 packages', 'Weak copyleft', '6 packages']) {
      expect(section.getByText(text)).toBeInTheDocument()
    }
  })

  it('LB-12.1 shows the not-legal-advice note', async () => {
    mockPython()
    renderPackages()
    expect((await licenses()).getByText(/licenses declared in the image's SBOM.*not legal advice/)).toBeInTheDocument()
  })

  it('LB-13.1 names each category control with its count', async () => {
    mockPython()
    renderPackages()
    expect(await screen.findByRole('button', { name: 'Strong copyleft, 7 packages' })).toHaveAttribute('aria-pressed', 'false')
  })

  it('LB-9.1 filters the table to the selected category', async () => {
    mockPython()
    renderPackages()
    const strong = await screen.findByRole('button', { name: 'Strong copyleft, 7 packages' })
    await userEvent.click(strong)
    expect(strong).toHaveAttribute('aria-pressed', 'true')
    expect(shownPackages()).toHaveLength(7)
    expect(shownPackages()).toEqual(expect.arrayContaining(['libgcc', 'readline', 'libzstd1']))
  })

  it('LB-9.2 clears the selection when the category is activated again', async () => {
    mockPython()
    renderPackages()
    const strong = await screen.findByRole('button', { name: 'Strong copyleft, 7 packages' })
    await userEvent.click(strong)
    await userEvent.click(strong)
    expect(strong).toHaveAttribute('aria-pressed', 'false')
    expect(shownPackages()).toHaveLength(29)
  })

  it('LB-9.3 clears the selection with Show all', async () => {
    mockPython()
    renderPackages()
    expect(screen.queryByRole('button', { name: 'Show all' })).toBeNull()
    await userEvent.click(await screen.findByRole('button', { name: 'Weak copyleft, 6 packages' }))
    expect(shownPackages()).toHaveLength(6)
    await userEvent.click(screen.getByRole('button', { name: 'Show all' }))
    expect(shownPackages()).toHaveLength(29)
    expect(screen.queryByRole('button', { name: 'Show all' })).toBeNull()
  })

  it('LB-10.1 combines the category with the text filter', async () => {
    mockPython()
    renderPackages()
    await userEvent.click(await screen.findByRole('button', { name: 'Strong copyleft, 7 packages' }))
    await userEvent.type(screen.getByRole('searchbox', { name: 'Filter packages' }), 'gcc')
    expect(shownPackages()).toEqual(['libgcc', 'libstdc++'])
  })

  it('LB-11.1 recalculates and clears the selection when the architecture changes', async () => {
    const fetchMock = mockPython()
    renderPackages()
    await userEvent.click(await screen.findByRole('button', { name: 'Strong copyleft, 7 packages' }))
    await userEvent.click(screen.getByRole('radio', { name: 'arm64' }))
    await waitFor(() => expect(screen.queryByRole('button', { pressed: true })).toBeNull())
    expect(fetchMock.mock.calls.some(([url]) => String(url).includes('arch=arm64'))).toBe(true)
    expect(await total()).toBe(pythonPackages.latest.arm64.total)
    expect(shownPackages()).toHaveLength(pythonPackages.latest.arm64.total)
  })

  it('LB-11.2 recalculates when the tag changes', async () => {
    mockPython()
    renderPackages()
    await userEvent.click(await screen.findByRole('button', { name: 'Strong copyleft, 7 packages' }))
    await userEvent.selectOptions(screen.getByRole('combobox', { name: 'Tag' }), 'latest-dev')
    expect(await screen.findByRole('button', { name: 'Strong copyleft, 24 packages' })).toHaveAttribute('aria-pressed', 'false')
    expect(await total()).toBe(76)
  })

  it('LB-13.2 toggles a focused category with Enter and Space', async () => {
    mockPython()
    renderPackages()
    const strong = await screen.findByRole('button', { name: 'Strong copyleft, 7 packages' })
    strong.focus()
    await userEvent.keyboard('{Enter}')
    expect(strong).toHaveAttribute('aria-pressed', 'true')
    await userEvent.keyboard(' ')
    expect(strong).toHaveAttribute('aria-pressed', 'false')
  })

  it('LB-5.1 shows the full WITH expression in the table', async () => {
    mockPython()
    renderPackages()
    await categoryButtons()
    const row = screen.getByRole('cell', { name: 'libgcc' }).closest('tr')!
    expect(within(row).getByText('GPL-3.0-or-later WITH GCC-exception-3.1')).toBeInTheDocument()
  })
})
