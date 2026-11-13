# W01 — Domain: security, quantity, ops, positions

Branch `feat/w-domain-security`. Owned: `internal/domain/{security,quantity,position}*.go`. Protocol: `docs/tasks/_PROTOCOL.md`. Plan: `docs/PLAN-v0.3.md`.

Spec: F "Titolo", "Operazione su titolo"; T "Tipi valore" (`Quantity`), Invarianti, Servizi (valore = quantità × prezzo con `big.Rat`, un solo arrotondamento).
1. `SecurityID`, `Security` (name, ticker/ISIN, kind stock/etf/bond/fund/crypto, currency, quantity scale 0..8), `Quantity` (int64 + scale; ops only with same scale; Add/Sub/Neg/String/Parse).
2. `SecurityOp` (buy/sell/dividend/coupon; date, account, security, qty, unit price, fees, optional cash tx id) with validating constructor.
3. `ComputePosition(ops, prices)`: quantity, average cost (PMC), cost basis, last known price on/before a date, market value, unrealized P&L, realized P&L on sells, income (dividends). Selling more than held → `ErrInsufficientQuantity`.
Accept: rapid — position quantity = Σ buys − Σ sells independent of op order within the same date; value rounding once; crypto scale 8 round-trip; coverage ≥ 90%.
