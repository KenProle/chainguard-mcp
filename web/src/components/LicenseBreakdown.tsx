import type { Package } from '../api'
import { type LicenseCategory, licenseBreakdown, licenseCategoryName } from '../licenses'
import { licenseCategoryColor } from '../styles'
import { HorizontalBars } from './HorizontalBars'

const packagesLabel = (n: number) => `${n} ${n === 1 ? 'package' : 'packages'}`

/**
 * Bars counting an image's packages per license category. Each bar is a toggle
 * that filters the package table to that category.
 */
export function LicenseBreakdown({
  packages,
  selected,
  onSelect,
}: {
  packages: Package[]
  selected: LicenseCategory | null
  onSelect: (category: LicenseCategory | null) => void
}) {
  const counts = licenseBreakdown(packages)
  const byName = new Map(counts.map((c) => [licenseCategoryName[c.category], c.category]))

  return (
    <section aria-labelledby="licenses-heading" className="mb-4 space-y-2">
      <div className="flex items-center justify-between gap-2">
        <h3 id="licenses-heading" className="text-sm font-medium">
          Licenses
        </h3>
        {selected && (
          <button
            type="button"
            onClick={() => onSelect(null)}
            className="rounded-md px-2 py-1 text-xs font-medium text-indigo-700 hover:bg-zinc-100 focus-visible:outline-2 focus-visible:outline-indigo-600 dark:text-indigo-300 dark:hover:bg-zinc-800"
          >
            Show all
          </button>
        )}
      </div>
      <HorizontalBars
        bars={counts.map(({ category, count }) => ({
          label: licenseCategoryName[category],
          value: count,
          valueLabel: packagesLabel(count),
          colorClass: licenseCategoryColor[category],
          accessibleLabel: `${licenseCategoryName[category]}, ${packagesLabel(count)}`,
        }))}
        selected={selected ? licenseCategoryName[selected] : undefined}
        onSelect={(label) => {
          const category = byName.get(label) ?? null
          onSelect(category === selected ? null : category)
        }}
      />
      <p className="text-xs text-zinc-500">
        Based on the licenses declared in the image&apos;s SBOM. Informational only, not legal advice.
      </p>
    </section>
  )
}
