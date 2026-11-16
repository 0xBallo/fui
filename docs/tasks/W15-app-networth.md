# W15 — App: net worth + runway extension

Branch `feat/w-app-networth`. Owned: `internal/app/reporting/{networth,runway}*.go` (existing files: extend minimally). Protocol: `docs/tasks/_PROTOCOL.md`. Plan: `docs/PLAN-v0.3.md`.

Net worth report/KPI composes `domain.NetWorth` from liquidity + positions (when `investments`) + latest asset valuations (when `assets`) − loan debts (when `loans`), monthly series; runway gets `IncludeInvestments` option (adds positions value to liquidity). Disabled module ⇒ component absent (not zero-with-label).
Accept: golden JSON; rapid — fee-less transfer leaves net worth unchanged; module off removes component; coverage ≥ 80%.
