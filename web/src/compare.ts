// Pure logic for the image page's Compare tab: choosing which two tags to
// compare, and the differences between them.

import type { Package } from './api'
import { formatBytes } from './styles'

export type Pair = { a: string; b: string }

/**
 * The pair the Compare tab opens on: latest and latest-dev when both exist,
 * otherwise the alphabetically first tag X that has an X-dev counterpart, or
 * null when the image has no -dev pair.
 */
export function defaultPair(tags: string[]): Pair | null {
  const has = new Set(tags)
  if (has.has('latest') && has.has('latest-dev')) return { a: 'latest', b: 'latest-dev' }
  const base = [...tags].sort().find((t) => has.has(`${t}-dev`))
  return base === undefined ? null : { a: base, b: `${base}-dev` }
}

export type VersionChange = { name: string; a: string; b: string }

export type PackageDiff = {
  onlyInA: Package[]
  onlyInB: Package[]
  versionChanged: VersionChange[]
}

/** Compares two package lists by package name. Each group is sorted by name. */
export function diffPackages(a: Package[], b: Package[]): PackageDiff {
  const byName = (pkgs: Package[]) => new Map(pkgs.map((p) => [p.name, p]))
  const aByName = byName(a)
  const bByName = byName(b)
  const sorted = (pkgs: Package[]) => [...pkgs].sort((x, y) => x.name.localeCompare(y.name))

  const versionChanged: VersionChange[] = []
  for (const p of aByName.values()) {
    const other = bByName.get(p.name)
    if (other && other.version !== p.version) versionChanged.push({ name: p.name, a: p.version, b: other.version })
  }
  versionChanged.sort((x, y) => x.name.localeCompare(y.name))

  return {
    onlyInA: sorted([...aByName.values()].filter((p) => !bByName.has(p.name))),
    onlyInB: sorted([...bByName.values()].filter((p) => !aByName.has(p.name))),
    versionChanged,
  }
}

export type SizeDifference = {
  /** Which side is larger; null when the sizes are nearly equal. */
  larger: 'a' | 'b' | null
  /** Absolute difference in bytes. */
  bytes: number
  /** Larger size divided by the smaller one. */
  ratio: number
  /** True when the sizes are within 1% of each other. */
  nearlyEqual: boolean
  /** e.g. "+245.7 MB". */
  differenceLabel: string
  /** e.g. "10.1×". */
  ratioLabel: string
}

/** Describes how two download sizes differ, as an amount and as a ratio. */
export function sizeDifference(aBytes: number, bBytes: number): SizeDifference {
  const big = Math.max(aBytes, bBytes)
  const small = Math.min(aBytes, bBytes)
  const bytes = big - small
  const nearlyEqual = bytes <= big * 0.01
  const ratio = small === 0 ? Infinity : big / small
  return {
    larger: nearlyEqual ? null : aBytes > bBytes ? 'a' : 'b',
    bytes,
    ratio,
    nearlyEqual,
    differenceLabel: `+${formatBytes(bytes)}`,
    ratioLabel: Number.isFinite(ratio) ? `${ratio.toFixed(1)}×` : '∞×',
  }
}
