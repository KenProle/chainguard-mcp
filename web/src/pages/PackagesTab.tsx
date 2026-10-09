import { useQuery } from '@tanstack/react-query'
import { useMemo, useState } from 'react'
import { api } from '../api'
import { LicenseBreakdown } from '../components/LicenseBreakdown'
import { Badge, Card, ErrorState, Loading } from '../components/ui'
import { type LicenseCategory, categorizeLicense } from '../licenses'
import { inputClass } from '../styles'

const arches = ['amd64', 'arm64'] as const

export function PackagesTab({ name, tag }: { name: string; tag: string }) {
  const [arch, setArch] = useState<(typeof arches)[number]>('amd64')
  const [filter, setFilter] = useState('')
  // The selected license category belongs to one tag and architecture, so a
  // switch clears it without an effect.
  const [selection, setSelection] = useState<{ key: string; category: LicenseCategory } | null>(null)
  const key = `${tag}/${arch}`
  const category = selection?.key === key ? selection.category : null
  const { data, error, isPending, refetch } = useQuery({
    queryKey: ['packages', name, tag, arch],
    queryFn: ({ signal }) => api.packages(name, tag, arch, signal),
  })

  const categories = useMemo(() => new Map(data?.packages.map((p) => [p, categorizeLicense(p.license)])), [data])
  const q = filter.trim().toLowerCase()
  const shown =
    data?.packages.filter((p) => (!q || p.name.includes(q) || p.origin.includes(q)) && (!category || categories.get(p) === category)) ?? []

  return (
    <div className="space-y-4">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <fieldset className="flex gap-1">
          <legend className="sr-only">Architecture</legend>
          {arches.map((a) => (
            <label
              key={a}
              className={`cursor-pointer rounded-lg border px-3 py-1.5 text-sm has-focus-visible:ring-2 has-focus-visible:ring-indigo-500 ${
                arch === a
                  ? 'border-indigo-600 bg-indigo-50 text-indigo-800 dark:border-indigo-400 dark:bg-indigo-500/10 dark:text-indigo-200'
                  : 'border-zinc-300 dark:border-zinc-700'
              }`}
            >
              <input type="radio" name="arch" value={a} checked={arch === a} onChange={() => setArch(a)} className="sr-only" />
              {a}
            </label>
          ))}
        </fieldset>
        <a
          href={api.sbomUrl(name, tag, arch)}
          download
          className="inline-flex items-center justify-center rounded-lg border border-zinc-300 px-3 py-1.5 text-sm font-medium hover:bg-zinc-100 dark:border-zinc-700 dark:hover:bg-zinc-800"
        >
          Download SBOM (SPDX JSON)
        </a>
      </div>

      {isPending ? (
        <Loading label="Reading the image's signed SBOM…" />
      ) : error ? (
        <ErrorState error={error} onRetry={() => refetch()} />
      ) : (
        <Card>
          <div className="mb-3 flex flex-wrap items-center gap-2">
            <span className="text-sm text-zinc-600 dark:text-zinc-400">{data.total} packages</span>
            {data.has_shell ? <Badge tone="amber">Has a shell</Badge> : <Badge tone="green">No shell</Badge>}
            {data.has_apk ? <Badge tone="amber">Has apk</Badge> : <Badge tone="green">No package manager</Badge>}
          </div>
          <LicenseBreakdown
            packages={data.packages}
            selected={category}
            onSelect={(c) => setSelection(c ? { key, category: c } : null)}
          />
          <label className="mb-3 block">
            <span className="sr-only">Filter packages</span>
            <input
              type="search"
              value={filter}
              onChange={(e) => setFilter(e.target.value)}
              placeholder="Filter by package or source, e.g. ssl"
              className={inputClass}
            />
          </label>
          <div className="overflow-x-auto">
            <table className="w-full text-left text-sm">
              <thead className="text-xs uppercase tracking-wide text-zinc-500">
                <tr>
                  <th className="py-2 pr-4 font-medium">Package</th>
                  <th className="py-2 pr-4 font-medium">Version</th>
                  <th className="py-2 pr-4 font-medium">Source</th>
                  <th className="py-2 font-medium">License</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-zinc-200 dark:divide-zinc-800">
                {shown.map((p) => (
                  <tr key={`${p.name}@${p.version}`}>
                    <td className="py-2 pr-4 font-medium">{p.name}</td>
                    <td className="py-2 pr-4 font-mono text-xs whitespace-nowrap">{p.version}</td>
                    <td className="py-2 pr-4 text-zinc-600 dark:text-zinc-400">{p.origin === p.name ? '—' : p.origin}</td>
                    <td className="py-2 text-xs text-zinc-600 dark:text-zinc-400">{p.license || '—'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
            {shown.length === 0 && <p className="py-4 text-center text-sm text-zinc-500">No packages match.</p>}
          </div>
        </Card>
      )}
    </div>
  )
}
