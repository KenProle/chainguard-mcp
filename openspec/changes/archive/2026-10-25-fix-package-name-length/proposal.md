# Proposal

## Why

A security scan (2026-10-24) found that package mapping accepts names of any length, and the version-stream rule's work grows much faster than the name: in a running server on 2026-10-25, a made-up hyphenated name of 4.8 KB took 20 seconds, and each doubling roughly tripled the time. `GET /api/packages/map` needs no login and is a plain GET, so a web page in the user's browser can send it even to a server bound to `127.0.0.1`. A few such requests keep every core busy.

## What Changes

- A name longer than 128 characters is refused as invalid input. Real Debian package names are far shorter.
- The version-stream rule scans the package index once per name instead of once per hyphen-separated prefix.
- Mapping stops between names when the request is cancelled.
- In HTTP mode the server limits request headers: 10 seconds to send them, 64 KB in size, and idle connections close after 2 minutes. There is no write timeout, because `/mcp` streams responses and comparisons can take a while.

## Capabilities

### New Capabilities
None.

### Modified Capabilities
- `package-mapping`: PM-9 adds the length limit. Its new scenario is PM-9.5.

## Decisions

Made with the user on 2026-10-25:
- **Fix the cost at its source and cap the input.** *Rejected:* rate limiting, which needs per-client state and doesn't help the browser case.

## Out of Scope

- Checking the `Host` header on `/api/` (the MCP library already does so on `/mcp`).
- The upstream comparison findings from the same scan (F2, F6).

## Impact

- **Go:** `packagemap.go` (`validateMapNames`, `MapPackages`, `ruleStreams`) and `main.go` (an `http.Server` with header limits instead of `http.ListenAndServe`).
- **API:** a name over 128 characters gets 400 `invalid_input`, as other invalid names do.
- **Dependencies:** none.
