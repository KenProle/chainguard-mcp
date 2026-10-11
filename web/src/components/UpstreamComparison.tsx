import { useQuery } from '@tanstack/react-query'
import type { ReactNode } from 'react'
import { api, ApiError, type UpstreamComparison as Comparison, type UpstreamSide } from '../api'
import { formatBytes, segmentButton, segmentGroup } from '../styles'
import {
  compareErrorMessage,
  comparePollMs,
  comparisonSummary,
  endOfLifeSentence,
  languageSummary,
  shortDate,
  totalsSentence,
  uncheckedMessage,
} from '../upstreamComparison'
import { ProgressBar } from './ProgressBar'
import { Card, ErrorState } from './ui'

const architectures = ['amd64', 'arm64']

/**
 * The comparison of an upstream image with its Chainguard replacement: size,
 * OS packages and known vulnerabilities (UC-6 to UC-8). The server computes it
 * as a background job, so this polls for progress until it is done (UC-9).
 * Every number is in the table; nothing depends on color or a chart (UC-14).
 */
export function UpstreamComparison({
  image,
  chainguardImage,
  arch,
  onArch,
  pollMs = comparePollMs,
}: {
  image: string
  chainguardImage: string
  arch: string
  onArch: (arch: string) => void
  pollMs?: number
}) {
  const { data, error, refetch, isFetching } = useQuery({
    queryKey: ['compare-upstream', image, arch],
    queryFn: ({ signal }) => api.compareUpstream(image, arch, signal),
    // Ask again while the job runs; stop once it is done or this section is
    // left (a query with no observers isn't refetched).
    refetchInterval: (query) => (query.state.data?.status === 'running' ? pollMs : false),
    // Keep polling in a background tab: people switch tabs while a first
    // comparison runs, and the server drops jobs nobody polls.
    refetchIntervalInBackground: true,
    retry: false,
    staleTime: 5 * 60_000,
  })

  return (
    <section aria-labelledby="compare-heading" className="space-y-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h2 id="compare-heading" className="text-lg font-semibold">
          Compare {image} with {chainguardImage}
        </h2>
        <div role="group" aria-label="Architecture" className={segmentGroup}>
          {architectures.map((a) => (
            <button key={a} type="button" aria-pressed={a === arch} onClick={() => onArch(a)} className={segmentButton(a === arch)}>
              {a}
            </button>
          ))}
        </div>
      </div>

      {error ? (
        <CompareError error={error} onRetry={() => refetch()} retrying={isFetching} />
      ) : data?.status === 'done' ? (
        <Result comparison={data} />
      ) : (
        <Progress checked={data?.checked ?? 0} total={data?.total ?? 0} />
      )}
    </section>
  )
}

function Progress({ checked, total }: { checked: number; total: number }) {
  const label =
    total > 0
      ? `Checking known vulnerabilities: ${checked.toLocaleString('en-US')} of ${total.toLocaleString('en-US')} packages checked`
      : "Reading both images' package lists. A first comparison takes about 20 seconds."
  return (
    <Card>
      <ProgressBar value={checked} max={total} label={label} />
    </Card>
  )
}

function CompareError({ error, onRetry, retrying }: { error: unknown; onRetry: () => void; retrying: boolean }) {
  // A subscription-only Chainguard image has its own explanation.
  if (error instanceof ApiError && error.code === 'not_public') return <ErrorState error={error} />
  const retryable = !(error instanceof ApiError && (error.code === 'unsupported_registry' || error.code === 'no_alternative'))
  return (
    <div role="alert" className="rounded-xl border border-red-300 bg-red-50 p-4 text-sm text-red-900 dark:border-red-500/40 dark:bg-red-500/10 dark:text-red-200">
      <p>{compareErrorMessage(error)}</p>
      {retryable && (
        <button type="button" onClick={onRetry} disabled={retrying} className="mt-2 font-medium underline underline-offset-2 disabled:opacity-50">
          Try again
        </button>
      )}
    </div>
  )
}

function Result({ comparison: c }: { comparison: Comparison }) {
  const up = c.upstream
  const cg = c.chainguard
  const summary = comparisonSummary(c)
  const eol = endOfLifeSentence(c)
  const noSbom = !up.sbom_published
  // Nothing checked is not "0 known vulnerabilities": show dashes, not zeros.
  const unchecked = !up.packages_checked
  const uncheckedNote = uncheckedMessage(up)
  const dash = <span aria-label="not available">—</span>

  const vuln = (side: UpstreamSide, pick: (v: NonNullable<UpstreamSide['vulnerabilities']>) => number): ReactNode =>
    side.vulnerabilities ? pick(side.vulnerabilities).toLocaleString('en-US') : dash

  const rows: { label: string; indent?: boolean; up: ReactNode; cg: ReactNode }[] = [
    { label: 'Download size', up: formatBytes(up.size_bytes), cg: formatBytes(cg.size_bytes) },
    { label: 'Built', up: shortDate(up.created) || dash, cg: shortDate(cg.created) || dash },
    { label: 'OS packages', up: unchecked ? dash : up.os_packages.toLocaleString('en-US'), cg: cg.os_packages.toLocaleString('en-US') },
    { label: 'Known vulnerabilities', up: vuln(up, (v) => v.total), cg: vuln(cg, (v) => v.total) },
    { label: 'Fix available', indent: true, up: vuln(up, (v) => v.fix_available), cg: vuln(cg, (v) => v.fix_available) },
    { label: 'No fix yet', indent: true, up: vuln(up, (v) => v.no_fix_yet), cg: vuln(cg, (v) => v.no_fix_yet) },
    { label: "Won't fix", indent: true, up: vuln(up, (v) => v.wont_fix), cg: vuln(cg, (v) => v.wont_fix) },
    {
      label: 'Language packages (not compared)',
      up: noSbom ? dash : languageSummary(up.language_packages),
      cg: languageSummary(cg.language_packages),
    },
  ]

  return (
    <div className="space-y-3">
      {eol && (
        <div role="note" className="rounded-xl border border-amber-300 bg-amber-50 p-4 text-sm text-amber-900 dark:border-amber-500/40 dark:bg-amber-500/10 dark:text-amber-200">
          {eol}
        </div>
      )}
      <Card>
        {summary && <p className="font-medium">{summary}</p>}
        {uncheckedNote && <p className="text-sm">{uncheckedNote}</p>}

        <table className="mt-3 w-full table-fixed text-sm">
          <caption className="sr-only">
            {up.reference} compared with {cg.reference}, {up.platform}
          </caption>
          <thead>
            <tr className="border-b border-zinc-200 text-left text-xs text-zinc-500 dark:border-zinc-800 dark:text-zinc-400">
              <th scope="col" className="w-2/5 py-2 pr-2 font-medium">
                <span className="sr-only">Measure</span>
              </th>
              <th scope="col" className="py-2 pr-2 font-mono font-medium break-all">
                {up.reference}
              </th>
              <th scope="col" className="py-2 font-mono font-medium break-all">
                {cg.reference}
              </th>
            </tr>
          </thead>
          <tbody className="divide-y divide-zinc-100 dark:divide-zinc-800">
            {rows.map((r) => (
              <tr key={r.label}>
                <th scope="row" className={`py-2 pr-2 text-left align-top font-normal ${r.indent ? 'pl-4 text-zinc-600 dark:text-zinc-400' : 'font-medium'}`}>
                  {r.label}
                </th>
                <td className="py-2 pr-2 align-top tabular-nums break-words">{r.up}</td>
                <td className="py-2 align-top tabular-nums break-words">{r.cg}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </Card>

      <div className="space-y-1 text-xs text-zinc-600 dark:text-zinc-400">
        <p>{totalsSentence(c)}</p>
        <p>{c.source}</p>
        <ul className="list-disc space-y-0.5 pl-5">
          {c.notes.map((n) => (
            <li key={n}>{n}</li>
          ))}
        </ul>
      </div>
    </div>
  )
}
