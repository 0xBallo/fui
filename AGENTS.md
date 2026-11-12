# FUI — rules for coding agents

FUI is a local-first personal-finance CLI + TUI in Go. Specs (Italian) are the source of truth:

- Functional: `docs/specs/funzionali.md`
- Technical: `docs/specs/tecniche.md` (architecture, conventions, tests — read the sections relevant to your task)
- Development plan and task list: `docs/PLAN.md`; your task: `docs/tasks/<TASK>.md`

**Scope is MVP v0.1 only** (see "Perimetro MVP e roadmap" in the functional spec). Do not build v0.2/v0.3 features
(budget, recurrences, loans, rules, splits, tags-UI, investments, simulations) unless your task says so.

## Architecture (enforced by `.golangci.yml` depguard/forbidigo)

- Module: `github.com/0xballo/fui`, Go `toolchain go1.27.4`, `CGO_ENABLED=0`.
- `internal/domain` imports only the standard library. Pure, deterministic: no I/O, no clock, no globals.
- `internal/app` = use cases + ports (interfaces). Imports domain only, never adapters.
- `internal/cli`, `internal/tui` import `app` and `domain` only — never `store` or `connector`.
- `pkg/fuiproto` imports nothing from `internal`.
- `cmd/fui/main.go` is the only composition root.
- No `float32/float64` in domain/app/store. No `time.Now()` outside `internal/clock`. No `net/http`.
- Money is `int64` minor units + currency. Dates have no time-of-day.

## Code conventions

- Code, identifiers, comments, CLI/TUI text, commit messages: **English**.
- Effective Go + Go Code Review Comments. `gofumpt` + `goimports` (local prefix `github.com/0xballo/fui`).
- Short single-purpose package names; no `util`, `common`, `helpers`, `misc`.
- Errors wrapped with context and `%w`; compare with `errors.Is/As`; handle an error once.
- `context.Context` first param for I/O. >4 params → options struct. No bool params that switch behaviour.
- No `panic` outside `main`, no `init()` with side effects, no mutable globals.
- Every exported symbol has a godoc comment. Comment the *why*, not the *what*.
- Keep it simple: implement what the task needs, no speculative abstractions or config. An interface exists only
  when there is a second implementation or a test needs it.

## Tests

- Table-driven tests next to the code (`_test.go`), `go-cmp` for diffs, `pgregory.net/rapid` for properties,
  native fuzzing for parsers/decoders, `testscript` for CLI e2e, `teatest` for TUI.
- Deterministic: injected clock (`clock.Fixed`), fixed seeds, no map-order dependence, must pass with `TZ=UTC`
  and `TZ=Pacific/Kiritimati`.
- No real personal data in `testdata` (synthetic or anonymized only).

## Workflow for each agent

1. You work **only inside your git worktree** on your branch. Do not touch other worktrees or the main checkout.
   Never read or write paths outside it (no `/tmp`, no `find /`, no module cache): inspect dependencies with
   `go doc -src <pkg> <symbol>` / `go list -m -versions <mod>`, experiment in `t.TempDir()` or `./.scratch`
   (add it to `.git/info/exclude`).
2. Stay inside the paths owned by your task (listed in your task file). `go.mod`/`go.sum` may be updated for new
   dependencies (only the ones allowed by the technical spec). If you need a change elsewhere, do not make it:
   write it under "Requests" in your report.
3. Git flow (see "Flusso Git" in the technical spec): `main` = PROD, `develop` = PREVIEW. Feature branches start
   from `develop`, hotfix branches from `main`; both are squash-merged through a PR with all CI checks green. Never
   commit directly on `develop` or `main`. Several commits per topic are fine (no atomicity required); the
   `pre-commit` hook (`make hooks` once per clone) runs fmt, build, vet, lint and tests, so every commit is green.
   Messages use **Conventional Commits**: `<type>(<scope>): <imperative lowercase summary ≤72 chars>`, scopes:
   `domain, app, cli, tui, store, import, connector/revolut, connector/widiba, proto, config, skill` (plus `build`,
   `ci`, `docs` as types); the PR title uses the same format, as it becomes the squash commit. Releases: `release/vX.Y.Z` from `develop`
   updates `CHANGELOG.md`, merges into `main`, gets the tag.
   **Never** add `Co-Authored-By`, or mention AI/Claude/Copilot/Gemini/agents in commit messages. Do not push.
4. Before declaring done, run and make green (on your packages):
   `gofumpt -l . ; go build ./... ; go vet ./... ; go test -race ./<your-pkgs>/... ; golangci-lint run ./<your-pkgs>/...`
5. When finished, write `.task-report.md` in the worktree root (it is gitignored) with:
   - Summary of what was built (packages, main exported API)
   - Verification commands run and their result
   - Deviations from the spec and why
   - Open issues / known gaps
   - Requests to other tasks
   Then reply with the single line `TASK DONE` (or `TASK BLOCKED: <reason>`).
6. If the coordinator sends review findings later, fix them with new commits, update `.task-report.md`,
   and reply `TASK DONE` again.
