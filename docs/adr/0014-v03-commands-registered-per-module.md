# ADR 0014: v0.3 services built in the composition root and commands gated by module

* Status: Accepted
* Date: 2026-11-15

## Context and Problem Statement

v0.3 adds investing, assets, simulations and insights. Each ships as a cobra constructor plus a service; nothing is reachable until one place composes them, and ADR 0009 requires every command to belong to a module.

## Decision Outcome

- `cmd/fui` builds `investing`, `assetting` and `simulating` services and stores them in `cli.Deps`; `internal/cli/root.go` registers `security`, `invest`, `position` (investments), `asset` (assets), `simulate` (simulations), `insights` (analytics) and `undo` (core), and `report compare`.
- The module gate stays the single enforcement point (annotation + `PersistentPreRunE`, hidden in `help --json` when off). Services also gate by the enabled set, so a nil or disabled service fails with `module_disabled`, never a panic.
- The TUI receives its v0.3 use cases through `tui.Config`; Root wires a feature only when its module is enabled, and the adapters in `cmd/fui` rebuild services per call so Settings can toggle modules live.

### Consequences

* **Positive:** one wiring point; module matrix testable end to end with testscript and per-preset `help --json` goldens.
* **Negative:** `cli.Deps` grows one field per service.
