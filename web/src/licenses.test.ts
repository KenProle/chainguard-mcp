import { describe, expect, it } from 'vitest'
import type { Package } from './api'
import { categorizeIdentifier, categorizeLicense, licenseBreakdown } from './licenses'
import { pythonPackages } from './test/fixtures/python'

const pkg = (name: string, license?: string): Package => ({ name, version: '1.0-r0', origin: name, license, distro: 'wolfi' })

describe('categorizeIdentifier', () => {
  it('LB-2.1 maps common identifiers', () => {
    expect(['PSF-2.0', 'blessing', 'LGPL-2.1-or-later', 'GPL-3.0-or-later'].map(categorizeIdentifier)).toEqual([
      'permissive',
      'permissive',
      'weak',
      'strong',
    ])
  })

  it('LB-2.2 treats an identifier outside the mapping as unrecognized', () => {
    expect(categorizeIdentifier('OLDAP-2.8')).toBe('unrecognized')
  })

  it('LB-2 recognizes every identifier in python:latest', () => {
    const ids = pythonPackages.latest.amd64.packages.flatMap((p) => (p.license ?? '').split(/\s+|[()]/))
    const unknown = ids.filter((id) => id && !['AND', 'OR', 'WITH', 'GCC-exception-3.1'].includes(id) && categorizeIdentifier(id) === 'unrecognized')
    expect(unknown).toEqual([])
  })

  it('LB-2 keeps LGPL and AGPL out of the GPL rule', () => {
    expect(categorizeIdentifier('LGPL-3.0-only')).toBe('weak')
    expect(categorizeIdentifier('AGPL-3.0-only')).toBe('strong')
    expect(categorizeIdentifier('GPL-2.0')).toBe('strong')
  })
})

describe('categorizeLicense', () => {
  it('LB-1.3 treats a missing or empty license as not declared', () => {
    expect(categorizeLicense(undefined)).toBe('none')
    expect(categorizeLicense('')).toBe('none')
    expect(categorizeLicense('  ')).toBe('none')
  })

  it('LB-3.1 uses strong copyleft for BSD-2-Clause AND GPL-2.0-only', () => {
    expect(categorizeLicense('BSD-2-Clause AND GPL-2.0-only')).toBe('strong')
  })

  it('LB-3.2 uses weak copyleft for MPL-2.0 AND MIT', () => {
    expect(categorizeLicense('MPL-2.0 AND MIT')).toBe('weak')
  })

  it("LB-3.3 uses strong copyleft for util-linux's 11-part expression", () => {
    const libuuid = pythonPackages.latest.amd64.packages.find((p) => p.name === 'libuuid')!
    expect(libuuid.license!.split(' AND ')).toHaveLength(11)
    expect(categorizeLicense(libuuid.license)).toBe('strong')
  })

  it('LB-3.4 is unrecognized when AND has an unknown part and no strong copyleft', () => {
    expect(categorizeLicense('MIT AND OLDAP-2.8')).toBe('unrecognized')
    expect(categorizeLicense('LGPL-2.1-only AND OLDAP-2.8')).toBe('unrecognized')
    expect(categorizeLicense('GPL-2.0-only AND OLDAP-2.8')).toBe('strong')
  })

  it('LB-4.1 uses weak copyleft for LGPL-3.0-or-later OR GPL-2.0-or-later', () => {
    expect(categorizeLicense('LGPL-3.0-or-later OR GPL-2.0-or-later')).toBe('weak')
  })

  it('LB-4.2 is unrecognized when OR has an unknown part and no permissive one', () => {
    expect(categorizeLicense('GPL-2.0-only OR OLDAP-2.8')).toBe('unrecognized')
    expect(categorizeLicense('MIT OR OLDAP-2.8')).toBe('permissive')
  })

  it('LB-5.1 categorizes a WITH exception by its base license', () => {
    expect(categorizeLicense('GPL-3.0-or-later WITH GCC-exception-3.1')).toBe('strong')
    expect(categorizeLicense('GPL-2.0-only WITH Linux-syscall-note')).toBe('strong')
  })

  it("LB-6.1 groups git's parenthesized choice", () => {
    expect(categorizeLicense('GPL-2.0-or-later AND ( Artistic-1.0-Perl OR GPL-1.0-or-later )')).toBe('strong')
    expect(categorizeLicense('MIT AND (LGPL-2.1-only OR GPL-2.0-only)')).toBe('weak')
  })

  it('LB-6.2 binds AND tighter than OR', () => {
    expect(categorizeLicense('MIT OR GPL-2.0-only AND LGPL-2.1-or-later')).toBe('permissive')
    expect(categorizeLicense('GPL-2.0-only AND LGPL-2.1-or-later OR MIT')).toBe('permissive')
  })

  it('treats malformed expressions as unrecognized without throwing', () => {
    for (const expr of ['MIT AND', '( MIT', 'MIT )', 'AND MIT', 'MIT MIT', 'GPL-2.0-only WITH', 'MIT OR ()']) {
      expect(categorizeLicense(expr)).toBe('unrecognized')
    }
  })

  it('matches operators case-insensitively', () => {
    expect(categorizeLicense('MIT and GPL-2.0-only')).toBe('strong')
  })
})

describe('licenseBreakdown', () => {
  it('LB-1.1 counts python:latest on amd64', () => {
    expect(licenseBreakdown(pythonPackages.latest.amd64.packages)).toEqual([
      { category: 'permissive', count: 16 },
      { category: 'strong', count: 7 },
      { category: 'weak', count: 6 },
    ])
  })

  it('LB-1.2 counts python:latest-dev on amd64, adding up to 76', () => {
    const breakdown = licenseBreakdown(pythonPackages['latest-dev'].amd64.packages)
    expect(breakdown).toEqual([
      { category: 'permissive', count: 37 },
      { category: 'strong', count: 24 },
      { category: 'weak', count: 13 },
      { category: 'unrecognized', count: 2 },
    ])
    expect(breakdown.reduce((n, c) => n + c.count, 0)).toBe(76)
  })

  it('LB-1.3 counts packages without a license as not declared', () => {
    expect(licenseBreakdown([pkg('a'), pkg('b', 'MIT')])).toEqual([
      { category: 'permissive', count: 1 },
      { category: 'none', count: 1 },
    ])
  })

  it('orders ties most restrictive first', () => {
    const pkgs = [pkg('a', 'MIT'), pkg('b', 'GPL-2.0-only'), pkg('c', 'LGPL-2.1-only'), pkg('d'), pkg('e', 'OLDAP-2.8')]
    expect(licenseBreakdown(pkgs).map((c) => c.category)).toEqual(['strong', 'weak', 'permissive', 'unrecognized', 'none'])
  })
})
