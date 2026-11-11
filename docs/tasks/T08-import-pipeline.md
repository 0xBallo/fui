# T08 — Import/export pipeline, connector runner, CSV connector

Branch `feat/import-pipeline`. Owned paths: `internal/app/importing/`, `internal/connector/`. You may extend
`internal/app/ports.go` (ImportRepo/TransactionRepo) **and** `internal/testutil/memstore` minimally if the pipeline
needs it — list every port change in your report (the SQLite store, T07, is built in parallel and will be aligned).
Read `AGENTS.md` first, then the merged `internal/domain` (incl. dedup/fingerprint/transfer-matching services),
`internal/app` (ports, Plan/Apply, `Backuper`), `pkg/fuiproto` (decoder, validator, records), `internal/config`
(CSV profiles).
Spec refs: technical spec "Connettori e protocollo fui/1" (interface, runner, stream validation, import pipeline
steps 1–8); functional spec "Import, export e riconciliazione", "Formato FUI" (incl. import/export in FUI format).

## Deliverables

1. **Sources** (`internal/connector`): a `Source` returning `iter.Seq2[fuiproto.Record, error]`:
   - `.fui.jsonl` file source;
   - built-in **CSV connector** driven by a `config.CSVProfile` (date/amount or debit+credit/description/external
     id columns, delimiter, encoding UTF-8 and Latin-1/Windows-1252, date format, decimal separator, skip rows,
     target account) emitting fui/1 records (deterministic ids when no external id);
   - **external runner**: discovers `fui-connector-<name>` in `<config>/connectors/` and `PATH`; `describe`;
     `fetch` via `exec.CommandContext` with timeout (config, default 120 s), temp working dir, reduced env (no
     `FUI_*`), config JSON on stdin, stdout streamed line by line (1 MiB cap), stderr captured for `--verbose`;
     non-zero exit or missing `end` → abort without writing.
2. **Pipeline** (`internal/app/importing`), steps 1–7 pure (no writes): validate stream; resolve `ref` → account
   via connector account refs (unknown ref → plan item asking for mapping, or create with `--create-accounts` from
   the account record); normalize descriptions; dedup by external id then fingerprint, mark doubtful (±2 days);
   propose transfer matches (counterparty IBAN of user accounts, then amount/date); categories/tags from FUI-format
   `category` are applied (new ones listed in the plan); build an `ImportPlan` (new / duplicate / doubtful / errors /
   new categories / balance-check discrepancies when `balance_after` is present). Apply (step 8): `Backuper.Backup`
   with reason `import`, then one `WithTx` creating an `import_batch` (with `source_sha256`; a file already imported
   is flagged in the plan) and the transactions. Row include/exclude overrides from the UI.
3. **Undo**: `PlanUndo(batch)` / `ApplyUndo` removes the batch's transactions if none is reconciled.
4. **Export**: transactions filtered like `tx list` to CSV (RFC 4180, header, dot decimal), JSON, and **FUI format**
   (`meta` connector `fui`, accounts involved, transactions keeping the original external id or `fui-<id>`,
   transfer legs share `transfer`, `category`/`notes`, reconciled exported as `booked`, final `balance` per account,
   `end`). Full dump (all entities, JSON or CSV one file per entity) for `restore`-compatible portability.
5. Connector listing/show/test use cases (`fui connector list/show/test`: run fetch + validation, no import).

## Acceptance

- Property tests: importing the same file twice creates no new transactions; export FUI → import into an empty
  memstore reproduces accounts, transactions, categories; `undo` restores the previous state (memstore `Snapshot`).
- Runner tests with a fake connector binary built in the test (timeouts, non-zero exit, truncated stream, stderr).
- CSV connector fuzz target; benchmark: 10 000-row CSV import plan+apply on memstore < 5 s.
- `golangci-lint run ./internal/app/importing/... ./internal/connector/...` clean; coverage ≥ 80 %.
- CLI commands for import/export are NOT in this task (the CLI is built in parallel); you will add them in a
  follow-up once the CLI is merged.
