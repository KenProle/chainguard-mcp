# Proposal

## Why

The `migrate_dockerfile` prompt tells the assistant to replace `apt-get install` with `apk add` and to "look up package names where they differ". No tool can do that lookup, so the assistant guesses. A wrong guess fails the build (`postgresql-dev` and `python3` don't exist in Wolfi). Worse, some wrong guesses succeed at install time and only break later: `libssl-dev` mapped to `libssl3` installs the runtime library but not the headers. Of 59 packages commonly installed in Dockerfiles, 27 have the same name in Wolfi, and the rest need knowledge the assistant may not have.

## What Changes

- **New MCP tool `map_packages` and endpoint `GET /api/packages/map`.**
  - **Input:** up to 100 Debian/Ubuntu (apt) package names.
  - **Output:** one result per name, with:
    - a status: `exact`, `mapped`, `candidates` or `unknown`
    - the Wolfi package(s) to `apk add`, and any alternatives
    - where the answer came from
    - a note when a choice was made, such as picking a version stream
- **Every answer is checked against Wolfi's live package index** (`packages.wolfi.dev`, anonymous). The tool never returns a package name that isn't in the index.
- **Hints from dfc are embedded.** dfc is Chainguard's open source Dockerfile Converter, and the hints come from its curated mapping file (Apache-2.0).
  - The file is kept verbatim, pinned to an upstream commit and attributed.
  - Hints whose target no longer exists are ignored.
  - A `-dev` request is never answered with a runtime-only package when a matching `-dev` package exists.
- **Rules cover names that neither the index nor dfc knows.** They use the provides data in the index (shared libraries, pkg-config modules, commands) and the `python3-x` → `py3-x` naming. Version streams (`nodejs-24`, `postgresql-18-dev`) are ranked newest-built first, and older streams are listed as alternatives, never chosen silently.
- **New run mode `-refresh-mappings <file>`** updates the embedded dfc snapshot from GitHub at a pinned commit and exits. It is a manual step, and the result is reviewed and committed like any change.
- **The `migrate_dockerfile` prompt's package step calls `map_packages`.** It reports `candidates` and `unknown` results as needing manual checks.
- **`openspec/config.yaml` names the new sources:** `packages.wolfi.dev` for lookups, and GitHub for the refresh step only.

## Capabilities

### New Capabilities
- `package-mapping`: maps apt package names to Wolfi apk package names.
  - **Lookup:** the lookup order, statuses, sources and version-stream choice.
  - **Package index:** caching and failure behavior.
  - **Interfaces:** the MCP tool and API endpoint, and the migrate prompt's use of the tool.
  - **dfc snapshot:** the embedded snapshot and its refresh.

### Modified Capabilities
None. `web-api`'s requirements already cover a new endpoint: the index lists every endpoint (WA-2.1 allows endpoints added since), and the README table covers them (WA-4). The new endpoint follows them unchanged.

## Decisions

Made with the user on 2026-10-19, during exploration:
- **Chainguard data only.** The package index comes from `packages.wolfi.dev`, which is run by Chainguard. The anonymous `apk.cgr.dev/chainguard` index lists the identical 15,799 names.
  - *Rejected:* Debian's `Contents` and `Packages` indexes. They would say exactly which files a Debian package ships, but they add a second large download and a non-Chainguard source. This can be revisited if too many results come back `unknown`.
- **apt first.** dnf/yum come in a later change, from the Fedora section of the same snapshot.
- **Embed dfc's mapping file, validated against the live index.**
  - *Rejected:* fetching it at runtime from GitHub (a new runtime source).
  - *Rejected:* writing our own table (re-deriving about 60 judgment calls dfc already makes).
  - *Rejected:* trusting dfc as is (7 of its 73 Debian targets are missing from today's index, and 4 map `-dev` requests to runtime packages).
- **A strict, purpose-built reader for the YAML file.** *Rejected:* adding a YAML module for one file with a fixed shape.
- **Manual refresh, with drift reported by the live tests.** *Rejected:* a scheduled workflow opening a PR. PRs opened with the workflow token don't run CI, so that would need extra secrets.
- **MCP tool and API only.** *Rejected for now:* a web UI page. The natural UI is pasting a whole Dockerfile, which belongs to the later converter change.

Made in this proposal (for the user to review):
- **Up to 100 names per request,** each a valid Debian package name.
- **The index is cached for 6 hours, and a stale copy is served if a refresh fails.**
- **The x86_64 index only.** Package names are the same across architectures.

## Out of Scope

- dnf/yum/microdnf names, and Alpine names (which mostly match Wolfi already).
- Parsing `RUN` lines or rewriting Dockerfiles. That is the converter change, which will call this.
- Checking a migrated Dockerfile, by static review or by building it.
- Package versions. Results name packages; they don't pin versions.
- Chainguard's subscription-only packages, which the anonymous index doesn't show.
- A web UI.

## Impact

- **Go server:**
  - New package index client and mapping logic, with a `Service.MapPackages` method.
  - The embedded dfc snapshot and its license.
  - The `-refresh-mappings` run mode in `main.go`.
  - The `map_packages` tool and an updated migrate prompt in `tools.go`.
  - An `apiRoutes` entry in `web.go`.
- **API:** new `GET /api/packages/map`. Errors: `invalid_input` (400) and `source_failed` (502).
- **Docs:** a README row for the endpoint, a third-party notice for dfc, and the CLAUDE.md data-source notes.
- **Config:** two sources are added to `openspec/config.yaml`.
- **Dependencies:** none added. The standard library handles `tar`, `gzip` and the strict reader.
- **Web UI:** no changes.
