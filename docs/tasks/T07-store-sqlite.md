# T07 — SQLite store, migrations, sqlc, backup/restore

Branch `feat/store-sqlite`. Owned path: `internal/store/` (plus `sqlc.yaml` at repo root and `go.mod/go.sum`).
Read `AGENTS.md` first, then the merged ports: `internal/app/ports.go` (+ the documented repo contract on `Repos`),
`internal/app/reporting` (`Source`), `internal/backup` (file naming/rotation), `internal/testutil/memstore`
(`RunContract`). Your store must satisfy the **same contract tests** as memstore.
Spec refs: technical spec "Modello dati e schema SQLite", "Persistenza, migrazioni e backup", "Codice generato";
functional spec "Persistenza, backup, sicurezza e configurazione".

## Deliverables

1. **Schema** (MVP tables only): `accounts`, `transactions`, `transfer_groups`, `categories`, `fx_rates`,
   `import_batches`, `connector_account_refs` (+ whatever the app ports need). `STRICT` tables, FK on, amounts
   `INTEGER` minor units, dates `TEXT` ISO, enums `TEXT CHECK(...)`, user names `UNIQUE COLLATE NOCASE`, indexes
   `(account_id, date, id)`, `(category_id, date)`, `(fingerprint)`, `UNIQUE(account_id, external_id)`,
   `UNIQUE(currency, date)` on fx_rates, `created_at/updated_at` UTC. Migrations in
   `internal/store/sqlite/migrations` embedded with `embed.FS` and run by `github.com/pressly/goose/v3` at open;
   forward only; schema version in `PRAGMA user_version`; a DB newer than the binary is refused with a coded error
   (`db_newer_than_binary`). A golden file of the resulting schema (`sqlite_master` dump) under `testdata`.
2. **Queries** with `sqlc` (`sqlc.yaml` at repo root, engine sqlite, output committed, `//go:generate` so
   `make generate` regenerates it). No string-built SQL; parameters only.
3. **Connection**: `modernc.org/sqlite` (no cgo), pragmas `foreign_keys=ON`, `journal_mode=WAL`,
   `synchronous=FULL`, `busy_timeout=5000`; one writer connection + small read pool. DB file `600`, dir `700`.
4. **Store** implementing `app.Store` (`Read`/`WithTx`, one SQL transaction per `WithTx`, writes inside `Read`
   fail) and all repos with the documented error mapping (`domain.ErrNotFound`, `app.ErrAlreadyExists`,
   `app.ErrInUse`); keyset pagination on `(date, id)`; balance via SUM using the index (no stored balance).
   Also implement `reporting.Source`.
5. **Backuper**: `Backup(ctx, reason)` = `VACUUM INTO` a path from `backup.FilePath`, file `600`, then
   `backup.Rotate(dir, keep)`; `Restore(ctx, path)` = `PRAGMA integrity_check` + schema version check on the
   candidate, backup of the current DB (reason `restore`), copy into a temp file in the DB dir, atomic `rename`,
   reopen. Automatic `migrate` backup before running pending migrations on an existing DB.
6. Error mapping helpers for `db_locked`, `db_corrupted`, `migration_failed` (domain/app coded errors, exit code 2
   in the CLI later).

## Acceptance

- `memstore.RunContract` passes against SQLite (temp dir). Integration tests for constraints, migrations from an
  empty DB, `db_newer_than_binary`, backup/restore round trip, rotation, crash-safety of `WithTx` (error mid-tx
  leaves no rows), two concurrent writers serialize.
- Benchmark: `account list` balances and keyset page over a synthetic 50 000-transaction DB (`internal/testutil`
  fixture generator with a fixed seed is fine to add under `internal/store/.../testdata` or as a test helper).
- `golangci-lint run ./internal/store/...` clean; coverage ≥ 80 %.
