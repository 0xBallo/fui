# G05 — Undo for every transaction write

Branch `feat/g05-undo` · worktree `../fui-wt/g05` · TL: Claude Sonnet medium · deps: **G04 merged** (same files).
Read first: `docs/PLAN-gaps.md` (§2 P3; §6), `docs/tasks/_PROTOCOL.md` (v0.5 addendum), ADR 0016,
F: "Requisiti funzionali — CLI" (`fui undo`); T: undo journal (`op_log`).

## Owned paths

`internal/app/undo*.go`, `internal/app/{transaction.go,tag.go}` (journal calls only), `internal/app/rules/` (apply
journaling), `internal/app/recurring/` and `internal/app/lending/` (confirm journaling), store `repo_oplog*` and
migration only if the journal format needs it, `internal/cli/undo*.go`, `internal/tui/` (undo key + status line),
`docs/adr/` is NOT owned (G10 amends ADR 0016).

## Gaps

| Id | Gap | Evidence | Expected |
| --- | --- | --- | --- |
| a3-5a | categorize, tag/untag, rules apply, recurring confirm, loan confirm not journaled | `app/transaction.go:609-618`, `app/tag.go:298,347` | P3: each writes one journal entry (multi-row ops = one entry); `fui undo` reverts the last one |
| a3-5b | Later tag/untag makes earlier entries stale | `app/undo.go:115-126,233` | Stale check compares only the fields the entry wrote (per-entry field mask) |
| a3-5c | Restore fails when payee/batch/recurrence was deleted | `app/undo.go:146-149,380` | Clear the dangling reference, restore the row, add a warning to the plan |
| a3-5d | No undo in the TUI | no match in `internal/tui` | Global key (TL proposes the key to the Architect; must not clash with screen keys) showing the plan in a confirm modal, then applying; status line reports the result |

## Suggested MTs

1. app: field-masked snapshots + stale check + dangling refs (rapid: do → undo restores the exact pre-state for
   random sequences of journaled ops on memstore).
2. app: journal categorize/tag/untag/rules apply.
3. app: journal recurring/loan confirm (loan confirm undo removes the principal transfer and interest rows).
4. cli + tui: undo listing of the new kinds, TUI key + modal, testscript `g05_undo.txtar`, goldens.

## DoD

PLAN §6, plus the rapid do/undo property across all journaled kinds; reconciled rows still refuse undo.
