import { useQuery, type UseQueryResult } from '@tanstack/react-query'
import { useSearchParams } from 'react-router'
import { api, ApiError, type ImageDetails, type ImagePackages } from '../api'
import { defaultPair, diffPackages, sizeDifference, type Pair } from '../compare'
import { HorizontalBars } from '../components/HorizontalBars'
import { Badge, Card, CopyableCode, ErrorState, Loading } from '../components/ui'
import { formatBytes } from '../styles'

const arches = ['amd64', 'arm64'] as const
type Arch = (typeof arches)[number]

const selectClass = 'w-full rounded-lg border border-zinc-300 bg-white px-3 py-2 dark:border-zinc-700 dark:bg-zinc-900'

/** The tag to pair with a newly chosen first tag: its -dev variant if there is one. */
function companion(a: string, tags: string[], current?: string): string {
  if (tags.includes(`${a}-dev`)) return `${a}-dev`
  if (current && current !== a && tags.includes(current)) return current
  return tags.find((t) => t !== a) ?? ''
}

/** The pair in the URL if it names two different existing tags, otherwise the default pair. */
function resolvePair(params: URLSearchParams, tags: string[]): Pair | null {
  const a = params.get('a')
  const b = params.get('b')
  if (a && b && a !== b && tags.includes(a) && tags.includes(b)) return { a, b }
  return defaultPair(tags)
}

export function CompareTab({ name, reference, tags }: { name: string; reference: string; tags: string[] }) {
  const [params, setParams] = useSearchParams()
  const arch: Arch = params.get('arch') === 'arm64' ? 'arm64' : 'amd64'
  const pair = resolvePair(params, tags)

  function update(changes: Record<string, string | null>) {
    setParams(
      (prev) => {
        const next = new URLSearchParams(prev)
        for (const [key, value] of Object.entries(changes)) {
          if (value === null) next.delete(key)
          else next.set(key, value)
        }
        return next
      },
      { replace: true },
    )
  }

  const chooseA = (a: string) => update({ a, b: companion(a, tags, pair?.b) })
  const chooseB = (b: string) => update({ a: pair?.a && pair.a !== b ? pair.a : (tags.find((t) => t !== b) ?? ''), b })

  return (
    <div className="space-y-4">
      {tags.length >= 2 && (
        <div className="flex flex-col gap-3 sm:flex-row sm:items-end">
          <label className="text-sm sm:w-48">
            <span className="mb-1 block text-xs font-medium uppercase tracking-wide text-zinc-500">First tag</span>
            <select value={pair?.a ?? ''} onChange={(e) => chooseA(e.target.value)} className={selectClass}>
              {!pair && <option value="">Choose a tag</option>}
              {tags.map((t) => (
                <option key={t} value={t}>
                  {t}
                </option>
              ))}
            </select>
          </label>
          <label className="text-sm sm:w-48">
            <span className="mb-1 block text-xs font-medium uppercase tracking-wide text-zinc-500">Second tag</span>
            <select value={pair?.b ?? ''} onChange={(e) => chooseB(e.target.value)} className={selectClass}>
              {!pair && <option value="">Choose a tag</option>}
              {tags
                .filter((t) => t !== pair?.a)
                .map((t) => (
                  <option key={t} value={t}>
                    {t}
                  </option>
                ))}
            </select>
          </label>
          {pair && (
            <fieldset className="flex gap-1 sm:ml-auto">
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
                  <input
                    type="radio"
                    name="compare-arch"
                    value={a}
                    checked={arch === a}
                    onChange={() => update({ arch: a === 'amd64' ? null : a })}
                    className="sr-only"
                  />
                  {a}
                </label>
              ))}
            </fieldset>
          )}
        </div>
      )}

      {pair && (
        <div>
          <span className="mb-1 block text-xs font-medium uppercase tracking-wide text-zinc-500">Pull</span>
          <div className="grid gap-2 lg:grid-cols-2">
            <CopyableCode value={`${reference}:${pair.a}`} />
            <CopyableCode value={`${reference}:${pair.b}`} />
          </div>
        </div>
      )}

      {pair ? (
        <Comparison name={name} pair={pair} arch={arch} />
      ) : (
        <Card>
          <p className="text-sm text-zinc-600 dark:text-zinc-400">This image has no -dev variant to compare against.</p>
          {tags.length >= 2 && <p className="mt-1 text-sm text-zinc-600 dark:text-zinc-400">Choose two tags above to compare them.</p>}
        </Card>
      )}
    </div>
  )
}

/** A packages request 404s when the tag isn't built for the architecture; say so. */
function describePackagesError(error: unknown, tag: string, arch: Arch): unknown {
  if (error instanceof ApiError && error.code === 'not_found') return new Error(`${tag} has no ${arch} variant.`)
  return error
}

function Comparison({ name, pair, arch }: { name: string; pair: Pair; arch: Arch }) {
  const details = (tag: string) => ({
    queryKey: ['details', name, tag],
    queryFn: ({ signal }: { signal: AbortSignal }) => api.details(name, tag, signal),
  })
  const packages = (tag: string) => ({
    queryKey: ['packages', name, tag, arch],
    queryFn: ({ signal }: { signal: AbortSignal }) => api.packages(name, tag, arch, signal),
  })
  const detailsA = useQuery(details(pair.a))
  const detailsB = useQuery(details(pair.b))
  const packagesA = useQuery(packages(pair.a))
  const packagesB = useQuery(packages(pair.b))

  const queries: { id: string; query: UseQueryResult<unknown>; error: (e: unknown) => unknown }[] = [
    { id: 'details-a', query: detailsA, error: (e) => e },
    { id: 'packages-a', query: packagesA, error: (e) => describePackagesError(e, pair.a, arch) },
    { id: 'details-b', query: detailsB, error: (e) => e },
    { id: 'packages-b', query: packagesB, error: (e) => describePackagesError(e, pair.b, arch) },
  ]
  // All or nothing: one failed side means no comparison, and retry refetches only that query.
  const failed = queries.filter((q) => q.query.isError)
  if (failed.length > 0) {
    return (
      <div className="space-y-3">
        {failed.map((q) => (
          <ErrorState key={q.id} error={q.error(q.query.error)} onRetry={() => q.query.refetch()} />
        ))}
      </div>
    )
  }
  if (!detailsA.data || !detailsB.data || !packagesA.data || !packagesB.data) {
    return <Loading label="Reading both images' signed SBOMs…" />
  }

  const sizeA = platformSize(detailsA.data, arch)
  const sizeB = platformSize(detailsB.data, arch)
  if (sizeA === undefined || sizeB === undefined) {
    const tag = sizeA === undefined ? pair.a : pair.b
    return <ErrorState error={new Error(`${tag} has no ${arch} variant.`)} />
  }

  return <ComparisonView pair={pair} arch={arch} sizes={[sizeA, sizeB]} packages={[packagesA.data, packagesB.data]} />
}

function platformSize(details: ImageDetails, arch: Arch): number | undefined {
  return details.platforms.find((p) => p.platform === `linux/${arch}`)?.size_bytes
}

function ComparisonView({
  pair,
  arch,
  sizes: [sizeA, sizeB],
  packages: [pkgsA, pkgsB],
}: {
  pair: Pair
  arch: Arch
  sizes: [number, number]
  packages: [ImagePackages, ImagePackages]
}) {
  const size = sizeDifference(sizeA, sizeB)
  const diff = diffPackages(pkgsA.packages, pkgsB.packages)
  const [larger, smaller] = size.larger === 'a' ? [pair.a, pair.b] : [pair.b, pair.a]
  const rows = [
    { tag: pair.a, size: sizeA, pkgs: pkgsA },
    { tag: pair.b, size: sizeB, pkgs: pkgsB },
  ]

  return (
    <div className="space-y-4">
      <Card>
        <p className="font-medium">
          {size.nearlyEqual
            ? `${pair.a} and ${pair.b} are about the same size on ${arch}.`
            : `${larger} is ${size.ratioLabel} larger than ${smaller} (${size.differenceLabel}) on ${arch}.`}
        </p>
        <table className="mt-3 w-full text-left text-sm">
          <thead className="text-xs uppercase tracking-wide text-zinc-500">
            <tr>
              <th className="py-2 pr-2 font-medium sm:pr-4">Tag</th>
              <th className="py-2 pr-2 font-medium sm:pr-4">Packages</th>
              <th className="py-2 pr-2 font-medium sm:pr-4">Size</th>
              <th className="py-2 pr-2 font-medium sm:pr-4">Shell</th>
              <th className="py-2 font-medium">apk</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-zinc-200 dark:divide-zinc-800">
            {rows.map((r) => (
              <tr key={r.tag}>
                <th scope="row" className="break-all py-2 pr-2 font-mono text-xs font-medium sm:pr-4">
                  {r.tag}
                </th>
                <td className="py-2 pr-2 tabular-nums sm:pr-4">{r.pkgs.total}</td>
                <td className="py-2 pr-2 whitespace-nowrap tabular-nums sm:pr-4">{formatBytes(r.size)}</td>
                <td className="py-2 pr-2 sm:pr-4">{r.pkgs.has_shell ? <Badge tone="amber">Yes</Badge> : <Badge tone="green">No</Badge>}</td>
                <td className="py-2">{r.pkgs.has_apk ? <Badge tone="amber">Yes</Badge> : <Badge tone="green">No</Badge>}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </Card>

      <Card>
        <h2 className="mb-3 font-semibold">Download size ({arch})</h2>
        <HorizontalBars bars={rows.map((r) => ({ label: r.tag, value: r.size, valueLabel: formatBytes(r.size) }))} />
      </Card>

      <Card className="space-y-4">
        <PackageGroup title={`Only in ${pair.a}`} names={diff.onlyInA.map((p) => p.name)} />
        <PackageGroup title={`Only in ${pair.b}`} names={diff.onlyInB.map((p) => p.name)} />
        <PackageGroup title="Different versions" names={diff.versionChanged.map((c) => `${c.name} ${c.a} → ${c.b}`)} />
      </Card>
    </div>
  )
}

function PackageGroup({ title, names }: { title: string; names: string[] }) {
  return (
    <section>
      <h2 className="mb-2 font-semibold">
        {title} ({names.length})
      </h2>
      {names.length === 0 ? (
        <p className="text-sm text-zinc-500">None</p>
      ) : (
        <ul className="flex flex-wrap gap-1.5">
          {names.map((n) => (
            <li key={n} className="break-all rounded-md bg-zinc-100 px-2 py-0.5 font-mono text-xs dark:bg-zinc-800">
              {n}
            </li>
          ))}
        </ul>
      )}
    </section>
  )
}
