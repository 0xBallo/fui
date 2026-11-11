# T01 — Domain value types and entities

Branch `feat/domain-core`. Owned path: `internal/domain/` (not `simulate/`). Read `AGENTS.md` first.
Spec refs: technical spec "Core di dominio", "Modello dati e schema SQLite" (conventions), "Riferimenti per nome",
"Importi e date"; functional spec "Modello di dominio".

## Deliverables

1. **Currency** — ISO 4217 code + minor-unit decimals, built only from a known table (at least EUR, USD, GBP, CHF,
   JPY, SEK, NOK, DKK, PLN, CZK, HUF, CAD, AUD, CNY). `ParseCurrency`.
2. **Money** — `int64` minor units + `Currency`. `Add`, `Sub`, `Neg`, `Sign`, `IsZero`, `Cmp`; error on currency
   mismatch (`ErrCurrencyMismatch`) and overflow. No implicit conversion. Implements `slog.LogValuer` returning
   `[redacted]` and `fmt.Stringer` returning the clear value (T03 uses this to un-redact when
   `FUI_LOG_SENSITIVE=1`).
3. **Rounding** — one function, half away from zero, used everywhere (fx conversion, parsing of >decimals input).
4. **Amount parsing/formatting** — single parser used by CLI and TUI: locale `it-IT` (comma decimal, dot thousands)
   and `en-US` (dot decimal, comma thousands); optional sign, optional thousands separators; rejects ambiguous or
   malformed input with a coded error (`invalid_amount`). Formatting for display per locale and a
   machine format. Round-trip property tests per locale (rapid) + fuzz target.
5. **Date** — year/month/day, no `time.Time` stored. `ParseDate` (ISO `YYYY-MM-DD`), relative parser
   (`today`, `yesterday`, `-3d`, `-2w`, `-1m`) taking the reference "today" as a parameter (no clock). `AddDays`,
   `AddMonths` (clamp to month end), compare helpers, `String()` ISO, locale display format. `FromTime(t)` /
   conversion helpers only at the edges. Fuzz target.
6. **Period** — closed date interval; `MonthPeriod(date, monthStartDay)` for the accounting month with
   `month_start_day` 1..31 (29–31 clamped to the last day of the month).
7. **Rate** — `int64` scaled 1e9; `Convert(Money, Rate, to Currency) (Money, error)` with a single rounding per
   conversion, respecting decimals of both currencies.
8. **Typed IDs** — `AccountID`, `TransactionID`, `CategoryID`, `TagID`, `PayeeID`, `TransferGroupID`,
   `ImportBatchID` on `int64`.
9. **Entities (MVP)** — `Account` (name, description, kind: cash/checking/credit_card/savings/investment/loan,
   currency, opening balance, opened on, archived), `Transaction` (date, operation date, value date, amount, payee,
   description, category, notes, status pending/booked/reconciled, kind
   payment/transfer/exchange/fee/interest/refund/investment/other, external id, fingerprint, import batch,
   transfer group), `TransferGroup` (two legs + optional fee), `Category` (name, kind income/expense), `FXRate`
   (currency, date, rate). Enums as typed strings with `Valid()`/parse; constructors validate.
   Default category list for `fui init` (English names).
10. **Errors** — `domain.Error{Code, Message, Details map[string]any}` with stable codes, plus sentinels
    `ErrAmbiguousName`, `ErrNotFound`, `ErrCurrencyMismatch`, `ErrReconciled`, `ErrOverflow`, `ErrInvalidAmount`,
    `ErrInvalidDate`, `ErrMissingField`. Export the list of all codes (the CLI maps them to exit codes).
11. **Name resolution** — pure `Resolve` over candidate names: exact case-insensitive match, then unique prefix,
    then unique substring; multiple candidates → `ambiguous_name` error with `candidates` in details.

## Acceptance

- Coverage of `internal/domain` ≥ 90 %. Property tests: amount and date format→parse round-trip for each locale;
  money add/sub associativity within same currency; conversion rounding is half-away-from-zero.
- Fuzz targets for amount and date parsers (run each ≥ 30 s locally once).
- `golangci-lint run ./internal/domain/...` clean (depguard: stdlib only; forbidigo: no float, no time.Now).
- Commits split logically (e.g. money+currency, date+period, rate, entities, errors+resolve).
