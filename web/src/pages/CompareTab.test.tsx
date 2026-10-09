import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useLocation } from 'react-router'
import { describe, expect, it, type Mock } from 'vitest'
import type { ImageDetails, ImagePackages } from '../api'
import { pythonDetails, pythonPackages, pythonTags } from '../test/fixtures/python'
import { mockApi, renderAt } from '../test/render'
import { ImagePage } from './ImagePage'

function LocationProbe() {
  return <span data-testid="location">{useLocation().search}</span>
}

function renderImage(path: string) {
  return renderAt(
    path,
    '/images/:name',
    <>
      <ImagePage />
      <LocationProbe />
    </>,
  )
}

const search = () => new URLSearchParams(screen.getByTestId('location').textContent ?? '')

const requested = (fetchMock: Mock, path: string) =>
  fetchMock.mock.calls.map(([url]) => new URL(String(url), 'http://localhost')).filter((u) => u.pathname.endsWith(path))

function mockPython(overrides: Parameters<typeof mockApi>[0] = {}) {
  return mockApi({
    '/api/images/python/tags': { body: pythonTags },
    '/api/images/python/details': (['latest', 'latest-dev'] as const).map((tag) => ({ query: { tag }, body: pythonDetails[tag] })),
    '/api/images/python/packages': (['latest', 'latest-dev'] as const).flatMap((tag) =>
      (['amd64', 'arm64'] as const).map((arch) => ({ query: { tag, arch }, body: pythonPackages[tag][arch] })),
    ),
    ...overrides,
  })
}

/** Mocks an image whose tags each have one package named after the tag and a size of 10 MB per position. */
function mockImage(name: string, tags: string[]) {
  const details = (tag: string): ImageDetails => ({
    image: name,
    tag,
    reference: `cgr.dev/chainguard/${name}:${tag}`,
    pinned_reference: `cgr.dev/chainguard/${name}:${tag}@sha256:0`,
    digest: 'sha256:0',
    platforms: ['amd64', 'arm64'].map((arch) => ({
      platform: `linux/${arch}`,
      digest: `sha256:${arch}`,
      size_bytes: (tags.indexOf(tag) + 1) * 10_000_000,
    })),
    user: '65532',
    runs_as_root: false,
    config_platform: 'linux/amd64',
  })
  const packages = (tag: string, arch: string): ImagePackages => ({
    image: name,
    tag,
    arch,
    digest: 'sha256:0',
    has_shell: tag.endsWith('-dev'),
    has_apk: tag.endsWith('-dev'),
    total: 1,
    packages: [{ name: `pkg-${tag}`, version: '1.0-r0', origin: `pkg-${tag}`, distro: 'wolfi' }],
  })
  return mockApi({
    [`/api/images/${name}/tags`]: { body: { image: name, reference: `cgr.dev/chainguard/${name}`, tags } },
    [`/api/images/${name}/details`]: tags.map((tag) => ({ query: { tag }, body: details(tag) })),
    [`/api/images/${name}/packages`]: tags.flatMap((tag) =>
      ['amd64', 'arm64'].map((arch) => ({ query: { tag, arch }, body: packages(tag, arch) })),
    ),
  })
}

const firstTag = () => screen.getByRole('combobox', { name: 'First tag' })
const secondTag = () => screen.getByRole('combobox', { name: 'Second tag' })
const options = (select: HTMLElement) => within(select).getAllByRole('option').map((o) => o.getAttribute('value'))
const rowFor = (tag: string) => screen.getByRole('rowheader', { name: tag }).closest('tr') as HTMLElement

describe('Compare tab', () => {
  it('appears as a tab, activates from the URL and hides the page tag selector', async () => {
    mockPython()
    renderImage('/images/python?tab=compare')

    expect(await screen.findByRole('tab', { name: 'Compare' })).toHaveAttribute('aria-selected', 'true')
    expect(screen.queryByRole('combobox', { name: 'Tag' })).not.toBeInTheDocument()
    expect(firstTag()).toBeInTheDocument()
  })

  it('VC-1.1 compares latest with latest-dev by default', async () => {
    mockPython()
    renderImage('/images/python?tab=compare')

    expect(await screen.findByText(/latest-dev is 10\.1× larger/)).toBeInTheDocument()
    expect(firstTag()).toHaveValue('latest')
    expect(secondTag()).toHaveValue('latest-dev')
  })

  it('VC-1.2 pairs a newly chosen first tag with its -dev variant', async () => {
    mockImage('demo', ['latest', 'latest-dev', 'next', 'next-dev'])
    renderImage('/images/demo?tab=compare')
    await screen.findByText(/latest-dev is/)

    await userEvent.setup().selectOptions(firstTag(), 'next')

    expect(firstTag()).toHaveValue('next')
    expect(secondTag()).toHaveValue('next-dev')
    expect(search().get('a')).toBe('next')
    expect(search().get('b')).toBe('next-dev')
    expect(await screen.findByText('Only in next-dev (1)')).toBeInTheDocument()
  })

  it('VC-2.1 compares node latest with latest-slim', async () => {
    const fetchMock = mockImage('node', ['latest', 'latest-dev', 'latest-slim'])
    renderImage('/images/node?tab=compare')
    await screen.findByText(/latest-dev is/)

    await userEvent.setup().selectOptions(secondTag(), 'latest-slim')

    expect(firstTag()).toHaveValue('latest')
    expect(secondTag()).toHaveValue('latest-slim')
    expect(await screen.findByText('Only in latest-slim (1)')).toBeInTheDocument()
    expect(requested(fetchMock, '/packages').map((u) => u.searchParams.get('tag'))).toContain('latest-slim')
  })

  it('VC-2.2 never offers the first tag as the second', async () => {
    mockImage('node', ['latest', 'latest-dev', 'latest-slim'])
    renderImage('/images/node?tab=compare')
    await screen.findByText(/latest-dev is/)

    expect(options(secondTag())).toEqual(['latest-dev', 'latest-slim'])
    await userEvent.setup().selectOptions(firstTag(), 'latest-slim')
    expect(options(secondTag())).not.toContain('latest-slim')
    expect(secondTag()).not.toHaveValue('latest-slim')
  })

  it('VC-3.1 explains that static has no -dev variant, and keeps the selectors', async () => {
    const fetchMock = mockImage('static', ['latest', 'latest-glibc', 'latest-glibc-tzdata', 'latest-tzdata'])
    const { container } = renderImage('/images/static?tab=compare')

    expect(await screen.findByText('This image has no -dev variant to compare against.')).toBeInTheDocument()
    expect(firstTag()).toBeInTheDocument()
    expect(secondTag()).toBeInTheDocument()
    expect(container.querySelector('svg')).toBeNull()
    expect(requested(fetchMock, '/packages')).toHaveLength(0)

    await userEvent.setup().selectOptions(firstTag(), 'latest-glibc')
    expect(await screen.findByText('Only in latest-glibc (1)')).toBeInTheDocument()
  })

  it('VC-3.2 shows the message without selectors for a single-tag image', async () => {
    mockImage('solo', ['latest'])
    renderImage('/images/solo?tab=compare')

    expect(await screen.findByText('This image has no -dev variant to compare against.')).toBeInTheDocument()
    expect(screen.queryByRole('combobox')).not.toBeInTheDocument()
  })

  it('VC-4.1 loads the comparison from the URL and keeps changes there', async () => {
    const fetchMock = mockImage('node', ['latest', 'latest-dev', 'latest-slim'])
    renderImage('/images/node?tab=compare&a=latest&b=latest-slim&arch=arm64')

    expect(await screen.findByText('Only in latest-slim (1)')).toBeInTheDocument()
    expect(firstTag()).toHaveValue('latest')
    expect(secondTag()).toHaveValue('latest-slim')
    expect(screen.getByRole('radio', { name: 'arm64' })).toBeChecked()
    expect(requested(fetchMock, '/packages').every((u) => u.searchParams.get('arch') === 'arm64')).toBe(true)

    await userEvent.setup().selectOptions(secondTag(), 'latest-dev')
    expect(search().get('b')).toBe('latest-dev')
    expect(search().get('arch')).toBe('arm64')
  })

  it.each([
    ['an unknown tag', 'a=nope&b=latest'],
    ['the same tag twice', 'a=latest&b=latest'],
  ])('falls back to the default pair for %s in the URL', async (_, query) => {
    mockPython()
    renderImage(`/images/python?tab=compare&${query}`)

    expect(await screen.findByText(/latest-dev is 10\.1× larger/)).toBeInTheDocument()
    expect(firstTag()).toHaveValue('latest')
    expect(secondTag()).toHaveValue('latest-dev')
  })

  it('VC-5.1 shows each tag’s package count and download size', async () => {
    mockPython()
    renderImage('/images/python?tab=compare')
    await screen.findByText(/latest-dev is/)

    expect(within(rowFor('latest')).getByText('29')).toBeInTheDocument()
    expect(within(rowFor('latest')).getByText('26.9 MB')).toBeInTheDocument()
    expect(within(rowFor('latest-dev')).getByText('76')).toBeInTheDocument()
    expect(within(rowFor('latest-dev')).getByText('272.6 MB')).toBeInTheDocument()
  })

  it('VC-6.1 states the size difference as a ratio and an amount', async () => {
    mockPython()
    renderImage('/images/python?tab=compare')

    expect(await screen.findByText('latest-dev is 10.1× larger than latest (+245.7 MB) on amd64.')).toBeInTheDocument()
  })

  it('VC-7.1 lists the packages only in latest-dev', async () => {
    mockPython()
    renderImage('/images/python?tab=compare')

    const heading = await screen.findByRole('heading', { name: 'Only in latest-dev (47)' })
    const group = heading.closest('section') as HTMLElement
    for (const name of ['gcc', 'bash', 'git', 'make']) expect(within(group).getByText(name)).toBeInTheDocument()
  })

  it('VC-7.2 says None for empty groups', async () => {
    mockPython()
    renderImage('/images/python?tab=compare')

    const onlyInLatest = (await screen.findByRole('heading', { name: 'Only in latest (0)' })).closest('section') as HTMLElement
    expect(within(onlyInLatest).getByText('None')).toBeInTheDocument()
    const changed = screen.getByRole('heading', { name: 'Different versions (0)' }).closest('section') as HTMLElement
    expect(within(changed).getByText('None')).toBeInTheDocument()
  })

  it('VC-8.1 shows whether each tag has a shell and apk', async () => {
    mockPython()
    renderImage('/images/python?tab=compare')
    await screen.findByText(/latest-dev is/)

    expect(within(rowFor('latest')).getAllByText('No')).toHaveLength(2)
    expect(within(rowFor('latest-dev')).getAllByText('Yes')).toHaveLength(2)
  })

  it('VC-13.1 presents sizes, counts and the ratio as text outside the chart', async () => {
    mockPython()
    renderImage('/images/python?tab=compare')
    await screen.findByText(/latest-dev is/)

    const chart = document.querySelector('[aria-hidden="true"] svg')?.closest('[aria-hidden="true"]') as HTMLElement
    expect(chart).not.toBeNull()
    chart.remove()
    const text = document.body.textContent ?? ''
    for (const value of ['26.9 MB', '272.6 MB', '29', '76', '10.1×']) expect(text).toContain(value)
  })

  it('VC-10.1 switches every number to arm64 without refetching details', async () => {
    const fetchMock = mockPython()
    renderImage('/images/python?tab=compare')
    await screen.findByText(/on amd64/)
    const detailsRequests = requested(fetchMock, '/details').length

    await userEvent.setup().click(screen.getByRole('radio', { name: 'arm64' }))

    expect(await screen.findByText(/on arm64/)).toBeInTheDocument()
    expect(within(rowFor('latest')).getByText('25.2 MB')).toBeInTheDocument()
    expect(within(rowFor('latest-dev')).getByText('256.4 MB')).toBeInTheDocument()
    const arm64 = requested(fetchMock, '/packages').filter((u) => u.searchParams.get('arch') === 'arm64')
    expect(arm64.map((u) => u.searchParams.get('tag')).sort()).toEqual(['latest', 'latest-dev'])
    expect(requested(fetchMock, '/details')).toHaveLength(detailsRequests)
  })

  it('VC-12.1 shows a failed side’s error with a retry and no partial comparison', async () => {
    const fetchMock = mockPython({
      '/api/images/python/packages': [
        { query: { tag: 'latest', arch: 'amd64' }, body: pythonPackages.latest.amd64 },
        { query: { tag: 'latest-dev' }, status: 502, body: { error: 'registry unavailable', code: 'upstream_error' } },
      ],
    })
    renderImage('/images/python?tab=compare')

    expect(await screen.findByRole('alert')).toHaveTextContent('registry unavailable')
    expect(screen.queryByText(/Only in/)).not.toBeInTheDocument()
    expect(screen.queryByRole('table')).not.toBeInTheDocument()

    const before = requested(fetchMock, '/packages').length
    await userEvent.setup().click(screen.getByRole('button', { name: 'Try again' }))
    await expect.poll(() => requested(fetchMock, '/packages').length).toBe(before + 1)
    expect(requested(fetchMock, '/packages').at(-1)?.searchParams.get('tag')).toBe('latest-dev')
  })

  it('VC-12.2 shows a loading indicator while a side is loading', async () => {
    mockPython({ '/api/images/python/packages': { pending: true } })
    renderImage('/images/python?tab=compare')

    expect(await screen.findByText(/Reading both images/)).toBeInTheDocument()
    expect(screen.getByRole('status')).toHaveTextContent(/Reading both images/)
    expect(screen.queryByRole('table')).not.toBeInTheDocument()
  })

  it('VC-11.1 shows the subscription message and makes no comparison requests', async () => {
    const fetchMock = mockApi({
      '/api/images/loki-fips/tags': { status: 403, body: { error: 'not public', code: 'not_public' } },
    })
    renderImage('/images/loki-fips?tab=compare')

    expect(await screen.findByText('This image requires a Chainguard subscription')).toBeInTheDocument()
    expect(requested(fetchMock, '/details')).toHaveLength(0)
    expect(requested(fetchMock, '/packages')).toHaveLength(0)
  })

  it('names a tag that lacks the selected architecture', async () => {
    mockPython({
      '/api/images/python/packages': [
        { query: { tag: 'latest', arch: 'arm64' }, body: pythonPackages.latest.arm64 },
        { query: { tag: 'latest-dev', arch: 'arm64' }, status: 404, body: { error: 'no arm64 manifest', code: 'not_found' } },
      ],
    })
    renderImage('/images/python?tab=compare&arch=arm64')

    expect(await screen.findByRole('alert')).toHaveTextContent('latest-dev has no arm64 variant.')
  })
})
