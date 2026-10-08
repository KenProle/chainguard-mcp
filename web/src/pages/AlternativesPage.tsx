import { useQuery } from '@tanstack/react-query'
import { useState, type FormEvent } from 'react'
import { Link, useSearchParams } from 'react-router'
import { api } from '../api'
import { Card, CopyableCode, ErrorState, FreeBadge, Loading } from '../components/ui'
import { buttonClass, inputClass } from '../styles'

const examples = ['node:20-alpine', 'python:3.12-slim', 'eclipse-temurin:21-jre', 'gcr.io/distroless/static-debian12']

export function AlternativesPage() {
  const [params, setParams] = useSearchParams()
  const image = params.get('image') ?? ''
  const [draft, setDraft] = useState(image)

  const { data, error, isFetching, refetch } = useQuery({
    queryKey: ['alternatives', image],
    queryFn: ({ signal }) => api.alternatives(image, signal),
    enabled: image !== '',
  })

  function search(ref: string) {
    setDraft(ref)
    setParams(ref ? { image: ref } : {})
  }
  function submit(e: FormEvent) {
    e.preventDefault()
    search(draft.trim())
  }

  return (
    <div className="space-y-5">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">Find a Chainguard alternative</h1>
        <p className="mt-1 text-sm text-zinc-600 dark:text-zinc-400">
          Enter an image you use today, from Docker Hub, Microsoft or distroless, to see its Chainguard replacement.
        </p>
      </div>

      <form onSubmit={submit} className="flex flex-col gap-2 sm:flex-row">
        <label className="flex-1">
          <span className="sr-only">Upstream image</span>
          <input value={draft} onChange={(e) => setDraft(e.target.value)} placeholder="e.g. node:20-alpine" className={inputClass} />
        </label>
        <button type="submit" disabled={!draft.trim()} className={buttonClass}>
          Find
        </button>
      </form>
      <div className="flex flex-wrap items-center gap-2 text-sm">
        <span className="text-zinc-500">Try:</span>
        {examples.map((ex) => (
          <button
            key={ex}
            type="button"
            onClick={() => search(ex)}
            className="rounded-full border border-zinc-300 px-3 py-1 font-mono text-xs hover:bg-zinc-100 dark:border-zinc-700 dark:hover:bg-zinc-800"
          >
            {ex}
          </button>
        ))}
      </div>

      {image === '' ? null : isFetching && !data ? (
        <Loading label="Searching the catalog…" />
      ) : error ? (
        <ErrorState error={error} onRetry={() => refetch()} />
      ) : data ? (
        <div className="space-y-4">
          {data.recommended ? (
            <Card className="border-indigo-300 dark:border-indigo-500/40">
              <p className="text-xs font-medium uppercase tracking-wide text-indigo-700 dark:text-indigo-300">Recommended</p>
              <div className="mt-1 flex flex-wrap items-center gap-2">
                <Link to={`/images/${data.recommended.image}`} className="text-xl font-semibold hover:underline">
                  {data.recommended.image}
                </Link>
                <FreeBadge free={data.recommended.free} />
              </div>
              <div className="mt-3 space-y-2">
                <CopyableCode value={`${data.recommended.reference}:latest`} />
                {data.dev_tag && <CopyableCode value={`${data.recommended.reference}:${data.dev_tag}`} />}
              </div>
            </Card>
          ) : (
            <Card>
              <p className="text-sm">No direct Chainguard equivalent found for {data.input}.</p>
            </Card>
          )}

          {data.notes.length > 0 && (
            <Card>
              <h2 className="mb-2 font-semibold">Migration notes</h2>
              <ul className="list-disc space-y-1 pl-5 text-sm text-zinc-700 dark:text-zinc-300">
                {data.notes.map((n) => (
                  <li key={n}>{n}</li>
                ))}
              </ul>
            </Card>
          )}

          {data.alternatives.length > 0 && (
            <Card>
              <h2 className="mb-2 font-semibold">Variants</h2>
              <ul className="divide-y divide-zinc-200 dark:divide-zinc-800">
                {data.alternatives.map((a) => (
                  <li key={a.image} className="flex items-center justify-between gap-3 py-2">
                    <Link to={`/images/${a.image}`} className="min-w-0 truncate font-medium hover:underline">
                      {a.image}
                    </Link>
                    <FreeBadge free={a.free} />
                  </li>
                ))}
              </ul>
            </Card>
          )}
        </div>
      ) : null}
    </div>
  )
}
