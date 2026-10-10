# Spec Delta

## Purpose

Makes the web server's JSON API discoverable without reading the source: `GET /api/` lists every endpoint with its parameters and an example, the README documents the same endpoints, and unknown API paths fail with a clear error.

## ADDED Requirements

### Requirement: WA-1 Endpoint index at /api/
When the server runs with `-http`, a `GET` request for exactly `/api/` SHALL return status 200 with a JSON object whose `endpoints` array has one entry per API endpoint. Each entry SHALL give the HTTP method, the path pattern (path parameters in braces, as in `/api/images/{name}/tags`), a one-line description, every path and query parameter (name, whether it is in the path or the query, whether it is required, and a description), and one example URL.

#### Scenario: WA-1.1 Index of the images endpoint
- **WHEN** a client sends `GET /api/`
- **THEN** the response is 200 with content type `application/json`
- **AND** it has an entry with method `GET` and path `/api/images` whose parameters are `query`, `free_only`, `limit` and `offset`, all in the query and none required

#### Scenario: WA-1.2 Path and required parameters
- **WHEN** a client sends `GET /api/`
- **THEN** the `/api/images/{name}/vulnerabilities` entry lists `name` as a required path parameter and `tag` and `id` as optional query parameters
- **AND** the `/api/alternatives` entry lists `image` as a required query parameter

#### Scenario: WA-1.3 Examples are relative URLs
- **WHEN** a client reads the index
- **THEN** every example is a path starting with `/api/`, with no scheme or host, such as `/api/images/python/details?tag=latest`, so it works behind any host name

### Requirement: WA-2 Index lists exactly the served endpoints
The index SHALL list every `/api/...` endpoint the server serves, each exactly once, and SHALL NOT list an endpoint the server does not serve. Each entry's example URL SHALL be answered by that endpoint, not by the unknown-endpoint error.

#### Scenario: WA-2.1 The current endpoints
- **WHEN** a client sends `GET /api/`
- **THEN** the index lists `/api/images`, `/api/images/{name}/tags`, `/api/images/{name}/details`, `/api/images/{name}/pin`, `/api/images/{name}/packages`, `/api/images/{name}/vulnerabilities`, `/api/images/{name}/sbom` and `/api/alternatives`, plus any API endpoint added since (such as `/api/groups` and `/api/families` from the catalog map)
- **AND** no other entries

#### Scenario: WA-2.2 Examples reach their endpoint
- **WHEN** a client requests each entry's example URL
- **THEN** none of the responses is the `unknown API endpoint` error

#### Scenario: WA-2.3 A new endpoint without an index entry
- **WHEN** a developer registers a new `/api/...` route without adding it to the index
- **THEN** the test suite fails

### Requirement: WA-3 Unknown API paths stay 404
Every request under `/api/` that matches no endpoint, other than `GET /api/` itself, SHALL return status 404 with the JSON error code `not_found`, and SHALL NOT fall through to the web UI.

#### Scenario: WA-3.1 Unknown path
- **WHEN** a client sends `GET /api/nope`
- **THEN** the response is 404 with `{"error":"unknown API endpoint","code":"not_found"}`

#### Scenario: WA-3.2 Image path without an operation
- **WHEN** a client sends `GET /api/images/python`
- **THEN** the response is 404 with code `not_found`

#### Scenario: WA-3.3 Index only answers GET
- **WHEN** a client sends `POST /api/`
- **THEN** the response is not 200 and contains no index

### Requirement: WA-4 README endpoint reference
The README SHALL describe the API as served under `/api/` and SHALL have a table with one row per endpoint in the index, giving its path, its parameters and one example URL.

#### Scenario: WA-4.1 README matches the index
- **WHEN** the test suite runs
- **THEN** it fails if any endpoint path in the index is missing from the README's endpoint table

#### Scenario: WA-4.2 Run section wording
- **WHEN** a reader looks at the README's Run section
- **THEN** it says the JSON API is under `/api/`, and that `GET /api/` lists the endpoints
