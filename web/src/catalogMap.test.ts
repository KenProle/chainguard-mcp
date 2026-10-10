import { describe, expect, it } from 'vitest'
import {
  familyRows,
  groupLabel,
  headerText,
  headerTooltip,
  imageStatus,
  mapStatusText,
  sortFamilyRows,
  statusCounts,
  statusesFrom,
  textWidth,
  tooltipText,
  treemapSummary,
} from './catalogMap'
import { catalogFamilies, catalogGroups, catalogImages, catalogPage, freeImages } from './test/fixtures/catalog'

const families = catalogFamilies.families
const allPages = [0, 1000, 2000, 3000].map((offset) => catalogPage(offset))
const allStatuses = statusesFrom(allPages)

describe('catalog fixture', () => {
  it('has 3,166 images in 1,751 families, 59 free', () => {
    expect(catalogImages).toHaveLength(3166)
    expect(families).toHaveLength(1751)
    expect(freeImages).toHaveLength(59)
    expect(allPages.reduce((n, p) => n + p.count, 0)).toBe(3166)
  })
})

describe('statuses', () => {
  it('CM-7.1 marks nginx free and its variants subscription', () => {
    expect(imageStatus('nginx', allStatuses)).toBe('free')
    for (const name of ['nginx-fips', 'nginx-iamguarded', 'nginx-iamguarded-fips']) {
      expect(imageStatus(name, allStatuses)).toBe('subscription')
    }
  })

  it('CM-7.2 counts 59 free and 3,107 subscription images', () => {
    expect(statusCounts(families, allStatuses)).toEqual({ free: 59, subscription: 3107, unknown: 0 })
  })

  it('CM-7.3 counts a failed check as not known', () => {
    const statuses = statusesFrom([catalogPage(0, 1000, ['apko']), ...allPages.slice(1)])
    expect(statuses.has('apko')).toBe(true)
    expect(imageStatus('apko', statuses)).toBe('unknown')
    expect(statusCounts(families, statuses)).toEqual({ free: 58, subscription: 3107, unknown: 1 })
  })

  it('treats unchecked images as not known', () => {
    expect(statusCounts(families, statusesFrom([]))).toEqual({ free: 0, subscription: 0, unknown: 3166 })
  })
})

describe('mapStatusText', () => {
  it('CM-8.1 reports the start of the check', () => {
    expect(mapStatusText(0, 3166, 1751, 0)).toBe('Checking free-tier status: 0 of 3,166 images')
  })

  it('CM-8.2 reports progress after a batch', () => {
    expect(mapStatusText(1000, 3166, 1751, 20)).toBe('Checked 1,000 of 3,166 images')
  })

  it('CM-8.3 summarizes the catalog when done', () => {
    expect(mapStatusText(3166, 3166, 1751, 59)).toBe('3,166 images in 1,751 families · 59 free')
    expect(mapStatusText(1, 1, 1, 0)).toBe('1 image in 1 family · 0 free')
  })
})

describe('familyRows', () => {
  it('CM-10.1 builds the nginx row', () => {
    const nginx = familyRows(families, allStatuses).find((r) => r.name === 'nginx')
    expect(nginx).toMatchObject({ link: 'nginx', imageCount: 4, freeCount: 1 })
    expect(nginx?.images.map((i) => [i.name, i.status])).toEqual([
      ['nginx', 'free'],
      ['nginx-fips', 'subscription'],
      ['nginx-iamguarded', 'subscription'],
      ['nginx-iamguarded-fips', 'subscription'],
    ])
  })

  it('CM-2.1 links a family without a base image to its first image', () => {
    const osShell = familyRows(families, allStatuses).find((r) => r.name === 'os-shell')
    expect(osShell?.link).toBe('os-shell-iamguarded')
  })

  it('CM-10.4 leaves the free count unknown while any image is unchecked', () => {
    const page = (names: string[]) => ({
      total: 3166,
      count: names.length,
      offset: 0,
      images: names.map((name) => ({ name, reference: `cgr.dev/chainguard/${name}`, free: freeImages.includes(name) })),
    })
    const rows = familyRows(families, statusesFrom([page(['apko', 'go', 'go-fips'])]))
    const go = rows.find((r) => r.name === 'go')
    expect(go?.images.find((i) => i.name === 'go')?.checked).toBe(true)
    expect(go?.images.find((i) => i.name === 'go-openssl')?.checked).toBe(false)
    expect(go?.freeCount).toBeNull()
    expect(rows.find((r) => r.name === 'apko')?.freeCount).toBe(1)
  })

  it('CM-10.2 sorts the families with a free image first', () => {
    const sorted = sortFamilyRows(familyRows(families, allStatuses), 'free')
    expect(sorted.slice(0, 59).every((r) => r.freeCount === 1)).toBe(true)
    expect(sorted[59].freeCount).toBe(0)
    expect(sorted.slice(0, 3).map((r) => r.name)).toEqual(['go', 'haproxy', 'jdk'])
  })

  it('sorts by image count, as the treemap does, and by name', () => {
    const rows = familyRows(families, allStatuses)
    expect(sortFamilyRows(rows, 'images').map((r) => r.name)).toEqual(families.map((f) => f.name))
    const byName = sortFamilyRows(rows, 'name').map((r) => r.name)
    expect(byName).toEqual([...byName].sort())
  })
})

describe('treemapSummary', () => {
  it('CM-12.1 summarizes the map for screen readers', () => {
    expect(treemapSummary(3166, 5, 'variant', 59)).toBe(
      'Treemap of 3,166 images in 5 variant groups: 59 free. The family table below lists every family.',
    )
    expect(treemapSummary(3166, 11, 'prefix', null)).toBe(
      'Treemap of 3,166 images in 11 name-prefix groups. Free-tier status is still loading. The family table below lists every family.',
    )
  })
})

describe('group text', () => {
  it('counts statuses per group', () => {
    const counts = catalogGroups('variant').groups.map((g) => statusCounts([g], allStatuses).free)
    expect(counts).toEqual([58, 0, 0, 0, 1])
  })

  it('names the groups an Other group merges', () => {
    const [other] = catalogGroups('prefix').groups
    expect(groupLabel(other, 'prefix')).toBe('Other (690 prefixes)')
    expect(groupLabel(catalogGroups('variant').groups[4], 'variant')).toBe('Other variants (7 kinds)')
    expect(groupLabel(catalogGroups('variant').groups[1], 'variant')).toBe('FIPS')
  })

  it('CM-9.1 describes a unit for its tooltip', () => {
    expect(tooltipText('nginx-fips', imageStatus('nginx-fips', allStatuses), 'FIPS')).toBe('nginx-fips · subscription · FIPS')
    expect(tooltipText('apko', 'unknown', 'Base images')).toBe('apko · not known · Base images')
  })

  it('sizes a tooltip for the longest image name within the map', () => {
    const longest = catalogImages.reduce((a, b) => (b.length > a.length ? b : a))
    const text = tooltipText(longest, 'subscription', 'Other variants (7 kinds)')
    expect(textWidth(text)).toBeGreaterThan(text.length * 6)
    expect(textWidth(text) + 16).toBeLessThan(1000)
  })

  it('CM-17.1 labels a header with the group and its count', () => {
    expect(headerText('Base images', 1730, 500)).toBe('Base images: 1,730')
    expect(headerText('FIPS', 1210, 400)).toBe('FIPS: 1,210')
  })

  it('CM-17.3 falls back to the label alone, then to the label cut short', () => {
    expect(headerText('IAM-guarded FIPS', 95, 140)).toBe('IAM-guarded FIPS')
    expect(headerText('kubernetes', 51, 80)).toBe('kubernet…')
    expect(headerText('kubernetes', 51, 10)).toBe('k…')
    expect(headerTooltip('kubernetes', 51)).toBe('kubernetes: 51 images')
  })
})
