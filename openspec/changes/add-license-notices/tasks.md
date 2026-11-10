# Tasks

## 1. Go notices file

- [x] 1.1 Add `TestGoNoticesFile` and the generator it uses (design D1): run `go list -deps` on the main package, collect each non-main module's path, version and root `LICENSE*`/`COPYING*`/`NOTICE*`/`PATENTS*` files, add the standard library's `LICENSE` and `PATENTS` from `go env GOROOT` with no version, and write or compare `notices/go-notices.json`. On a difference, fail naming each missing, extra or changed module plus the command `UPDATE_FIXTURES=1 go test -run TestGoNoticesFile ./...` (LN-6). Generate the file with that command and commit it. Verify: the test passes; editing one version in the JSON makes it fail naming that module and the command (LN-6.1); the stdlib entry has no version field (LN-6.2).
- [x] 1.2 Embed `notices/` with `//go:embed notices` and add a strict decoder for it. Add tests named by scenario ID: "LN-1.1" (`golang.org/x/sync` v0.20.0 has both `LICENSE` and `PATENTS` text), "LN-1.2" (the go-sdk v1.8.0 entry starts with the MIT-to-Apache transition paragraph), "LN-1.3" (the stdlib entry has `LICENSE` and `PATENTS`), "LN-2.2" (no entry names `vite`, `vitest`, `typescript`, `jsdom` or `oxlint`). Verify: `go test -run 'TestLN' -v ./...` passes.

## 2. UI notices from Vite

- [x] 2.1 In `web/vite.config.ts`, set `build.license` to `{ fileName: 'licenses.json' }` (design D3). Verify: `npm --prefix web run build` writes `web/dist/licenses.json` listing exactly `@tanstack/query-core`, `@tanstack/react-query`, `react`, `react-dom`, `react-router` and `scheduler`, each with non-empty `text`; `web/dist/.gitkeep` is still there; `npm --prefix web run lint` and `typecheck` pass.
- [x] 2.2 Save that build's `licenses.json` as `testdata/vite-licenses-2026-11-10.json`, and add a strict Go decoder for it that rejects an unknown shape and an entry with empty `text` (naming the package). Add tests: "LN-1.5" (`react-router` 8.4.0, MIT, with text), "LN-2.1" (`@remix-run/route-pattern` and `cookie-es` absent), and the empty-text and wrong-shape errors. Verify: the tests pass.

## 3. Rendering and the -licenses flag

- [x] 3.1 Add `notices.go` rendering the plain text in design D4's order (header with version, stdlib, Go modules by path, dfc with the commit from `dfc-source.json` and `LICENSE-dfc`, UI packages by name), taking the Go notices, the mappings FS and the UI FS as inputs. Apply the UI rule (LN-7): `index.html` without `licenses.json` is an error; no `index.html` prints "The web UI is not included in this build." Add tests with `fstest.MapFS` UIs: "LN-1.4" (dfc entry names commit `84e5c1cd7648d4896dbc56c49a7a7bdd1638b0cd`), "LN-7.1", "LN-7.2", and that two renders are byte-identical. Verify: the tests pass.
- [x] 3.2 Add the build-info cross-check as a pure function comparing `debug.ReadBuildInfo()` deps with the Go notices' modules, and call it from rendering (design D2, LN-2.3). Test "LN-2.3" with fake lists: equal lists pass; a missing, extra or different-version module fails naming it; nil build info skips the check. Verify: the test passes.
- [x] 3.3 Add the `-licenses` flag to `main.go`, handled next to `-version`: print the notices to stdout and exit 0, or print the error to stderr and exit 1 (LN-3). Add test "LN-3.1" that builds the binary into a temp dir with `-ldflags "-X main.version=1.4.0"`, runs it with `-licenses`, and checks exit 0, the first line `chainguard-mcp 1.4.0 third-party notices`, and that the output contains `golang.org/x/sync`. Verify: the test passes on Windows and Linux (CI).
- [x] 3.4 Document the flag: README (run modes and a short "Third-party notices" section: where the notices are and how to print them) and CLAUDE.md (the flag in "Run modes", a `notices.go`/`notices/` bullet under Architecture with the regeneration command and the D2 cross-check, `LN` (`license-notices`) in the capability prefixes). Verify: the README command works with a local `go build` binary.

## 4. Shipping the file

- [ ] 4.1 `.goreleaser.yaml`: add the hook `sh -c 'go run -ldflags "-X main.version={{ .Version }}" . -licenses > THIRD_PARTY_NOTICES'` after the web build, and add `THIRD_PARTY_NOTICES` to `archives.files`; add `/THIRD_PARTY_NOTICES` to `.gitignore` (design D5, LN-4). Verify: `goreleaser check` passes (in CI's `Release config check`), and `git status` stays clean after the hook runs.
- [ ] 4.2 In `ci.yml`'s `Release config check` job, after the snapshot, check that `dist/chainguard-mcp_*_linux_amd64.tar.gz` contains `THIRD_PARTY_NOTICES` and that it equals the unpacked binary's `-licenses` output (LN-4.1). Verify: the job is green on the PR, and its log shows the check ran.
- [ ] 4.3 Dockerfile: in the build stage, run `go run . -licenses > /out/THIRD_PARTY_NOTICES` before cross-compiling; in the final stage, `COPY --from=build /out/THIRD_PARTY_NOTICES /THIRD_PARTY_NOTICES` (LN-5). In `ci.yml`'s smoke test, check that `docker run --rm chainguard-mcp -licenses` exits 0 and contains `golang.org/x/sync`, `react-router` and `dfc`, and that `docker cp` of `/THIRD_PARTY_NOTICES` from a created container equals that output (LN-3.2, LN-5.1). Verify: the `Docker build` job is green on the PR.

## 5. Verify end to end

- [x] 5.1 Build the UI and the binary locally (`npm --prefix web run build`, `go build -o chainguard-mcp.exe .`), run `./chainguard-mcp.exe -licenses`, and check the output against `go version -m chainguard-mcp.exe` (same modules and versions) and `web/dist/licenses.json` (same packages) (LN-1, LN-2.3). Then run `./chainguard-mcp.exe -http 127.0.0.1:8080` and confirm in the browser pane that the UI still loads and `/licenses.json` is served. Record the outputs in the PR description.
- [ ] 5.2 Confirm `go test ./...`, `go vet ./...`, `npm --prefix web run lint`, `npm --prefix web run typecheck`, `npm --prefix web test` and `openspec validate --all --strict` pass locally, and that all six required checks on the PR are green.

## Workflow follow-up

- Run `/code-review` and `/security-review` on the PR and post their results on it before merging; squash-merge.
- Archive the change with `/opsx:archive` and sync the `license-notices` spec.
