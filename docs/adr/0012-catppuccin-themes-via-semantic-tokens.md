# ADR 0012: Catppuccin-based TUI themes through semantic tokens

* Status: Accepted
* Date: 2026-11-12

## Context and Problem Statement

The TUI needs consistent theming (light/dark variants, accessible contrast, no color-only
semantics) while avoiding hard-coded colors scattered across components. Direct palette usage in
every widget makes theme changes risky and inconsistent.

## Considered Options

1. **Literal colors in components:** fastest to write, but unmaintainable and inconsistent.
2. **Terminal-default-only styling:** simple, but insufficient control for contrast and visual
   hierarchy.
3. **Semantic token layer in `internal/tui/theme` mapped to Catppuccin flavors, with automatic
   light/dark resolution:** centralized and testable.

## Decision Outcome

Chosen option: **Semantic tokens mapped to Catppuccin themes**, because:
- Components reference meaning (`PrimaryText`, `Danger`, `BorderMuted`, ...) instead of concrete
  colors, so theme swaps do not touch feature code.
- Catppuccin provides coherent palettes (Latte, Frappé, Macchiato, Mocha) for predictable cross-
  screen styling.
- `theme=auto` can resolve light/dark flavor from terminal background detection while preserving
  explicit user choices.

### Consequences

* **Positive:**
  - Centralized theming with clear ownership in `internal/tui/theme`.
  - Easier accessibility testing and contrast guarantees across all screens.
  - Lint rules can forbid literal colors outside theme packages.
* **Negative:**
  - Introducing new UI semantics may require expanding the token catalog.
  - Theme package complexity grows with detection and compatibility logic.
