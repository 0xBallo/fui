# T05 — Domain services and reporting use cases

Branch `feat/domain-services`. Owned paths: new files in `internal/domain/` (do not modify existing files except to
add small missing helpers, listed in your report) and `internal/app/reporting/`. Read `AGENTS.md` first, then the
merged `internal/domain` package and reuse its types (`Money`, `Date`, `Period`, `Rate`, `Convert`, `RoundDiv`,
`Transaction`, `Account`, errors). T04 is building `internal/app` (ports, core use cases) in parallel: do not import
or edit it; `reporting` defines its own small consumer-side port.
Spec refs: technical spec "Servizi di dominio", "Invarianti verificate con test property-based"; functional spec
"Regole di business", "Deduplicazione", "Report e analytics" (MVP rows), "KPI: autonomia".

## Deliverables (pure functions in `internal/domain`)

1. **Balance** — opening balance + sum of movements up to a date (inclusive).
2. **Fingerprint and dedup** — `fingerprint = sha256(account | date | amount | normalized description)` (normalize:
   trim, collapse spaces, case-fold); classify an incoming row against existing ones as `new`, `duplicate` (same
   external id or same fingerprint) or `doubtful` (same account and amount, date within ±2 days).
3. **Transfer matching** — propose (never decide) pairs between rows/transactions of different user accounts: first
   by counterparty IBAN belonging to a user account, then by opposite amount and date within 2 days.
4. **Runway KPI** — liquidity = cash + checking + savings balances minus credit-card debt (investment and loan
   excluded); reference monthly spending = median of total outflows of the last 12 complete months (transfers and
   investment kinds excluded, loan instalments included); fewer than 3 complete months → coded "not enough data"
   error; result = months + days + estimated zero date. Integer/rational arithmetic only (no floats).
5. **Spending by category** and **cash flow** — over a period, excluding transfers and investment movements;
   multi-currency totals converted to the base currency at the rate valid on each transaction date (rate lookup is a
   function parameter); cash flow grouped by accounting month honouring `month_start_day`; uncategorized bucket.

## Deliverables (`internal/app/reporting`)

- Use cases `Spending`, `CashFlow`, `Runway` returning structured results (suitable for JSON/CSV/table rendering
  and the TUI dashboard), parameters per the functional spec (period default = current accounting month, accounts
  filter, month range). Define a small `Source` interface (list accounts, list transactions in a period with
  filters, balance at date, fx rate on or before date) and a `Clock`-like `Now() time.Time` dependency; the SQLite
  store will implement `Source` later. Test with an in-package fake.

## Acceptance

- Property tests (rapid): balance independent of insertion order; dedup idempotence (classifying a set against
  itself yields only duplicates); transfer without fee nets to zero; runway monotonic in liquidity.
- Table tests for median (even/odd counts), month boundaries with `month_start_day` 27 and 31, Kiritimati/UTC.
- `internal/domain` coverage stays ≥ 90 %; `golangci-lint run ./internal/domain/... ./internal/app/reporting/...`
  clean.
