# W07 — Store: securities (0012)

Branch `feat/w-store-security`. Owned: store `migrations/0012_securities.sql`, `queries/security*`, `repo_securities*`, ports for it in `internal/app/ports.go` (add only the new port interface). Protocol: `docs/tasks/_PROTOCOL.md`. Plan: `docs/PLAN-v0.3.md`.

Spec: T Modello dati (`securities` with `quantity_scale`, `security_prices`, `security_ops` with `cash_tx_id`). Read W01 domain types. Forward-only migration (test up on populated 0011 archive), sqlc, repo + memstore-shared contract tests (`memstore.RunContract` pattern), `reset` and `dump/restore` extended for the new tables, FK RESTRICT on securities in use.
Accept: contract tests, migration test, `go generate` clean.
