# Proposal

## Why

The release binaries and the Docker image carry third-party code whose licenses ask for their notices to go with every copy, and today almost none do. Checked on 2026-11-10 (Trim Reaper pass 6, `go version -m` on a fresh build and a Vite build with license output):

- **Go, compiled into the binary:** the Go standard library and runtime (BSD-3, plus `PATENTS`); `golang.org/x/oauth2`, `x/sync`, `x/sys` and `x/time` (BSD-3, the last three with `PATENTS`); `github.com/yosida95/uritemplate/v3` (BSD-3); `github.com/google/jsonschema-go`, `github.com/segmentio/encoding` and `github.com/segmentio/asm` (MIT); `github.com/modelcontextprotocol/go-sdk` v1.8.0 (Apache-2.0, with MIT for contributions not yet relicensed, as its LICENSE explains).
- **Embedded data:** Chainguard's dfc package mappings (Apache-2.0). `mappings/LICENSE-dfc` is embedded through `//go:embed mappings`, but nothing shows it.
- **Web UI bundle, embedded from `web/dist`:** `react`, `react-dom`, `scheduler`, `@tanstack/react-query`, `@tanstack/query-core` and `react-router` (MIT). The minifier strips their license comments. The only notice that survives is Tailwind CSS's banner at the top of the CSS file.

None of these has an Apache `NOTICE` file, so the license texts are all that's required. Release archives contain only the binary, `README.md` and `LICENSE`, and the Docker image contains only `/chainguard-mcp`. BSD-3 asks for the notice "in the documentation and/or other materials provided with the distribution", MIT for it to be included in all copies, and Apache-2.0 for a copy of the license. This is a reading of the license terms, not legal advice.

## What Changes

- The binary carries the notices for everything it contains: Go modules and the standard library, the dfc mappings and the web UI bundle.
- A new `-licenses` flag prints those notices and exits. It's the only practical way to read them from the Docker image, which has no shell.
- Release archives and the Docker image include a `THIRD_PARTY_NOTICES` file, generated from the same data the flag prints.
- The web UI build records the licenses of the packages it actually bundles, using Vite's built-in `build.license` option, so a package that is tree-shaken out isn't listed and a new one is listed automatically.
- The Go notices are generated from the module graph into a committed file, and a test fails when that file no longer matches `go.mod`, so a dependency change can't silently drop or keep a notice.
- No new dependencies.

## Decisions made with the user

- **How to read the notices:** a `-licenses` flag plus the `THIRD_PARTY_NOTICES` file in archives and the image. Rejected: *a "Licenses" link in the web UI footer* (not chosen; the flag and file cover every distributed copy) and *an MCP resource* (MCP clients rarely show such resources).
- **Approach:** generate the notices with the toolchain already in use (Vite's `build.license`, and `go list` from a test). Rejected: *Google's `go-licenses`* (another tool to install, pin and keep up to date, for a job a short local generator does).

## Out of scope

- A Licenses page or link in the web UI, and an MCP resource or tool for notices.
- License compliance of the Docker base image (`cgr.dev/chainguard/static`), which Chainguard documents in the image's own SBOM.
- Tailwind CSS's notice beyond the banner its build already keeps at the top of the CSS file.
- Notices for development-only tooling (Vite, Vitest, TypeScript, …), which isn't distributed.
- Checking licenses against an allow-list, or failing a build on a license type. The policy's license check when adding or upgrading a dependency stays manual.
- The project site, which ships no third-party code.

## Capabilities

### New Capabilities
- `license-notices`: the third-party license notices that travel with every distributed copy of the server (binary, release archive, Docker image), how they are read, and how they are kept in step with the dependencies.

### Modified Capabilities

None.

## Impact

- **Go:** `main.go` gets the `-licenses` flag; a new file builds the notices from an embedded, committed Go notices file, the embedded dfc license and the embedded UI's license file; a new test generates the Go notices file (`UPDATE_FIXTURES=1`) and fails when it's stale.
- **Web:** `web/vite.config.ts` turns on `build.license` with a JSON file name. The file lands in `web/dist`, which the binary already embeds, so it is also served as a static file by the web server.
- **Build and release:** the Dockerfile's build stage writes `THIRD_PARTY_NOTICES` and the final stage copies it in; `.goreleaser.yaml` writes it in a `before` hook and adds it to `archives.files`; CI's Docker smoke test checks the flag.
- **Docs:** README (the flag, and where notices live), CLAUDE.md (run modes, the regeneration command, the `LN` prefix).
- No API changes. No new dependencies.
