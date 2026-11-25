# Worker protocol (all tasks from wave 6 on)

Read `AGENTS.md` first; it is binding. Extra rules learned in previous waves:

- Stay inside your worktree. Scratch files go in `./.scratch/` (already git-excluded). Never touch `/tmp`, `$HOME`,
  the module cache or other worktrees; inspect deps with `go doc -src` / `go list -m`. Do not write `/dev/null`
  inside `$(...)`.
- **Never run the `fui` binary against the real archive/config.** Any manual run must sandbox every path:
  `XDG_CONFIG_HOME=$PWD/.scratch/cfg XDG_DATA_HOME=$PWD/.scratch/data XDG_STATE_HOME=$PWD/.scratch/state go run ./cmd/fui ...`
  (prefer tests/testscript, which already sandbox). Touching `~/.config/fui` or `~/.local/*/fui` is an incident.
- Work on a feature branch from `develop` (hotfix: from `main`); never commit on `develop`/`main`. Conventional
  Commits, as many per topic as useful (the branch is squash-merged); the `pre-commit` hook keeps each one green.
  **No `Co-authored-by` trailer, no mention of AI/agents/Claude/Copilot/Gemini** in commit messages. Do not push,
  do not merge.
- Before declaring done run on the whole repo: `gofumpt -l . ; go build ./... ; go vet ./... ;
  go test -race -count=1 ./... ; golangci-lint run ./...` — all green. Also
  `go test -count=1 -cover ./<your-pkgs>/...`: every package you own must stay ≥ 80% (`internal/domain` ≥ 90%).
- Only change files listed under "Owned paths". Anything else → `REQ:` line in the report.

## Report (`.task-report.md` in the worktree root, gitignored) — terse key/value, no prose

```
STATUS: done|blocked:<reason>
COMMITS: <n> <sha-first>..<sha-last>
API: <pkg>.<Symbol>(<sig>) ; ...            # new/changed exported API only
FILES: <path>[+|~|-] ; ...                  # + new, ~ changed, - deleted
VERIFY: fmt=ok build=ok vet=ok test=ok lint=ok cover=<pkg>:<pct>,...
DEV: <deviation from spec>: <why> ; ...     # or -
GAPS: <known gap> ; ...                     # or -
REQ: <task/owner>: <change needed> ; ...    # or -
```

Then reply with the single line `TASK DONE` (or `TASK BLOCKED: <reason>`). If you later receive review findings
(format `F<n> <path>:<line> <problem> -> <expected>`), fix each with new commits, update the report adding
`FIXED: F1,F2,...`, and reply `TASK DONE` again.

## v0.3 addendum (token-lean)

- Branch from `develop` (worktree is created for you). Do not push or merge.
- Final answer is ONE line only: `RESULT <task> <done|blocked:<reason>> commits=<n> cover=<pct>` and nothing else;
  all details go in `.task-report.md` (same format as above).
- If you cannot complete (quota, ambiguity), commit WIP, write the report with `STATUS: blocked:<why>` and a
  `RESUME:` line saying exactly what is left, then emit the RESULT line.
- Prefer reading only the spec sections cited in your brief; do not read unrelated docs/tasks.

## v0.5 addendum — Team Lead / developer flow (epics G01–G09, see `docs/PLAN-gaps.md`)

Roles, limits and decision rights are in `PLAN-gaps.md` §1. Everything above still applies to developers; the Team
Lead (TL) applies it to the epic branch.

### Worktrees and branches

- Epic: branch `feat/g<nn>-<slug>` from `develop`, worktree `../fui-wt/g<nn>` (created by the Architect; the TL
  runs there).
- Mini task (MT): branch `feat/g<nn>-<slug>-mt<k>` from the current epic head, worktree `../fui-wt/g<nn>-mt<k>`,
  created by the TL: `git worktree add ../fui-wt/g<nn>-mt<k> -b feat/g<nn>-<slug>-mt<k> feat/g<nn>-<slug>`.
  The TL may touch its own epic worktree and its MT worktrees, nothing else. Remove MT worktrees after merging.
- TL integrates an MT with `git merge --no-ff feat/g<nn>-<slug>-mt<k>` in the epic worktree, after review.
- Only the TL pushes (the epic branch) and opens the PR (`gh pr create --base develop`). Developers never push.

### MT card (TL → dev), key-value lines, no prose

```
MT g01.3
WT ../fui-wt/g01-mt3  BR feat/g01-budget-mt3  BASE feat/g01-budget
OWN internal/domain/budget_period.go ; internal/domain/budget_period_test.go
READ internal/domain/budget.go:40-120 ; F:Budget §"Periodo"
API domain.BudgetPeriod{Month,Year} ; func (b Budget) PeriodRange(d Date) (Period, error)
DO 1) <imperative step> 2) <step> 3) <step>
TEST table: <cases> ; rapid: <property> ; golden: <file>
DOD verify=full cover:internal/domain>=90
```

Rules for cards: one layer, ≤ ~300 changed lines, only the files worth reading, exact API signatures when another
MT depends on them. The card is sent as the first prompt to a fresh Copilot session started in the MT worktree,
prefixed by the line `Read AGENTS.md and docs/tasks/_PROTOCOL.md (binding), then execute:`.

### Dev → TL

One line only: `RESULT g01.3 done|blocked:<reason> c=<commits> cov=<pkg>:<pct>`; details in `.task-report.md`
(format above). Review findings TL → dev: `F<n> <path>:<line> <problem> -> <expected>` (one per line); dev fixes
with new commits and answers `RESULT g01.3 fixed=F1,F2 c=+<n>`.

### TL → Architect events (proactive, one line each, via `herdr agent prompt architect "<line>"`)

| Event | Line |
| --- | --- |
| Start | `[G01] EVT=start mts=<n> plan=<mt ids with one-word topic>` |
| MT merged | `[G01] EVT=mt id=g01.3 st=merged left=<n>` |
| Product question | `[G01] EVT=question id=Q<n> ctx=<≤30 words> opts=a)<..>\|b)<..> rec=<a\|b>` |
| Blocked | `[G01] EVT=blocked why=<≤20 words> need=<what>` |
| PR ready | `[G01] EVT=pr url=<url> gaps=<closed ids> known=<ids or ->` |
| Fix pushed | `[G01] EVT=fixed pr=<n> F=<ids>` |

Architect → TL: `[G01] ANS Q<n>=<a|b> <note>`, `[G01] REVIEW pr=<n>` followed by `F<n>` lines, `[G01] APPROVED`,
`[G01] TASK <new fix task>`. Do not wait idle for an answer: continue with MTs that do not depend on it.

### herdr cheat sheet for TLs (verify syntax with `herdr pane` / `herdr agent`)

```
herdr pane split --current --direction right --cwd ../fui-wt/g01-mt1 --no-focus   # dev 1 (dev 2: split dev1 pane --direction down)
herdr agent start g01-d1 --kind copilot --pane <pane_id> -- --allow-all-tools --model claude-sonnet-5.5 --reasoning-effort medium
herdr agent prompt g01-d1 "<card>" ; then herdr agent read g01-d1 --source recent --lines 30
```

Known pitfalls: (1) after `agent prompt` to Copilot check the screen shows it working, else `herdr agent send-keys
g01-d1 enter`; (2) herdr may show Copilot `blocked` while it works: read the screen before acting; (3) a Copilot
path-access dialog for paths outside its worktree (`/tmp`, `cd sub && …`) must be denied with `esc` and the dev told
to run from the worktree root; (4) `copilot --continue` is machine-wide: for a new MT start a fresh session; (5)
Copilot adds `Co-authored-by: Copilot`: before pushing, `git log origin/develop..HEAD --format=%B | grep -ci
co-authored-by` must be 0, else `git filter-branch -f --msg-filter "grep -vi '^co-authored-by:' || true"
origin/develop..HEAD`; (6) every manual `fui` run uses the sandboxed XDG paths (see above); (7) after rebasing on
`develop` regenerate sqlc/schema and goldens, and run `go mod tidy` if imports changed; (8) `ls` is aliased to a
slow tool on this volume: use `command ls`.
