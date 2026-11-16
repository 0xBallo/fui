# W05 — Connector OFX

Branch `feat/w-conn-ofx`. Owned: `internal/connector/ofx*.go`, `internal/connector/testdata/ofx*` (do NOT edit registry.go: expose `NewOFX(...)`). Protocol: `docs/tasks/_PROTOCOL.md`. Plan: `docs/PLAN-v0.3.md`.

Spec: F "Connettori" (integrato), "Formato FUI"; follow `internal/connector/csv.go` shape. Parse OFX 1.x (SGML) and 2.x (XML): bank/credit-card statements (STMTTRN: DTPOSTED, TRNAMT, FITID→external id, NAME/MEMO, TRNTYPE→kind), currency, account ref (ACCTID, BANKID → ref/IBAN when valid). Output fui/1 records through the same Source/pipeline contract as CSV. Tolerant of unclosed SGML tags, CP1252/UTF-8, signed amounts with ',' or '.', timezone suffix in dates (date only, no time-of-day).
Accept: synthetic anonymized fixtures (no real data), table tests, native fuzz on the parser, malformed file ⇒ error not panic; coverage ≥ 80%. No new dependency (stdlib `encoding/xml` + hand SGML normalizer).
