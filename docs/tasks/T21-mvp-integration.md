# T21 — MVP integration: e2e, skill, ADRs, coverage top-up

Branch `feat/mvp-integration` (based on `main` after waves 6–8). Owned paths: `cmd/fui/` (tests + testdata
scripts), `internal/skill/`, `docs/adr/`, `README.md`, new `_test.go` files anywhere (no production changes
outside `cmd/fui` and `internal/skill`; production bugs found → fix only if ≤ 10 lines, else `REQ:`).
Protocol: `docs/tasks/_PROTOCOL.md`.

Spec: tecniche.md "Matrice dei moduli", "Golden file come contratto", ADR table (0009–0012); funzionali.md
"Uso con agenti AI", "Moduli" (skill bullet), "Reset dell'archivio".

1. testscript e2e (`cmd/fui/testdata/...`, run by `TestE2E`): (a) search: add tx, `tx search` with accents/
   case-insensitive partial match, filters, `--json` summary, `export` with filters; (b) reset: `reset --yes`,
   `reset --keep-settings --yes`, agent mode without `--yes` → exit code of `confirmation_required`, backup file
   with reason `reset` present; (c) modules matrix: `init --preset expense` and `--preset full`, `module list
   --json`, enable/disable with dependencies, `help --json` golden per preset (`expense`, `full`), a disabled
   module's command → `module_disabled` (if no non-core command exists yet, assert via `module list` and the
   golden only, and say so).
2. Skill (`internal/skill`): document `tx search` + filters, `fui reset` (never without explicit user request),
   `fui module list --json` as the first call to know available commands; update its golden.
3. ADRs 0009–0012 (modules registry, FTS5 search + single TxFilter, reset by rebuild + rename, Catppuccin themes
   via tokens) in the existing ADR format; update ADR 0002 wording on quantity scale.
4. Run `./.github/workflows/scripts/coverage.sh --enforce`; raise any package below 80% that the MVP owns
   (`internal/cli`, `internal/tui/components`, `internal/backup`, …) with meaningful tests until the gate passes
   with margin (total ≥ 82%).
