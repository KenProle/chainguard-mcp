# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

A Go MCP server (official SDK `github.com/modelcontextprotocol/go-sdk`) that lets AI assistants discover and inspect Chainguard container images. Everything is one `main` package. It talks to Chainguard's public endpoints anonymously, so most tools only work for free-tier images.

## Commands

```bash
go build -o chainguard-mcp.exe .                     # build (Windows; drop .exe elsewhere)
go test ./...                                        # offline tests (fake backend, ~1s)
go test -run TestSaveSBOM -v ./...                   # single test
CHAINGUARD_LIVE=1 go test -run TestLive -v ./...     # end-to-end against real Chainguard endpoints
CHAINGUARD_LIVE=1 go test -run 'TestLive/pin_image' -v ./...  # one live subtest
gofmt -l . && go vet ./...                           # CI fails on unformatted files
docker build -t chainguard-mcp .
```

Run modes: stdio by default; `-http 127.0.0.1:8080` serves Streamable HTTP at `/mcp` (plus `/healthz`); `-sbom-dir` sets where `save_sbom` writes; `-version`. In Docker, `-http=` (empty) switches the container to stdio.

On Windows, rebuilding the exe while a Claude Code session is running it leaves a `chainguard-mcp.exe~` backup. It's gitignored; don't commit it.

## Architecture

`tools.go` defines `Service` (bundling `Catalog`, `Registry`, `SecDB`, `SBOMDir`) and registers every tool and the `migrate_dockerfile` prompt in `newServer`. Each tool handler takes a typed input struct: the SDK generates the tool's JSON schema from the struct's `json` and `jsonschema` tags, so adding a parameter means adding a field. Output structs are returned as structured content the same way.

Data sources, and facts about them that aren't obvious from the code:

- **Image list (`catalog.go`):** cgr.dev doesn't support the registry catalog API, so the list comes from the `images.chainguard.dev` sitemap (`/directory/image/<name>/overview` URLs). Cached 1h; serves stale data if a refresh fails.
- **Registry (`registry.go`):** anonymous token per repository from `cgr.dev/token`. The token endpoint returns 403 for paid *and* nonexistent images, and that's how free-tier status is detected (`IsPublic`, cached 24h). Multi-scope tokens only grant the first scope, so tokens are fetched and cached per image. Pullable references always use the host `cgr.dev` (`Registry.Ref`) even when `BaseURL` points at a test server. Errors use the sentinels `ErrNotPublic` and `ErrNotFound`; tool handlers wrap them with `notPublicHint`.
- **SBOMs (`sbom.go`):** Chainguard attaches cosign attestations to each *platform* manifest (not the index) under the tag `sha256-<hex>.att`. The SPDX layer is picked by its `predicateType` annotation; the blob is a DSSE envelope whose base64 payload is an in-toto statement with the SPDX document as its predicate. apko lists every package twice (distro purl and origin purl), so packages are merged by name@version. A package's origin (source package, e.g. `libssl3` → `openssl`) comes from its `DESCRIBED_BY` relationship to a melange package named `<origin>.yaml`.
- **Vulnerabilities (`vulns.go`):** the Wolfi secdb maps origin package → version → IDs *fixed* in that version; version `"0"` means never affected. It doesn't list open vulnerabilities, so tool output must not claim an image is vulnerability-free. Lookups are by origin, and installed vs. fixed versions use `compareAPKVersions` (Alpine version rules, not semver). Only `wolfi`-namespace packages are covered. Wolfi splits some packages into version streams (`openssl-4.0`) while keeping their history under the base name (`openssl`), so `lookupSecfixes` falls back to the base name and then skips fixes newer than the installed version, since those may belong to another stream.
- **Alternatives (`alternatives.go`):** `imageAliases` maps upstream names (last path component, or the last two such as `dotnet/aspnet`) to Chainguard names, then validates against the catalog. Related images must be the base name plus only known variant suffixes (`variantSuffixes`: `-fips`, `-iamguarded`, …), because many unrelated images share a prefix (`node-local-dns`, `postgres-operator`).

The free tier only publishes `latest`/`latest-dev`-style tags; versioned tags need a subscription.

`save_sbom` is only registered when `Service.SBOMDir` is set. `main.go` defaults it to the working directory in stdio mode and leaves it empty in HTTP mode, so remote clients can't write to the server's disk unless `-sbom-dir` is given. Writes go through `os.Root` with validated plain file names; keep it that way.

## Tests

- `fake_test.go`: `fakeChainguard` serves the sitemap, token endpoint, registry (tags, manifests, blobs, `.att` attestations shaped like apko's) and secdb. `newFakeService` wires a `Service` to it. Add fixtures here when a tool needs new data.
- `tools_test.go`: tool-level tests through a real in-memory MCP client (`mcpClient`, `mustCall`, `callTool` in `mcp_test.go`). `callTool` returns the tool's error text, for asserting error cases.
- `unit_test.go`: pure functions (version comparison, reference parsing, validation).
- `live_test.go`: skipped unless `CHAINGUARD_LIVE=1`; not run in CI. Chainguard rebuilds images constantly and renames packages, so assert stable properties (an OpenSSL library has an openssl origin), not exact package names or counts.

## CI and releases

- `.github/workflows/ci.yml` runs gofmt (Linux only), vet and tests on Linux and Windows, `-race` on Linux, a Docker build plus `-version` smoke test, and a GoReleaser snapshot.
- `main` is protected by a ruleset that requires the CI jobs named `Test (ubuntu-latest)`, `Test (windows-latest)`, `Docker build` and `Release config check`. Renaming a job means updating the ruleset too. The repo owner has an admin bypass, so direct pushes print "Bypassed rule violations"; that's expected.
- Pushing a `v*` tag runs `release.yml`: GoReleaser builds linux/darwin/windows × amd64/arm64 archives and publishes a GitHub Release. The version is injected with `-X main.version`. Commits prefixed `docs:` or `test:` are left out of the release changelog.
