import { useQuery } from '@tanstack/react-query'
import { api, type AdvisoryReport, type AdvisoryStatus, type OpenVulnerability } from '../api'
import { advisoryStatus, advisoryStatusOrder } from '../styles'
import { Badge, Card, ErrorState, Loading, Stat } from './ui'

const osvUrl = (id: string) => `https://osv.dev/vulnerability/${encodeURIComponent(id)}`

/** A vulnerability ID linked to its OSV.dev page. */
function IdLink({ id }: { id: string }) {
  return (
    <a
      href={osvUrl(id)}
      target="_blank"
      rel="noreferrer"
      className="font-mono text-sm font-medium break-all text-indigo-700 underline underline-offset-2 dark:text-indigo-300"
    >
      {id}
    </a>
  )
}

/** One vulnerability: its ID, status and each affected package (OA-6, OA-7). */
export function OpenVulnerabilityRow({ vuln }: { vuln: OpenVulnerability }) {
  const status = advisoryStatus[vuln.status]
  return (
    <li className="rounded-lg border border-zinc-200 p-3 text-sm dark:border-zinc-800">
      <div className="flex flex-wrap items-center gap-2">
        <IdLink id={vuln.id} />
        <Badge tone={status.tone}>{status.text}</Badge>
        {vuln.aliases?.map((a) => (
          <span key={a} className="font-mono text-xs break-all text-zinc-500 dark:text-zinc-400">
            {a}
          </span>
        ))}
      </div>
      <ul className="mt-2 space-y-1.5">
        {vuln.packages.map((p) => (
          <li key={p.name} className="flex flex-wrap items-baseline gap-x-2 gap-y-0.5">
            <span className="font-medium break-all">{p.name}</span>
            <span className="font-mono text-xs">{p.version}</span>
            <span className="text-xs text-zinc-600 dark:text-zinc-400">{advisoryStatus[p.status].text}</span>
            {p.fixed_version && <span className="text-xs text-zinc-600 dark:text-zinc-400">Fixed in {p.fixed_version}.</span>}
            {p.components?.map((c) => (
              <span key={c.name} className="text-xs text-zinc-600 dark:text-zinc-400">
                Found in {c.name} ({c.kind}).
              </span>
            ))}
            <span className="font-mono text-xs break-all text-zinc-500 dark:text-zinc-400">{p.advisories.join(', ')}</span>
          </li>
        ))}
      </ul>
    </li>
  )
}

/** The open advisory for a looked-up ID, or the statement that there is none (OA-9). */
export function AdvisoryLookup({ name, tag, id }: { name: string; tag: string; id: string }) {
  const { data, error, isPending } = useQuery({
    queryKey: ['advisories', name, tag, id],
    queryFn: ({ signal }) => api.advisories(name, tag, id, signal),
  })
  return (
    <div className="mt-3">
      <h3 className="mb-1 text-sm font-semibold">Open advisory</h3>
      {isPending ? (
        <p role="status" className="text-sm text-zinc-600 dark:text-zinc-400">
          Checking OSV.dev and Chainguard's advisory feed…
        </p>
      ) : error ? (
        <ErrorState error={error} />
      ) : data.match ? (
        <ul>
          <OpenVulnerabilityRow vuln={data.match} />
        </ul>
      ) : (
        <p className="text-sm text-zinc-600 dark:text-zinc-400">{data.lookup_note}</p>
      )}
    </div>
  )
}

function Summary({ report }: { report: AdvisoryReport }) {
  const present = advisoryStatusOrder.filter((s) => report.counts[s] > 0)
  const absent = advisoryStatusOrder.filter((s) => report.counts[s] === 0)
  return (
    <>
      <div className="mb-3 grid grid-cols-2 gap-3 sm:grid-cols-3">
        <Stat label="Open advisories" value={report.total} />
        {present.map((s: AdvisoryStatus) => (
          <Stat key={s} label={advisoryStatus[s].text} value={report.counts[s]} />
        ))}
      </div>
      {absent.length > 0 && report.total > 0 && (
        <p className="mb-3 text-xs text-zinc-600 dark:text-zinc-400">
          {absent.map((s) => `0 ${advisoryStatus[s].text.toLowerCase()}`).join(', ')}.
        </p>
      )}
    </>
  )
}

/** The Security tab's Open advisories section (OA-6 to OA-8, OA-13). */
export function OpenAdvisories({ name, tag }: { name: string; tag: string }) {
  const query = useQuery({
    queryKey: ['advisories', name, tag],
    queryFn: ({ signal }) => api.advisories(name, tag, undefined, signal),
  })
  const report = query.data
  return (
    <Card>
      <section aria-labelledby="open-advisories-heading">
        <h2 id="open-advisories-heading" className="mb-1 font-semibold">
          Open advisories
        </h2>
        {query.isPending ? (
          <Loading label="Checking this image's packages on OSV.dev and Chainguard's advisory feed…" hint="This usually takes a few seconds." />
        ) : query.error ? (
          <ErrorState error={query.error} onRetry={() => query.refetch()} />
        ) : (
          <>
            <p className="mb-3 text-sm text-zinc-600 dark:text-zinc-400">{report!.source}</p>
            <Summary report={report!} />
            {report!.note && <p className="mb-3 text-sm text-zinc-600 dark:text-zinc-400">{report!.note}</p>}
            {report!.vulnerabilities.length > 0 && (
              <ul className="space-y-2">
                {report!.vulnerabilities.map((v) => (
                  <OpenVulnerabilityRow key={v.id} vuln={v} />
                ))}
              </ul>
            )}
            {!!report!.not_covered?.length && (
              <p className="mt-3 text-xs text-zinc-600 dark:text-zinc-400">
                Not looked up, because they come from another source: {report!.not_covered.join(', ')}.
              </p>
            )}
          </>
        )}
      </section>
    </Card>
  )
}
