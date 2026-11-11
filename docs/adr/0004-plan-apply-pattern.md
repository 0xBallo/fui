# ADR 0004: Plan/Apply pattern for all write operations

* Status: Accepted
* Date: 2026-11-11

## Context and Problem Statement

Modifying financial records requires extreme care. Users running CLI commands or interactive TUI actions need clear previews of changes before committing them. Furthermore, autonomous AI agents operating on personal financial databases must never execute irreversible destructive operations blindly without explicit validation and preview capabilities.

## Considered Options

1. **Direct mutation with rollback on failure:** Execute SQL statements immediately, attempting rollback if an error occurs.
2. **Implicit Unit of Work with dirty tracking:** Maintain modified object graphs in memory and flush on commit.
3. **Explicit two-phase Plan/Apply pattern:** Separate each mutation use case into a pure planning phase that builds a detailed, human- and machine-readable execution plan, followed by an atomic apply phase that executes the plan in a single transaction.

## Decision Outcome

Chosen option: **Explicit two-phase Plan/Apply pattern**, because:
- **Planning is pure and non-destructive:** The `Plan` step computes diffs (transactions to add, accounts to update, categories to assign), validates all domain invariants, and checks for duplicates without opening write transactions.
- **Universal dry-run and preview support:** The CLI `--dry-run` flag and TUI confirmation modals directly consume the output of the `Plan` phase without duplicate implementation.
- **Agent safety:** Autonomous agents can call the planning API to inspect diffs and verify invariants before issuing an apply instruction.
- **Atomic execution:** The `Apply` step executes the pre-computed plan inside a single SQLite transaction preceded by an automatic backup.

### Consequences

* **Positive:**
  - Standardized preview diffs across CLI, TUI, and scripting interfaces.
  - Testable planning logic independent of database state and transaction locks.
  - Guaranteed transactional atomicity: all operations succeed or none do.
* **Negative:**
  - Requires authoring two discrete steps (Plan and Apply) for every mutating use case.
  - Increases the number of domain transfer objects representing mutation plans.
