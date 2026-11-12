# V16 — App: recurrences
Branch `feat/v-app-recur`. Owned: `internal/app/recurring/` (new) or `internal/app/recurring*.go`, `internal/app/importing/recur*.go` (new) + minimal hook.

Protocol: `docs/tasks/_PROTOCOL.md` (binding). App layer: use cases with the Plan/Apply pattern (see `internal/app/plan.go`, existing use cases), `context.Context` first, imports only `domain`/`module`; tests against `internal/testutil/memstore` (table-driven + rapid where listed). Module gate: use cases of a module call `Require(<id>)` on the `module.Set` the service holds/receives (see how `internal/app/module.go` exposes it; add the minimal plumbing if missing). No CLI/TUI here. Read the merged domain types and store ports first. Coverage of touched app packages ≥ 85%. Plan: `docs/PLAN-v0.2.md` §3–4.

Module `recurring`. Plan/Apply: add/list/edit/rm, `run --until` creates pending tx (idempotent: running twice creates nothing new), skip next, confirm pending → booked. Import: a booked imported tx matching a pending one (domain `MatchPending`) turns it booked instead of creating a duplicate; ignored when the module is off. Default: `run` is on demand only (open question), note in DEV.
