# Specs

Features are developed spec-first. Each feature gets a numbered folder with three documents, written and reviewed in order:

| Document | Answers | Reviewed before |
|---|---|---|
| `spec.md` | What and why: user stories, testable acceptance criteria, scope | design starts |
| `design.md` | How: components, data flow, API changes, edge cases | tasks are planned |
| `tasks.md` | Ordered small tasks, each linked to acceptance criteria and tests | coding starts |

Acceptance criteria are numbered (`AC-1.3`) so tests, tasks and pull requests can refer to them. Every criterion must be testable; if it can't be checked by a test or a clear manual step, it isn't finished.

A spec's **Status** moves through Draft → Approved → Implemented. Change an approved spec deliberately, in its own commit, rather than letting the code drift from it.

| # | Feature | Status |
|---|---|---|
| 001 | [Variant comparison](001-variant-comparison/spec.md) | Approved |
| 002 | [License breakdown](002-license-breakdown/spec.md) | Approved |
