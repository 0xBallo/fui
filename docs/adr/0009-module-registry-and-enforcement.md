# ADR 0009: Static module registry and end-to-end module enforcement

* Status: Accepted
* Date: 2026-11-12

## Context and Problem Statement

FUI MVP v0.1 introduces optional feature modules while keeping a single binary and one archive
schema. We need a deterministic source of truth for module identities, dependencies, defaults, and
presets, plus consistent behavior when a module is disabled across CLI, TUI, and use cases.

## Considered Options

1. **Dynamic module metadata from config/plugins:** flexible but runtime-fragile, hard to validate,
   and inconsistent with a local-first, deterministic core.
2. **Ad-hoc module checks per layer:** simple to start, but duplicates dependency logic and drifts
   between app, CLI, and TUI.
3. **Static in-code module registry with immutable sets and required checks at command/use-case
   boundaries:** one canonical dependency graph used everywhere.

## Decision Outcome

Chosen option: **Static in-code module registry with immutable sets and required checks**, because:
- `internal/module` keeps module IDs, dependency graph, defaults, and preset membership in one
  testable place.
- Module activation/deactivation can be computed as dependency closure (`enable`) and dependent
  impact (`disable`) before apply.
- The app gate (`Require`) and CLI command annotation enforce `module_disabled` consistently,
  while hidden commands stay out of `help`/`help --json`.

### Consequences

* **Positive:**
  - Deterministic module behavior across layers and environments.
  - Validation catches invalid `modules.enabled` config at startup.
  - Presets (`expense`, `full`, `custom`) are explicit CLI contract values.
* **Negative:**
  - Adding or changing a module requires code and tests, not just config.
  - Command registration must keep module annotations complete to avoid drift.
