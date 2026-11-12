# V14 — App: rules use cases and import hook
Branch `feat/v-app-rule`. Owned: `internal/app/rules/` (new) or `internal/app/rule*.go`, `internal/app/importing/rules*.go` (new) + the minimal hook call in the pipeline.

Protocol: `docs/tasks/_PROTOCOL.md` (binding). App layer: use cases with the Plan/Apply pattern (see `internal/app/plan.go`, existing use cases), `context.Context` first, imports only `domain`/`module`; tests against `internal/testutil/memstore` (table-driven + rapid where listed). Module gate: use cases of a module call `Require(<id>)` on the `module.Set` the service holds/receives (see how `internal/app/module.go` exposes it; add the minimal plumbing if missing). No CLI/TUI here. Read the merged domain types and store ports first. Coverage of touched app packages ≥ 85%. Plan: `docs/PLAN-v0.2.md` §3–4.

Module core. 1. Plan/Apply: rule add/edit/rm/list, `test` (which rule would act on a given text/tx, no writes), `apply` on existing tx with filter + `--dry-run` plan. 2. Import step 4: after dedup, apply enabled rules; plan shows rule effects per row. Re-apply is idempotent.
