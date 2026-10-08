import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render } from '@testing-library/react'
import type { ReactNode } from 'react'
import { MemoryRouter, Route, Routes } from 'react-router'
import { vi } from 'vitest'

type Responses = Record<string, { status?: number; body: unknown }>

/**
 * Stubs fetch with canned responses keyed by API path (without the query
 * string), and returns the mock so tests can inspect the requested URLs.
 */
export function mockApi(responses: Responses) {
  const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
    const url = new URL(String(input), 'http://localhost')
    const match = responses[url.pathname]
    if (!match) {
      return new Response(JSON.stringify({ error: `no mock for ${url.pathname}`, code: 'not_found' }), { status: 404 })
    }
    return new Response(JSON.stringify(match.body), {
      status: match.status ?? 200,
      headers: { 'Content-Type': 'application/json' },
    })
  })
  vi.stubGlobal('fetch', fetchMock)
  return fetchMock
}

/** Renders element at path with the app's providers and a matching route. */
export function renderAt(path: string, routePath: string, element: ReactNode) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={[path]}>
        <Routes>
          <Route path={routePath} element={element} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  )
}
