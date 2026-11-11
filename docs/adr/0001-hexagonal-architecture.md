# ADR 0001: Hexagonal architecture with pure domain and depguard enforcement

* Status: Accepted
* Date: 2026-11-11

## Context and Problem Statement

Personal finance applications require high precision, rigorous data validation, and verifiable business logic that remains independent from database drivers, terminal UIs, or external connectors. Without strict architectural boundaries, persistence details, I/O side-effects, and framework abstractions inevitably bleed into core financial calculations, impeding testing and long-term maintenance.

## Considered Options

1. **Layered N-tier architecture:** Domain models coupled directly with database entities (e.g. ORM tags) and shared across all layers.
2. **Package-by-feature structure without boundary enforcement:** Packages organized by domain concepts but allowed to import one another pragmatically.
3. **Hexagonal architecture (ports and adapters) with linter-enforced dependency rules:** Pure domain package importing only standard library, application layer exposing ports and use cases, adapters implementing ports, with boundaries strictly validated by `depguard`.

## Decision Outcome

Chosen option: **Hexagonal architecture with pure domain and depguard enforcement**, because:
- `internal/domain` remains completely deterministic, containing pure domain logic without I/O, system clock dependencies, floating-point math, or global variables.
- `internal/app` encapsulates application use cases and orchestrates domain entities through ports (interfaces), preventing adapter leaks.
- Adapters (`internal/store`, `internal/connector`, `internal/cli`, `internal/tui`) depend inwards on `app` and `domain`. CLI and TUI are explicitly forbidden from importing persistence adapters.
- Rules are enforced automatically by `depguard` and `forbidigo` in `.golangci.yml`, ensuring architectural compliance in CI.

### Consequences

* **Positive:**
  - Fast, deterministic unit tests for financial calculations without mock frameworks.
  - Pluggable storage and UI interfaces (e.g., swapping CLI, TUI, or mock stores).
  - Clear separation of concerns and zero risk of accidental network or disk I/O in domain logic.
* **Negative:**
  - Requires mapping between domain models and persistence or protocol representations.
  - Increases the number of interfaces and package boundaries.
