# Design

**No API changes.** The Go server, MCP tools and web API are unchanged. No Go or npm dependency is added.

## Decisions

**Pin format: `uses: owner/repo@<40 hex> # vX.Y.Z`.** This is the form Dependabot recognizes: it updates the SHA and the version comment together. SHAs are looked up live (`gh api repos/<owner>/<repo>/git/ref/tags/<tag>`, dereferencing annotated tags such as `github/codeql-action`) and never copied from memory.

**The test is a text check, like the others in `workflows_test.go`.** For every `.github/workflows/*.yml`, each `uses:` line must match `uses:\s+[\w.-]+/[\w./-]+@[0-9a-f]{40}\s+# v\d+(\.\d+)*\s*$`. Comments are not stripped for this check, because the version comment is part of the rule. Local (`./`) and `docker://` references would need an exception; there are none, and the test fails on them so one gets added deliberately.

**Dependabot stays monthly.** A pinned Action no longer picks up patch releases on its own, so fixes arrive with the monthly grouped PR. Rejected for now: weekly (more PRs to review; revisit with the Docker pinning decision).

## Risks

- **`security-scan.yml`** (unmerged) fails the new test until its three tag references are pinned. That is intended; whichever PR merges second pins them.
- A reviewer must still check that a Dependabot SHA bump matches the release in its comment; the test only checks the format.
