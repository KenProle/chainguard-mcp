# Tasks

## 1. Route table and index

- [x] 1.1 Check whether `add-catalog-map` is on `main` (`git log main --oneline -- groups.go`). If it is, merge `main` into this branch (via the app's sync tool) so `/api/groups` and `/api/families` are included in every task below; record the outcome in this task. Verify with `go build ./...`. Outcome (2026-10-10): not on `origin/main` (4db9358), so this change covers the eight current endpoints; `add-catalog-map` must add its two routes to `apiRoutes` and the README table when it merges.
- [x] 1.2 In `web.go`, add the `apiRoute`/`apiParam` types and an `apiRoutes` table holding every current `/api/...` route's path, description, parameters, example and handler, and register the routes from it; keep the `/api/` catch-all (WA-2, WA-3). Verify the existing `TestAPIEndpoints`, `TestAPIErrors` and `TestAPISBOMDownload` still pass unchanged.
- [x] 1.3 Register `GET /api/{$}` returning `{"endpoints":[...]}` built from the table (WA-1). Add `web_test.go` tests `WA-1.1 index lists images parameters`, `WA-1.2 index marks path and required parameters` and `WA-1.3 index examples are relative`; verify with `go test -run 'TestAPIIndex' -v ./...`.
- [x] 1.4 Add drift tests (WA-2): `WA-2.1 index lists exactly the expected endpoints`, `WA-2.2 every example reaches its endpoint` (against `newFakeService`), and `WA-2.3 every /api route literal in web.go is in the index` (source scan). Verify they pass, and that adding a throwaway `mux.HandleFunc("GET /api/x", ...)` outside the table makes WA-2.3 fail, then remove it.
- [x] 1.5 Add 404 cases (WA-3): `WA-3.1` `/api/nope`, `WA-3.2` `/api/images/python`, `WA-3.3` `POST /api/` returns no index. Verify with `go test -run 'TestAPIErrors|TestAPIIndex' -v ./...`.

## 2. Documentation

- [x] 2.1 README (WA-4): change the Run comment to "JSON API under `/api/` (`GET /api/` lists the endpoints)", and add a "JSON API" subsection with one curl example and a table of every endpoint, its parameters and its example URL, matching the index. Update `uiNotBuiltPage` in `web.go` to say `/api/` lists the endpoints. Add `WA-4.1 README lists every index endpoint`; verify with `go test -run 'TestAPIIndex' -v ./...`.
- [x] 2.2 CLAUDE.md: note that API routes are registered from `apiRoutes` in `web.go`, and a new endpoint needs a table entry and a README row (the tests enforce both). Add `WA` (`web-api`) to the list of capability prefixes. Verify by reading the diff.

## 3. Verification

- [x] 3.1 Run `gofmt -l .`, `go vet ./...` and `go test ./...`; all clean. Run `openspec validate --all --strict`.
- [x] 3.2 Build the binary and run `./chainguard-mcp.exe -http 127.0.0.1:8080`; check that `curl http://127.0.0.1:8080/api/` returns the index, that each example URL from the index returns 200 against the live endpoints (the SBOM example as an `application/spdx+json` download), that `/api/nope` returns 404 `not_found`, and that `/` still serves the UI.

## Workflow follow-up

- Commit and push only with the user's OK, then confirm CI passes.
- Archive with `/opsx:archive`, which creates `openspec/specs/web-api/spec.md`.
