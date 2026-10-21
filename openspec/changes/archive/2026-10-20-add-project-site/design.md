# Design

## Context

See proposal.md for why. The repository has no Pages site and no static-site tooling. The web UI lives in `web/` (Vite/React) and is embedded into the Go binary, which is unrelated to this page. `docs/screenshots/` holds seven screenshots captured on 2026-10-15, which the README uses. `.dockerignore` allowlists what the image build sees, so a new `site/` folder doesn't reach Docker. CI's required jobs are `Test (…)`, `Web`, `Docker build` and `Release config check`, and `go test ./...` runs on Linux and Windows.

**No API changes.** The Go server, MCP tools, JSON API and web UI are untouched. The only Go addition is a test file.

## Goals / Non-Goals

**Goals:**
- A page that is easy to edit by hand: one HTML file and one stylesheet.
- PS-1 to PS-6, PS-9, PS-10.1, PS-10.5 and PS-11 are checked by `go test ./...`, so CI enforces the wording rules on every push.
- The page can be previewed locally on Windows with no Node and no new tools.

**Non-Goals:**
- A shared design system with the web UI. The page borrows its feel (system fonts, a restrained palette, light and dark), not its code.
- Generating the page's numbers from tool output at build time. They're typed in by hand from a dated run (proposal: static, dated).

## Decisions

### Layout of the files

```
site/
  index.html     the page
  style.css      all styles (no inline style, per the CSP)
  icon.svg       the project's own simple icon (favicon); no Chainguard artwork
site_test.go     wording and structure checks (package main)
.github/workflows/pages.yml
```

The published site is assembled in the workflow:

```
site/*                      --> _site/
docs/screenshots/<used>.png --> _site/screenshots/
```

`index.html` references screenshots as `screenshots/<name>.png`. Copying them at deploy time keeps `docs/screenshots/` as the only copy (PS-12). The alternative was duplicating them into `site/`, where they'd drift when the README's screenshots are retaken. For local preview, the tasks give a short PowerShell step that assembles a scratch `_site` the same way. The page loads only relative files, so `_site/index.html` can be opened directly, with no server.

### Page sections and stable IDs

The page is written as controlled, flat markup so the test can check it with the standard library (`regexp`, `strings`, `html.UnescapeString`). No HTML parser dependency is added (`golang.org/x/net/html` would be new).

```
<header id="hero">     benefit line, one real find_alternative + pin_image exchange
                       for node:20 (no CVE numbers), install button, notice (PS-1)
<section id="why">     what an assistant gets wrong without live data: package names
                       (27 of 59 the same, measured 2026-10-19), libssl-dev -> libssl3
                       lacks headers, version tags need a subscription, non-root/no shell
<section id="install"> one <details> per client, Claude Code open (PS-5, PS-10.4)
<section id="workflow"> 4 question/answer pairs as text (PS-6)
<section id="ask">     "What else you can ask": one question per remaining tool, phrased
                       as an outcome, each naming its tool (checked by PS-6.2)
<section id="compare"> dated table: node:20, python:3.12-slim, nginx:latest
                       with the caveat (PS-2.2, PS-3, PS-4.1)
<section id="sources"> SBOMs, Chainguard advisory feed, OSV.dev, Wolfi index; grype check
<section id="ui">      3 <figure>s: catalog-map, history-view, upstream-comparison (PS-4.2)
<section id="limits">  free tier / latest, Docker Hub only, OS packages only (PS-7)
<section id="related"> complements Chainguard's docs MCP server, with link (PS-8)
<footer>               notice again, repo, README, latest release, license (PS-1, PS-11)
```

Sections are never nested, so a section's content is the text between `<section id="x"` and the next `</section>`.

**Install instructions use `<details>`, not tabs.** `<details>`/`<summary>` is keyboard-operable and announced correctly by screen readers with no script. The alternative, CSS-only radio "tabs", works by keyboard but exposes radio buttons, not a tablist, to assistive technology. Claude Code's is open by default.

Per client:
- **Claude Code:** `claude mcp add -s user chainguard -- /path/to/chainguard-mcp`, matching the README.
- **Cursor** (`~/.cursor/mcp.json`) and **Claude Desktop** (`claude_desktop_config.json`): `{"mcpServers": {"chainguard": {"command": "/path/to/chainguard-mcp"}}}`.
- **VS Code** (`.vscode/mcp.json`): `{"servers": {"chainguard": {"type": "stdio", "command": "/path/to/chainguard-mcp"}}}`.

Each client's file location and format is checked against that client's current documentation during implementation. Clients change these.

### The example numbers

From `compare_with_upstream` on 2026-10-20, amd64, abridged to the totals. Sizes are compressed download sizes in decimal MB, as the README describes them:

| Upstream | Known vulns | OS pkgs | Size | Chainguard `latest` | Known vulns | OS pkgs | Size |
|---|---|---|---|---|---|---|---|
| `node:20` | 875 | 393 | 398.4 MB | `node` | 1 | 28 | 66.0 MB |
| `python:3.12-slim` | 69 | 84 | 43.3 MB | `python` | 0 | 29 | 28.2 MB |
| `nginx:latest` | 187 | 148 | 63.9 MB | `nginx` | 0 | 23 | 8.5 MB |

The page also notes that Node.js 20 reached end of life on 2026-04-30, as the tool reports. The table is a real `<table>` with a `<caption>` holding the `<time datetime="2026-10-20">`, and its column headers say "Known vulnerabilities" (PS-2.2). There is no chart, so no text equivalent is needed.

The workflow answers are abridged from real output of the same day. They are not invented, and each answer block carries the same `<time>`.

### What `site_test.go` checks

| Check | Requirement |
|---|---|
| `id="hero"` and `<footer>` each contain "not affiliated with Chainguard" | PS-1.1 |
| every `<img src>` is `icon.svg` or `screenshots/<name>.png`, and that file exists in `docs/screenshots/` | PS-1.2, PS-12.3 |
| banned words (case-insensitive; `\bsecure\b` as a word, so "security" is fine) absent from the whole file | PS-2.1 |
| `id="compare"` contains the caveat phrases "graded by its own maintainers" and "known vulnerabilities" | PS-2.2, PS-3.1 |
| `id="compare"` and every `<figure>` contain `<time datetime="YYYY-MM-DD">` | PS-4.1, PS-4.2 |
| `id="install"` mentions each client; the release link is `releases/latest`; no `releases/tag/v` link; `go install` is followed by a no-web-UI note in the same block | PS-5 |
| every `<code class="tool">` name is a tool or prompt the server lists, via the in-memory MCP client (`mcpClient` with an `SBOMDir` set, so `save_sbom` is registered); the four workflow names are present | PS-6 |
| `id="limits"` mentions free tier, Docker Hub and OS packages | PS-7.1 |
| a link to `mcp.edu.chainguard.dev` or Chainguard's docs MCP page in `id="related"` | PS-8.1 |
| every `src`, and `href` on `<link rel="stylesheet">`/`rel="icon"`, is relative; the CSP meta has `default-src 'self'` | PS-9 |
| no `<script` | PS-10.1 |
| every `<img>` has non-empty `alt` | PS-10.5 |
| links to the repo, `#readme` or `README.md`, `releases/latest` and `LICENSE` | PS-11.1 |

PS-10.2 to PS-10.4 (narrow screen, dark mode, keyboard) and PS-12 (the workflow's triggers and publishing) are checked by hand in the verification task, the way the web UI's layout is checked in review.

Test names start with the scenario ID, per the project's convention (`t.Run("PS-2.1 no banned wording", …)`).

### Content Security Policy

Pages can't set response headers, so the page uses `<meta http-equiv="Content-Security-Policy" content="default-src 'self'; img-src 'self'; style-src 'self'; base-uri 'none'; form-action 'none'">`. That mirrors the web UI's policy and backs PS-9 at runtime as well as in the test. `frame-ancestors` doesn't work in a meta tag, which is acceptable for a static page.

### Link previews

The page sets `<title>`, `<meta name="description">` and Open Graph tags (`og:title`, `og:description`, `og:url`, and `og:image` pointing at the published `screenshots/upstream-comparison.png` by absolute URL). Shared links are how a page like this spreads, and a preview card helps. These are metadata the browser doesn't fetch, so PS-9 still holds. The test's relative-path rule covers `src` and `<link>` `href` only, not `<meta content>`.

### The Pages workflow

`.github/workflows/pages.yml`, named `Pages` (not a required check):

```
on:
  push: { branches: [main], paths: [site/**, docs/screenshots/**, .github/workflows/pages.yml] }
  workflow_dispatch:
permissions: { contents: read, pages: write, id-token: write }
concurrency: { group: pages, cancel-in-progress: false }
jobs:
  build:  checkout -> assemble _site (cp site/*, cp the used screenshots)
          -> actions/configure-pages -> actions/upload-pages-artifact (path: _site)
  deploy: needs build, environment github-pages (url from deploy step)
          -> actions/deploy-pages
```

Action versions follow the repo's habit of the newest major (`actions/checkout@v7`), checked at implementation time. The assemble step copies only the screenshots `index.html` references (found with `grep -o 'screenshots/[a-z-]*\.png'`), so unused images aren't published.

*Alternatives:* Jekyll's default Pages build (runs on every push, processes Markdown we don't have) and a `gh-pages` branch (a second copy that drifts, and pushes to it are blocked or bypass-noisy under the ruleset). `actions/deploy-pages` publishes an artifact and pushes no commits.

### Styling

System font stack (no web fonts, PS-9). Colors as CSS custom properties with a `prefers-color-scheme: dark` override. A single column with a max width of about 72ch, 16 px side padding, `pre { overflow-x: auto }` so only code blocks scroll at 375 px, and `img { max-width: 100%; height: auto }`. Focus styles stay visible. No Chainguard colors or logo; the accent is a neutral blue.

### Docs

- README: one line under the title linking to the site.
- CLAUDE.md: `PS` (`project-site`) in the prefix list, plus a short note on `site/`, the test and the workflow.
- `docs/visualization-ideas.md` is the web UI backlog. The "refresh the numbers daily" idea goes in proposal.md's out-of-scope list, not there.

### Dependencies

None added. The test uses only the standard library and the existing MCP SDK test helpers. The workflow uses GitHub's first-party Pages actions.

## Risks / Trade-offs

- [The numbers go stale] → They're labelled with their date (PS-4), like the README's screenshots. Re-run and update them when the screenshots are retaken.
- [A regex check on HTML can miss things a parser would catch] → The page uses controlled flat markup with stable IDs, and the checks are about content, not structure. Anything they can't see (layout, contrast, keyboard) is in the manual verification task.
- [The banned list is a blocklist] → It catches the phrases most likely to be written. The review step reads the copy against PS-2 as well.
- [Client config formats change] → Each snippet is checked against the client's docs during implementation. The page links to the README for the authoritative version.
- [First deploy fails because Pages isn't enabled] → The user enables Pages (Settings → Pages → Source: GitHub Actions) before the first push that touches `site/`. The workflow can be re-run by hand after that.
- [`go install` needs Go 1.27] → The install section says so (from `go.mod`). The release binary is the first option.

## Migration Plan

1. User enables Pages with source "GitHub Actions".
2. Merge to `main`; `pages.yml` deploys. If it fails, re-run with `workflow_dispatch`. Rolling back means reverting the commit, or unpublishing in Settings → Pages.
3. User sets the repo's homepage URL and topics.
