import { useQuery } from '@tanstack/react-query'
import type { ReactNode } from 'react'
import { api } from '../api'
import { Badge, Card, CopyableCode, ErrorState, Loading } from '../components/ui'
import { formatBytes } from '../styles'

function Field({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="grid gap-1 sm:grid-cols-[10rem_1fr] sm:gap-4">
      <dt className="text-sm font-medium text-zinc-500 dark:text-zinc-400">{label}</dt>
      <dd className="min-w-0 text-sm">{children}</dd>
    </div>
  )
}

function Command({ parts }: { parts?: string[] }) {
  if (!parts?.length) return <span className="text-zinc-500">none</span>
  return <code className="break-all font-mono text-xs">{JSON.stringify(parts)}</code>
}

export function OverviewTab({ name, tag }: { name: string; tag: string }) {
  const { data, error, isPending, refetch } = useQuery({
    queryKey: ['details', name, tag],
    queryFn: ({ signal }) => api.details(name, tag, signal),
  })
  if (isPending) return <Loading label="Loading image details…" />
  if (error) return <ErrorState error={error} onRetry={() => refetch()} />

  return (
    <div className="space-y-4">
      <Card>
        <h2 className="mb-2 font-semibold">Pinned reference</h2>
        <p className="mb-3 text-sm text-zinc-600 dark:text-zinc-400">
          Use this in a Dockerfile <code className="font-mono">FROM</code> line for reproducible builds. The tag alone moves as Chainguard rebuilds the image.
        </p>
        <CopyableCode value={data.pinned_reference} />
      </Card>

      <Card>
        <h2 className="mb-3 font-semibold">Runtime</h2>
        <dl className="space-y-3">
          <Field label="Runs as">
            <span className="mr-2 font-mono text-xs">{data.user || 'root'}</span>
            {data.runs_as_root ? <Badge tone="amber">root</Badge> : <Badge tone="green">non-root</Badge>}
          </Field>
          <Field label="Entrypoint">
            <Command parts={data.entrypoint} />
          </Field>
          <Field label="Command">
            <Command parts={data.cmd} />
          </Field>
          {data.working_dir && (
            <Field label="Working directory">
              <code className="font-mono text-xs">{data.working_dir}</code>
            </Field>
          )}
          {data.env && data.env.length > 0 && (
            <Field label="Environment">
              <ul className="space-y-1">
                {data.env.map((e) => (
                  <li key={e}>
                    <code className="break-all font-mono text-xs">{e}</code>
                  </li>
                ))}
              </ul>
            </Field>
          )}
          {data.created && <Field label="Built">{new Date(data.created).toLocaleString()}</Field>}
          {data.source && (
            <Field label="Source">
              <a href={data.source} target="_blank" rel="noreferrer" className="break-all text-indigo-600 hover:underline dark:text-indigo-400">
                {data.source}
              </a>
            </Field>
          )}
        </dl>
        <p className="mt-3 text-xs text-zinc-500">Runtime settings read from {data.config_platform}.</p>
      </Card>

      <Card>
        <h2 className="mb-3 font-semibold">Platforms</h2>
        <div className="overflow-x-auto">
          <table className="w-full text-left text-sm">
            <thead className="text-xs uppercase tracking-wide text-zinc-500">
              <tr>
                <th className="py-2 pr-4 font-medium">Platform</th>
                <th className="py-2 pr-4 font-medium">Download size</th>
                <th className="py-2 font-medium">Digest</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-zinc-200 dark:divide-zinc-800">
              {data.platforms.map((p) => (
                <tr key={p.digest}>
                  <td className="py-2 pr-4 font-mono text-xs">{p.platform}</td>
                  <td className="py-2 pr-4 whitespace-nowrap">{formatBytes(p.size_bytes)}</td>
                  <td className="py-2 font-mono text-xs text-zinc-500" title={p.digest}>
                    {p.digest.slice(0, 19)}…
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </Card>
    </div>
  )
}
