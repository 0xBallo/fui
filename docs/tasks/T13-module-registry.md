# T13 — Module registry, config key, enable/disable use cases

Branch `feat/module-core`. Owned paths: `internal/module/` (new), `internal/config/` (only what `modules.enabled`
needs), `internal/app/module.go` + test (new), `internal/app/errors.go` (new code only), `.golangci.yml` (depguard
rule for `internal/module`: stdlib only). Protocol: `docs/tasks/_PROTOCOL.md`.

Spec: tecniche.md "Moduli" (+ "Configurazione"), funzionali.md "Moduli".

1. `internal/module`: `type ID string` with the 10 ids of the spec table; constant registry (deps, default-on,
   in `expense` preset); `type Set` immutable (sorted ids, `Has`, `Require(id) error`, `IDs()`);
   `Preset(name) (Set, error)` for `expense|full` (`custom` is resolved by the caller from `--modules`);
   `Default() Set`; `Validate(ids []ID) (Set, error)` (unknown id / missing dependency / missing core → error
   naming the module); `Enable(s Set, id) (Set, added []ID)` closes over dependencies; `Disable(s Set, id)
   (Set, dependents []ID, err)` — `core` cannot be disabled; dependents must be disabled too (caller confirms).
   Sentinel `ErrDisabled` (wrapped with the module id and the hint `fui module enable <id>`).
   Tests: table-driven + `rapid` property "any sequence of Enable/Disable(+dependents) yields a dependency-closed
   set containing core".
2. `internal/config`: key `modules.enabled` (list of ids) validated through `module.Validate`; default =
   `module.Default()`; invalid → config error with the key name. Check how config writes keys (`config set`) and
   expose what the app needs to persist the list.
3. `internal/app`: Plan/Apply use cases `PlanEnableModule`/`PlanDisableModule` (plan lists modules added /
   dependents disabled; apply writes only `modules.enabled` through the existing config port — add a small port
   if none exists) and `ListModules` (id, enabled, default, deps, preset membership). App error code
   `module_disabled` mapped from `module.ErrDisabled` (look at `internal/app/errors.go` `Codes`).
   No CLI in this task (T14).
