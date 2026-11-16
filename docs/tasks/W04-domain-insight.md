# W04 — Domain: insights + period comparison

Branch `feat/w-domain-insight`. Owned: `internal/domain/{insight,compare}*.go`. Protocol: `docs/tasks/_PROTOCOL.md`. Plan: `docs/PLAN-v0.3.md`.

Spec: F "Analisi automatiche" (three analyses, default thresholds, all configurable), "Confronto periodi".
1. `InsightConfig` with defaults (anomaly factor 2.5 over 6-month median per payee/category, subscription tolerance ±5% min 3 occurrences at weekly/monthly/yearly regular interval, trend ±25% vs mean of previous 3 periods with a minimum amount).
2. `DetectAnomalies`, `DetectSubscriptions` (skip those already matching a recurrence; flag price changes of known ones), `DetectTrends` over `[]Transaction` (transfers/investment excluded, splits per row). `Insight{Kind, Key (stable hash for dismissal), Subject, Detail, Amount}`; `Key` must be stable across runs.
3. `ComparePeriods(a, b Period, txs, byCategory bool)` → rows (category, A, B, delta, delta %), totals.
Accept: table tests per analysis + boundary at thresholds; rapid — keys stable/deterministic, results independent of input order; coverage ≥ 90%.
