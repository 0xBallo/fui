# U13 — Dashboard: frame, cycles, net worth
Branch `feat/u13-dash-frame`. Owned: `internal/tui/screens/dashboard/` except `insight*` (U17 owns those).

Protocol: `docs/tasks/_PROTOCOL.md` (binding, v0.3 addendum). Plan: `docs/PLAN-tui-revamp.md` §2 (decisions are binding). Effort: low. Wiring note: `cmd/fui` passes `tuiUseCases{*app.Service, ...}`, so a method of `*app.Service` added to a screen `UseCases` interface needs no wiring; anything else is an **optional port** with a setter, wired by U27.

Read only: `dashboard/{model,networth,ports,keymap}.go`, `internal/domain/cycle.go` (U02), `components/bigtext.go` (U03), `app.NetWorth` in `internal/app/fx.go`.

Do:
1. **Sections**: in `section.go` define
   ```go
   type sectionParams struct{ Period domain.Period; AsOf, Today domain.Date; Base domain.Currency; MonthStartDay int }
   type section interface {
       load(p sectionParams) tea.Cmd           // nil when disabled
       update(msg tea.Msg) (tea.Cmd, bool)     // true = consumed (own msg types only, plus keys when focused)
       view(width int) string                  // "" = hidden
       keys() []key.Binding
   }
   ```
   Create stubs `runway.go`, `flow.go`, `recent.go`, `alerts.go`, each with an unexported struct implementing `section` that returns nil/""; U14–U17 fill them. Model holds the five sections in order: net worth, runway, flow, recent, alerts, and batches their `load`.
2. **Cycle**: model state `cycle domain.Cycle` (default month) and `anchor domain.Date` (`Today()` on Init). Keys: `p` next cycle, `h/←` previous, `l/→` next (disabled when the period contains today). Lifetime `since` = date of the oldest transaction (`ListTx` sorted by date ascending, `Limit: 1`; add `ListTx` to `UseCases`). Header line: `◀ November 2026 · month ▶` (quarter `Q4 2026`, year `2026`, lifetime `since 2024-03-01`). `AsOf = min(period.To, today)`.
3. **Net worth section** (`networth.go`): gross = Σ Converted of non-loan lines of `NetWorth(At: AsOf)`, net = gross + Σ loan lines. Gross in `components.BigText` (fallback to bold text when `BigTextWidth` > width) followed by the currency code; below, small: `net of loans <amount>`; v0.3 positions/assets lines stay as muted lines. Errors keep the current `n/a (...)` text.
4. **Layout**: drop the accounts table and the spending chart (and the `Spending` port if now unused). Render sections joined by blank lines inside a `bubbles/viewport` (up/down/pgup/pgdown scroll when no section is focused); width ≥ 120 → two columns (net worth+runway+flow | recent+alerts). Keep the quick-add form and the existing insight panel rendered after the sections until U17.
Tests: gross/net helper table test (no loans, loans, archived excluded as today); cycle keys change the period and reload; next disabled at current period; golden 80×20 and 140×30 with the fixture.
