# W11 — App: investing

Branch `feat/w-app-invest`. Owned: `internal/app/investing/`, `internal/testutil/memstore/security*`. Protocol: `docs/tasks/_PROTOCOL.md`. Plan: `docs/PLAN-v0.3.md`.

Plan/Apply use cases (pattern ADR 0004, see `internal/app/budgeting` for shape), module gate `Require(investments)` (crypto kind requires `crypto`): AddSecurity, SetPrice (manual, by date), Buy/Sell/Dividend (optionally creates the linked cash transaction with `kind=investment` on a linked liquidity account and stores `cash_tx_id`; investment-kind txs stay out of reports/budgets), ListPositions(account?, date?) using `domain.ComputePosition`, position value converted to base currency with fx.
Accept: dry-run, selling more than held rejected, cash tx linked/removed consistently, rapid — Σ cash movements = Σ ops amounts + fees; coverage ≥ 80%.
