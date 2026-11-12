# ADR 0002: Integer minor units for amounts and single-point half-away-from-zero rounding

* Status: Accepted
* Date: 2026-11-11

## Context and Problem Statement

IEEE-754 floating-point types (`float32`, `float64`) cannot accurately represent decimal fractions (e.g., `0.1 + 0.2 != 0.3`). In personal finance software, accumulated rounding errors and fractional discrepancies undermine user trust and corrupt accounting integrity. Furthermore, inconsistent rounding strategies across exchange rate conversions and investment balances create irreconcilable reporting differences.

## Considered Options

1. **Native binary floating point (`float64`):** Built-in and convenient, but inherently imprecise for decimal currency calculations.
2. **Arbitrary-precision decimal library (e.g. `shopspring/decimal`):** Accurate representation, but adds an external dependency, heap allocations, and substantial runtime performance overhead.
3. **Scaled integer arithmetic (`int64` minor units) with centralized half-away-from-zero rounding:** Amounts stored as integer minor units (cents), exchange rates scaled to $10^9$, asset quantities stored as integer values with per-asset scale (`quantity_scale`), with half-away-from-zero rounding performed at a single centralized point in domain.

## Decision Outcome

Chosen option: **Scaled integer arithmetic (`int64` minor units) with centralized half-away-from-zero rounding**, because:
- Monetary amounts are represented exactly without floating-point artifacts using native `int64` minor units paired with an ISO currency code.
- Rounding policy is uniformly defined as half-away-from-zero (`math.Round` logic on scaled integers) and centralized in a single domain conversion function.
- Linters (`forbidigo` in `.golangci.yml`) strictly forbid `float32` and `float64` across `internal/domain`, `internal/app`, and `internal/store` (with an explicit exception only for Monte Carlo simulations in `internal/domain/simulate`).

### Consequences

* **Positive:**
  - Zero rounding drift or balance discrepancies.
  - Blazing-fast arithmetic operations with zero heap allocation overhead.
  - Zero external dependencies for currency math.
* **Negative:**
  - Ingestion and presentation layers must explicitly parse from and format to decimal string representations.
  - Arithmetic operations must be mindful of `int64` limits, although `int64` minor units comfortably accommodate values up to $\pm 9.22 \times 10^{16}$ cents.
