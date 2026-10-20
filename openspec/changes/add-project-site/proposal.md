# Proposal

## Why

The only way to learn what chainguard-mcp does is to find the repository and read a long README (the repo has 1 star and no topics). Developers who already use Claude Code, Cursor or other MCP clients are the people the tool helps most, and they need a short page that shows the payoff in a minute and gets them to install it. GitHub Pages hosts that for free, next to the code.

## What Changes

- **A one-page site in `site/`**, published at `https://kenprole.github.io/chainguard-mcp/`. It is plain HTML and CSS with no build step, and it works without JavaScript. It has:
  - **A hero** with a one-line benefit, the install steps, and the "unofficial, not affiliated with Chainguard" notice.
  - **Install and config snippets** for Claude Code (`claude mcp add`), Cursor, Claude Desktop and VS Code. The binary comes from the latest GitHub Release, or from `go install github.com/KenProle/chainguard-mcp@latest` (MCP tools only; that build has no web UI).
  - **"Ask your assistant"**: a few questions and answers written as text, following the migration workflow (`find_alternative`, `compare_with_upstream`, `pin_image`, the `migrate_dockerfile` prompt).
  - **Dated example output**: the upstream comparison for `node:20`, `python:3.12-slim` and `nginx:latest` from 2026-10-20, with the "each image is graded by its own maintainers; known vulnerabilities only" caveat next to the numbers.
  - **What the tool reads**: signed SBOMs, Chainguard's advisory feed, OSV.dev, Wolfi's package index. Also the check against grype.
  - **The web UI, briefly**: three existing screenshots with their capture date.
  - **Limits**: the free tier is `latest` only, comparisons are Docker Hub only, and only OS packages are compared.
  - **How it relates to Chainguard's own docs MCP server**: it complements it.
  - **Links to the README** for everything else.
- **A `pages.yml` workflow** deploys `site/` plus the screenshots it uses. It runs on pushes to `main` that touch them, and on manual dispatch.
- **A Go test (`site_test.go`) in `go test ./...`** checks the page's wording and structure: the notice, no banned security claims, a date on every example, and no third-party resources. CI enforces the rules with no new job.
- **README and CLAUDE.md** link to the site. CLAUDE.md adds the `PS` prefix and describes `site/`.

## Capabilities

### New Capabilities
- `project-site`: the public landing page. It covers its content rules (unofficial notice, honest wording, dated examples, install paths), its self-containment and accessibility, and how it's deployed.

### Modified Capabilities
None. No tool, API or web UI behavior changes.

## Decisions

Made with the user on 2026-10-20, during exploration:
- **Audience: developers already using Claude Code, Cursor and similar MCP clients.** The call to action is installing the tool. MCP comes first and the web UI second.
  - *Rejected:* platform and security teams evaluating Chainguard. That leads with CVE evidence and turns the page into a Chainguard sales sheet.
- **Lead with what the tool does, not Chainguard's CVE numbers.** The numbers appear as dated example output with the caveat beside them, so the page doesn't read as an unofficial Chainguard ad.
- **Page only, using the existing install paths.** Publishing a container image to `ghcr.io` is a separate change, proposed as its own session. The page gets the one-line `docker run` install when that lands.
  - *Rejected:* waiting for a one-line install before shipping the page.
- **Static, dated numbers** from a live run on 2026-10-20. This follows the README's "Screenshots are from …" convention.
  - *Rejected for now:* a daily workflow that refreshes the comparison numbers (more moving parts, and it would share Docker Hub's anonymous limit with the history job). Kept as a backlog idea.
  - *Rejected:* a hosted live demo. It needs a server, and Docker Hub's anonymous rate limit is per IP.
- **A small OpenSpec capability with a Go wording test**, so the rules most likely to slip on a marketing page are enforced in CI.
  - *Rejected:* a plain docs change with no spec.
- **Questions and answers as text, not chat screenshots.** Text is readable in both themes and searchable, and it doesn't borrow a client's branding.
- **Say that it complements Chainguard's docs MCP server** (`mcp.edu.chainguard.dev`), rather than ignore it.
- **Plain HTML and CSS in `site/`, deployed with `actions/deploy-pages`.**
  - *Rejected:* reusing the Vite/React app (a build step for a static page).
  - *Rejected:* Jekyll or a `gh-pages` branch (an extra toolchain, or a branch that drifts).
  - *Rejected:* the `docs/` folder, which already holds the backlog and screenshots.
- **A short page that links to the README**, so the two don't drift apart.
- **No analytics.** Awareness is measured with GitHub's Traffic page (views and referrers), stars and release downloads.

Made in this proposal (for the user to review):
- **The screenshots are copied from `docs/screenshots/` into the deploy by `pages.yml`**, not duplicated in `site/`.
- **The page has a `Content-Security-Policy` meta tag** like the web UI's (`default-src 'self'`). Pages can't set headers.
- **Install snippets link to the latest release**, not a version number, so they don't go stale with each release.

## Out of Scope

- Publishing a container image (`ghcr.io`), Homebrew/Scoop packages, or an MCP Registry listing.
- Refreshing the numbers automatically, and any live demo or hosted instance.
- A custom domain, analytics or tracking.
- Repository settings: enabling Pages (source: GitHub Actions), the homepage URL and topics. The user changes these by hand; the tasks list them.
- Any change to the Go server, MCP tools, JSON API or web UI.

## Impact

- New: `site/index.html`, `site/style.css`, an icon, `site_test.go`, `.github/workflows/pages.yml`.
- Changed: `README.md` (link to the site), `CLAUDE.md` (the `PS` prefix and the site).
- No new Go or npm dependencies. The Docker image is unaffected (`.dockerignore` excludes `site/`). The new workflow is not a required check, so the `main` ruleset doesn't change.
