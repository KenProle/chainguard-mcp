// Wording and formatting for the upstream comparison (UC-6 to UC-8, UC-11).
// Kept out of the component file so React fast refresh keeps working.

import { ApiError, type UpstreamComparison } from './api'

/** How often the page asks a running comparison for progress. */
export const comparePollMs = 1000

/** "1 known vulnerability", "0 known vulnerabilities", "833 known vulnerabilities". */
export function countPhrase(n: number): string {
  return `${n.toLocaleString('en-US')} known ${n === 1 ? 'vulnerability' : 'vulnerabilities'}`
}

/** "2026-04-22T03:00:00Z" → "2026-04-22"; empty when unknown. */
export function shortDate(iso?: string): string {
  return iso ? iso.slice(0, 10) : ''
}

/** The Chainguard image's name, e.g. "node" from cgr.dev/chainguard/node:latest. */
export function chainguardName(c: UpstreamComparison): string {
  const last = c.chainguard.reference.split('/').pop() ?? c.chainguard.reference
  return last.split(':')[0]
}

/**
 * The comparison's one-paragraph answer (UC-6.1): fixes that exist but aren't
 * in the image, and package counts. Empty when the upstream image publishes no
 * SBOM, so there is nothing to compare.
 */
export function comparisonSummary(c: UpstreamComparison): string {
  const up = c.upstream
  const cg = c.chainguard
  if (!up.packages_checked || !up.vulnerabilities || !cg.vulnerabilities) return ''
  const n = (v: number) => v.toLocaleString('en-US')
  return (
    `${c.input} has ${n(up.vulnerabilities.fix_available)} known vulnerabilities with a fix available; ` +
    `Chainguard's ${chainguardName(c)} has ${n(cg.vulnerabilities.fix_available)}. ` +
    `It installs ${n(up.os_packages)} OS packages against ${n(cg.os_packages)}.`
  )
}

/**
 * The totals with their source and date (UC-6.2). Written out so a zero reads
 * "0 known vulnerabilities" next to where it came from, and never as a claim
 * about the image.
 */
export function totalsSentence(c: UpstreamComparison): string {
  const up = c.upstream.vulnerabilities
  const cg = c.chainguard.vulnerabilities
  if (!cg) return ''
  const when = `OSV.dev, checked ${shortDate(c.checked_at)}`
  const chainguard = `Chainguard's ${chainguardName(c)}: ${countPhrase(cg.total)}`
  return up ? `${c.input}: ${countPhrase(up.total)}. ${chainguard} (${when}).` : `${chainguard} (${when}).`
}

/**
 * Why a side's packages weren't checked (UC-2.2, UC-15), or "" when
 * they were. Says plainly that nothing checked is not the same as nothing found.
 */
export function uncheckedMessage(side: UpstreamComparison['upstream']): string {
  if (side.packages_checked) return ''
  switch (side.unchecked_reason) {
    case 'no_sbom':
      return `No SBOM published for ${side.reference}: its packages and known vulnerabilities can't be listed. Its size and build date are still shown.`
    case 'no_os_packages':
      return `${side.reference}'s SBOM lists no distribution packages (it may be statically built), so its known vulnerabilities can't be checked. That is not the same as having none.`
    case 'unsupported_distribution':
      return `${side.reference}'s packages come from a distribution with no vulnerability data here, so its known vulnerabilities can't be checked. That is not the same as having none.`
  }
  return `${side.reference}'s packages couldn't be checked for known vulnerabilities.`
}

export function endOfLifeSentence(c: UpstreamComparison): string {
  const e = c.end_of_life
  if (!e) return ''
  return `${e.product} ${e.cycle} reached end of life on ${e.date} and may no longer receive security updates.`
}

export function languageSummary(languages?: { ecosystem: string; count: number }[]): string {
  if (!languages || languages.length === 0) return 'None'
  return languages.map((l) => `${l.count.toLocaleString('en-US')} ${l.ecosystem}`).join(', ')
}

/** What the page tells the user when a comparison fails (UC-2.3, UC-11). */
export function compareErrorMessage(error: unknown): string {
  if (error instanceof ApiError) {
    switch (error.code) {
      case 'rate_limited':
        return "Docker Hub's limit on anonymous requests was reached. Try again later."
      case 'unsupported_registry':
        return 'Only Docker Hub images can be compared. Images on other registries still get a recommendation above.'
      case 'source_failed':
        return `${error.source ?? 'A data source'} failed, so the comparison can't be shown. Try again.`
      case 'network':
        return error.message
    }
    return error.message
  }
  return error instanceof Error ? error.message : 'Something went wrong.'
}
