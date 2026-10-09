import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render } from '@testing-library/react'
import type { ReactNode } from 'react'
import { MemoryRouter, Route, Routes } from 'react-router'
import { vi } from 'vitest'

type MockResponse = {
  /** Query parameters the request must have for this response to apply. */
  query?: Record<string, string>
  /** Never answer, to leave the request pending. */
  pending?: boolean
  status?: number
  body?: unknown
}

type Responses = Record<string, MockResponse | MockResponse[]>

/**
 * Stubs fetch with canned responses keyed by API path (without the query
 * string), and returns the mock so tests can inspect the requested URLs. A path
 * can map to several responses; the first whose query parameters all match the
 * request is used.
 */
export function mockApi(responses: Responses) {
  const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
    const url = new URL(String(input), 'http://localhost')
    const candidates = responses[url.pathname] ?? []
    const match = (Array.isArray(candidates) ? candidates : [candidates]).find((r) =>
      Object.entries(r.query ?? {}).every(([key, value]) => url.searchParams.get(key) === value),
    )
    if (!match) {
      return new Response(JSON.stringify({ error: `no mock for ${url.pathname}${url.search}`, code: 'not_found' }), { status: 404 })
    }
    if (match.pending) return new Promise<Response>(() => {})
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
