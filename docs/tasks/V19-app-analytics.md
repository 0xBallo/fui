# V19 — App: forecast, reports, saved reports
Branch `feat/v-app-analytics`. Owned: `internal/app/reporting/` (new files), memstore report files if needed.

Protocol: `docs/tasks/_PROTOCOL.md` (binding). App layer (Plan/Apply where writes, module.Set passed like recurring/lending). Spec: funzionali.md "Report e analytics", "Previsione di cassa".

Module `analytics` (+ loans/budget data only when those modules are on). 1. Forecast: opening balances + recurrences + loan installments + estimated variable spend (3-month average or budget) → domain forecast; horizon from config `forecast.horizon` (default 90 days). 2. Reports: budget vs actual, balance over time, remaining debt, interest paid, net worth over time (monthly granularity, archived accounts included in history — default for the open question), top payees. 3. Saved reports: save/list/run/rm with relative periods (last-month, this-year, last-N-months). Golden JSON per report; rapid: net worth unchanged by a fee-less transfer.
