# Design

## Context

`newWebHandler` in `web.go` registers eight `GET /api/...` routes one `mux.HandleFunc` call at a time, then a `/api/` catch-all that returns 404 `not_found`, then the UI at `/`. Go's `ServeMux` doesn't expose its registered patterns, so nothing can list the routes today without a second, hand-kept list. The in-flight `add-catalog-map` change (not on `main` yet) adds `/api/groups` and `/api/families` the same way.

Web API changes: yes. One new response (`GET /api/`); every existing endpoint, response and error code is unchanged. No Service or MCP tool changes. No new dependencies.

## Goals / Non-Goals

**Goals:**
- One place in `web.go` defines each API route's path, parameters, description, example and handler, so the index can't disagree with what's served.
- Tests that catch a route added outside that list, and a README table that falls behind.

**Non-Goals:**
- Typed response schemas in the index. It describes inputs only; outputs are documented by the MCP tools' output schemas and `web/src/api.ts`.
- Building an OpenAPI document or Swagger UI now (assessed below).

## Decisions

### 1. A route table drives both registration and the index

Add an `apiRoute` type (path pattern, description, parameters, example, handler) and an `apiRoutes` slice in `web.go`. `newWebHandler` loops over it, registering `"GET " + path` for each, then registers the index at `GET /api/{$}` and keeps the `/api/` catch-all. The index handler serializes the table (minus handlers) as `{"endpoints":[...]}`, computed once.

Each entry: `method`, `path`, `description`, `parameters` (`name`, `in` = `path`|`query`, `required`, `description`), `example`. Parameter descriptions are written in the table, reusing the wording of the matching `jsonschema` tags in `service.go` where there is one. They can't be derived from the input structs automatically, because the HTTP parameters don't map one to one onto struct fields (the path's `{name}` is the struct's `image`; `/api/images/{name}/details` takes a plain `tag` with no struct at all).

Alternatives considered:
- *A hand-written index next to the existing `HandleFunc` calls.* Two lists to keep in sync; the drift test would be the only guard.
- *Wrapping `ServeMux` to record patterns as they're registered.* Records paths but not parameters or examples, which still need a table.

`GET /api/{$}` matches only `/api/` exactly, and as a `GET` pattern it also answers `HEAD`. `POST /api/` falls through to the catch-all and gets 404 `not_found` (WA-3.3). A request for `/api` (no slash) is redirected to `/api/` by `ServeMux`, as it is today.

### 2. Drift tests (WA-2, WA-4)

In `web_test.go`:
- **Index vs. expected list:** the test holds the expected set of paths (WA-2.1), so adding an endpoint means updating the test on purpose.
- **Index vs. source:** scan the package's non-test `.go` files for `Handle`/`HandleFunc` calls with a literal pattern, and fail on any that mentions `/api` other than the index (`GET /api/{$}`) and the catch-all (`/api/`). Table routes are registered from a computed string, so they never match. This is what catches a route registered outside the table (WA-2.3), since `ServeMux` can't be asked for its patterns. It's a text scan, which is crude, but it's a dozen lines and routes in this codebase are always literals.
- **Examples reach their endpoint:** request each example against `newFakeService` and require that none returns the `unknown API endpoint` error (WA-2.2). Examples use images and tags the fake serves (`python` `latest`/`latest-dev`, `node`), which are also free on the live registry; the SBOM example uses the default `amd64`, since the fake has no arm64 attestation.
- **README vs. index:** read `README.md` and require every index path to appear in it inside backticks (WA-4.1).

### 3. README

The Run comment becomes "JSON API under /api/ (GET /api/ lists the endpoints)". A new "JSON API" subsection under Run has one curl line for `GET /api/`, a table (endpoint, parameters, example) whose examples match the index's, and the error codes.

### 4. Ordering with add-catalog-map

Whichever change merges second adds the other's routes: if `add-catalog-map` is on `main` when this is applied, its two `HandleFunc` calls move into the table with entries and README rows; if it merges later, the source-scan test fails on its new literals until it does the same. Task 1.1 checks which case applies.

### 5. OpenAPI document: assessed, recommend deferring

**What it would take.** Serve `/api/openapi.json` as an OpenAPI 3.1 document. 3.1 uses JSON Schema 2020-12, which is what `github.com/google/jsonschema-go` produces; that module is already in `go.mod` as an indirect dependency of the MCP SDK, which uses it (`jsonschema.For`) to derive each tool's input and output schema from the same structs the API returns. So:
- Add an output type to each `apiRoute` (e.g. `reflect.TypeFor[GetImageTagsOutput]()`), and build `components.schemas` with `jsonschema.ForType`, rewriting its `$defs` references into `#/components/schemas/...`.
- `paths` come from the route table: parameters as they are, 200 responses referencing the output schema, `/sbom` as `application/spdx+json` with a free-form schema, and 400/403/404/502 referencing an `apiError` schema with the `code` enum.
- Importing `jsonschema-go` directly turns an indirect dependency into a direct one (no new module, but it should be justified in that change's design), and its version would then move with the MCP SDK's.
- Tests: no OpenAPI validator without a new dependency, so structural checks only (every route present, every `$ref` resolves), plus a manual check with an external validator.

Roughly 150–250 lines of Go plus tests, and a contract to keep correct as output structs change.

**What it would give.** Import into Postman/Insomnia/Bruno, client generation, and a machine-readable contract. But today the only API client is this project's own UI, which mirrors the types by hand in `web/src/api.ts` and has its own tests; generating `api.ts` from the document would need an npm code generator (a new dependency) and a build step. Programmatic consumers that want typed schemas already get them from the MCP server: `tools/list` publishes each tool's input and output schema, derived from the same structs.

**Swagger UI is out of scope.** The server's CSP is `default-src 'self'` with no inline script or style. The standard Swagger UI setup loads its bundle from a CDN and starts it with an inline `<script>`, both blocked. Self-hosting it would mean adding `swagger-ui-dist` (a multi-megabyte npm dependency embedded in the binary), moving the start-up code into a file, and confirming the bundle works without inline styles, all for a page that an OpenAPI file plus any external viewer already covers. Loosening the CSP for it is not an option.

**Recommendation: defer.** Build the JSON index and README table now; they answer the user's question (what's at `/api/`?) with no dependency and little code. The route table from decision 1 is the hook a later OpenAPI change needs: it adds an output type per route and a generator, without restructuring. Revisit when there's a real consumer: someone generating a client, a request to import the API into a tool, or a decision to generate `web/src/api.ts` from Go instead of mirroring it by hand.

## Risks / Trade-offs

- [Parameter descriptions in the table drift from the `jsonschema` tags] → They're short and reviewed together in `web.go`; an OpenAPI change would later derive output schemas, not these.
- [The source scan misses a route built from a non-literal string, or flags a literal that isn't a route] → Routes in this codebase are literals; if a false positive appears, the scan's exclusion list is in the test.
- [Examples hit the live registry when someone runs them by hand] → They use free-tier images (`python`, `node`) and `latest`, which the free tier publishes.
- [The index is a new public response shape] → It's additive and documented by WA-1; nothing in the UI depends on it.
