import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import type { AdvisoryReport, VulnReport } from '../api'
import { pythonTags } from '../test/fixtures/python'
import { pythonAdvisoriesLatest, pythonAdvisoriesLatestDev } from '../test/fixtures/pythonAdvisories'
import { pythonVulnsLatest, pythonVulnsLatestDev } from '../test/fixtures/pythonVulns'
import { mockApi, renderAt } from '../test/render'
import { ImagePage } from './ImagePage'

type Mocks = {
  latest?: AdvisoryReport | { status: number; body: unknown }
  extra?: Parameters<typeof mockApi>[0]
}

function mockPython({ latest = pythonAdvisoriesLatest, extra = {} }: Mocks = {}) {
  const advisory = 'status' in latest ? { status: latest.status, body: latest.body } : { body: latest }
  return mockApi({
    ...extra,
    '/api/images/python/tags': { body: pythonTags },
    '/api/images/python/vulnerabilities': [
      ...((extra['/api/images/python/vulnerabilities'] as never[]) ?? []),
      { query: { tag: 'latest' }, body: pythonVulnsLatest },
      { query: { tag: 'latest-dev' }, body: pythonVulnsLatestDev },
    ],
    '/api/images/python/advisories': [
      ...((extra['/api/images/python/advisories'] as never[]) ?? []),
      { query: { tag: 'latest' }, ...advisory },
      { query: { tag: 'latest-dev' }, body: pythonAdvisoriesLatestDev },
    ],
  })
}

const renderSecurity = () => renderAt('/images/python?tab=security', '/images/:name', <ImagePage />)
const section = async () => within(await screen.findByRole('region', { name: 'Open advisories' }))
const rowFor = async (id: string) => {
  const s = await section()
  const link = await s.findByRole('link', { name: id })
  return { link, row: within(link.closest('li')!) }
}

describe('OpenAdvisories', () => {
  it('OA-6.1 shows CVE-2026-12345 as not yet assessed, with its package, advisories and a link', async () => {
    mockPython()
    renderSecurity()
    const { link, row } = await rowFor('CVE-2026-12345')
    expect(link).toHaveAttribute('href', 'https://osv.dev/vulnerability/CVE-2026-12345')
    expect(row.getAllByText('Not yet assessed')[0]).toHaveClass('ring-1') // the badge
    const li = row.getByText('python-3.15').closest('li')!
    expect(within(li).getByText('3.15.0_git20261010-r0')).toBeInTheDocument()
    expect(within(li).getByText('CGA-2w26-qvh7-3jhg, CGA-mp3h-w2rr-vq2f')).toBeInTheDocument()
  })

  it('OA-6.2 puts the 17 vulnerabilities waiting on an upstream fix before the 4 not yet assessed', async () => {
    mockPython()
    renderAt('/images/python?tab=security&tag=latest-dev', '/images/:name', <ImagePage />)
    const s = await section()
    await s.findByRole('link', { name: 'CVE-2026-6844' })
    const rows = s.getAllByRole('link').map((l) => ({ id: l.textContent!, status: l.closest('li')!.querySelector('.ring-1')!.textContent }))
    expect(rows).toHaveLength(21)
    expect(rows.slice(0, 17).every((r) => r.status === 'Waiting on an upstream fix')).toBe(true)
    expect(rows.slice(17).map((r) => r.id)).toEqual(['CVE-2025-15367', 'CVE-2025-47273', 'CVE-2026-12345', 'CVE-2026-21895'])
    const ids = rows.slice(0, 17).map((r) => r.id)
    expect(ids).toEqual([...ids].sort())
  })

  it('OA-7.1 names the Rust crate in uv', async () => {
    mockPython()
    renderAt('/images/python?tab=security&tag=latest-dev', '/images/:name', <ImagePage />)
    const { row } = await rowFor('CVE-2026-21895')
    const li = row.getByText('uv').closest('li')!
    expect(within(li).getByText('Found in rsa (Rust crate).')).toBeInTheDocument()
  })

  it('OA-7.2 names the bundled Python package in py3.15-pip-base', async () => {
    mockPython()
    renderAt('/images/python?tab=security&tag=latest-dev', '/images/:name', <ImagePage />)
    const { row } = await rowFor('CVE-2025-47273')
    const li = row.getByText('py3.15-pip-base').closest('li')!
    expect(within(li).getByText('Found in setuptools (Python package).')).toBeInTheDocument()
  })

  it('OA-8.1 says whose advisories these are, via OSV.dev, and when they were checked', async () => {
    mockPython()
    renderSecurity()
    const s = await section()
    expect(
      await s.findByText(/Chainguard's published advisories, found via OSV\.dev with their status from Chainguard's advisory feed, checked 2026-10-11 19:48 UTC/),
    ).toBeInTheDocument()
  })

  it('OA-5.1 shows the total and the count for each status that has any', async () => {
    mockPython()
    renderAt('/images/python?tab=security&tag=latest-dev', '/images/:name', <ImagePage />)
    const s = await section()
    await s.findByText('Open advisories', { selector: 'div' })
    const stat = (label: string) => s.getByText(label, { selector: 'div' }).previousElementSibling?.textContent
    expect(stat('Open advisories')).toBe('21')
    expect(stat('Waiting on an upstream fix')).toBe('17')
    expect(stat('Not yet assessed')).toBe('4')
    expect(s.getByText(/0 fix available, 0 confirmed, 0 fix not planned\./)).toBeInTheDocument()
  })

  it('OA-8.2 says no advisories are published when the total is 0, without claiming the image is clean', async () => {
    const zero: AdvisoryReport = {
      ...pythonAdvisoriesLatest,
      total: 0,
      counts: { fix_available: 0, pending_upstream_fix: 0, confirmed: 0, fix_not_planned: 0, not_yet_assessed: 0 },
      vulnerabilities: [],
      note: "No open advisories are published for this image's packages as of 2026-10-11 19:14 UTC. This covers Chainguard's published advisories only; it is not a full scan.",
    }
    mockPython({ latest: zero })
    renderSecurity()
    const s = await section()
    expect(await s.findByText(/No open advisories are published for this image's packages as of 2026-10-11 19:14 UTC/)).toBeInTheDocument()
    expect(document.body.textContent).not.toMatch(/vulnerability-free|no vulnerabilities|secure/i)
  })

  it('OA-8.3 explains what not yet assessed means', async () => {
    mockPython()
    renderSecurity()
    const s = await section()
    expect(await s.findByText(/detected but not yet assessed by Chainguard; some may turn out not to apply/)).toBeInTheDocument()
  })

  it('OA-13.1 shows an error with a retry and no counts while the fixes still show', async () => {
    mockPython({ latest: { status: 502, body: { error: 'OSV.dev is down', code: 'source_failed' } } })
    renderSecurity()
    const s = await section()
    expect(await s.findByRole('alert')).toHaveTextContent('OSV.dev is down')
    expect(s.getByRole('button', { name: 'Try again' })).toBeInTheDocument()
    expect(s.queryByText('Open advisories', { selector: 'div' })).toBeNull()
    expect(await screen.findByRole('region', { name: 'Fixes per source package' })).toBeInTheDocument()
  })

  it('OA-13.2 shows the subscription message and no counts for a subscription-only image', async () => {
    mockApi({
      '/api/images/loki-fips/tags': { status: 403, body: { error: 'not public', code: 'not_public' } },
      '/api/images/loki-fips/advisories': { status: 403, body: { error: 'not public', code: 'not_public' } },
    })
    renderAt('/images/loki-fips?tab=security', '/images/:name', <ImagePage />)
    expect(await screen.findByText('This image requires a Chainguard subscription')).toBeInTheDocument()
    expect(screen.queryByText('Open advisories', { selector: 'div' })).toBeNull()
  })

  it('OA-13.3 reloads for latest-dev when the tag changes', async () => {
    mockPython()
    renderSecurity()
    const s = await section()
    await s.findByRole('link', { name: 'CVE-2026-12345' })
    expect(s.getAllByRole('link')).toHaveLength(2)
    await userEvent.selectOptions(screen.getByRole('combobox', { name: 'Tag' }), 'latest-dev')
    await waitFor(() => expect(s.getAllByRole('link')).toHaveLength(21))
    const stat = (label: string) => s.getByText(label, { selector: 'div' }).previousElementSibling?.textContent
    expect(stat('Open advisories')).toBe('21')
  })

  it('OA-13.4 shows a wait of a few seconds with the expected time', async () => {
    mockApi({
      '/api/images/python/tags': { body: pythonTags },
      '/api/images/python/vulnerabilities': { body: pythonVulnsLatest },
      '/api/images/python/advisories': { pending: true },
    })
    renderSecurity()
    const s = await section()
    expect(await s.findByText(/usually takes a few seconds/)).toBeInTheDocument()
  })
})

describe('Lookup with open advisories', () => {
  const lookup = async (id: string) => {
    await userEvent.type(await screen.findByRole('textbox', { name: 'Vulnerability ID' }), id)
    await userEvent.click(screen.getByRole('button', { name: 'Check' }))
  }
  const noFix = (id: string): VulnReport => ({ ...pythonVulnsLatest, id, packages: undefined, matches: undefined })

  it('OA-9.1 says CVE-2026-12345 is an open advisory, not yet assessed, for python-3.15', async () => {
    const match = pythonAdvisoriesLatest.vulnerabilities.find((v) => v.id === 'CVE-2026-12345')!
    mockPython({
      extra: {
        '/api/images/python/vulnerabilities': [{ query: { id: 'CVE-2026-12345' }, body: noFix('CVE-2026-12345') }],
        '/api/images/python/advisories': [{ query: { id: 'CVE-2026-12345' }, body: { ...pythonAdvisoriesLatest, id: 'CVE-2026-12345', match } }],
      },
    })
    renderSecurity()
    await lookup('cve-2026-12345')
    const heading = await screen.findByRole('heading', { name: 'Open advisory' })
    const block = within(heading.parentElement!)
    expect((await block.findAllByText('Not yet assessed'))[0]).toHaveClass('ring-1') // the badge
    expect(block.getByText('python-3.15')).toBeInTheDocument()
  })

  it('OA-9.4 says no open advisory is published for an advisory the feed has as fixed, with the recorded fix beside it', async () => {
    mockPython({
      extra: {
        '/api/images/python/vulnerabilities': [
          {
            query: { id: 'CVE-2026-89092' },
            body: {
              ...pythonVulnsLatest,
              id: 'CVE-2026-89092',
              packages: undefined,
              matches: [{ package: 'glibc-2.44', installed_version: '2.44-r8', status: 'fixed', fixed_version: '2.44-r8', subpackages: ['glibc-2.44'] }],
            },
          },
        ],
        '/api/images/python/advisories': [
          {
            query: { id: 'CVE-2026-89092' },
            body: {
              ...pythonAdvisoriesLatest,
              id: 'CVE-2026-89092',
              lookup_note: "No open advisory for CVE-2026-89092 is published for this image's packages as of 2026-10-11 19:48 UTC. This covers Chainguard's published advisories only; it is not a full scan.",
            },
          },
        ],
      },
    })
    renderSecurity()
    await lookup('CVE-2026-89092')
    expect(await screen.findByText(/No open advisory for CVE-2026-89092 is published/)).toBeInTheDocument()
    expect(screen.getByText('Fixed')).toBeInTheDocument()
    expect(screen.queryByText('Waiting on an upstream fix')).toBeNull()
  })

  it('OA-9.3 says no open advisory is published for an ID without one, without calling the image unaffected', async () => {
    mockPython({
      extra: {
        '/api/images/python/vulnerabilities': [{ query: { id: 'CVE-2020-0001' }, body: noFix('CVE-2020-0001') }],
        '/api/images/python/advisories': [
          {
            query: { id: 'CVE-2020-0001' },
            body: {
              ...pythonAdvisoriesLatest,
              id: 'CVE-2020-0001',
              lookup_note:
                "No open advisory for CVE-2020-0001 is published for this image's packages as of 2026-10-11 19:14 UTC. This covers Chainguard's published advisories only; it is not a full scan.",
            },
          },
        ],
      },
    })
    renderSecurity()
    await lookup('CVE-2020-0001')
    expect(await screen.findByText(/No open advisory for CVE-2020-0001 is published/)).toBeInTheDocument()
    expect(document.body.textContent).not.toMatch(/unaffected|vulnerability-free|no vulnerabilities|secure/i)
  })
})
