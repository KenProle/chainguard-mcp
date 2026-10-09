// Turns the Security tab's per-package summary into rows for the fixes chart.
// The Wolfi security database records fixes, not open vulnerabilities, so
// nothing here may describe a package as unaffected or vulnerability-free.

import type { PackageFixes } from './api'

export type FixRow = {
  package: string
  /** Fixes included in the installed version. */
  fixed: number
  /** Vulnerabilities recorded as never affecting the package. */
  notAffected: number
  /** Fixes in newer versions than the one installed. */
  pending: number
}

/**
 * Rows for every source package with at least one record, packages with fixes
 * not yet installed first, then by fixes included and never-affected records
 * (most first), then by name. withoutRecords counts the packages left out.
 */
export function fixesChart(packages: PackageFixes[] = []): { rows: FixRow[]; withoutRecords: number } {
  const all = packages.map((p) => ({
    package: p.package,
    fixed: p.fixed_count,
    notAffected: p.not_affected_count,
    pending: p.pending_fixes?.length ?? 0,
  }))
  const rows = all
    .filter((r) => r.fixed + r.notAffected + r.pending > 0)
    .sort(
      (a, b) =>
        b.pending - a.pending ||
        b.fixed - a.fixed ||
        b.notAffected - a.notAffected ||
        (a.package < b.package ? -1 : a.package > b.package ? 1 : 0),
    )
  return { rows, withoutRecords: all.length - rows.length }
}

/** The line about packages without records, or undefined when every package has records. */
export function fixesWithoutRecordsText(withoutRecords: number, total: number): string | undefined {
  if (withoutRecords === 0) return undefined
  if (withoutRecords === total) {
    return `None of this image's ${total} source ${total === 1 ? 'package has' : 'packages have'} records in the Wolfi security database.`
  }
  return withoutRecords === 1
    ? '1 more source package has no records in the Wolfi security database.'
    : `${withoutRecords} more source packages have no records in the Wolfi security database.`
}
