# V10 — Store: recurrences
Branch `feat/v-store-recur`. Owned: store migration+queries+`repo_recurrences*.go`, recurrence port, memstore `recurrences*.go`.

Protocol: `docs/tasks/_PROTOCOL.md` (binding). Store layer: forward-only goose migration in its own commit with a migration test that runs it on a populated archive at the previous version; sqlc queries in `internal/store/sqlite/queries/`, regenerate with `go generate ./internal/store/...` (commit generated `sqlc/` + `schema.sql` + `testdata/schema.golden`); repo implementing a new app port (add the port interface to `internal/app/ports.go` and to `Repos`; implement it in `internal/testutil/memstore` too, plus the shared contract test). Read the merged domain types first. Migration number: take the next free number on `main` when you start; if another store task merged first, renumber on rebase. Coverage of `internal/store/sqlite` ≥ 80%. Plan: `docs/PLAN-v0.2.md` §2–4. Spec: tecniche.md "Modello dati e schema SQLite", "Persistenza e migrazioni".

1. Table `recurrences` with explicit rule columns (every, unit, day_of_month, start, end, next_date, template fields); `transactions.recurrence_id` (nullable FK, ON DELETE SET NULL) and `due_date` (nullable). The migration alters an MVP-critical table: test it on a populated archive and check FTS triggers still work.
2. Repo: CRUD, `Due(until)`, `PendingOf(recurrence)`.
