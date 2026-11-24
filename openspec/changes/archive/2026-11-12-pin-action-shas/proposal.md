# Proposal

## Why

The workflows named their Actions by major tag (`actions/checkout@v7`). A tag can be moved or hijacked, which would change what runs in `release.yml` (`contents: write`) and the other workflows without any commit here. On 2026-11-12 all seven third-party Actions in the five workflows resolved to a commit on their latest release. PR 14 pins them; this change records the rule and adds the test that keeps it.

## What Changes

- Every `uses:` in `.github/workflows/*.yml` names a full 40-character commit SHA, followed by a comment with the release (`# v7.0.1`).
- A test fails when a `uses:` line is not pinned that way, so a new or edited workflow cannot undo it.
- DV-7's wording and its first scenario say "pinned by SHA" instead of "pinned by major tag"; Dependabot's behavior is unchanged (it bumps the SHA and the comment together).
- CLAUDE.md states the convention and how to look up a SHA.

## Decisions made with the user

- **Pin the Actions** now, as its own PR. Rejected: *leave major tags* (the supply-chain exposure above); *pin only the workflows holding secrets* (inconsistent, and the test would need exceptions).
- **Add the enforcing test** (code review finding 1 on PR 14). Rejected: *rely on review* (the rule erodes silently).

## Out of scope

- Pinning the Dockerfile's `cgr.dev/chainguard/*:latest` bases by digest. DV-7 still forbids Dockerfile version-update PRs; whether Dependabot can bump digests from `cgr.dev` is untested, so that is a separate decision.
- GoReleaser's own version (`version: "~> v2"` in `release.yml`) and `govulncheck@v1.9.0`, which are tool versions, not `uses:` lines.
- Changing Dependabot's monthly interval.
- `security-scan.yml` on the unmerged `add-security-scan-gate` branch, which must be pinned there before it merges; this test will fail until it is.

## Capabilities

### Modified Capabilities
- `dependency-vulnerability-checks`: DV-7 wording; new DV-8.

## Impact

`.github/workflows/{ci,history,pages,release,vulncheck}.yml`, `workflows_test.go`, `CLAUDE.md`. No code that ships changes.
