# Tasks

Commands are for Git Bash on Windows as in CLAUDE.md, except where PowerShell is named. Test names start with the scenario ID they cover. Example numbers come from `compare_with_upstream` on 2026-10-20 (amd64); see design.md for the approach and the table.

## 1. Test first

- [x] 1.1 Write `site_test.go` (package `main`) with the checks in design.md's "What `site_test.go` checks" table, one `t.Run` per scenario (`PS-1.1`, `PS-1.2`, `PS-2.1`, `PS-2.2`, `PS-3.1`, `PS-4.1`, `PS-4.2`, `PS-5.1`, `PS-5.2`, `PS-5.3`, `PS-6.1`, `PS-6.2`, `PS-7.1`, `PS-8.1`, `PS-9.1`, `PS-9.2`, `PS-10.1`, `PS-10.5`, `PS-11.1`, `PS-12.3`). Use only the standard library plus `mcpClient` for the tool and prompt list (with `SBOMDir` set so `save_sbom` is registered). Verify: `go test -run TestSite -v ./...` fails because `site/index.html` doesn't exist yet, and `go vet ./...` is clean.

## 2. The page

- [x] 2.1 Write `site/index.html`, `site/style.css` and `site/icon.svg` with the sections and IDs in design.md (hero, install, workflow, compare, sources, ui, limits, related, footer), the CSP meta tag, Open Graph tags and no `<script>`. Use the 2026-10-20 numbers from design.md's table and the 2026-10-15 date for the three screenshots (`catalog-map.png`, `history-view.png`, `upstream-comparison.png`). Write the workflow answers abridged from real tool output run the same day as the numbers, not invented (PS-1 to PS-11). Verify: `go test -run TestSite -v ./...` passes, and `go test ./...` passes.
- [x] 2.2 Check each client's install snippet against that client's current MCP documentation (Claude Code, Cursor, Claude Desktop, VS Code), and fix any file location or key that differs from design.md (PS-5.1). Verify: each snippet matches the client's docs, with the doc URL noted in the commit message.
- [x] 2.3 Read the page's copy against PS-2 and the proposal's honesty decisions, beyond what the banned list catches: nothing implies affiliation, endorsement or a security guarantee, and every number has its caveat nearby. Verify: a list of any phrases changed, in the task's completion note (or "none").
- [x] 2.4 Added in review (2026-10-20), after comparing with another tool's landing page: a `why` section on what an assistant gets wrong without live data, one real `find_alternative` and `pin_image` exchange for `node:20` in the hero (it shows the headline's promise; a first draft used `map_packages`, which the user found out of place), and a `ask` grid with one question per remaining tool. Verify: `go test -run TestSite ./...` passes (PS-6.2 checks the new tool names), and the page is checked at 375 px and desktop in light and dark mode.

## 3. Deployment

- [x] 3.1 Add `.github/workflows/pages.yml` as in design.md: triggers on pushes to `main` touching `site/**`, `docs/screenshots/**` or the workflow itself, plus `workflow_dispatch`; it assembles `_site` from `site/` and only the screenshots `index.html` references; it uses `configure-pages`, `upload-pages-artifact` and `deploy-pages` at their current major versions (PS-12). Verify: the workflow's assemble commands, run locally in Git Bash into a scratch folder, produce `index.html`, `style.css`, `icon.svg` and exactly three PNGs under `screenshots/`; `actionlint` passes if it's installed (the first real run is checked in the workflow follow-up).

## 4. Docs

- [x] 4.1 Add the site link under the README's title, and to CLAUDE.md: `PS` (`project-site`) in the capability prefix list, and a short note on `site/`, `site_test.go` and `pages.yml` (that the page is hand-written, numbers are dated, and the test enforces the wording). Verify: `go test ./...` (which includes `TestAPIIndex`'s README checks) still passes, and both files mention the site.

## 5. Verification

- [x] 5.1 Run `gofmt -l . && go vet ./...`, `go test ./...`, the web lint, typecheck and tests (unchanged UI, as a guard) and `openspec validate --all --strict`. Verify: all pass.
- [x] 5.2 Preview the assembled page locally. In PowerShell: `Remove-Item -Recurse -Force _preview -ErrorAction SilentlyContinue; New-Item -ItemType Directory _preview/screenshots | Out-Null; Copy-Item site/* _preview; Copy-Item docs/screenshots/catalog-map.png,docs/screenshots/history-view.png,docs/screenshots/upstream-comparison.png _preview/screenshots`, then open `_preview/index.html` in the browser pane (delete `_preview` afterwards; it must not be committed). Check at 375 px wide and on desktop, in light and dark mode, and with the keyboard only: tab through every link, and open Cursor's install instructions with Enter. Check the console has no CSP errors. Verify: screenshots of each mode and width, and a note that PS-10.2, PS-10.3 and PS-10.4 hold.

## Workflow follow-up

- User: enable Pages (Settings → Pages → Source: GitHub Actions) before the first push that touches `site/`.
- After pushing, confirm the `Pages` run deployed, open `https://kenprole.github.io/chainguard-mcp/`, check every screenshot loads (PS-12.3), and confirm a Go-only push doesn't trigger the workflow (PS-12.2).
- User: set the repo's homepage URL to the site and add topics (`mcp`, `mcp-server`, `chainguard`, `containers`, `sbom`).
- Review the change before archiving, and confirm CI passed on the implementing commit.
- Archive with `/opsx:archive`, which creates the `project-site` living spec.
- When the ghcr.io image change lands, add its one-line `docker run` install to the page.
