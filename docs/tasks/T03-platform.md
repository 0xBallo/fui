# T03 — Config, clock, logging, backup files, ADRs

Branch `feat/platform`. Owned paths: `internal/config/`, `internal/clock/`, `internal/logging/`,
`internal/backup/`, `docs/adr/`. Read `AGENTS.md` first. These packages must NOT import `internal/domain`
(it is being written in parallel); use plain Go types.
Spec refs: technical spec "Configurazione, log e sicurezza", "Persistenza, migrazioni e backup" (Backup),
"Decisioni tecniche e punti aperti"; functional spec "Persistenza, backup, sicurezza e configurazione".

## Deliverables

1. **`internal/config`** — `Config` struct with defaults in code: `currency` (EUR), `locale` (it-IT; allowed
   it-IT, en-US), `month_start_day` (1, range 1..31), `week_start` (monday), `theme` (terminal | high-contrast),
   `backup.keep` (10), `connectors.timeout` (120s), `import.profiles` (map of CSV profiles: date column, amount
   column OR debit/credit columns, description column, external id column, delimiter, encoding, date format,
   decimal separator, skip rows, target account).
   - TOML via `github.com/pelletier/go-toml/v2`, strict decoding: unknown keys → warnings (returned, not logged),
     invalid values → error naming the key.
   - Precedence flags > `FUI_*` env vars > file > defaults resolved in ONE function with a test per level.
     Env: `FUI_DB`, `FUI_CURRENCY`, `FUI_LOCALE`, `FUI_LOG`, `FUI_AGENT`, ... (document the list).
   - XDG paths: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, falling back to `~/.config/fui`,
     `~/.local/share/fui`, `~/.local/state/fui` on Linux AND macOS. DB default `<data>/fui.db`, backups
     `<data>/backups`. Home dir and env are injected (no direct `os.Getenv` inside the resolve logic).
   - `Get(key)` / `Set(key, value)` for `fui config get/set` (dotted keys), writing the TOML file with `600`
     perms and dir `700`.
2. **`internal/clock`** — `System` (only place allowed to call `time.Now()`) and `Fixed` for tests; method
   `Now() time.Time`.
3. **`internal/logging`** — `log/slog` JSON handler writing to `<state>/fui.log` (file `600`, dir `700`), level
   `warn` by default, `debug` with verbose / `FUI_LOG=debug`. Never logs to stdout. Sensitive values: domain types
   implement `slog.LogValuer` returning `[redacted]` and `fmt.Stringer` with the clear value; when
   `FUI_LOG_SENSITIVE=1` wrap the handler so that, before delegating, any unresolved `LogValuer` attr that also
   implements `fmt.Stringer` is replaced by its `String()`. Test with a local fake type.
4. **`internal/backup`** (file-level only, the SQL `VACUUM INTO` is done by the store later) — file name
   `fui-YYYYMMDD-HHMMSS-<reason>.db` with reasons `manual|import|migrate|bulk|restore`; create backup dir `700`;
   `Rotate(dir, keep)` deletes the oldest backups beyond `keep` (only files matching the naming pattern);
   `List(dir)`. Time comes from a parameter (no clock call inside).
5. **ADRs** `docs/adr/0001…0008` in short MADR format (context, options, decision, consequences) for the decisions
   listed in the technical spec table "ADR", plus `docs/adr/README.md` index. English.

## Acceptance

- Tests per precedence level, strict decode (unknown key warning, invalid value error with key name), XDG
  fallback, rotation keeps the newest `keep` files, permissions are `600`/`700`.
- `golangci-lint run ./internal/config/... ./internal/clock/... ./internal/logging/... ./internal/backup/...`
  clean; coverage ≥ 80 %.
