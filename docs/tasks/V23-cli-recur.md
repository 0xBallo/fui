# V23 — CLI: recurring, loan
Branch `feat/v-cli-recur`. Owned: `internal/cli/{recurring,loan}*.go`, render files.

Protocol: `docs/tasks/_PROTOCOL.md` (binding). CLI layer rules as in `V20-cli-split.md` (thin commands, module annotation, no `root.go`/`help.json` edits — V32 registers; test constructors directly).

Modules `recurring` / `loans`. `fui recurring add/list/run --until/skip/confirm`, `fui loan add (--computed|--import file)/show/schedule/event`. Render installment mismatch warnings and proposed rate.
