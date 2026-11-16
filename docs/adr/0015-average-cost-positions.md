# ADR 0015: Positions by weighted average cost, computed from operations

* Status: Accepted
* Date: 2026-11-15

## Context and Problem Statement

Investment positions need a cost basis. FIFO and specific-lot methods need lot tracking and tax rules that are out of scope; positions must still be exact integers.

## Decision Outcome

A position is a pure function of the ordered operations of one (account, security) pair, using the weighted average cost (PMC). Quantity is an integer in the security's scale, cost basis in minor units with half-away-from-zero rounding (ADR 0002). Prices are manual (`security price`); market value and unrealized gain need a price, otherwise the position is shown without them. Dividends and coupons are booked as `interest` income, never as `investment`.

### Consequences

* **Positive:** deterministic, no lot table, easy to property-test.
* **Negative:** FIFO reporting is not possible without a new ADR.
