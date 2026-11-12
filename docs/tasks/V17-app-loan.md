# V17 — App: loans
Branch `feat/v-app-loan`. Owned: `internal/app/lending/` (new) or `internal/app/loan*.go`.

Protocol: `docs/tasks/_PROTOCOL.md` (binding). App layer: use cases with the Plan/Apply pattern (see `internal/app/plan.go`, existing use cases), `context.Context` first, imports only `domain`/`module`; tests against `internal/testutil/memstore` (table-driven + rapid where listed). Module gate: use cases of a module call `Require(<id>)` on the `module.Set` the service holds/receives (see how `internal/app/module.go` exposes it; add the minimal plumbing if missing). No CLI/TUI here. Read the merged domain types and store ports first. Coverage of touched app packages ≥ 85%. Plan: `docs/PLAN-v0.2.md` §3–4.

Module `loans` (requires recurring). Plan/Apply: add (computed or imported schedule), show/schedule, event (new version from date), installments → pending tx on the debit account; confirm splits principal (transfer to the loan liability account) and interest (+fees) to a dedicated category; amount mismatch → warning + proposed new rate (propose only). Invariant test: confirming an installment changes net worth by −(interest+fees).
