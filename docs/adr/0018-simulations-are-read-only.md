# ADR 0018: Simulations never write transactions

* Status: Accepted
* Date: 2026-11-15

## Context and Problem Statement

Hypothetical scenarios (investment, PAC, cash flow) must not be confused with real data or change balances, reports or undo history.

## Decision Outcome

`simulating.Service` computes from the archive (positions, recent cash flow) and returns results; the only persistence is the named scenario parameters in `saved_simulations` (`simulate save/list/compare/rm`). A lone `--rate` gives pessimistic and optimistic rates at base ∓ 2 pp. Monte Carlo and AI are out of v0.3.

### Consequences

* **Positive:** e2e can assert that the transaction list is byte-identical after any simulate command.
* **Negative:** no per-account cash-flow scenarios.
