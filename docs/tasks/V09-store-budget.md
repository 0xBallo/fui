# V09 — Store: budgets
Branch `feat/v-store-budget`. Owned: store migration+queries+`repo_budgets*.go`, budget port, memstore `budgets*.go`.

Protocol: `docs/tasks/_PROTOCOL.md` (binding). Store layer: forward-only goose migration in its own commit with a migration test that runs it on a populated archive at the previous version; sqlc queries in `internal/store/sqlite/queries/`, regenerate with `go generate ./internal/store/...` (commit generated `sqlc/` + `schema.sql` + `testdata/schema.golden`); repo implementing a new app port (add the port interface to `internal/app/ports.go` and to `Repos`; implement it in `internal/testutil/memstore` too, plus the shared contract test). Read the merged domain types first. Migration number: take the next free number on `main` when you start; if another store task merged first, renumber on rebase. Coverage of `internal/store/sqlite` ≥ 80%. Plan: `docs/PLAN-v0.2.md` §2–4. Spec: tecniche.md "Modello dati e schema SQLite", "Persistenza e migrazioni".

1. Tables per the merged `domain.Budget` model: budgets (mode, target category or tag, amount, carry flag, thresholds), monthly overrides, envelope moves/assignments; one budget per category per period enforced by a UNIQUE constraint.
2. Repo: CRUD, budgets for a month, moves for a month range.
3. `reset --keep-settings` deletes budgets (they are data, not settings).
