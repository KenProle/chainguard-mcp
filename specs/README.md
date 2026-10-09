# Specs

Features are developed spec-first. Each feature gets a numbered folder with three documents, written and reviewed in order:

| Document | Answers | Reviewed before |
|---|---|---|
| `spec.md` | What and why: user stories, testable acceptance criteria, scope | design starts |
| `design.md` | How: components, data flow, API changes, edge cases | tasks are planned |
| `tasks.md` | Ordered small tasks, each linked to acceptance criteria and tests | coding starts |

Specs are plain Markdown. User stories use "As a *role*, I want *goal*, so that *benefit*". Each acceptance criterion is one of:

- a **scenario**, for behavior the user triggers: **Given** *context*, **when** *action*, **then** *outcome*
- a **rule** in [EARS](https://alistairmavin.com/ears/) form, for things that must always hold:
  - "The *system* **shall** …" (always)
  - "**When** *trigger*, the *system* **shall** …" (in response to an event)
  - "**While** *state*, the *system* **shall** …" (during a state)
  - "**If** *unwanted condition*, **then** the *system* **shall** …" (errors and edge cases)

Criteria are numbered (`AC-1.3`) so tests, tasks and pull requests can refer to them; a test that covers a criterion includes its ID in its name. Every criterion must be testable; if it can't be checked by a test or a clear manual step, it isn't finished.

A spec's **Status** moves through Draft → Approved → Implemented. Change an approved spec deliberately, in its own commit, rather than letting the code drift from it.

| # | Feature | Status |
|---|---|---|
| 001 | [Variant comparison](001-variant-comparison/spec.md) | Approved |
| 002 | [License breakdown](002-license-breakdown/spec.md) | Approved |
