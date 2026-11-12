# T18 — `fui reset` (backup reason, store rebuild, use case, CLI)

Branch `feat/reset`. Owned paths: `internal/backup/` , `internal/store/sqlite/reset.go` + test (new),
`internal/app/reset.go` + test (new), `internal/app/ports.go` (one new port method/interface only),
`internal/testutil/memstore/` (implement the new port), `internal/cli/reset.go` + test (new), registration
line in `internal/cli/root.go`, `internal/cli/testdata/` goldens it changes, `cmd/fui/` wiring only.
Protocol: `docs/tasks/_PROTOCOL.md`.

Spec: funzionali.md "Reset dell'archivio"; tecniche.md CLI "Reset", Persistenza "Reset" and "Backup",
invariant "`fui reset --keep-settings` conserva ...".

1. `internal/backup`: reason `reset`; `Rotate` never deletes `reset` backups (they don't count toward `keep`).
2. Store: build the fresh archive in a temp file in the same directory (migrations + default categories, reuse
   the init code path), with `--keep-settings` copy the settings that exist today (categories,
   connector-account associations; tags/rules/import profiles do not exist yet — note it in GAPS), create the
   `reset` backup, then atomic `rename` over the archive and reopen. Crash-safety: the old archive stays intact
   if any step before rename fails (test it).
3. App: `PlanReset(ctx, ResetRequest{KeepSettings bool})` lists what is deleted (counts per entity) and what is
   kept; `Apply` runs it. If no archive exists, delegate to init.
4. CLI `fui reset [--keep-settings] [--dry-run] [--yes]`: in a TTY without `--yes`, confirmation requires typing
   `reset` (not y/N); without TTY or with `FUI_AGENT=1` and no `--yes` → `confirmation_required`. Config file
   untouched. Update `help --json` golden.
5. Tests: property (rapid) for the keep-settings invariant on the real SQLite store; CLI tests with memstore.
