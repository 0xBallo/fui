# T17 — CLI filter flags, `tx search`, export filters, summary

Branch `feat/search-cli` (based on `main` after T14 and T16). Owned paths: `internal/cli/filterflags.go` (new),
`internal/cli/tx_list.go`, `internal/cli/tx_search.go` (new), `internal/cli/tx.go` (registration),
`internal/cli/export.go`, `internal/cli/render/` (summary row), `internal/cli/testdata/` goldens, `internal/app/
transaction.go` (`ListTxRequest` fields + resolution, summary), `internal/app/importing/export.go` only if the
request type forces it. Protocol: `docs/tasks/_PROTOCOL.md`.

Spec: funzionali.md "Ricerca e filtri delle transazioni"; tecniche.md CLI "Ricerca e filtri". Read the merged
`domain.TxFilter`, `app.ResolveTxFilter`/`ListTx`, and T16's notes: zero `TxFilter` = all/date-asc, newest-first
listing needs `Desc`, cursor only for date sort, request text must be normalized with `textnorm.Normalize` (app
already does it — keep one place).

1. `filterflags.go`: one function that registers the flags (`--from --to --month --year -a/--account
   -c/--category --uncategorized --payee --min --max --income --expense --status --kind --batch --sort --desc
   --limit`; `-t/--tag` is v0.2: do not add) and one that reads them into `app.ListTxRequest`. Mount it on
   `tx list`, `tx search <text>` (new; text is the positional arg) and `export`. Remove the old `--text` flag from
   `tx list` (search is `tx search`); keep `--after-date/--after-id` paging on `tx list` only.
2. `ListTxRequest` gains Month, Year, Uncategorized, Payees (names; payees table does not exist yet → resolve
   to an `invalid_filter`/not-found error if any is given, note in GAPS), Kinds, Batch, Sign, Sort, Desc.
   Incompatible flags (`--month` with `--from/--to`, `--income` with `--expense`, `--year` with `--month`) →
   `invalid_filter` (exit 1). Default order of `tx list`/`tx search`: newest first; `--sort` without `--desc`
   is ascending.
3. Summary: table output ends with row count and the sum of amounts (one sum per currency); JSON output is
   `{"items":[...],"summary":{"count":n,"totals":[{"currency":"EUR","amount":"..."}]},"next":...}` — check the
   current `tx list --json` shape and keep it backward-compatible where possible (state any break in DEV).
4. Tests: table-driven flag parsing incl. every incompatible combination, goldens for `tx list`/`tx search`
   text+JSON, export with filters, agent mode. Update `help --json` golden. `internal/cli` coverage ≥ 80%.
