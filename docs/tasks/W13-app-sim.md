# W13 — App: simulations

Branch `feat/w-app-sim`. Owned: `internal/app/simulating/`, `internal/testutil/memstore/sim*`. Protocol: `docs/tasks/_PROTOCOL.md`. Plan: `docs/PLAN-v0.3.md`.

Use cases: Run (investment/pac/cashflow; "initial = current positions value" option reads positions; cashflow starts from the real forecast), Save/List/Delete/Rerun, Compare (2–3 saved scenarios side by side, error beyond 3). Never writes transactions (assert in tests via a store spy). `Require(simulations)`.
Accept: run without saving writes nothing; compare >3 rejected; params JSON round-trips; coverage ≥ 80%.
