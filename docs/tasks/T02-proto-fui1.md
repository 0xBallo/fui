# T02 — fui/1 protocol package

Branch `feat/proto-fui1`. Owned path: `pkg/fuiproto/` (incl. `pkg/fuiproto/conformance/`). Read `AGENTS.md` first.
Spec refs: functional spec "Formato FUI" and "Contratto dei connettori esterni"; technical spec "Connettori e
protocollo fui/1". `pkg/fuiproto` is the only public package: it must not import anything from `internal/`.

## Deliverables

1. **Types** for every record: `meta`, `account`, `transaction` (all fields of the spec table, including
   `original`, `raw`, `tags`, `category_hint`, `category`, `transfer`, `balance_after`), `balance`, `schedule`,
   `installment`, `error`, `end`; plus `Describe` output (protocol, name, version, accepted file types, config
   params: key/type/required/description) and the `fetch` stdin request (`protocol`, `config`, `file`, `since`).
   Amounts are `int64` minor units; dates are `YYYY-MM-DD` strings validated by the package.
   Constants: protocol `fui/1`, connector exit codes (0 ok, 2 bad config, 3 unreadable source).
2. **Decoder** — streaming JSONL reader returning `iter.Seq2[Record, error]`; max line length 1 MiB; unknown JSON
   fields ignored; a different protocol major is rejected.
3. **Encoder** — deterministic JSONL writer (stable field order, `omitempty` for optional fields).
4. **Stream validator** — state machine: `meta` first with protocol major 1; each `account` declared before use;
   transaction currency equals its account currency; `schedule` before its `installment`s; `end` last with counts
   matching (`transactions`, `errors`); nothing after `end`; missing `end` = truncated. Distinguish
   **structural violations** (abort the whole import) from **invalid rows** (reported per line, row excluded).
   Validate required fields, enum values (`status`, `kind`, account `kind`), date format.
5. **Conformance suite** (`pkg/fuiproto/conformance`) — given a connector executable path and test input files:
   runs `describe` (valid JSON, protocol fui/1), runs `fetch` with a stdin request, checks exit code 0, validates the
   stream, runs `fetch` twice and checks transaction ids are stable; also checks a bad-config case exits 2 when
   the caller provides one. Exposed as a function usable from `go test` (e.g. `conformance.Run(t, opts)`).
6. A tiny fake connector under `pkg/fuiproto/conformance/testdata` (Go `main` built in the test, or a test helper
   binary via `os.Args` re-exec) to test the suite itself.

## Acceptance

- Example stream from the functional spec decodes and validates.
- Table tests for every validator rule (both structural and row-level). Fuzz target for the decoder.
- Round-trip encode→decode equality (go-cmp).
- `golangci-lint run ./pkg/...` clean; coverage ≥ 85 %.
