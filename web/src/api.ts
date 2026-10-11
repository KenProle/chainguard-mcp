// Typed client for the Go server's JSON API (web.go). The types mirror the Go
// output structs in service.go, details.go, sbom.go, vulns.go, alternatives.go,
// groups.go, compare.go, compare_job.go and endoflife.go. The MCP tools return
// the same structs, except ImageFamilies, ImageGroups and the CompareJob
// wrapper, which only the web UI uses.

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

/** An image and its variants, e.g. nginx with nginx-fips. Images are in name order; the first is the base image when it exists. */
export type ImageFamily = {
  name: string
  images: string[]
}

export type ImageFamilies = {
  total: number
  families: ImageFamily[]
}

export type GroupBy = 'variant' | 'prefix'

/** A run of a group's images sharing a name prefix. */
export type ImageBlock = {
  prefix: string
  count: number
}

/**
 * Images sharing a variant kind (e.g. FIPS) or name prefix; folded is set on
 * the Other group. images are ordered block by block, as blocks describes.
 */
export type ImageGroup = {
  label: string
  folded?: number
  images: string[]
  blocks: ImageBlock[]
}

export type ImageGroups = {
  group_by: GroupBy
  total: number
  groups: ImageGroup[]
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

/** Known vulnerabilities counted once each, by fix status (compare.go, osv.go). */
export type VulnCounts = {
  total: number
  fix_available: number
  no_fix_yet: number
  wont_fix: number
}

export type LanguageCount = {
  ecosystem: string
  count: number
}

/** One image in an upstream comparison. */
export type UpstreamSide = {
  reference: string
  digest: string
  platform: string
  created?: string
  size_bytes: number
  sbom_published: boolean
  os_packages: number
  language_packages?: LanguageCount[]
  /** Absent when no SBOM is published. */
  vulnerabilities?: VulnCounts
}

export type EndOfLifeNote = {
  product: string
  cycle: string
  /** YYYY-MM-DD */
  date: string
}

export type UpstreamComparison = {
  input: string
  upstream: UpstreamSide
  chainguard: UpstreamSide
  end_of_life?: EndOfLifeNote
  source: string
  /** RFC 3339 */
  checked_at: string
  notes: string[]
}

/**
 * A comparison running as a background job (compare_job.go). total is 0 until
 * the packages to look up are known. When status is done, the comparison's
 * fields are set too.
 */
export type CompareJob =
  | { status: 'running'; checked: number; total: number }
  | ({ status: 'done'; checked: number; total: number } & UpstreamComparison)

export type ApiErrorCode =
  | 'invalid_input'
  | 'not_public'
  | 'not_found'
  | 'upstream_error'
  | 'network'
  | 'unsupported_registry'
  | 'no_alternative'
  | 'rate_limited'
  | 'source_failed'

export class ApiError extends Error {
  readonly code: ApiErrorCode
  readonly status: number
  /** The data source that failed, for code source_failed. */
  readonly source?: string

  constructor(message: string, code: ApiErrorCode, status: number, source?: string) {
    super(message)
    this.name = 'ApiError'
    this.code = code
    this.status = status
    this.source = source
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
    let source: string | undefined
    try {
      const body = (await resp.json()) as { error?: string; code?: ApiErrorCode; source?: string }
      if (body.error) message = body.error
      if (body.code) code = body.code
      source = body.source
    } catch {
      // Not a JSON error body; keep the generic message.
    }
    throw new ApiError(message, code, resp.status, source)
  }
  return (await resp.json()) as T
}

const image = (name: string) => `/images/${encodeURIComponent(name)}`

export const api = {
  listImages: (p: { query?: string; freeOnly?: boolean; limit?: number; offset?: number }, signal?: AbortSignal) =>
    get<ImageList>('/images', { query: p.query, free_only: p.freeOnly, limit: p.limit, offset: p.offset }, signal),
  families: (query: string, signal?: AbortSignal) => get<ImageFamilies>('/families', { query }, signal),
  groups: (query: string, groupBy: GroupBy, signal?: AbortSignal) =>
    get<ImageGroups>('/groups', { query, group_by: groupBy }, signal),
  tags: (name: string, signal?: AbortSignal) => get<ImageTags>(`${image(name)}/tags`, {}, signal),
  details: (name: string, tag: string, signal?: AbortSignal) =>
    get<ImageDetails>(`${image(name)}/details`, { tag }, signal),
  packages: (name: string, tag: string, arch: string, signal?: AbortSignal) =>
    get<ImagePackages>(`${image(name)}/packages`, { tag, arch }, signal),
  vulnerabilities: (name: string, tag: string, id?: string, signal?: AbortSignal) =>
    get<VulnReport>(`${image(name)}/vulnerabilities`, { tag, id }, signal),
  alternatives: (ref: string, signal?: AbortSignal) => get<AlternativesResult>('/alternatives', { image: ref }, signal),
  compareUpstream: (ref: string, arch: string, signal?: AbortSignal) =>
    get<CompareJob>('/compare-upstream', { image: ref, arch }, signal),
  sbomUrl: (name: string, tag: string, arch: string) => `/api${image(name)}/sbom${buildQuery({ tag, arch })}`,
}
