# U02 — Domain: calculation cycles
Branch `feat/u02-domain-cycle`. Owned: `internal/domain/cycle.go`, `internal/domain/cycle_test.go`.

Protocol: `docs/tasks/_PROTOCOL.md` (binding, v0.3 addendum). Plan: `docs/PLAN-tui-revamp.md` §2 (decisions are binding). Effort: low. Domain rules: stdlib only, pure, no clock.

Read only: `internal/domain/period.go`, `internal/domain/date.go` (API only), `internal/app/reporting/period.go` (how years are built).

Build:
```go
type Cycle string // CycleMonth "month", CycleQuarter "quarter", CycleYear "year", CycleLifetime "lifetime"
func ParseCycle(s string) (Cycle, error)            // invalid -> *Error CodeInvalidValue
func (c Cycle) Next() Cycle                         // month→quarter→year→lifetime→month
// CyclePeriod returns the period of kind c containing anchor. Month = MonthPeriod(anchor, startDay);
// quarter/year = calendar; lifetime = [since, anchor] (since zero -> anchor).
func CyclePeriod(c Cycle, anchor Date, startDay int, since Date) (Period, error)
// ShiftCycle moves anchor by n periods of kind c (n<0 = back); lifetime returns anchor unchanged.
func ShiftCycle(c Cycle, anchor Date, n, startDay int) (Date, error)
```
Tests: table tests incl. startDay 1/15/31, Feb/leap years, quarter edges, year edges, n=±1/±13; rapid properties: `CyclePeriod` contains anchor; shifting +n then -n lands in the same period; consecutive months/quarters are contiguous (no gap/overlap). Coverage ≥ 90%.
