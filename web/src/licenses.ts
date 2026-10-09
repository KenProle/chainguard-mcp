// Pure logic for the Packages tab's license breakdown: sorting each package's
// declared SPDX license expression into one category.

import type { Package } from './api'

export type LicenseCategory = 'permissive' | 'weak' | 'strong' | 'none' | 'unrecognized'

export const licenseCategoryName: Record<LicenseCategory, string> = {
  permissive: 'Permissive',
  weak: 'Weak copyleft',
  strong: 'Strong copyleft',
  none: 'Not declared',
  unrecognized: 'Unrecognized',
}

/**
 * License identifiers by category, matched case-insensitively. Families match
 * by prefix (`BSD-2-Clause`, `GPL-3.0-or-later`); anchoring keeps `LGPL-` and
 * `AGPL-` out of the `GPL-` rule. Anything not listed is unrecognized.
 */
const mapping: [RegExp, LicenseCategory][] = [
  // Permissive: notices and attribution only.
  [/^MIT(-|$)/i, 'permissive'],
  [/^Apache-/i, 'permissive'],
  [/^BSD-/i, 'permissive'],
  [/^PSF-/i, 'permissive'],
  [/^ISC$/i, 'permissive'],
  [/^Zlib$/i, 'permissive'],
  [/^blessing$/i, 'permissive'], // SQLite's public-domain dedication
  [/^CC-PDDC$/i, 'permissive'], // public domain
  // Weak copyleft: changes to the licensed files or library must be shared.
  [/^LGPL-/i, 'weak'],
  [/^MPL-/i, 'weak'],
  [/^EPL-/i, 'weak'],
  // Strong copyleft: the whole derived work must be shared under the license.
  [/^GPL-/i, 'strong'],
  [/^AGPL-/i, 'strong'],
]

/** The category of a single license identifier, e.g. `MIT` → permissive. */
export function categorizeIdentifier(id: string): LicenseCategory {
  return mapping.find(([pattern]) => pattern.test(id))?.[1] ?? 'unrecognized'
}

// Restrictiveness of the categories AND and OR choose between.
type Known = 'permissive' | 'weak' | 'strong'
type Part = Known | 'unrecognized'
const rank: Record<Known, number> = { permissive: 0, weak: 1, strong: 2 }

/** All licenses apply: the most restrictive part wins, but an unknown part makes the result unknown unless a part is strong copyleft. */
function combineAnd(parts: Part[]): Part {
  if (parts.includes('strong')) return 'strong'
  if (parts.includes('unrecognized')) return 'unrecognized'
  return (parts as Known[]).reduce((a, b) => (rank[b] > rank[a] ? b : a))
}

/** A choice of licenses: the least restrictive part wins, but an unknown part makes the result unknown unless a part is permissive. */
function combineOr(parts: Part[]): Part {
  if (parts.includes('permissive')) return 'permissive'
  if (parts.includes('unrecognized')) return 'unrecognized'
  return (parts as Known[]).reduce((a, b) => (rank[b] < rank[a] ? b : a))
}

class Malformed extends Error {}

/**
 * The category of a package's license expression. Follows SPDX precedence
 * (WITH, then AND, then OR, with parentheses grouping); a WITH exception keeps
 * its base license's category. A missing expression is "none"; a malformed one
 * is unrecognized.
 */
export function categorizeLicense(expression?: string): LicenseCategory {
  const tokens = (expression ?? '').replace(/[()]/g, ' $& ').split(/\s+/).filter(Boolean)
  if (tokens.length === 0) return 'none'

  let i = 0
  const peek = () => tokens[i]?.toUpperCase()
  const isOperator = (t?: string) => t === 'AND' || t === 'OR' || t === 'WITH' || t === '(' || t === ')'

  const term = (): Part => {
    if (peek() === '(') {
      i++
      const inner = or()
      if (peek() !== ')') throw new Malformed()
      i++
      return inner
    }
    const id = tokens[i]
    if (id === undefined || isOperator(id.toUpperCase())) throw new Malformed()
    i++
    if (peek() === 'WITH') {
      i++
      const exception = tokens[i]
      if (exception === undefined || isOperator(exception.toUpperCase())) throw new Malformed()
      i++
    }
    return categorizeIdentifier(id) as Part
  }
  const and = (): Part => {
    const parts = [term()]
    while (peek() === 'AND') {
      i++
      parts.push(term())
    }
    return combineAnd(parts)
  }
  const or = (): Part => {
    const parts = [and()]
    while (peek() === 'OR') {
      i++
      parts.push(and())
    }
    return combineOr(parts)
  }

  try {
    const result = or()
    return i === tokens.length ? result : 'unrecognized'
  } catch (e) {
    if (e instanceof Malformed) return 'unrecognized'
    throw e
  }
}

export type CategoryCount = { category: LicenseCategory; count: number }

// Tie-break order: the most restrictive category first.
const order: LicenseCategory[] = ['strong', 'weak', 'permissive', 'unrecognized', 'none']

/** Package counts per non-empty category, largest first. */
export function licenseBreakdown(packages: Package[]): CategoryCount[] {
  const counts = new Map<LicenseCategory, number>()
  for (const p of packages) {
    const c = categorizeLicense(p.license)
    counts.set(c, (counts.get(c) ?? 0) + 1)
  }
  return order
    .filter((category) => counts.has(category))
    .map((category) => ({ category, count: counts.get(category)! }))
    .sort((a, b) => b.count - a.count)
}
