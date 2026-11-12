# V25 — CLI: dynamic shell completion
Branch `feat/v-cli-complete`. Owned: `internal/cli/completion*.go`.

Protocol: `docs/tasks/_PROTOCOL.md` (binding). CLI layer rules as in `V20-cli-split.md` (thin commands, module annotation, no `root.go`/`help.json` edits — V32 registers; test constructors directly).

Module core. Dynamic completion (`ValidArgsFunction`/`RegisterFlagCompletionFunc`) for account, category, tag, payee, loan, saved-report names; if the archive is missing/locked return no suggestions silently (default for the open question).
