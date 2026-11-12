# V15 — App: budget use cases
Branch `feat/v-app-budget`. Owned: `internal/app/budgeting/` (new) or `internal/app/budget*.go`.

Protocol: `docs/tasks/_PROTOCOL.md` (binding). App layer: use cases with the Plan/Apply pattern (see `internal/app/plan.go`, existing use cases), `context.Context` first, imports only `domain`/`module`; tests against `internal/testutil/memstore` (table-driven + rapid where listed). Module gate: use cases of a module call `Require(<id>)` on the `module.Set` the service holds/receives (see how `internal/app/module.go` exposes it; add the minimal plumbing if missing). No CLI/TUI here. Read the merged domain types and store ports first. Coverage of touched app packages ≥ 85%. Plan: `docs/PLAN-v0.2.md` §3–4.

Module `budget` (`Require`). Plan/Apply: set (per category/tag, month override), status for a month (caps or envelopes via `budget.mode` config), assign/move (envelopes), copy from previous month. Feed the domain engine with tx rows (split-aware, pending as planned) from the store. rapid: the V02 invariants hold through the store.
