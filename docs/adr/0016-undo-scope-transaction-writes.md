# ADR 0016: Generic undo covers every transaction write

* Status: Accepted
* Date: 2026-11-15 (amended 2026-11-25, v0.5)

## Context and Problem Statement

v0.3 adds `fui undo` backed by an `op_log` journal. Journaling every write (categories, budgets, rules, tags) multiplies the restore logic and the ways a later write can make an entry stale.

## Decision Outcome

The journal records every transaction write with a JSON snapshot: tx add, edit, rm, transfer, split and unsplit, categorize, tag and untag, rules apply, recurring confirm and loan confirm. `fui undo` (and `u` in the TUI) always shows the plan and asks for confirmation (`--yes` to skip). Budget, rule and security writes are still not journaled, and `import undo` remains the way to revert an import (including the automatic installment and recurrence confirmations the importer makes).

Entries that touch only some fields carry a field mask (category, tags, status, payee): the stale check compares and the restore overwrites only those fields, so later edits of other fields survive. An entry whose rows were removed, changed or reconciled since fails and rolls back. When a restored row references a payee, import batch or recurrence that no longer exists, the reference is cleared and a warning is shown.

### Consequences

* **Positive:** every way of changing a transaction can be reverted through one restore path.
* **Negative:** users still cannot undo non-transaction writes; the interest category created by a loan confirm survives its undo.
