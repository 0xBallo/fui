# W06 — Connector CAMT.053

Branch `feat/w-conn-camt`. Owned: `internal/connector/camt*.go`, `internal/connector/testdata/camt*` (do NOT edit registry.go: expose `NewCAMT(...)`). Protocol: `docs/tasks/_PROTOCOL.md`. Plan: `docs/PLAN-v0.3.md`.

Spec: F "Connettori" (integrato). Parse ISO 20022 camt.053.001.02/.08 via `encoding/xml`: Stmt/Acct/IBAN → account ref, Ntry (Amt+Ccy, CdtDbtInd sign, BookgDt, AcctSvcrRef/NtryRef → external id, RmtInf/Ustrd + counterparty name → description/payee, Rvsl), multiple statements per file, TxDtls batches. Output fui/1 records like the CSV connector.
Accept: synthetic fixtures for both versions, table tests, fuzz, malformed ⇒ error; coverage ≥ 80%. No new dependency.
