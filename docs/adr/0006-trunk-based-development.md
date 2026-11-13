# ADR 0006: Trunk-based development with Conventional Commits and rebase merges

* Status: Superseded by [ADR 0013](0013-git-flow-with-develop-and-squash-merges.md)
* Date: 2026-11-11

## Context and Problem Statement

Long-lived feature branches and complex branch-merging workflows (such as GitFlow) lead to painful merge conflicts, delayed integration issues, and tangled git histories. When regressions occur, non-linear commit graphs make tools like `git bisect` difficult and unreliable. FUI requires a disciplined development workflow that maintains a constantly releasable `main` branch with clean, traceable history.

## Considered Options

1. **GitFlow:** Strict develop, feature, release, and hotfix branches with recurring merge commits.
2. **GitHub Flow with squash merges:** Simple branch model, but squash merges erase the logical granularity of atomic commits upon merging to `main`.
3. **Trunk-based development with atomic Conventional Commits and rebase merges:** Short-lived branches rebased onto `main`, where every single commit compiles, passes all tests, and represents an atomic logical change adhering to Conventional Commits.

## Decision Outcome

Chosen option: **Trunk-based development with atomic Conventional Commits and rebase merges**, because:
- The `main` branch remains clean, linear, and releasable at all times.
- Atomic commits isolate changes into discrete steps (e.g. refactoring separate from behavior change, schema migration separate from query change).
- Every commit on `main` compiles and passes `make check`, ensuring `git bisect` can locate regressions automatically without hitting broken intermediate commits.
- Conventional Commits (`<type>(<scope>): <summary>`) establish machine-readable commit semantics for automated changelog generation and version calculation via `git-cliff`.

### Consequences

* **Positive:**
  - Effortless regression debugging with `git bisect`.
  - Transparent code review through small, coherent, reviewable commits.
  - Automated release notes and semantic versioning.
* **Negative:**
  - Requires developers to maintain rebase discipline and clean up commit history before merging.
  - Intermediate commits must build and pass tests independently.
