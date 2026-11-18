# U19 — Transactions: date picker in filters
Branch `feat/u19-tx-datepicker`. Owned: `internal/tui/screens/transactions/filter.go`, `filter_test.go` (new or internal test), keymap entries for the picker in `keymap.go`.

Protocol: `docs/tasks/_PROTOCOL.md` (binding, v0.3 addendum). Plan: `docs/PLAN-tui-revamp.md` §2 (decisions are binding). Effort: low.

Read only: `transactions/filter.go` (`filterFrom`/`filterTo`), `components/datepicker.go` (U10), `transactions/keymap.go`.

Do: when `From` or `To` is focused in the filter panel, `ctrl+d` (help `ctrl+d pick date`) opens the `DatePicker` on the field's current value (or today from `uc.Today()` already loaded by the screen; never `time.Now()`); enter writes `YYYY-MM-DD` into the field, esc returns to the panel unchanged. Typing a date by hand keeps working. While the picker is open the screen's `InputActive()` stays true and View shows the picker in a `components.Modal`.
Tests: open/pick/cancel on both fields, invalid current value opens on today, keymap collision test green.
