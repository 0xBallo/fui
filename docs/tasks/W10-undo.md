# W10 — Undo journal: store + app (0015)

Branch `feat/w-undo`. Owned: store `migrations/0015_op_log.sql`, `repo_oplog*`, `internal/app/undo*.go`, memstore, a minimal hook in `internal/app/{transaction,transfer,split}.go` apply paths. Protocol: `docs/tasks/_PROTOCOL.md`. Plan: `docs/PLAN-v0.3.md`.

Spec: F roadmap "annullamento generico". Design: after each successful Apply of tx add/edit/rm, transfer, split/unsplit, append an `op_log` row (id, kind, JSON inverse payload = snapshot needed to revert, created_at, undone_at nullable). Use case `Undo.Plan/Apply` reverts the most recent not-undone entry (LIFO), refuses if affected transactions are reconciled (`ErrReconciled`) or changed since (stale check), writes an automatic backup first only if existing backup policy for risky ops requires it. `import undo` is untouched.
Accept: add→undo removes; rm→undo restores same id/fields/tags/splits; edit→undo restores; two undos walk back; stale/reconciled refusal; Plan dry-run; property test on random op sequences.
