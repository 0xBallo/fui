# T14 — CLI for modules: `fui module`, `init --preset/--modules`, command annotations

Branch `feat/module-cli` (based on `main` after T13 is merged). Owned paths: `internal/cli/module*.go` (new),
`internal/cli/init.go`, `internal/cli/root.go` (hook + registration), `internal/cli/help_json.go`,
`internal/cli/exitcode.go`, every `internal/cli/*.go` command constructor (one annotation line each),
`internal/cli/testdata/` goldens, `internal/cli/deps.go`, `cmd/fui/` wiring only. Protocol: `docs/tasks/_PROTOCOL.md`.

Spec: tecniche.md "Moduli" + CLI "Moduli", "Modalità agente"; funzionali.md "Moduli" (Attivazione, Effetti).
Read the merged `internal/module` and `internal/app/module.go` first.

1. `fui module list [--json]` (id, enabled, default, depends_on, presets — stable JSON, golden),
   `fui module enable <id>` / `disable <id>`: Plan/Apply with `--dry-run`/`--yes`, the plan shows modules added
   for dependencies (warning) or dependents that will be disabled (needs confirmation; agent mode without `--yes`
   → `confirmation_required`).
2. `fui init --preset expense|full|custom` and `--modules a,b,c` (`custom` requires `--modules`; `--modules`
   alone implies custom); in a TTY without flags ask the profile with a prompt (reuse `prompt.go`); non-TTY
   default = `module.Default()`. Persist `modules.enabled` through the app use case.
3. Every cobra command gets `Annotations["module"]` (all current commands are `core`); a root
   `PersistentPreRunE` hook returns `module_disabled` (exit 1, message with `fui module enable <id>`) when the
   module is off; commands of disabled modules are `Hidden` in `--help` and omitted from `help --json`.
   Test that walks the whole cobra tree and fails if a command lacks the annotation or uses an unknown id.
4. Exit-code table: `module_disabled` → 1. Update `help --json` golden.
