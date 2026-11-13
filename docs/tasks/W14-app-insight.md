# W14 — App: insights, compare report

Branch `feat/w-app-insight`. Owned: `internal/app/reporting/insight*.go`, `internal/app/reporting/compare*.go`, `memstore/insight*`. Protocol: `docs/tasks/_PROTOCOL.md`. Plan: `docs/PLAN-v0.3.md`.

Use cases: Insights.Run (loads txs/recurrences/dismissed keys, config `insights.*` with defaults, returns non-dismissed), Dismiss(key), AcceptSubscription(key) → creates a recurrence via the recurring app use case, ComparePeriods report (relative period vocabulary reused from saved reports). Gated on `analytics`.
Accept: dismissed never reappears; accept creates exactly one recurrence (idempotent); golden JSON for compare; coverage ≥ 80%.
