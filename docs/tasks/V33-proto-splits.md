# V33 — fui/1 split rows round-trip

Branch `feat/v-proto-splits`. Owned: `pkg/fuiproto/` (record type, JSON schema, conformance, goldens),
`internal/app/importing/` (export + apply of split rows), `internal/cli/testdata/` goldens only if they change.
Protocol: `docs/tasks/_PROTOCOL.md`.

Spec: tecniche.md "Connettori e protocollo fui/1" (compatibility rules: additive optional fields only, no major
bump), invariant "Export in formato FUI e reimport ... stessi conti, movimenti, categorie e tag (round-trip)".

1. Add an optional `splits` array to `TransactionRecord` (category name, amount minor units, note), validated by the
   stream validator (sum == amount, ≥ 2 rows, same currency); old streams without it stay valid. Update the JSON
   schema and conformance suite.
2. Export writes split rows; import creates them (categories resolved/created like the transaction category) via
   the existing split use case/repo.
3. Extend the FUI round-trip property test to include splits, tags and payees.
