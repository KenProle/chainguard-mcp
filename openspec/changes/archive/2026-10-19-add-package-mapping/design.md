# Design

## Context

See proposal.md for why. The requirements are in `specs/package-mapping/spec.md` (PM-1 to PM-17).

What the code already has, and this design reuses:
- **`Service` as the single home for logic.** MCP tools (`tools.go`) and the `apiRoutes` table (`web.go`) are thin wrappers.
- **The catalog's caching shape** (`catalog.go`): a TTL, a mutex held during refresh, and the stale copy served when a refresh fails.
- **Embedded data via `//go:embed`** (`history_embed.go`).
- **Run modes that exit without serving** (`-record-history` and `-backfill-history` in `main.go`).
- **`*SourceError`** (`upstream.go`), already mapped to 502 `source_failed`, and `*InputError`, mapped to 400.
- **`compareAPKVersions`** (`vulns.go`) for apk version ordering.

Facts measured on 2026-10-19 that shape the approach:
- **The index file.** `packages.wolfi.dev/os/x86_64/APKINDEX.tar.gz` is 10.7 MB gzipped (43 MB of text), with 124,054 entries: every build of 15,799 package names. Each entry has `P` (name), `V` (version), `t` (build time), `o` (origin) and `p` (provides: `cmd:`, `so:`, `pc:` and bare virtual names such as `python3`).
- **dfc's `builtin-mappings.yaml`** (58 KB) has two top-level keys, `images:` and `packages:`. Under `packages:`, each distro maps a name to a list of names, and one entry is the empty list `[]`. The file contains no quoted strings, tabs, anchors or multi-line values.
- **A prototype of the lookup over 59 commonly installed apt packages** gave 27 `exact`, 14 `mapped`, 15 `candidates` and 3 `unknown` (`python3-venv`, `dnsutils`, `default-jre`).

## Goals / Non-Goals

**Goals:**
- One lookup path, shared by the tool, the API and, later, the converter.
- Deterministic results for a given index and snapshot, so tests can pin them.
- A small memory footprint: keep what the lookup needs, not the 124k entries.

**Non-Goals:**
- Debian-side knowledge of package contents (no Debian indexes).
- An arm64 index.
- Version pinning.

## Decisions

### 1. New files, and what changes in existing ones

- **`apkindex.go`:** `PackageIndex`, which fetches, parses and caches Wolfi's index. It follows `Catalog`: fields `HTTP`, `URL` and `TTL` (6h), a mutex, and a stale copy on failure.
- **`mappings.go`:** the strict dfc reader, the embedded snapshot (`//go:embed mappings`), and the `-refresh-mappings` run mode.
- **`packagemap.go`:** `Service.MapPackages` plus its input and output types, with the resolution order, the rules and stream ranking.
- **`mappings/`:** the committed files:
  - `dfc-builtin-mappings.yaml` (verbatim)
  - `dfc-source.json` (`{"commit": "<sha>", "fetched": "<date>"}`)
  - `LICENSE-dfc` (dfc's Apache-2.0 license)
- **Existing files:**
  - `Service` gains a `Packages *PackageIndex` field.
  - `tools.go` registers `map_packages` and edits step 4 of `migratePrompt`.
  - `web.go` gains one `apiRoutes` entry.
  - `main.go` gains the `-refresh-mappings` flag.
- **Go server and web API change:** one new endpoint, `GET /api/packages/map`. No web UI change.

### 2. What the parsed index keeps

The index is read as a stream (`gzip` → `tar` → the `APKINDEX` member → line scanner) and reduced to:
- `pkgs map[string]pkgInfo`, where `pkgInfo` holds the newest build's `version`, `built` and `origin`. Newest means the largest `t`.
- `provides map[string][]string`: each provided token, without its version constraint, to the package names that provide it. A lower-cased copy of the `so:` keys handles `libGL.so.1` versus `libgl1`.

That is about 16k plus 27k entries, a few MB, against 43 MB of text. Parsing happens once per refresh, under the lock. Concurrent first callers wait on that one download, the same as `Catalog`.

*Alternative:* keep every build per name. That's only needed for version pinning, which is out of scope.

### 3. Resolution, in `MapPackages`

For each distinct name (PM-4):
1. **`exact`:** the name is in `pkgs`.
2. **`mapped`:** dfc's Debian mapping for the name.
   - Keep only targets that are in `pkgs` (PM-2).
   - If the name ends in `-dev`, replace each non-`-dev` target with `<target>-dev` or `<origin>-dev`, whichever is in `pkgs` first (PM-5).
   - If no target is left, fall through, unless the mapping was `[]` (PM-4.5).
3. **`candidates`:** the rules, in this order. Every hit is recorded with its source string (PM-6).
   - a bare virtual provide equal to the name, then the one equal to its last hyphen-separated word (`golang-go` → `go`)
   - for `python3-<x>`, the name `py3-<x>`
   - for `lib<x>-dev`, the names `<x>-dev` and `lib<x>-dev`, then the provides `pc:<x>` and `pc:lib<x>`. These are tried with `<x>` as given and with its trailing version digits removed.
   - for `lib<x><N>`, the provides `so:lib<x>.so.<N>`, then the names `lib<x>` and `<x>`
   - the provides `cmd:<name>` and `cmd:<last hyphen segment>`
   - version streams of the name (`<base>-<ver>[-<suffix>]`)
   - all of the above again, for a name with a leading `default-` removed
4. **`unknown`:** nothing was found.

The first rule that has any hit decides; later rules are not consulted (pooling them would add unrelated `cmd:` matches).

### 4. Choosing among candidates (PM-7)

1. **Prefer the right kind of package:** `-dev` packages for a `-dev` request, and not `-minimal` variants otherwise. Fall back to all matches if that leaves none.
2. **Drop streams that can't be chosen:**
   - Prereleases: the version has `_alpha`, `_beta`, `_pre` or `_rc`, or is a `_git` snapshot toward a first release: `x.y.0_git…` or a dotless version (`3.15.0_git20261017`). A `_git` version of a later patch (`3.14.8_git20261014`) is not a prerelease, because Wolfi builds its stable python streams that way. (`_p` is a patch level, not a prerelease.)
   - Streams last built more than 180 days before the most recently built match.

   These stay in `alternatives`, so nothing is hidden.
3. **If the remaining matches are streams of one family** (the same name with only the version segment different), choose the highest version, compared with `compareAPKVersions` on the version segment.
4. **Otherwise, choose the most recently built.** Ties go to the higher version, then to name order.

Alternatives are the other matches in the same order, capped at 5. The `note` is set whenever a stream was chosen.

*Alternative:* newest build time alone. The prototype showed it picks `python-3.15`, a git snapshot, and on equal build times `python3-as-3.10`. Both are wrong.

### 5. Output types

```go
type MapPackagesInput struct {
    Names []string `json:"names" jsonschema:"apt (Debian/Ubuntu) package names, e.g. ['build-essential','libpq-dev']; at most 100"`
}
type PackageMappingResult struct {
    Results       []PackageMapping `json:"results"`
    IndexFetched  time.Time        `json:"index_fetched"`
    MappingCommit string           `json:"mapping_commit"` // dfc commit of the embedded snapshot
}
type PackageMapping struct {
    Name         string   `json:"name"`
    Status       string   `json:"status"` // exact | mapped | candidates | unknown
    Packages     []string `json:"packages"`
    Alternatives []string `json:"alternatives,omitempty"`
    Source       string   `json:"source,omitempty"`
    Note         string   `json:"note,omitempty"`
}
```

`Packages` is always present, and empty for `unknown` and for the `[]` mapping. The API splits `names` on commas and trims spaces. Validation follows PM-9.

### 6. The strict dfc reader

A line-based reader that accepts only:
- blank lines and `#` comments
- the top-level keys `images:` and `packages:`
- under `images:`, 4-space `key: value` lines (read, but unused for now)
- under `packages:`, 4-space `<distro>:` or `<distro>: {}`, then 8-space `<name>:` or `<name>: []`, then 12-space `- <name>`

Any other line is an error naming its line number. The reader returns the Debian section as `map[string][]string`.

It is used in three places:
- at startup, on the embedded file
- by `-refresh-mappings`, on the downloaded file, before writing anything
- by the offline test (PM-15.2)

A malformed embedded file can't reach a release, because the test fails first.

*Alternative:* add a YAML module (`go.yaml.in/yaml/v3`). It was rejected with the user: a dependency for one file with a fixed shape.

### 7. `-refresh-mappings <file>`

The run mode makes anonymous GitHub requests:
1. `GET https://api.github.com/repos/chainguard-dev/dfc/commits?path=pkg/dfc/builtin-mappings.yaml&per_page=1` gets the latest commit SHA.
2. It downloads `https://raw.githubusercontent.com/chainguard-dev/dfc/<sha>/pkg/dfc/builtin-mappings.yaml`, and `LICENSE` at the same SHA.
3. It parses the download with the strict reader, and refuses on error (PM-16.2).
4. It writes the YAML, `dfc-source.json` and `LICENSE-dfc` next to `<file>`.
5. It prints the Debian keys added and removed, and the mappings that changed.

It's used only by a developer, never at runtime. The anonymous API limit of 60 requests an hour is ample.

### 8. Tests and fixtures

**Fixture.** `testdata/apkindex-2026-10-19.tar.gz` is a trimmed real index: every build of the roughly 300 names the tests and the prototype sample touch, including all `nodejs-*`, `postgresql-*`, `python-3.*`, `python3-as-*` and `mysql-*` streams.
- It's produced by a test helper behind `UPDATE_FIXTURES=1` from a downloaded index, like the existing fixtures.
- `fakeChainguard` serves it, and `newFakeService` points `PackageIndex.URL` at it.

**Where the tests live:**
- Resolution tests in `unit_test.go` run directly against the parsed fixture.
- Tool tests go through `mcpClient`.
- API tests go in `web_test.go`.
- `TestAPIIndex` already enforces the route table and the README.

**Live test.** A `TestLive/package_mapping` subtest runs against the real index and prints the drift report (PM-17). It asserts only stable properties: `build-essential` maps to `build-base`, and `libpq-dev` gives a `postgresql-*-dev`.

### 9. Config and docs

- **`openspec/config.yaml`'s source list gains:**
  - Wolfi's package index (`packages.wolfi.dev`, for package lookups)
  - GitHub (dfc's mapping file, for `-refresh-mappings` only)
- **README:**
  - the endpoint row
  - the run mode
  - a "Third-party data" note crediting dfc (Apache-2.0), without implying any affiliation
- **CLAUDE.md** gets a data-source bullet in the existing style. It covers the index facts, the dfc snapshot, the `-dev` rule, stream choice, and that the index is Wolfi's (identical to the anonymous `apk.cgr.dev/chainguard` listing).

### 10. Dependencies

None added. `compress/gzip`, `archive/tar`, `bufio` and `embed` are in the standard library.

## Risks / Trade-offs

- **[The first call downloads 10.7 MB, which takes a few seconds]** → It happens once every 6 hours per process. The tool description says a first call can take a few seconds. There's no UI, so the progress rule for waits doesn't apply.
- **[The rules can be confidently wrong]**, for example a `cmd:` match on a common command name → Rule results are always `candidates` with their source, never `mapped`, and the prompt lists them for manual checking.
- **[The stream choice may not match the user's version]**, for example `nodejs-26` when the Dockerfile used Node 22 → The `note` says so, and the alternatives list the other streams. The converter can later read the version from the Dockerfile.
- **[dfc's file format changes upstream]** → The strict reader refuses it at refresh time, and the embedded copy keeps working.
- **[dfc's mappings go stale]** → Every target is validated against the live index. The live drift report shows what to refresh.
- **[Free `-dev` images might use a repository other than Wolfi]**, so mapped names might not install → A task checks `/etc/apk/repositories` in a free `-dev` image before the change is done. If it isn't Wolfi, the index URL changes. The approach stays the same.
- **[Wolfi's package list differs slightly between architectures]** → We use only names, which are almost always the same on both. The spec scopes this to the x86_64 index.

## Migration Plan

This is additive. A new tool, endpoint and run mode, and an edited prompt step. There is nothing to migrate, and rolling back means reverting the commit.

## Open Questions

- **The exact 180-day cutoff for "actively built".** This can be tuned during implementation against the fixture, without changing the spec. PM-7's scenarios pin the behavior that matters.
