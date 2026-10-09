import { describe, expect, it } from 'vitest'
import { pythonDetails, pythonPackages } from './fixtures/python'
import { mockApi } from './render'

describe('mockApi', () => {
  it('selects between responses for one path by query parameters', async () => {
    mockApi({
      '/api/images/python/details': [
        { query: { tag: 'latest' }, body: { tag: 'latest' } },
        { query: { tag: 'latest-dev' }, body: { tag: 'latest-dev' } },
      ],
    })

    const dev = await fetch('/api/images/python/details?tag=latest-dev')
    const latest = await fetch('/api/images/python/details?tag=latest')
    const other = await fetch('/api/images/python/details?tag=next')

    expect(await dev.json()).toEqual({ tag: 'latest-dev' })
    expect(await latest.json()).toEqual({ tag: 'latest' })
    expect(other.status).toBe(404)
  })
})

describe('python fixtures', () => {
  it('match the examples in the variant comparison spec', () => {
    expect(pythonPackages.latest.amd64.total).toBe(29)
    expect(pythonPackages.latest.amd64.packages).toHaveLength(29)
    expect(pythonPackages['latest-dev'].amd64.total).toBe(76)
    expect(pythonPackages['latest-dev'].amd64.packages).toHaveLength(76)

    const amd64Size = (tag: 'latest' | 'latest-dev') =>
      pythonDetails[tag].platforms.find((p) => p.platform === 'linux/amd64')?.size_bytes
    expect(amd64Size('latest')).toBe(26_882_525)
    expect(amd64Size('latest-dev')).toBe(272_631_719)
  })
})
