# U14 — Dashboard: runway countdown
Branch `feat/u14-dash-runway`. Owned: `internal/tui/screens/dashboard/runway.go`, `runway_test.go`.

Protocol: `docs/tasks/_PROTOCOL.md` (binding, v0.3 addendum). Plan: `docs/PLAN-tui-revamp.md` §2 (decisions are binding). Effort: low. Do not edit `model.go`: the section stub and the `section` interface come from U13 (`section.go`).

Read only: `dashboard/section.go`, `dashboard/runway.go` (stub), `reporting.RunwayParams/RunwayResult` in `internal/app/reporting/runway.go`, the old `viewRunway` in git history (`git show develop:internal/tui/screens/dashboard/model.go`).

Do: load `Runway(RunwayParams{AsOf: p.AsOf, BaseCurrency: p.Base, MonthStartDay: p.MonthStartDay})` in a `tea.Cmd` returning an own msg type; view = countdown `1y 4m 12d` (calendar difference AsOf → ZeroDate, pure helper `countdown(from, to domain.Date) (y, m, d int)`) in `th.Title` and `zero on 2027-03-30` in `th.Muted`. `domain.ErrNotEnoughData` → muted "not enough data (needs 3 complete months)"; other errors → `screens.ErrorMsg`.
Tests: `countdown` table test (same day, month-end 31→28, leap day, > 10 years); view for available/not-enough-data; msg routing.
