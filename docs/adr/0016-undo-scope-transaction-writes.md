# ADR 0016: Generic undo limited to transaction writes

* Status: Accepted
* Date: 2026-11-15

## Context and Problem Statement

v0.3 adds `fui undo` backed by an `op_log` journal. Journaling every write (categories, budgets, rules, tags) multiplies the restore logic and the ways a later write can make an entry stale.

## Decision Outcome

The journal records tx add, edit, rm, transfer, split and unsplit with a JSON snapshot; `fui undo` always shows the plan and asks for confirmation (`--yes` to skip). Categorize, tag and untag, budget, rule and security writes are not journaled; `import undo` remains the way to revert an import. An entry whose referenced rows were deleted later fails and rolls back.

### Consequences

* **Positive:** small, safe scope with one restore path.
* **Negative:** users cannot undo non-transaction writes; widening the scope needs a new ADR.
