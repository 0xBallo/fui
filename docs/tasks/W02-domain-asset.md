# W02 — Domain: asset, valuation, net worth composer

Branch `feat/w-domain-asset`. Owned: `internal/domain/{asset,networth}*.go`. Protocol: `docs/tasks/_PROTOCOL.md`. Plan: `docs/PLAN-v0.3.md`.

Spec: F "Bene", Report (Patrimonio netto); T Servizi (Patrimonio netto: liquidità + posizioni + ultima valutazione beni − debiti, solo moduli attivi; bene collegato a mutuo mostra valore netto).
1. `AssetID`, `Asset` (name, kind property/vehicle/other, purchase date+value, optional `LoanID`), `Valuation` (date, value), `LatestValuation(asset, date)` (falls back to purchase value).
2. `NetWorth(in NetWorthInput) NetWorthResult`: input = liquidity, positions value, assets (valuations), loan residual debts, flags for active modules; output components + total + per-asset net-of-debt value.
Accept: rapid — total = Σ components; disabling a module removes exactly its component; valuation on a date uses the last valuation ≤ date; coverage ≥ 90%.
