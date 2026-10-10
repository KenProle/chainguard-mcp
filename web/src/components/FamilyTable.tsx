import { Link } from 'react-router'
import type { FamilyRow, FamilySort } from '../catalogMap'
import { FreeBadge } from './ui'

const columns: { sort: FamilySort; label: string; className: string }[] = [
  { sort: 'name', label: 'Family', className: 'text-left' },
  { sort: 'images', label: 'Images', className: 'w-20 text-right' },
  { sort: 'free', label: 'Free', className: 'w-16 text-right' },
]

/**
 * The catalog map's text equivalent: one row per family with its image count,
 * free image count and images, each a link. Column headers sort the table.
 */
export function FamilyTable({ rows, sort, onSort }: { rows: FamilyRow[]; sort: FamilySort; onSort: (sort: FamilySort) => void }) {
  return (
    <div className="overflow-hidden rounded-xl border border-zinc-200 bg-white dark:border-zinc-800 dark:bg-zinc-900">
      <table className="w-full table-fixed text-sm">
        <caption className="sr-only">Image families</caption>
        <thead className="border-b border-zinc-200 bg-zinc-50 text-xs text-zinc-600 dark:border-zinc-800 dark:bg-zinc-800/60 dark:text-zinc-400">
          <tr>
            {columns.map((c) => (
              <th
                key={c.sort}
                scope="col"
                aria-sort={sort === c.sort ? (c.sort === 'name' ? 'ascending' : 'descending') : undefined}
                className={`px-3 py-2 font-medium sm:px-4 ${c.className}`}
              >
                <button
                  type="button"
                  onClick={() => onSort(c.sort)}
                  className={`rounded hover:text-zinc-900 focus-visible:outline-2 focus-visible:outline-indigo-600 dark:hover:text-zinc-100 ${
                    sort === c.sort ? 'font-semibold text-zinc-900 dark:text-zinc-100' : ''
                  }`}
                >
                  {c.label}
                  {sort === c.sort && <span aria-hidden="true">{c.sort === 'name' ? ' ↑' : ' ↓'}</span>}
                </button>
              </th>
            ))}
          </tr>
        </thead>
        <tbody className="divide-y divide-zinc-200 dark:divide-zinc-800">
          {rows.map((row) => (
            <tr key={row.name} className="align-top">
              <td className="px-3 py-2 sm:px-4">
                <Link to={`/images/${row.link}`} className="font-medium hover:underline">
                  {row.name}
                </Link>
                <ul className="mt-1 flex flex-wrap gap-x-3 gap-y-1 text-xs">
                  {row.images.map((img) => (
                    <li key={img.name} className="flex min-w-0 flex-wrap items-center gap-1">
                      <Link to={`/images/${img.name}`} className="min-w-0 wrap-break-word font-mono text-zinc-600 hover:underline dark:text-zinc-400">
                        {img.name}
                      </Link>{' '}
                      <FreeBadge free={img.status === 'unknown' ? undefined : img.status === 'free'} />
                    </li>
                  ))}
                </ul>
              </td>
              <td className="px-3 py-2 text-right tabular-nums sm:px-4">{row.imageCount}</td>
              <td className="px-3 py-2 text-right tabular-nums sm:px-4">
                {row.freeCount ?? (
                  <>
                    <span aria-hidden="true">—</span>
                    <span className="sr-only">Not known yet</span>
                  </>
                )}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
