// The real catalog captured on 2026-10-09: 3,166 images in 1,751 families, 59
// of them free. The JSON files are written from testdata/ by the Go test
// TestCatalogFamiliesFixture, so they always match the server's families and
// groups.

import type { GroupBy, ImageFamilies, ImageGroups, ImageList } from '../../api'
import families from './catalogFamilies.json'
import free from './catalogFree.json'
import groups from './catalogGroups.json'

export const catalogFamilies: ImageFamilies = families

/** GET /api/groups for the whole catalog or the search "nginx". */
export function catalogGroups(groupBy: GroupBy, query: '' | 'nginx' = ''): ImageGroups {
  return (groups as Record<string, ImageGroups>)[query ? `${groupBy}:${query}` : groupBy]
}
export const freeImages: string[] = free

/** Every image name in sorted order, as the image list returns them. */
export const catalogImages = families.families.flatMap((f) => f.images).sort()

const freeSet = new Set(free)
const matches = (name: string, query: string) => name.includes(query.toLowerCase())

/**
 * GET /api/families for a search, filtered from the whole-catalog response as
 * the server does: matching images only, empty families dropped, ordered by
 * image count, then name.
 */
export function familiesFor(query = ''): ImageFamilies {
  const filtered = families.families
    .map((f) => ({ name: f.name, images: f.images.filter((i) => matches(i, query)) }))
    .filter((f) => f.images.length > 0)
    .sort((a, b) => b.images.length - a.images.length || (a.name < b.name ? -1 : 1))
  return { total: filtered.reduce((n, f) => n + f.images.length, 0), families: filtered }
}

/**
 * A page of GET /api/images, with each image's status. Images in failed get
 * no status, as when a check fails.
 */
export function catalogPage(offset: number, limit = 1000, failed: string[] = [], query = ''): ImageList {
  const all = catalogImages.filter((name) => matches(name, query))
  const images = all.slice(offset, offset + limit).map((name) => ({
    name,
    reference: `cgr.dev/chainguard/${name}`,
    ...(failed.includes(name) ? {} : { free: freeSet.has(name) }),
  }))
  return { total: all.length, count: images.length, offset, images }
}
