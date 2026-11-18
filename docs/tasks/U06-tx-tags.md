# U06 — App: tags in TxView
Branch `feat/u06-tx-tags`. Owned: `internal/app/transaction.go`, `internal/app/transaction_test.go`.

Protocol: `docs/tasks/_PROTOCOL.md` (binding, v0.3 addendum). Plan: `docs/PLAN-tui-revamp.md` §2 (decisions are binding). Effort: low.

Read only: `internal/app/transaction.go` (`TxView`, `ListTx`, the view builder near line 720), `TagRepo` in `internal/app/ports.go`.

Do: add `Tags []string` (tag names, ordered by tag ID) to `TxView` and fill it in `ListTx` with `r.Tags().TagsOf` per row inside the same read transaction (`// ponytail: one query per row, fine for a 200-row page; batch it if pages grow`). Other builders of TxView may leave it empty.
Check: `go test ./...` — if a CLI JSON golden changes because TxView is marshalled directly, do NOT update CLI files: write `REQ: U27: ...` and keep the field tagged `json:"tags,omitempty"` only if that avoids the change.
Tests: listing returns tags for tagged rows, empty for untagged, works with the tag filter.
