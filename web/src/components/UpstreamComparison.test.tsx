import { focusManager } from '@tanstack/react-query'
import { cleanup, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import type { CompareJob, UpstreamComparison as Comparison } from '../api'
import { AlternativesPage } from '../pages/AlternativesPage'
import { mockApi, renderAt } from '../test/render'
import { UpstreamComparison } from './UpstreamComparison'

// node:20 on 2026-10-11, as measured for the spec.
const node20: Comparison = {
  input: 'node:20',
  upstream: {
    reference: 'docker.io/library/node:20',
    digest: 'sha256:aaaa',
    platform: 'linux/amd64',
    created: '2026-04-22T03:00:00Z',
    size_bytes: 398_400_000,
    sbom_published: true,
    packages_checked: true,
    os_packages: 393,
    language_packages: [{ ecosystem: 'npm', count: 193 }],
    vulnerabilities: { total: 833, fix_available: 295, no_fix_yet: 325, wont_fix: 213 },
  },
  chainguard: {
    reference: 'cgr.dev/chainguard/node:latest',
    digest: 'sha256:bbbb',
    platform: 'linux/amd64',
    created: '2026-10-09T01:00:00Z',
    size_bytes: 65_900_000,
    sbom_published: true,
    packages_checked: true,
    os_packages: 28,
    vulnerabilities: { total: 22, fix_available: 0, no_fix_yet: 22, wont_fix: 0 },
  },
  end_of_life: { product: 'Node.js', cycle: '20', date: '2026-04-30' },
  source: "Known vulnerabilities per each distribution's published advisories, via OSV.dev.",
  checked_at: '2026-10-11T12:00:00Z',
  notes: ['Only installed OS packages are compared.'],
}

const done = (c: Comparison): CompareJob => ({ status: 'done', checked: 550, total: 550, ...c })

function renderSection(opts: { arch?: string; onArch?: (a: string) => void } = {}) {
  return renderAt(
    '/',
    '/',
    <UpstreamComparison image="node:20" chainguardImage="node" arch={opts.arch ?? 'amd64'} onArch={opts.onArch ?? (() => {})} pollMs={10} />,
  )
}

describe('UpstreamComparison', () => {
  it('UC-6.1 leads with fix-available and package counts', async () => {
    mockApi({ '/api/compare-upstream': { body: done(node20) } })
    renderSection()

    expect(
      await screen.findByText(
        "node:20 has 295 known vulnerabilities with a fix available; Chainguard's node has 0. It installs 393 OS packages against 28.",
      ),
    ).toBeInTheDocument()

    // Every number is in the table as text (UC-14).
    const table = screen.getByRole('table')
    const row = (name: string) => within(within(table).getByRole('rowheader', { name })).getByText(name).closest('tr')!
    expect(within(row('OS packages')).getAllByRole('cell').map((c) => c.textContent)).toEqual(['393', '28'])
    expect(within(row('Known vulnerabilities')).getAllByRole('cell').map((c) => c.textContent)).toEqual(['833', '22'])
    expect(within(row('Fix available')).getAllByRole('cell').map((c) => c.textContent)).toEqual(['295', '0'])
    expect(within(row('No fix yet')).getAllByRole('cell').map((c) => c.textContent)).toEqual(['325', '22'])
    expect(within(row("Won't fix")).getAllByRole('cell').map((c) => c.textContent)).toEqual(['213', '0'])
    expect(within(row('Download size')).getAllByRole('cell').map((c) => c.textContent)).toEqual(['398.4 MB', '65.9 MB'])
    expect(within(row('Built')).getAllByRole('cell').map((c) => c.textContent)).toEqual(['2026-04-22', '2026-10-09'])
    expect(within(row('Language packages (not compared)')).getAllByRole('cell').map((c) => c.textContent)).toEqual(['193 npm', 'None'])
  })

  it('UC-6.2 reads zero as "0 known vulnerabilities" with the source and date, and never claims security', async () => {
    const zero = structuredClone(node20)
    zero.chainguard.vulnerabilities = { total: 0, fix_available: 0, no_fix_yet: 0, wont_fix: 0 }
    mockApi({ '/api/compare-upstream': { body: done(zero) } })
    renderSection()

    expect(await screen.findByText(/Chainguard's node: 0 known vulnerabilities \(OSV\.dev, checked 2026-10-11\)/)).toBeInTheDocument()
    expect(screen.getByText(/via OSV\.dev/)).toBeInTheDocument()
    expect(document.body.textContent).not.toMatch(/vulnerability-free|secure\b|safe/i)
  })

  it('UC-8.1 notes that the upstream release is past its end of life', async () => {
    mockApi({ '/api/compare-upstream': { body: done(node20) } })
    renderSection()
    expect(
      await screen.findByText('Node.js 20 reached end of life on 2026-04-30 and may no longer receive security updates.'),
    ).toBeInTheDocument()
  })

  it('UC-8.2 shows no end-of-life note when the server sends none', async () => {
    const noEol: Comparison = { ...node20, end_of_life: undefined }
    mockApi({ '/api/compare-upstream': { body: done(noEol) } })
    renderSection()
    await screen.findByRole('table')
    expect(screen.queryByText(/end of life/)).not.toBeInTheDocument()
  })

  it('UC-2.2 explains an upstream image without an SBOM and still shows its size and build date', async () => {
    const noSbom: Comparison = structuredClone(node20)
    noSbom.input = 'bitnami/redis:7.2'
    noSbom.upstream = { ...noSbom.upstream, reference: 'docker.io/bitnami/redis:7.2', sbom_published: false, packages_checked: false, unchecked_reason: 'no_sbom', os_packages: 0, language_packages: undefined, vulnerabilities: undefined }
    mockApi({ '/api/compare-upstream': { body: done(noSbom) } })
    renderSection()

    expect(await screen.findByText(/No SBOM published for docker\.io\/bitnami\/redis:7\.2/)).toBeInTheDocument()
    const table = screen.getByRole('table')
    expect(within(table).getByText('398.4 MB')).toBeInTheDocument()
    expect(within(table).getByText('2026-04-22')).toBeInTheDocument()
    // No counts for the upstream side, and no summary comparing them.
    expect(screen.queryByText(/with a fix available;/)).not.toBeInTheDocument()
    expect(screen.queryByText('393')).not.toBeInTheDocument()
  })

  it.each([
    ['UC-15.1', 'no_os_packages', 'busybox:latest', /lists no distribution packages \(it may be statically built\)/],
    ['UC-15.2', 'unsupported_distribution', 'fedora:40', /come from a distribution with no vulnerability data here/],
  ] as const)('%s never shows an unchecked upstream image as 0 known vulnerabilities', async (_, reason, input, message) => {
    const unchecked: Comparison = structuredClone(node20)
    unchecked.input = input
    unchecked.upstream = {
      ...unchecked.upstream,
      reference: `docker.io/library/${input}`,
      packages_checked: false,
      unchecked_reason: reason,
      os_packages: 0,
      vulnerabilities: undefined,
    }
    mockApi({ '/api/compare-upstream': { body: done(unchecked) } })
    renderSection()

    expect(await screen.findByText(message)).toBeInTheDocument()
    expect(screen.getByText(/That is not the same as having none\./)).toBeInTheDocument()
    // Dashes, not zeros, on the upstream side; the Chainguard side is unchanged.
    const table = screen.getByRole('table')
    const row = (name: string) => within(within(table).getByRole('rowheader', { name })).getByText(name).closest('tr')!
    expect(within(row('OS packages')).getAllByRole('cell').map((c) => c.textContent)).toEqual(['—', '28'])
    expect(within(row('Known vulnerabilities')).getAllByRole('cell').map((c) => c.textContent)).toEqual(['—', '22'])
    expect(screen.queryByText(/with a fix available;/)).not.toBeInTheDocument()
    expect(screen.queryByText(new RegExp(`${input}: 0 known`))).not.toBeInTheDocument()
  })

  it('UC-1.1 lets the user choose the architecture', async () => {
    const onArch = vi.fn()
    mockApi({ '/api/compare-upstream': { body: done(node20) } })
    renderSection({ onArch })
    await screen.findByRole('table')

    expect(screen.getByRole('button', { name: 'amd64' })).toHaveAttribute('aria-pressed', 'true')
    await userEvent.setup().click(screen.getByRole('button', { name: 'arm64' }))
    expect(onArch).toHaveBeenCalledWith('arm64')
  })

  it('UC-1.1 asks the server for the chosen architecture', async () => {
    const fetchMock = mockApi({
      '/api/alternatives': {
        body: {
          input: 'node:20',
          recommended: { image: 'node', reference: 'cgr.dev/chainguard/node', free: true },
          alternatives: [],
          notes: [],
        },
      },
      '/api/compare-upstream': [
        { query: { arch: 'arm64' }, body: done({ ...node20, upstream: { ...node20.upstream, platform: 'linux/arm64' } }) },
        { query: { arch: 'amd64' }, body: done(node20) },
      ],
    })
    renderAt('/alternatives?image=node:20', '/alternatives', <AlternativesPage />)
    await screen.findByRole('table')

    await userEvent.setup().click(screen.getByRole('button', { name: 'arm64' }))
    await waitFor(() => expect(screen.getByRole('table', { name: /linux\/arm64/ })).toBeInTheDocument())
    const urls = fetchMock.mock.calls.map((c) => String(c[0]))
    expect(urls.some((u) => u.includes('/api/compare-upstream') && u.includes('arch=arm64'))).toBe(true)
  })

  it('UC-9.1 shows packages checked out of the total while it runs, then the comparison', async () => {
    const responses: CompareJob[] = [
      { status: 'running', checked: 0, total: 0 },
      { status: 'running', checked: 210, total: 550 },
      done(node20),
    ]
    let calls = 0
    const fetchMock = vi.fn(async () => {
      const body = responses[Math.min(calls++, responses.length - 1)]
      return new Response(JSON.stringify(body), { status: 200, headers: { 'Content-Type': 'application/json' } })
    })
    vi.stubGlobal('fetch', fetchMock)
    renderSection()

    // Before the packages are known, the expected time is given instead.
    expect(await screen.findByText(/about 20 seconds/)).toBeInTheDocument()
    const bar = await screen.findByRole('progressbar')
    expect(await screen.findByText(/210 of 550 packages checked/)).toBeInTheDocument()
    expect(bar).toHaveAttribute('aria-valuenow', '210')
    expect(bar).toHaveAttribute('aria-valuemax', '550')

    expect(await screen.findByRole('table')).toBeInTheDocument()
    expect(screen.queryByRole('progressbar')).not.toBeInTheDocument()

    // Polling stops once it is done.
    const after = fetchMock.mock.calls.length
    await new Promise((r) => setTimeout(r, 100))
    expect(fetchMock.mock.calls.length).toBe(after)
  })

  it('UC-9.1 keeps polling while the tab is in the background', async () => {
    const responses: CompareJob[] = [{ status: 'running', checked: 0, total: 0 }, { status: 'running', checked: 210, total: 550 }, done(node20)]
    let calls = 0
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => new Response(JSON.stringify(responses[Math.min(calls++, responses.length - 1)]), { status: 200, headers: { 'Content-Type': 'application/json' } })),
    )
    focusManager.setFocused(false)
    try {
      renderSection()
      expect(await screen.findByRole('table')).toBeInTheDocument()
    } finally {
      focusManager.setFocused(undefined)
    }
  })

  it('UC-9 stops polling when the page is left', async () => {
    const fetchMock = vi.fn(
      async () => new Response(JSON.stringify({ status: 'running', checked: 1, total: 550 }), { status: 200, headers: { 'Content-Type': 'application/json' } }),
    )
    vi.stubGlobal('fetch', fetchMock)
    renderSection()
    await screen.findByText(/1 of 550 packages checked/)
    await waitFor(() => expect(fetchMock.mock.calls.length).toBeGreaterThan(2))

    cleanup()
    const after = fetchMock.mock.calls.length
    await new Promise((r) => setTimeout(r, 100))
    expect(fetchMock.mock.calls.length).toBe(after)
  })

  it('UC-11.1 says Docker Hub rate-limited the request, and offers a retry', async () => {
    mockApi({ '/api/compare-upstream': { status: 429, body: { error: 'Docker Hub: registry rate limit reached', code: 'rate_limited' } } })
    renderSection()
    expect(await screen.findByText("Docker Hub's limit on anonymous requests was reached. Try again later.")).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Try again' })).toBeInTheDocument()
  })

  it('UC-11.2 names the failed source, shows no counts and retries', async () => {
    let fail = true
    const fetchMock = vi.fn(async () =>
      fail
        ? new Response(JSON.stringify({ error: 'OSV.dev failed: boom', code: 'source_failed', source: 'OSV.dev' }), { status: 502 })
        : new Response(JSON.stringify(done(node20)), { status: 200 }),
    )
    vi.stubGlobal('fetch', fetchMock)
    renderSection()

    expect(await screen.findByText("OSV.dev failed, so the comparison can't be shown. Try again.")).toBeInTheDocument()
    expect(screen.queryByRole('table')).not.toBeInTheDocument()
    expect(screen.queryByText(/known vulnerabilities/)).not.toBeInTheDocument()

    fail = false
    await userEvent.setup().click(screen.getByRole('button', { name: 'Try again' }))
    expect(await screen.findByRole('table')).toBeInTheDocument()
  })

  it('UC-2.3 says only Docker Hub images can be compared, with no retry', async () => {
    mockApi({
      '/api/compare-upstream': { status: 400, body: { error: 'only Docker Hub images can be compared', code: 'unsupported_registry' } },
    })
    renderSection()
    expect(await screen.findByText(/Only Docker Hub images can be compared/)).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Try again' })).not.toBeInTheDocument()
  })
})
