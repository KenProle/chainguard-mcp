# Specs (moving to OpenSpec)

This project is moving to [OpenSpec](https://github.com/Fission-AI/OpenSpec) for spec-driven development. New and migrated work lives in [`openspec/`](../openspec): living specs in `openspec/specs/`, proposed changes in `openspec/changes/`.

The variant comparison is the trial migration. Once it has gone through the full OpenSpec cycle (design, tasks, implementation, archive), the license breakdown moves too and this folder is removed.

| # | Feature | Status |
|---|---|---|
| 001 | Variant comparison | Moved to [`openspec/changes/add-variant-comparison`](../openspec/changes/add-variant-comparison) |
| 002 | [License breakdown](002-license-breakdown/spec.md) | Approved; moves to OpenSpec after the trial |

The conventions used in 002 carry over to OpenSpec almost unchanged: requirements use **SHALL**, scenarios use **WHEN** / **THEN**, and tests are named after the requirement or scenario they cover.
