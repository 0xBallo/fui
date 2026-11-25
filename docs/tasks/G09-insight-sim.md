# G09 — Insights, simulations, connectors

Branch `feat/g09-insight-sim` · worktree `../fui-wt/g09` · TL: Claude Sonnet medium · deps: none.
Read first: `docs/PLAN-gaps.md` (§5; §6), `docs/tasks/_PROTOCOL.md` (v0.5 addendum), ADR 0017, ADR 0018,
F: "Report e analytics" (Analisi automatiche, Simulazioni), "Previsione di cassa", "Formato FUI" (`error`,
`balance`), "Import" (OFX, CAMT.053); T: "Connettori", `internal/domain/simulate`.

## Owned paths

`internal/domain/insight*.go`, `internal/domain/simulate/`, `internal/app/reporting/insight*.go`,
`internal/app/simulating/`, `internal/config/insights*.go`, `internal/connector/{ofx,camt}*.go` + their testdata,
`internal/cli/{insights*,simulate*}.go` + render files, `internal/tui/screens/report/sim*`.

## Gaps

| Id | Gap | Evidence | Expected |
| --- | --- | --- | --- |
| a3-3 | Subscriptions with an existing recurrence still reported | `domain/insight.go:237-238,269`, `reporting/insight.go:236,326` | Load recurrences; a candidate matching a recurrence (payee or description, amount within tolerance, compatible period) is not reported |
| a3-4 | Insights single currency | `domain/insight.go:145,246,359` | Amounts converted to the base currency at the transaction date (missing rate → excluded and counted in a warning, like runway `rate_missing`); `trend_min_amount` in base currency |
| a3-17 | Hard-coded thresholds | `config/insights.go:20-26` vs `domain/insight.go:24-33` | Config keys for anomaly window (months), anomaly minimum history, trend periods (naming like ADR 0017 `insights.*`) |
| a3-8 | Cash-flow simulation total only; TUI form lacks events; TUI cannot remove | `simulating/simulating.go:507-541`, `screens/report/sim_form.go:48-49` | `simulate cashflow --account A` (repeatable) and per-account series in the result; TUI form adds events (one-off, recurring change, stop income, new loan) and account selection; `d` removes a saved simulation (confirm) |
| a3-9 | Connector error records and balances | `connector/camt.go:326-330,366-380`, `connector/ofx.go:156-176` | CAMT `error.line` = source XML line; batch whose TxDtls do not sum → single tx + `error`/warning record; OFX: a bad STMTTRN yields an `error` record instead of aborting; `LEDGERBAL` → `balance` record |

## Suggested MTs

1. domain/insight: recurrence-aware subscriptions + FX inputs + configurable windows (table tests, rapid: converting a
   single-currency archive at rate 1 gives today's results).
2. app/reporting + config: load recurrences and rates, new keys, CLI `config` docs/golden.
3. domain/simulate + app/simulating: per-account cash flow (rapid: sum of per-account series = total series).
4. connector: CAMT line numbers + batch fallback; OFX error records + LEDGERBAL (fuzz both decoders, goldens).
5. cli + tui: `simulate cashflow --account`, TUI sim form events/accounts/remove, goldens, testscripts.

## DoD

PLAN §6, plus fuzz targets for OFX/CAMT still green and golden `fui/1` outputs for the new error/balance records.
