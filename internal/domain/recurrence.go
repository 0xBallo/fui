package domain

import (
	"math/big"
	"strings"
)

// RecurrenceUnit is the cadence unit of a recurrence rule.
type RecurrenceUnit string

// Recurrence units.
const (
	RecurrenceDays   RecurrenceUnit = "days"
	RecurrenceWeeks  RecurrenceUnit = "weeks"
	RecurrenceMonths RecurrenceUnit = "months"
)

// Valid reports whether u is a supported recurrence unit.
func (u RecurrenceUnit) Valid() bool {
	switch u {
	case RecurrenceDays, RecurrenceWeeks, RecurrenceMonths:
		return true
	}
	return false
}

// ParseRecurrenceUnit parses a recurrence unit (case-insensitive).
func ParseRecurrenceUnit(s string) (RecurrenceUnit, error) {
	return parseEnum("unit", s, RecurrenceUnit.Valid)
}

// RecurrenceRule defines an every-N cadence; monthly rules may fix a specific
// day of month. If DayOfMonth is 0 on monthly rules, the start date day is used.
type RecurrenceRule struct {
	Every      int
	Unit       RecurrenceUnit
	DayOfMonth int
}

// NewRecurrenceRule validates and builds a recurrence rule.
func NewRecurrenceRule(every int, unit RecurrenceUnit, dayOfMonth int) (RecurrenceRule, error) {
	if every <= 0 {
		return RecurrenceRule{}, invalidValue("every", "must be > 0")
	}
	if !unit.Valid() {
		return RecurrenceRule{}, enumError("unit", string(unit))
	}
	switch unit {
	case RecurrenceDays:
		if every > maxShiftDays {
			return RecurrenceRule{}, invalidValue("every", "days cadence out of range")
		}
		if dayOfMonth != 0 {
			return RecurrenceRule{}, invalidValue("day_of_month", "must be 0 for day cadence")
		}
	case RecurrenceWeeks:
		if every > maxShiftDays/7 {
			return RecurrenceRule{}, invalidValue("every", "weeks cadence out of range")
		}
		if dayOfMonth != 0 {
			return RecurrenceRule{}, invalidValue("day_of_month", "must be 0 for week cadence")
		}
	case RecurrenceMonths:
		if every > maxShiftMonths {
			return RecurrenceRule{}, invalidValue("every", "months cadence out of range")
		}
		if dayOfMonth < 0 || dayOfMonth > 31 {
			return RecurrenceRule{}, invalidValue("day_of_month", "must be 0 or in [1..31]")
		}
	}
	return RecurrenceRule{Every: every, Unit: unit, DayOfMonth: dayOfMonth}, nil
}

func (r RecurrenceRule) monthDay(start Date) int {
	if r.DayOfMonth > 0 {
		return r.DayOfMonth
	}
	return start.Day()
}

// Recurrence is a periodic transaction template with scheduling boundaries.
type Recurrence struct {
	AccountID   AccountID
	CategoryID  CategoryID
	Description string
	Amount      Money
	Rule        RecurrenceRule
	Start       Date
	End         Date // zero means no end
	NextDate    Date
}

// NewRecurrence validates and builds a recurrence.
func NewRecurrence(r Recurrence) (Recurrence, error) {
	if r.AccountID == 0 {
		return Recurrence{}, missingField("account_id")
	}
	if r.Amount.Currency().IsZero() {
		return Recurrence{}, missingField("amount")
	}
	if r.Start.IsZero() {
		return Recurrence{}, missingField("start")
	}
	if r.NextDate.IsZero() {
		return Recurrence{}, missingField("next_date")
	}
	if _, err := NewRecurrenceRule(r.Rule.Every, r.Rule.Unit, r.Rule.DayOfMonth); err != nil {
		return Recurrence{}, err
	}
	r.Description = strings.TrimSpace(r.Description)
	if r.NextDate.Before(r.Start) {
		return Recurrence{}, invalidValue("next_date", r.NextDate.String())
	}
	if !r.End.IsZero() {
		if r.End.Before(r.Start) {
			return Recurrence{}, invalidValue("end", r.End.String())
		}
		if r.NextDate.After(r.End) {
			return Recurrence{}, invalidValue("next_date", r.NextDate.String())
		}
	}
	return r, nil
}

// Advance moves r.NextDate to the next occurrence. If the recurrence is
// exhausted by End, NextDate becomes the zero Date.
func Advance(r Recurrence) (Recurrence, error) {
	next, ok, err := nextRecurrenceDate(r, r.NextDate)
	if err != nil {
		return Recurrence{}, err
	}
	if !ok {
		r.NextDate = Date{}
		return r, nil
	}
	r.NextDate = next
	return r, nil
}

// Expand returns every recurrence date in [from, until], inclusive.
func Expand(r Recurrence, from, until Date) []Date {
	if !validExpandWindow(r, from, until) {
		return nil
	}
	return collectOccurrences(r, alignToFrom(r, from), until)
}

func validExpandWindow(r Recurrence, from, until Date) bool {
	if from.IsZero() || until.IsZero() || until.Before(from) || r.NextDate.IsZero() {
		return false
	}
	return r.End.IsZero() || !from.After(r.End)
}

func alignToFrom(r Recurrence, from Date) Date {
	current := r.NextDate
	if current.Before(r.Start) {
		current = r.Start
	}
	for current.Before(from) {
		next, ok, err := nextRecurrenceDate(r, current)
		if err != nil || !ok || !next.After(current) {
			return Date{}
		}
		current = next
	}
	return current
}

func collectOccurrences(r Recurrence, current, until Date) []Date {
	if current.IsZero() || current.After(until) {
		return nil
	}
	out := make([]Date, 0, 8)
	for !current.After(until) {
		out = append(out, current)
		next, ok, err := nextRecurrenceDate(r, current)
		if err != nil || !ok || !next.After(current) {
			return out
		}
		current = next
	}
	return out
}

func nextRecurrenceDate(r Recurrence, current Date) (Date, bool, error) {
	if current.IsZero() {
		return Date{}, false, missingField("date")
	}
	var (
		next Date
		err  error
	)
	switch r.Rule.Unit {
	case RecurrenceDays:
		next, err = current.AddDays(r.Rule.Every)
	case RecurrenceWeeks:
		next, err = current.AddDays(7 * r.Rule.Every)
	case RecurrenceMonths:
		next, err = nextMonthlyDate(current, r.Rule.Every, r.Rule.monthDay(r.Start))
	default:
		return Date{}, false, enumError("unit", string(r.Rule.Unit))
	}
	if err != nil {
		return Date{}, false, err
	}
	if !r.End.IsZero() && next.After(r.End) {
		return Date{}, false, nil
	}
	return next, true, nil
}

func nextMonthlyDate(current Date, everyMonths, dayOfMonth int) (Date, error) {
	monthStart, err := NewDate(current.Year(), current.Month(), 1)
	if err != nil {
		return Date{}, err
	}
	targetMonth, err := monthStart.AddMonths(everyMonths)
	if err != nil {
		return Date{}, err
	}
	dom := dayOfMonth
	if dom < 1 {
		dom = current.Day()
	}
	return NewDate(targetMonth.Year(), targetMonth.Month(), min(dom, daysIn(targetMonth.Year(), targetMonth.Month())))
}

// MatchPending finds the pending transaction that best matches booked.
//
// Rules:
//   - same account
//   - same currency
//   - amount within toleranceMinor (0 means exact)
//   - date distance <= 3 days
//
// Tie-break is deterministic: closest date first, then lower id.
func MatchPending(pending []Transaction, booked Transaction, toleranceMinor int64) (Transaction, bool) {
	if booked.AccountID == 0 || booked.Date.IsZero() || booked.Amount.Currency().IsZero() {
		return Transaction{}, false
	}
	tolerance := toleranceMinor
	if tolerance < 0 {
		tolerance = -tolerance
	}
	best := Transaction{}
	found := false
	bestDistance := 4
	for i := range pending {
		candidate := pending[i]
		if !pendingCandidateMatches(candidate, booked, tolerance) {
			continue
		}
		distance := dateDistanceDays(candidate.Date, booked.Date)
		if !found || distance < bestDistance || (distance == bestDistance && candidate.ID < best.ID) {
			best = candidate
			found = true
			bestDistance = distance
		}
	}
	if !found {
		return Transaction{}, false
	}
	return best, true
}

func pendingCandidateMatches(candidate, booked Transaction, tolerance int64) bool {
	if candidate.Status != StatusPending || candidate.AccountID != booked.AccountID {
		return false
	}
	if candidate.Date.IsZero() || candidate.Amount.Currency().IsZero() {
		return false
	}
	if candidate.Amount.Currency() != booked.Amount.Currency() {
		return false
	}
	if !withinAmountTolerance(candidate.Amount.Minor(), booked.Amount.Minor(), tolerance) {
		return false
	}
	return dateDistanceDays(candidate.Date, booked.Date) <= 3
}

func withinAmountTolerance(leftMinor, rightMinor, tolerance int64) bool {
	delta := new(big.Int).Sub(big.NewInt(leftMinor), big.NewInt(rightMinor))
	delta.Abs(delta)
	return delta.Cmp(big.NewInt(tolerance)) <= 0
}
