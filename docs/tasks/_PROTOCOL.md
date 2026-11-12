# Worker protocol (all tasks from wave 6 on)

Read `AGENTS.md` first; it is binding. Extra rules learned in previous waves:

- Stay inside your worktree. Scratch files go in `./.scratch/` (already git-excluded). Never touch `/tmp`, `$HOME`,
  the module cache or other worktrees; inspect deps with `go doc -src` / `go list -m`. Do not write `/dev/null`
  inside `$(...)`.
- Atomic Conventional Commits, each one builds and passes tests. **No `Co-authored-by` trailer, no mention of
  AI/agents/Claude/Copilot/Gemini** in commit messages. Do not push, do not merge.
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
(format `F<n> <path>:<line> <problem> -> <expected>`), fix each with new atomic commits, update the report adding
`FIXED: F1,F2,...`, and reply `TASK DONE` again.
