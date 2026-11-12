# V08 — Store: rules
Branch `feat/v-store-rule`. Owned: store migration+queries+`repo_rules*.go`, rule port, memstore `rules*.go`.

Protocol: `docs/tasks/_PROTOCOL.md` (binding). Store layer: forward-only goose migration in its own commit with a migration test that runs it on a populated archive at the previous version; sqlc queries in `internal/store/sqlite/queries/`, regenerate with `go generate ./internal/store/...` (commit generated `sqlc/` + `schema.sql` + `testdata/schema.golden`); repo implementing a new app port (add the port interface to `internal/app/ports.go` and to `Repos`; implement it in `internal/testutil/memstore` too, plus the shared contract test). Read the merged domain types first. Migration number: take the next free number on `main` when you start; if another store task merged first, renumber on rebase. Coverage of `internal/store/sqlite` ≥ 80%. Plan: `docs/PLAN-v0.2.md` §2–4. Spec: tecniche.md "Modello dati e schema SQLite", "Persistenza e migrazioni".

1. Table `rules` (name UNIQUE NOCASE, priority, enabled, conditions JSON, actions JSON) with JSON validated through the domain decoder on write (reject invalid).
2. Repo: CRUD, `ListEnabledByPriority`.
3. `reset --keep-settings` keeps rules (extend the reset test).
