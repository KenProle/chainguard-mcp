import { useQuery } from '@tanstack/react-query'
import { Link, useParams, useSearchParams } from 'react-router'
import { api } from '../api'
import { Badge, CopyableCode, ErrorState, Loading } from '../components/ui'
import { CompareTab } from './CompareTab'
import { OverviewTab } from './OverviewTab'
import { PackagesTab } from './PackagesTab'
import { SecurityTab } from './SecurityTab'

const tabs = [
  { id: 'overview', label: 'Overview' },
  { id: 'packages', label: 'Packages & SBOM' },
  { id: 'security', label: 'Security' },
  { id: 'compare', label: 'Compare' },
] as const

type TabId = (typeof tabs)[number]['id']

export function ImagePage() {
  const { name = '' } = useParams()
  const [params, setParams] = useSearchParams()
  const tab: TabId = tabs.some((t) => t.id === params.get('tab')) ? (params.get('tab') as TabId) : 'overview'
  const tag = params.get('tag') || 'latest'

  const tags = useQuery({ queryKey: ['tags', name], queryFn: ({ signal }) => api.tags(name, signal) })

  function set(key: string, value: string, fallback: string) {
    setParams(
      (prev) => {
        const next = new URLSearchParams(prev)
        if (value === fallback) next.delete(key)
        else next.set(key, value)
        return next
      },
      { replace: true },
    )
  }

  return (
    <div className="space-y-5">
      <nav aria-label="Breadcrumb" className="text-sm">
        <Link to="/" className="text-indigo-600 hover:underline dark:text-indigo-400">
          Catalog
        </Link>
        <span className="mx-1.5 text-zinc-400">/</span>
        <span className="text-zinc-600 dark:text-zinc-400">{name}</span>
      </nav>

      <div className="flex flex-wrap items-center gap-3">
        <h1 className="text-2xl font-semibold tracking-tight">{name}</h1>
        {tags.isSuccess && <Badge tone="green">Free</Badge>}
      </div>

      {tags.isPending ? (
        <Loading />
      ) : tags.error ? (
        <ErrorState error={tags.error} onRetry={() => tags.refetch()} />
      ) : (
        <>
          <div className="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
            <div className="min-w-0 flex-1 sm:max-w-xl">
              <span className="mb-1 block text-xs font-medium uppercase tracking-wide text-zinc-500">Pull</span>
              <CopyableCode value={`${tags.data.reference}:${tag}`} />
            </div>
            {/* The Compare tab picks its own two tags. */}
            {tab !== 'compare' && (
              <label className="text-sm">
                <span className="mb-1 block text-xs font-medium uppercase tracking-wide text-zinc-500">Tag</span>
                <select
                  value={tag}
                  onChange={(e) => set('tag', e.target.value, 'latest')}
                  className="rounded-lg border border-zinc-300 bg-white px-3 py-2 dark:border-zinc-700 dark:bg-zinc-900"
                >
                  {(tags.data.tags.includes(tag) ? tags.data.tags : [tag, ...tags.data.tags]).map((t) => (
                    <option key={t} value={t}>
                      {t}
                    </option>
                  ))}
                </select>
              </label>
            )}
          </div>

          {/* The baseline is an inset shadow, not a border the tabs overlap, so the
              scrolling container has no vertical overflow (and no vertical scrollbar). */}
          <div
            role="tablist"
            aria-label="Image information"
            className="flex gap-1 overflow-x-auto overflow-y-hidden shadow-[inset_0_-1px_0_var(--color-zinc-200)] dark:shadow-[inset_0_-1px_0_var(--color-zinc-800)]"
          >
            {tabs.map((t) => (
              <button
                key={t.id}
                type="button"
                role="tab"
                id={`tab-${t.id}`}
                aria-selected={tab === t.id}
                aria-controls={`panel-${t.id}`}
                onClick={() => set('tab', t.id, 'overview')}
                className={`whitespace-nowrap border-b-2 px-3 py-2 text-sm font-medium ${
                  tab === t.id
                    ? 'border-indigo-600 text-indigo-700 dark:border-indigo-400 dark:text-indigo-300'
                    : 'border-transparent text-zinc-600 hover:text-zinc-900 dark:text-zinc-400 dark:hover:text-zinc-100'
                }`}
              >
                {t.label}
              </button>
            ))}
          </div>

          <div role="tabpanel" id={`panel-${tab}`} aria-labelledby={`tab-${tab}`}>
            {tab === 'overview' && <OverviewTab name={name} tag={tag} />}
            {tab === 'packages' && <PackagesTab name={name} tag={tag} />}
            {tab === 'security' && <SecurityTab name={name} tag={tag} />}
            {tab === 'compare' && <CompareTab name={name} tags={tags.data.tags} />}
          </div>
        </>
      )}
    </div>
  )
}
