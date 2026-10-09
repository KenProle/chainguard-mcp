import { useQuery } from '@tanstack/react-query'
import { useState, type FormEvent } from 'react'
import { api, type CVEMatch } from '../api'
import { FixesChart } from '../components/FixesChart'
import { Badge, Card, ErrorState, Loading } from '../components/ui'
import { buttonClass, inputClass } from '../styles'

const statusLabel: Record<CVEMatch['status'], { text: string; tone: 'green' | 'gray' | 'red' }> = {
  fixed: { text: 'Fixed', tone: 'green' },
  not_affected: { text: 'Not affected', tone: 'gray' },
  vulnerable: { text: 'Fix not installed', tone: 'red' },
}

function LookupResult({ name, tag, id }: { name: string; tag: string; id: string }) {
  const { data, error, isPending } = useQuery({
    queryKey: ['vuln', name, tag, id],
    queryFn: ({ signal }) => api.vulnerabilities(name, tag, id, signal),
  })
  if (isPending) return <Loading label={`Looking up ${id}…`} />
  if (error) return <ErrorState error={error} />
  if (!data.matches?.length) {
    return (
      <p className="text-sm text-zinc-600 dark:text-zinc-400">
        No record of <strong>{data.id}</strong> for any package in this image.
      </p>
    )
  }
  return (
    <ul className="space-y-2">
      {data.matches.map((m) => (
        <li key={m.package} className="rounded-lg border border-zinc-200 p-3 text-sm dark:border-zinc-800">
          <div className="flex flex-wrap items-center gap-2">
            <Badge tone={statusLabel[m.status].tone}>{statusLabel[m.status].text}</Badge>
            <span className="font-medium">{m.package}</span>
            <span className="font-mono text-xs text-zinc-500">{m.installed_version}</span>
          </div>
          <p className="mt-1 text-zinc-600 dark:text-zinc-400">
            {m.fixed_version && <>Fixed in {m.fixed_version}. </>}
            Installed as {m.subpackages.join(', ')}.
            {m.records_from && <> Based on the security records for {m.records_from}.</>}
          </p>
        </li>
      ))}
    </ul>
  )
}

export function SecurityTab({ name, tag }: { name: string; tag: string }) {
  const [draft, setDraft] = useState('')
  const [lookup, setLookup] = useState('')
  const summary = useQuery({
    queryKey: ['vulnSummary', name, tag],
    queryFn: ({ signal }) => api.vulnerabilities(name, tag, undefined, signal),
  })

  function submit(e: FormEvent) {
    e.preventDefault()
    setLookup(draft.trim().toUpperCase())
  }

  const pending = summary.data?.packages?.filter((p) => p.pending_fixes?.length) ?? []

  return (
    <div className="space-y-4">
      <Card>
        <h2 className="mb-1 font-semibold">Look up a vulnerability</h2>
        <p className="mb-3 text-sm text-zinc-600 dark:text-zinc-400">Check whether a CVE or GHSA affects this image's packages.</p>
        <form onSubmit={submit} className="flex flex-col gap-2 sm:flex-row">
          <label className="flex-1">
            <span className="sr-only">Vulnerability ID</span>
            <input value={draft} onChange={(e) => setDraft(e.target.value)} placeholder="e.g. CVE-2022-3602" className={inputClass} />
          </label>
          <button type="submit" disabled={!draft.trim()} className={buttonClass}>
            Check
          </button>
        </form>
        {lookup && (
          <div className="mt-4">
            <LookupResult name={name} tag={tag} id={lookup} />
          </div>
        )}
      </Card>

      <Card>
        <h2 className="mb-3 font-semibold">Recorded fixes</h2>
        {summary.isPending ? (
          <Loading label="Checking packages against the Wolfi security database…" />
        ) : summary.error ? (
          <ErrorState error={summary.error} onRetry={() => summary.refetch()} />
        ) : (
          <>
            <div className="mb-4 grid grid-cols-2 gap-3 sm:grid-cols-3">
              <Stat label="Fixes included" value={summary.data.total_fixed} />
              <Stat label="Never affected" value={summary.data.total_not_affected} />
              <Stat label="Fixes not yet installed" value={pending.reduce((n, p) => n + (p.pending_fixes?.length ?? 0), 0)} />
            </div>
            {pending.length > 0 && (
              <div className="mb-4 rounded-lg border border-red-300 p-3 text-sm dark:border-red-500/40">
                <p className="font-medium">Newer package versions fix:</p>
                <ul className="mt-1 list-disc pl-5">
                  {pending.map((p) => (
                    <li key={p.package}>
                      {p.package} {p.installed_version}: {p.pending_fixes?.join(', ')}
                    </li>
                  ))}
                </ul>
              </div>
            )}
            <FixesChart packages={summary.data.packages} />
            <div className="overflow-x-auto">
              <table className="w-full text-left text-sm">
                <thead className="text-xs uppercase tracking-wide text-zinc-500">
                  <tr>
                    <th className="py-2 pr-4 font-medium">Source package</th>
                    <th className="py-2 pr-4 font-medium">Version</th>
                    <th className="py-2 pr-4 text-right font-medium">Fixes</th>
                    <th className="py-2 text-right font-medium">Not affected</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-zinc-200 dark:divide-zinc-800">
                  {summary.data.packages?.map((p) => (
                    <tr key={p.package}>
                      <td className="py-2 pr-4 font-medium">{p.package}</td>
                      <td className="py-2 pr-4 font-mono text-xs whitespace-nowrap">{p.installed_version}</td>
                      <td className="py-2 pr-4 text-right tabular-nums">{p.fixed_count}</td>
                      <td className="py-2 text-right tabular-nums">{p.not_affected_count}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </>
        )}
      </Card>

      <p className="rounded-lg bg-zinc-100 p-3 text-xs text-zinc-600 dark:bg-zinc-800/60 dark:text-zinc-400">
        This data comes from the Wolfi security database, which records which vulnerabilities each package version fixes. It
        doesn't list unfixed vulnerabilities, so it isn't a full scan. Use a scanner such as grype for that.
      </p>
    </div>
  )
}

function Stat({ label, value }: { label: string; value: number }) {
  return (
    <div className="rounded-lg bg-zinc-50 p-3 dark:bg-zinc-800/50">
      <div className="text-2xl font-semibold tabular-nums">{value.toLocaleString()}</div>
      <div className="text-xs text-zinc-600 dark:text-zinc-400">{label}</div>
    </div>
  )
}
