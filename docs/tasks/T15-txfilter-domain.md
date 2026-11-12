# T15 — Domain TxFilter, Match oracle, search-text normalization

Branch `feat/txfilter-domain`. Owned paths: `internal/domain/txfilter.go` + test (new), `internal/textnorm/`
(new), `internal/domain/*_test.go` (new tests only, to raise coverage), `go.mod`/`go.sum` (promote
`golang.org/x/text` to a direct dependency). Protocol: `docs/tasks/_PROTOCOL.md`.

Spec: tecniche.md "Core di dominio" (`TxFilter`, "Filtro delle transazioni") and "Ricerca testuale";
funzionali.md "Ricerca e filtri delle transazioni".

1. `internal/textnorm` (stdlib + `golang.org/x/text`, must NOT be imported by domain/app):
   `Normalize(s string) string` — lower-case, NFD + drop combining marks (`unicode.Mn`), collapse whitespace,
   trim. `SearchText(description, payee, notes string) string` joins the normalized parts with a single space.
   Fuzz test (`FuzzNormalize`: idempotent, no uppercase, no `Mn` runes) + table test with Italian accents.
2. `internal/domain/txfilter.go`: immutable `TxFilter` built by `NewTxFilter(TxFilterSpec) (TxFilter, error)`:
   period (From/To inclusive; `Month`/`Year` convenience exclusive with From/To → `ErrInvalidFilter`
   "invalid_filter"), AccountIDs, CategoryIDs + `Uncategorized` (OR together), PayeeIDs, Statuses, Kinds,
   ImportBatchID, MinAbs/MaxAbs on the absolute value in minor units, Sign (`any|income|expense`), Text
   (already normalized by the caller), Sort (`date|amount|payee`), Desc, Limit. Tags are v0.2: leave them out.
   Getters only, slices copied. Different filters AND, repeated values OR.
   `func (f TxFilter) Match(tx Transaction, searchText string) bool` (searchText = normalized text of tx; Text
   matches as substring). `func (f TxFilter) Less(a, b Transaction, payeeA, payeeB string) bool` gives the sort
   order with `ID` as final tie-breaker (deterministic), so stores can be checked against it.
   Register the new error code in `domain.Codes` if that is how codes are listed.
3. Raise `internal/domain` coverage to ≥ 90% (currently 88.1%) with meaningful tests of existing code.

No changes to `internal/app` (T16 switches the port to the domain filter).
