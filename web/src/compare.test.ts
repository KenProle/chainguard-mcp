import { describe, expect, it } from 'vitest'
import type { Package } from './api'
import { defaultPair, diffPackages, sizeDifference } from './compare'
import { pythonPackages } from './test/fixtures/python'

const pkg = (name: string, version: string): Package => ({ name, version, origin: name, distro: 'wolfi' })

describe('defaultPair', () => {
  it('VC-1.1 picks latest and latest-dev when both exist', () => {
    expect(defaultPair(['latest-slim', 'latest', 'next', 'next-dev', 'latest-dev'])).toEqual({ a: 'latest', b: 'latest-dev' })
  })

  it('VC-1.2 falls back to another base tag with a -dev counterpart', () => {
    expect(defaultPair(['next-dev', 'next', 'latest'])).toEqual({ a: 'next', b: 'next-dev' })
  })

  it('VC-3.1 returns null for static, which has no -dev tag', () => {
    expect(defaultPair(['latest', 'latest-glibc', 'latest-glibc-tzdata', 'latest-tzdata'])).toBeNull()
  })

  it('VC-3.2 returns null for a single tag', () => {
    expect(defaultPair(['latest'])).toBeNull()
  })
})

describe('diffPackages', () => {
  it('VC-7.1 finds the 47 packages latest-dev adds to python', () => {
    const diff = diffPackages(pythonPackages.latest.amd64.packages, pythonPackages['latest-dev'].amd64.packages)
    expect(diff.onlyInB).toHaveLength(47)
    expect(diff.onlyInB.map((p) => p.name)).toEqual(expect.arrayContaining(['gcc', 'bash', 'git', 'make']))
    expect(diff.onlyInA).toEqual([])
    expect(diff.versionChanged).toEqual([])
  })

  it('VC-7.2 leaves groups empty for identical lists', () => {
    const pkgs = [pkg('glibc', '2.44-r8'), pkg('python-3.14', '3.14.1-r0')]
    expect(diffPackages(pkgs, pkgs)).toEqual({ onlyInA: [], onlyInB: [], versionChanged: [] })
  })

  it('reports packages present in both at different versions', () => {
    const diff = diffPackages([pkg('zlib', '1.3-r1'), pkg('bash', '5.2-r1')], [pkg('zlib', '1.3-r2'), pkg('make', '4.4-r0')])
    expect(diff.versionChanged).toEqual([{ name: 'zlib', a: '1.3-r1', b: '1.3-r2' }])
    expect(diff.onlyInA.map((p) => p.name)).toEqual(['bash'])
    expect(diff.onlyInB.map((p) => p.name)).toEqual(['make'])
  })
})

describe('sizeDifference', () => {
  it('VC-6.1 states a large difference as an amount and a ratio', () => {
    const d = sizeDifference(26_882_525, 272_631_719)
    expect(d.differenceLabel).toBe('+245.7 MB')
    expect(d.ratioLabel).toBe('10.1×')
    expect(d.larger).toBe('b')
    expect(d.nearlyEqual).toBe(false)
  })

  it('VC-6.2 treats sizes within 1% as nearly equal', () => {
    const d = sizeDifference(100_000_000, 100_900_000)
    expect(d.nearlyEqual).toBe(true)
    expect(d.larger).toBeNull()
  })

  it('reports the first side as larger when the order is reversed', () => {
    const d = sizeDifference(272_631_719, 26_882_525)
    expect(d.larger).toBe('a')
    expect(d.ratioLabel).toBe('10.1×')
    expect(d.differenceLabel).toBe('+245.7 MB')
  })
})
