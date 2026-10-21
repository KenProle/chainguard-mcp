# project-site Specification

## Purpose

A one-page public site on GitHub Pages that shows developers using MCP clients what chainguard-mcp does and how to install it, without implying any affiliation with Chainguard or claiming more about image security than the tool's data supports.

PS-1 to PS-4 cover honesty (notice, wording, caveat, dates); PS-5 to PS-8 the content (install, workflow, limits, related tools); PS-9 to PS-11 the page itself (self-contained, accessible, links); PS-12 deployment. The example numbers come from `compare_with_upstream` run against the live endpoints on 2026-10-20 (amd64).

## Requirements

### Requirement: PS-1 Unofficial notice
The page SHALL say that the project is unofficial and not affiliated with Chainguard, both in its first screen (the hero) and in its footer. The page SHALL NOT use Chainguard's logo or other Chainguard brand images.

#### Scenario: PS-1.1 Notice in hero and footer
- **WHEN** the page is loaded
- **THEN** the hero and the footer each contain the words "not affiliated with Chainguard"

#### Scenario: PS-1.2 Only the project's own images
- **WHEN** the page's images are listed
- **THEN** each is one of the project's own screenshots or its own icon

### Requirement: PS-2 No security overclaims
The page SHALL NOT describe any image as secure, vulnerability-free or free of CVEs. Counts of zero SHALL be worded as known vulnerabilities in a comparison, not as an absence of vulnerabilities.

#### Scenario: PS-2.1 Banned wording
- **WHEN** the page's text is searched, ignoring case
- **THEN** it contains none of the word "secure" or the phrases "vulnerability-free", "vulnerability free", "CVE-free", "CVE free", "zero vulnerabilities", "zero CVEs", "no vulnerabilities" or "no CVEs"

#### Scenario: PS-2.2 A zero count
- **WHEN** the comparison shows `python:3.12-slim` with 69 known vulnerabilities against 0 for `cgr.dev/chainguard/python:latest`
- **THEN** the 0 appears in a column or label that says "known vulnerabilities"

### Requirement: PS-3 Caveat beside comparison numbers
Wherever the page shows vulnerability counts, the same section SHALL say that each image is graded by its own maintainers' advisories and that the counts are known vulnerabilities only, not a guarantee about either image.

#### Scenario: PS-3.1 Caveat in the comparison section
- **WHEN** the comparison section is read
- **THEN** it contains the caveat that each image is graded by its own maintainers and that the counts are known vulnerabilities only

### Requirement: PS-4 Dated examples
Every example of tool output and every screenshot on the page SHALL show the date it was captured, as a machine-readable date.

#### Scenario: PS-4.1 Comparison date
- **WHEN** the comparison of `node:20` (875 known vulnerabilities, 393 OS packages, 398.4 MB) with `cgr.dev/chainguard/node:latest` (1, 28, 66.0 MB) is shown
- **THEN** it is labelled with its capture date, 2026-10-20

#### Scenario: PS-4.2 Screenshot dates
- **WHEN** a screenshot of the web UI is shown
- **THEN** its caption gives the date it was taken

### Requirement: PS-5 Install for MCP clients
The page SHALL show how to install the server and add it to Claude Code, Cursor, Claude Desktop and VS Code. The binary SHALL be offered from the repository's latest release, and through `go install`, saying that a `go install` build has the MCP tools but no web UI. Install links SHALL NOT name a release version.

#### Scenario: PS-5.1 Each client
- **WHEN** the install section is read
- **THEN** it has a configuration for each of Claude Code, Cursor, Claude Desktop and VS Code

#### Scenario: PS-5.2 Latest release
- **WHEN** the download link is followed
- **THEN** it points at the repository's latest release, not a numbered one such as `v0.3.0`

#### Scenario: PS-5.3 go install
- **WHEN** the `go install github.com/KenProle/chainguard-mcp@latest` option is shown
- **THEN** it says that this build has no web UI

### Requirement: PS-6 Workflow shown as questions and answers
The page SHALL show the migration workflow as questions to an assistant with abridged answers, as text: finding an alternative, comparing with upstream, pinning by digest and migrating a Dockerfile. Every MCP tool or prompt the page names SHALL be one the server provides.

#### Scenario: PS-6.1 Workflow steps
- **WHEN** the workflow section is read
- **THEN** it names `find_alternative`, `compare_with_upstream`, `pin_image` and `migrate_dockerfile`

#### Scenario: PS-6.2 Only real tool names
- **WHEN** every tool or prompt name on the page is checked against the server's registered tools and prompts
- **THEN** each one exists

### Requirement: PS-7 Limits stated
The page SHALL state the tool's main limits: detail tools work for free-tier images only, whose public tags are `latest`-style; upstream comparisons cover Docker Hub images only; and only OS packages are compared.

#### Scenario: PS-7.1 Limits section
- **WHEN** the limits are read
- **THEN** they mention the free tier, Docker Hub and OS packages

### Requirement: PS-8 Related Chainguard tools
The page SHALL say that the tool complements Chainguard's own documentation MCP server, and link to it.

#### Scenario: PS-8.1 Docs MCP server
- **WHEN** the page is read
- **THEN** it links to Chainguard's documentation MCP server and describes this tool as complementing it

### Requirement: PS-9 Self-contained page
The page SHALL load only resources published with it: no third-party scripts, styles, fonts, images, analytics or trackers. It SHALL declare a Content Security Policy that allows only its own origin. Links to other sites are allowed.

#### Scenario: PS-9.1 No external resources
- **WHEN** the page's `src` and stylesheet `href` attributes are listed
- **THEN** every one is a relative path

#### Scenario: PS-9.2 Content Security Policy
- **WHEN** the page's head is read
- **THEN** it declares a Content Security Policy with `default-src 'self'`

### Requirement: PS-10 Works without JavaScript and for everyone
The page SHALL work fully without JavaScript, be usable at 375 px wide without horizontal scrolling, follow the system's light or dark mode, give every image a text alternative, and let a keyboard user reach every link and open each client's install instructions.

#### Scenario: PS-10.1 No scripts
- **WHEN** the page is read
- **THEN** it contains no `<script>` element

#### Scenario: PS-10.2 Narrow screen
- **WHEN** the page is shown 375 px wide
- **THEN** nothing scrolls horizontally except code blocks, which scroll within themselves

#### Scenario: PS-10.3 Dark mode
- **WHEN** the system uses a dark color scheme
- **THEN** the page uses dark colors with readable contrast

#### Scenario: PS-10.4 Keyboard install instructions
- **WHEN** a keyboard user moves to Cursor's install instructions and presses Enter
- **THEN** the Cursor configuration is shown

#### Scenario: PS-10.5 Image text
- **WHEN** the page's images are listed
- **THEN** each has non-empty alt text

### Requirement: PS-11 Links to full documentation
The page SHALL link to the repository, its README for full documentation, its releases and its license, rather than repeat the README's reference material.

#### Scenario: PS-11.1 Repository links
- **WHEN** the page's links are listed
- **THEN** they include the repository, the README, the latest release and the license

### Requirement: PS-12 Deployment
The site SHALL be published to GitHub Pages from `main` whenever a push changes the site or the screenshots it uses, and on demand. The published screenshots SHALL be taken from the repository's existing screenshots, not separate copies.

#### Scenario: PS-12.1 Push to main
- **WHEN** a commit on `main` changes `site/` or `docs/screenshots/`
- **THEN** the Pages workflow runs and publishes the site

#### Scenario: PS-12.2 Unrelated push
- **WHEN** a commit on `main` changes only Go code
- **THEN** the Pages workflow doesn't run

#### Scenario: PS-12.3 Screenshots published
- **WHEN** the published site is loaded
- **THEN** every screenshot it shows loads
