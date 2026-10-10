# Proposal

## Why

The README's Run section says `-http` serves the "JSON API at /api/", which reads as if `GET /api/` shows something. It doesn't: it returns `{"error":"unknown API endpoint","code":"not_found"}` from the `/api/` catch-all in `web.go`. Nothing documents the endpoints or their query parameters except the code and `web/src/api.ts`, so anyone scripting against the API (curl, a notebook, another tool) has to read Go source to find them.

## What Changes

- `GET /api/` (exactly that path) returns a small JSON index of every API endpoint: method, path, a one-line description, its path and query parameters, and one example URL.
- Every other unknown `/api/...` path keeps returning 404 with code `not_found`, as today.
- The index is built from the same list the server registers its API routes from, and a test checks that the index, the registered routes and the README all list exactly the same endpoints, so they can't drift.
- README: the Run section says "JSON API under `/api/`", and a new table lists every `GET /api/...` endpoint with its parameters and one example URL each.
- The "UI not built" page mentions that `/api/` lists the endpoints.

Not breaking: no existing endpoint, response shape or error code changes.

### Out of scope

- **Swagger UI** (or any interactive API explorer). The server's Content Security Policy (`default-src 'self'`, no inline script or style) blocks the usual setup, which loads Swagger UI from a CDN and starts it with an inline script; self-hosting it would add a large npm dependency. See design.md.
- **A generated OpenAPI document** (`/api/openapi.json`). design.md assesses it and recommends deferring it; the route list this change introduces is shaped so it can be added later without restructuring.
- Versioning the API, authentication, CORS, or any new data endpoint.
- Changing `web/src/api.ts`; the UI doesn't use the index.

### Decisions made with the user

- The user asked whether `/api/` should show the API, perhaps as Swagger docs. Chosen: a README table plus a JSON index at `GET /api/`. Rejected: leaving `/api/` as a 404 and only fixing the README wording (the URL people try first would still say "unknown API endpoint"); serving Swagger UI (blocked by the CSP, see above); making `/api/` an HTML page (the API is JSON everywhere else, and HTML would need its own styling under the CSP).
- OpenAPI is assessed, not built; the recommendation in design.md (defer) is pending the user's review of this proposal.
- The work is its own change, separate from `add-catalog-map`. If that change has merged before this one is applied, its `/api/groups` and `/api/families` endpoints are included in the index and README table; if it merges after, the drift test fails until it adds them.

## Capabilities

### New Capabilities

- `web-api`: discoverability of the web server's JSON API: the endpoint index at `GET /api/`, its completeness, the 404 behavior for unknown API paths, and the README's endpoint reference.

### Modified Capabilities

None.

## Impact

- `web.go`: API routes registered from a single route list; new `GET /api/{$}` handler; catch-all unchanged. `uiNotBuiltPage` wording.
- `web_test.go`: index tests, drift test (index vs. registered routes vs. README), updated 404 cases.
- `README.md`: Run section wording and an endpoint table.
- No Service, MCP tool, UI or dependency changes.
