import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { mockApi, renderAt } from '../test/render'
import { AlternativesPage } from './AlternativesPage'
import { CatalogPage } from './CatalogPage'
import { ImagePage } from './ImagePage'

const imageList = {
  total: 2,
  count: 2,
  offset: 0,
  images: [
    { name: 'python', reference: 'cgr.dev/chainguard/python', free: true },
    { name: 'python-fips', reference: 'cgr.dev/chainguard/python-fips', free: false },
  ],
}

describe('CatalogPage', () => {
  it('lists images with their free-tier status', async () => {
    mockApi({ '/api/images': { body: imageList } })
    renderAt('/', '/', <CatalogPage />)

    const python = await screen.findByRole('link', { name: /^python cgr\.dev/ })
    expect(python).toHaveAttribute('href', '/images/python')
    expect(within(python).getByText('Free')).toBeInTheDocument()
    expect(within(screen.getByRole('link', { name: /python-fips/ })).getByText('Subscription')).toBeInTheDocument()
    expect(screen.getByText('2 images')).toBeInTheDocument()
  })

  it('sends the search and free-only filter to the API', async () => {
    const fetchMock = mockApi({ '/api/images': { body: imageList } })
    renderAt('/', '/', <CatalogPage />)
    await screen.findByText('2 images')

    const user = userEvent.setup()
    await user.type(screen.getByRole('searchbox', { name: 'Search images' }), 'py')
    await user.click(screen.getByRole('checkbox', { name: 'Free only' }))

    await expect.poll(() => fetchMock.mock.calls.map(([url]) => String(url))).toContainEqual(
      expect.stringMatching(/query=py.*free_only=true/),
    )
  })
})

describe('ImagePage', () => {
  it('explains when an image needs a subscription', async () => {
    mockApi({
      '/api/images/loki-fips/tags': {
        status: 403,
        body: { error: 'loki-fips: image is not publicly accessible', code: 'not_public' },
      },
    })
    renderAt('/images/loki-fips', '/images/:name', <ImagePage />)
    expect(await screen.findByText('This image requires a Chainguard subscription')).toBeInTheDocument()
  })

  it('shows the pinned reference and runtime user', async () => {
    mockApi({
      '/api/images/python/tags': { body: { image: 'python', reference: 'cgr.dev/chainguard/python', tags: ['latest', 'latest-dev'] } },
      '/api/images/python/details': {
        body: {
          image: 'python',
          tag: 'latest',
          reference: 'cgr.dev/chainguard/python:latest',
          pinned_reference: 'cgr.dev/chainguard/python:latest@sha256:abc',
          digest: 'sha256:abc',
          platforms: [{ platform: 'linux/amd64', digest: 'sha256:def0000000000000000000', size_bytes: 31457280 }],
          user: '65532',
          runs_as_root: false,
          entrypoint: ['/usr/bin/python'],
          config_platform: 'linux/amd64',
        },
      },
    })
    renderAt('/images/python', '/images/:name', <ImagePage />)

    expect(await screen.findByText('cgr.dev/chainguard/python:latest@sha256:abc')).toBeInTheDocument()
    expect(screen.getByText('non-root')).toBeInTheDocument()
    expect(screen.getByText('30 MB')).toBeInTheDocument()
    expect(screen.getByRole('tab', { name: 'Overview' })).toHaveAttribute('aria-selected', 'true')
  })

  it('looks up a CVE on the Security tab', async () => {
    mockApi({
      '/api/images/python/tags': { body: { image: 'python', reference: 'cgr.dev/chainguard/python', tags: ['latest'] } },
      '/api/images/python/vulnerabilities': {
        body: {
          image: 'python',
          tag: 'latest',
          digest: 'sha256:abc',
          id: 'CVE-2022-3602',
          matches: [
            {
              package: 'openssl-4.0',
              installed_version: '4.0.3-r5',
              status: 'fixed',
              fixed_version: '3.0.7-r0',
              subpackages: ['openssl-4.0-libssl'],
              records_from: 'openssl',
            },
          ],
          total_fixed: 334,
          total_not_affected: 51,
          note: '',
        },
      },
    })
    renderAt('/images/python?tab=security', '/images/:name', <ImagePage />)

    const user = userEvent.setup()
    await user.type(await screen.findByRole('textbox', { name: 'Vulnerability ID' }), 'cve-2022-3602')
    await user.click(screen.getByRole('button', { name: 'Check' }))

    expect(await screen.findByText('Fixed')).toBeInTheDocument()
    expect(screen.getByText(/Based on the security records for openssl/)).toBeInTheDocument()
    expect(screen.getByText(/doesn't list unfixed vulnerabilities/)).toBeInTheDocument()
  })
})

describe('AlternativesPage', () => {
  it('recommends a Chainguard image for an example', async () => {
    mockApi({
      '/api/alternatives': {
        body: {
          input: 'node:20-alpine',
          recommended: { image: 'node', reference: 'cgr.dev/chainguard/node', free: true },
          alternatives: [{ image: 'node-fips', reference: 'cgr.dev/chainguard/node-fips', free: false }],
          dev_tag: 'latest-dev',
          notes: ['Use :latest-dev for build stages.'],
        },
      },
    })
    renderAt('/alternatives', '/alternatives', <AlternativesPage />)

    await userEvent.setup().click(screen.getByRole('button', { name: 'node:20-alpine' }))

    expect(await screen.findByRole('link', { name: 'node' })).toHaveAttribute('href', '/images/node')
    expect(screen.getByText('cgr.dev/chainguard/node:latest-dev')).toBeInTheDocument()
    expect(screen.getByText('Use :latest-dev for build stages.')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'node-fips' })).toBeInTheDocument()
  })
})
