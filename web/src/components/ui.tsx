import { useState, type ReactNode } from 'react'
import { ApiError } from '../api'

type Tone = 'green' | 'amber' | 'red' | 'gray' | 'indigo'

const toneClasses: Record<Tone, string> = {
  green: 'bg-emerald-50 text-emerald-800 ring-emerald-600/20 dark:bg-emerald-500/10 dark:text-emerald-300 dark:ring-emerald-400/30',
  amber: 'bg-amber-50 text-amber-800 ring-amber-600/20 dark:bg-amber-500/10 dark:text-amber-300 dark:ring-amber-400/30',
  red: 'bg-red-50 text-red-800 ring-red-600/20 dark:bg-red-500/10 dark:text-red-300 dark:ring-red-400/30',
  gray: 'bg-zinc-100 text-zinc-700 ring-zinc-500/20 dark:bg-zinc-500/10 dark:text-zinc-300 dark:ring-zinc-400/30',
  indigo: 'bg-indigo-50 text-indigo-800 ring-indigo-600/20 dark:bg-indigo-500/10 dark:text-indigo-300 dark:ring-indigo-400/30',
}

export function Badge({ tone = 'gray', children }: { tone?: Tone; children: ReactNode }) {
  return (
    <span className={`inline-flex items-center rounded-md px-2 py-0.5 text-xs font-medium ring-1 ring-inset ${toneClasses[tone]}`}>
      {children}
    </span>
  )
}

/** Shows whether an image is in the free tier; renders nothing if unknown. */
export function FreeBadge({ free }: { free?: boolean }) {
  if (free === undefined) return null
  return free ? <Badge tone="green">Free</Badge> : <Badge tone="amber">Subscription</Badge>
}

export function Card({ children, className = '' }: { children: ReactNode; className?: string }) {
  return (
    <div className={`rounded-xl border border-zinc-200 bg-white p-4 shadow-sm sm:p-5 dark:border-zinc-800 dark:bg-zinc-900 ${className}`}>
      {children}
    </div>
  )
}

export function Loading({ label = 'Loading…', hint }: { label?: string; hint?: string }) {
  return (
    <div role="status" className="flex flex-col items-center gap-2 py-10 text-sm text-zinc-500 dark:text-zinc-400">
      <span className="size-6 animate-spin rounded-full border-2 border-zinc-300 border-t-indigo-600 dark:border-zinc-700 dark:border-t-indigo-400" aria-hidden="true" />
      <span>{label}</span>
      {hint && <span className="max-w-sm text-center text-xs">{hint}</span>}
    </div>
  )
}

/** Explains an API error; subscription-only images get a friendly message. */
export function ErrorState({ error, onRetry }: { error: unknown; onRetry?: () => void }) {
  if (error instanceof ApiError && error.code === 'not_public') {
    return (
      <Card className="border-amber-300 dark:border-amber-500/40">
        <h2 className="font-semibold">This image requires a Chainguard subscription</h2>
        <p className="mt-1 text-sm text-zinc-600 dark:text-zinc-400">
          Its details, packages and SBOM aren't available anonymously. Free-tier images can be inspected without signing in.
        </p>
      </Card>
    )
  }
  const message = error instanceof Error ? error.message : 'Something went wrong.'
  return (
    <div role="alert" className="rounded-xl border border-red-300 bg-red-50 p-4 text-sm text-red-900 dark:border-red-500/40 dark:bg-red-500/10 dark:text-red-200">
      <p>{message}</p>
      {onRetry && (
        <button type="button" onClick={onRetry} className="mt-2 font-medium underline underline-offset-2">
          Try again
        </button>
      )}
    </div>
  )
}

export function CopyButton({ text, label = 'Copy' }: { text: string; label?: string }) {
  const [state, setState] = useState<'idle' | 'copied' | 'failed'>('idle')
  const copy = async () => {
    try {
      await navigator.clipboard.writeText(text)
      setState('copied')
    } catch {
      setState('failed')
    }
    setTimeout(() => setState('idle'), 2000)
  }
  return (
    <button
      type="button"
      onClick={copy}
      aria-label={`${label}: ${text}`}
      className="shrink-0 rounded-md border border-zinc-300 px-2 py-1 text-xs font-medium hover:bg-zinc-100 dark:border-zinc-700 dark:hover:bg-zinc-800"
    >
      {state === 'copied' ? 'Copied' : state === 'failed' ? 'Copy failed' : label}
    </button>
  )
}

/** A monospace value that wraps long references, with a copy button. */
export function CopyableCode({ value }: { value: string }) {
  return (
    <div className="flex items-start gap-2">
      <code className="min-w-0 flex-1 break-all rounded-md bg-zinc-100 px-2 py-1 font-mono text-xs dark:bg-zinc-800">{value}</code>
      <CopyButton text={value} />
    </div>
  )
}

