# V18 — App: reconciliation
Branch `feat/v-app-recon`. Owned: `internal/app/reconciling/` (new) or `internal/app/reconcile*.go`, the confirmation check in `internal/app/transaction.go` edit path.

Protocol: `docs/tasks/_PROTOCOL.md` (binding). App layer: use cases with the Plan/Apply pattern (see `internal/app/plan.go`, existing use cases), `context.Context` first, imports only `domain`/`module`; tests against `internal/testutil/memstore` (table-driven + rapid where listed). Module gate: use cases of a module call `Require(<id>)` on the `module.Set` the service holds/receives (see how `internal/app/module.go` exposes it; add the minimal plumbing if missing). No CLI/TUI here. Read the merged domain types and store ports first. Coverage of touched app packages ≥ 85%. Plan: `docs/PLAN-v0.2.md` §3–4.

Module core. Plan/Apply `reconcile <account> --balance --date`: difference, unreconciled list; closes (marks reconciled) only at zero difference. Editing/deleting a reconciled tx requires an explicit flag in the request (error code `reconciled` otherwise); `import undo` keeps refusing reconciled tx.
