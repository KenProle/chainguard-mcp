import { Link, NavLink, Route, Routes } from 'react-router'
import { AlternativesPage } from './pages/AlternativesPage'
import { CatalogPage } from './pages/CatalogPage'
import { ImagePage } from './pages/ImagePage'

const navClass = ({ isActive }: { isActive: boolean }) =>
  `rounded-md px-3 py-1.5 text-sm font-medium ${
    isActive
      ? 'bg-zinc-100 text-zinc-900 dark:bg-zinc-800 dark:text-zinc-100'
      : 'text-zinc-600 hover:text-zinc-900 dark:text-zinc-400 dark:hover:text-zinc-100'
  }`

export function App() {
  return (
    <div className="flex min-h-dvh flex-col bg-zinc-50 text-zinc-900 dark:bg-zinc-950 dark:text-zinc-100">
      <a href="#main" className="sr-only focus:not-sr-only focus:absolute focus:left-4 focus:top-4 focus:z-10 focus:rounded focus:bg-white focus:px-3 focus:py-2 dark:focus:bg-zinc-900">
        Skip to content
      </a>
      <header className="border-b border-zinc-200 bg-white dark:border-zinc-800 dark:bg-zinc-900">
        <div className="mx-auto flex max-w-5xl flex-wrap items-center justify-between gap-2 px-4 py-3">
          <Link to="/" className="flex items-center gap-2 font-semibold">
            <img src="/favicon.svg" alt="" className="size-7" />
            <span>Chainguard Image Explorer</span>
          </Link>
          <nav aria-label="Main" className="flex gap-1">
            <NavLink to="/" end className={navClass}>
              Catalog
            </NavLink>
            <NavLink to="/alternatives" className={navClass}>
              Find alternative
            </NavLink>
          </nav>
        </div>
      </header>

      <main id="main" className="mx-auto w-full max-w-5xl flex-1 px-4 py-6 sm:py-8">
        <Routes>
          <Route path="/" element={<CatalogPage />} />
          <Route path="/images/:name" element={<ImagePage />} />
          <Route path="/alternatives" element={<AlternativesPage />} />
          <Route path="*" element={<NotFound />} />
        </Routes>
      </main>

      <footer className="border-t border-zinc-200 px-4 py-4 text-center text-xs text-zinc-500 dark:border-zinc-800">
        An unofficial tool built on Chainguard's public image data. Not affiliated with Chainguard.{' '}
        <a href="https://github.com/KenProle/chainguard-mcp" className="underline underline-offset-2">
          Source on GitHub
        </a>
      </footer>
    </div>
  )
}

function NotFound() {
  return (
    <div className="py-10 text-center">
      <h1 className="text-xl font-semibold">Page not found</h1>
      <Link to="/" className="mt-2 inline-block text-indigo-600 hover:underline dark:text-indigo-400">
        Back to the catalog
      </Link>
    </div>
  )
}
