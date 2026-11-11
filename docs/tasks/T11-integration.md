# T11 — Composition root, e2e tests, agent skill, CI

Branch `feat/integration`. Owned paths: `cmd/fui/`, `internal/skill/`, new files in `internal/cli/` for
`skill install|show`, `testdata/` (repo root) and `.github/workflows/`. Read `AGENTS.md` first, then the merged
`internal/cli` (`Deps`, `Execute`), `internal/tui` (constructor/`Run`), `internal/app`, `internal/app/reporting`,
`internal/app/importing`, `internal/connector` (`Registry`), `internal/store/sqlite` (`Open`, `Options`),
`internal/config` (`Resolve`), `internal/logging`, `internal/clock`.
Spec refs: technical spec "Architettura" (composition root), "Sicurezza", "Test e prevenzione delle regressioni"
(testscript level, determinism), "CI/CD e distribuzione"; functional spec "Casi d'uso", "Uso con agenti AI".

## Deliverables

1. **`cmd/fui/main.go`** — the only place that knows every implementation: resolve config (flags pre-parsed for
   `--db`/`--verbose`, env, file), set up logging (file only, never stdout), system clock, open the SQLite store
   (migrations + automatic backup), build `app.Service`, reporting use cases (store as `reporting.Source`),
   importing service with `connector.Registry`, the TUI runner, and `cli.Deps`; `os.Exit(cli.Execute(...))`.
   `--version` from `-ldflags -X main.version`. Commands that don't need the DB (`--help`, `help --json`,
   `--version`, `completion`, `config`, `skill show`) must not create it.
2. **Security test**: `go list -deps ./cmd/fui` must not contain `net/http` or `net/rpc` (test in `cmd/fui`).
3. **Agent skill** — `internal/skill` embeds `SKILL.md` (when to use FUI, command reference, JSON output format,
   typical flows: monthly import, reviewing uncategorized transactions, reports/runway; the four safety rules of the
   functional spec, verbatim in meaning) with the FUI version injected at build time; `fui skill show` prints it,
   `fui skill install [--target dir]` writes `<target>/fui/SKILL.md` (default `~/.claude/skills`), file `600`,
   dir `700`, `--dry-run` supported.
4. **E2E `testscript`** (`rogpeppe/go-internal/testscript`) running the real `fui` binary on temp archives with
   `TZ` and a fixed date via an env override honoured only by the clock wiring for tests (e.g.
   `FUI_TEST_TODAY`, documented as test-only): at least one script per MVP use case — UC-01 quick add, UC-03 import
   (CSV profile + `.fui.jsonl` + duplicate re-import + undo), UC-04 balances/net worth, UC-05 category report +
   CSV/JSON export, UC-06 backup + restore, UC-10 scripting (`--json` + agent mode `FUI_AGENT=1` refusing writes
   without `--yes`, `help --json`, exit codes 0/1/2). Also run both bank connectors' binaries through `fui import`.
5. **CI** `.github/workflows/ci.yml`: jobs lint (gofumpt, `go mod tidy` diff, generated code diff, golangci-lint),
   test (`make test` on ubuntu + macos, Go 1.26 and 1.27, `TZ=UTC` and `TZ=Pacific/Kiritimati`), e2e,
   fuzz-short (30 s per fuzz target), security (`govulncheck`, `go mod verify`, the no-net test), build
   (`make build`, plus a non-blocking windows build). No release/goreleaser job in this iteration.
6. `make check` green locally; update `README.md` (install, quick start, commands overview, skill install).

## Acceptance

- `go test ./...` including testscripts green with `-race`; startup of `fui account list` on a 50 000-transaction
  archive measured (report the number; target < 100 ms).
- `golangci-lint run ./...` clean.
