// Typed client for the Go server's JSON API (web.go). The types mirror the Go
// output structs in service.go, details.go, sbom.go, vulns.go and
// alternatives.go, which the MCP tools also return.

export type ImageSummary = {
  name: string
  reference: string
  free?: boolean
}

export type ImageList = {
  total: number
  count: number
  offset: number
  images: ImageSummary[]
}

export type ImageTags = {
  image: string
  reference: string
  tags: string[]
}

export type PlatformInfo = {
  platform: string
  digest: string
  size_bytes: number
}

export type ImageDetails = {
  image: string
  tag: string
  reference: string
  pinned_reference: string
  digest: string
  created?: string
  source?: string
  platforms: PlatformInfo[]
  user: string
  runs_as_root: boolean
  entrypoint?: string[]
  cmd?: string[]
  working_dir?: string
  env?: string[]
  config_platform: string
}

export type Package = {
  name: string
  version: string
  origin: string
  license?: string
  distro: string
}

export type ImagePackages = {
  image: string
  tag: string
  arch: string
  digest: string
  has_shell: boolean
  has_apk: boolean
  total: number
  packages: Package[]
}

export type VulnStatus = 'not_affected' | 'fixed' | 'vulnerable'

export type CVEMatch = {
  package: string
  installed_version: string
  status: VulnStatus
  fixed_version?: string
  subpackages: string[]
  records_from?: string
}

export type PackageFixes = {
  package: string
  installed_version: string
  fixed_count: number
  not_affected_count: number
  pending_fixes?: string[]
  records_from?: string
}

export type VulnReport = {
  image: string
  tag: string
  digest: string
  id?: string
  matches?: CVEMatch[]
  packages?: PackageFixes[]
  total_fixed: number
  total_not_affected: number
  uncovered?: string[]
  note: string
}

export type Alternative = {
  image: string
  reference: string
  free?: boolean
}

export type AlternativesResult = {
  input: string
  recommended?: Alternative
  alternatives: Alternative[]
  dev_tag?: string
  notes: string[]
}

export type ApiErrorCode = 'invalid_input' | 'not_public' | 'not_found' | 'upstream_error' | 'network'

export class ApiError extends Error {
  readonly code: ApiErrorCode
  readonly status: number

  constructor(message: string, code: ApiErrorCode, status: number) {
    super(message)
    this.name = 'ApiError'
    this.code = code
    this.status = status
  }
}

type Params = Record<string, string | number | boolean | undefined>

function buildQuery(params: Params): string {
  const q = new URLSearchParams()
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== '' && value !== false) q.set(key, String(value))
  }
  const s = q.toString()
  return s ? `?${s}` : ''
}

async function get<T>(path: string, params: Params = {}, signal?: AbortSignal): Promise<T> {
  let resp: Response
  try {
    resp = await fetch(`/api${path}${buildQuery(params)}`, { signal })
  } catch (err) {
    if (err instanceof DOMException && err.name === 'AbortError') throw err
    throw new ApiError("Couldn't reach the server. Is it still running?", 'network', 0)
  }
  if (!resp.ok) {
    let message = `Request failed (${resp.status})`
    let code: ApiErrorCode = 'upstream_error'
    try {
      const body = (await resp.json()) as { error?: string; code?: ApiErrorCode }
      if (body.error) message = body.error
      if (body.code) code = body.code
    } catch {
      // Not a JSON error body; keep the generic message.
    }
    throw new ApiError(message, code, resp.status)
  }
  return (await resp.json()) as T
}

const image = (name: string) => `/images/${encodeURIComponent(name)}`

export const api = {
  listImages: (p: { query?: string; freeOnly?: boolean; limit?: number; offset?: number }, signal?: AbortSignal) =>
    get<ImageList>('/images', { query: p.query, free_only: p.freeOnly, limit: p.limit, offset: p.offset }, signal),
  tags: (name: string, signal?: AbortSignal) => get<ImageTags>(`${image(name)}/tags`, {}, signal),
  details: (name: string, tag: string, signal?: AbortSignal) =>
    get<ImageDetails>(`${image(name)}/details`, { tag }, signal),
  packages: (name: string, tag: string, arch: string, signal?: AbortSignal) =>
    get<ImagePackages>(`${image(name)}/packages`, { tag, arch }, signal),
  vulnerabilities: (name: string, tag: string, id?: string, signal?: AbortSignal) =>
    get<VulnReport>(`${image(name)}/vulnerabilities`, { tag, id }, signal),
  alternatives: (ref: string, signal?: AbortSignal) => get<AlternativesResult>('/alternatives', { image: ref }, signal),
  sbomUrl: (name: string, tag: string, arch: string) => `/api${image(name)}/sbom${buildQuery({ tag, arch })}`,
}
