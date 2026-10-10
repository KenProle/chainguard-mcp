// Counting, sorting and wording for the catalog map. Groups and families come
// from the server (GET /api/groups, GET /api/families); free-tier statuses
// arrive in batches from the image list and are merged here.

import type { GroupBy, ImageFamily, ImageGroup, ImageList } from './api'

/**
 * Free-tier status by image name: true for free, false for subscription, null
 * when the image was checked but the check failed. Images not in the map
 * haven't been checked yet.
 */
export type StatusMap = ReadonlyMap<string, boolean | null>

export type FreeStatus = 'free' | 'subscription' | 'unknown'

/** Merges image list pages into a status map. */
export function statusesFrom(pages: ImageList[]): StatusMap {
  const statuses = new Map<string, boolean | null>()
  for (const page of pages) {
    for (const img of page.images) statuses.set(img.name, img.free ?? null)
  }
  return statuses
}

export function imageStatus(name: string, statuses: StatusMap): FreeStatus {
  const free = statuses.get(name)
  return free === true ? 'free' : free === false ? 'subscription' : 'unknown'
}

/** Counts images by status across families or groups. */
export function statusCounts(sets: { images: string[] }[], statuses: StatusMap): Record<FreeStatus, number> {
  const counts = { free: 0, subscription: 0, unknown: 0 }
  for (const set of sets) {
    for (const name of set.images) counts[imageStatus(name, statuses)]++
  }
  return counts
}

export type FamilyRow = {
  name: string
  /** The image the family name links to: its base image, or its first image when there is none. */
  link: string
  images: { name: string; status: FreeStatus; checked: boolean }[]
  imageCount: number
  /** Free images, or null while any of the family's images is still unchecked. */
  freeCount: number | null
}

export function familyRows(families: ImageFamily[], statuses: StatusMap): FamilyRow[] {
  return families.map((f) => {
    const images = f.images.map((name) => ({ name, status: imageStatus(name, statuses), checked: statuses.has(name) }))
    return {
      name: f.name,
      link: f.images[0],
      images,
      imageCount: images.length,
      freeCount: images.every((i) => i.checked) ? images.filter((i) => i.status === 'free').length : null,
    }
  })
}

export type FamilySort = 'images' | 'free' | 'name'

export const familySorts: FamilySort[] = ['images', 'free', 'name']

/**
 * Sorts rows by image count (the treemap's order), by free images, or by
 * name. Ties fall back to image count, then name. Rows whose free count isn't
 * known yet sort after those whose count is.
 */
export function sortFamilyRows(rows: FamilyRow[], sort: FamilySort): FamilyRow[] {
  const byName = (a: FamilyRow, b: FamilyRow) => (a.name < b.name ? -1 : a.name > b.name ? 1 : 0)
  const byImages = (a: FamilyRow, b: FamilyRow) => b.imageCount - a.imageCount || byName(a, b)
  const compare = {
    images: byImages,
    free: (a: FamilyRow, b: FamilyRow) => (b.freeCount ?? -1) - (a.freeCount ?? -1) || byImages(a, b),
    name: byName,
  }[sort]
  return [...rows].sort(compare)
}

const count = (n: number, one: string, many: string) => `${n.toLocaleString('en-US')} ${n === 1 ? one : many}`

/** The status line above the map, while statuses load and once they have. */
export function mapStatusText(checked: number, total: number, families: number, free: number): string {
  if (checked >= total) return `${count(total, 'image', 'images')} in ${count(families, 'family', 'families')} · ${free.toLocaleString('en-US')} free`
  if (checked === 0) return `Checking free-tier status: 0 of ${count(total, 'image', 'images')}`
  return `Checked ${checked.toLocaleString('en-US')} of ${count(total, 'image', 'images')}`
}

const groupNoun: Record<GroupBy, [string, string]> = {
  variant: ['variant group', 'variant groups'],
  prefix: ['name-prefix group', 'name-prefix groups'],
}

/** The treemap's accessible name. free is null while statuses are loading. */
export function treemapSummary(total: number, groups: number, groupBy: GroupBy, free: number | null): string {
  const [one, many] = groupNoun[groupBy]
  const status = free === null ? '. Free-tier status is still loading.' : `: ${free.toLocaleString('en-US')} free.`
  return `Treemap of ${count(total, 'image', 'images')} in ${count(groups, one, many)}${status} The family table below lists every family.`
}

/** A group's name for headers, tooltips and bars; Other says how many groups it merges. */
export function groupLabel(group: ImageGroup, groupBy: GroupBy): string {
  if (!group.folded) return group.label
  return `${group.label} (${count(group.folded, groupBy === 'prefix' ? 'prefix' : 'kind', groupBy === 'prefix' ? 'prefixes' : 'kinds')})`
}

const statusWords: Record<FreeStatus, string> = { free: 'free', subscription: 'subscription', unknown: 'not known' }

export function tooltipText(name: string, status: FreeStatus, group: string): string {
  return `${name} · ${statusWords[status]} · ${group}`
}

/**
 * Estimated width of text in the treemap's SVG units at its 12-unit font size.
 * Image names are lowercase ASCII, so a generous average character width is
 * enough to size header text and the tooltip box without measuring.
 */
export function textWidth(text: string): number {
  return text.length * 7
}

/**
 * A header's text: "FIPS: 1,210" if it fits in width, else the label alone,
 * else the label cut short with an ellipsis ("kubern…"). Never empty, so a
 * header band always names its group; the header's tooltip gives the rest.
 */
export function headerText(label: string, images: number, width: number): string {
  const padding = 16
  const full = `${label}: ${images.toLocaleString('en-US')}`
  if (textWidth(full) + padding <= width) return full
  if (textWidth(label) + padding <= width) return label
  const chars = Math.max(1, Math.floor((width - padding) / textWidth('x')) - 1)
  return `${label.slice(0, chars)}…`
}

/** The tooltip for a group's header: its full name and image count. */
export function headerTooltip(label: string, images: number): string {
  return `${label}: ${count(images, 'image', 'images')}`
}
