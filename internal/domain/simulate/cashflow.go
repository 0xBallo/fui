package simulate

import (
	"fmt"
	"math"

	"github.com/0xballo/fui/internal/domain"
)

// EventKind classifies a hypothetical cash-flow event.
type EventKind int

// Event kinds.
const (
	// OneOff adds Amount (signed) on Date.
	OneOff EventKind = iota + 1
	// RecurringChange adds Amount (signed) every month from Date on.
	RecurringChange
	// StopIncome drops every positive base flow dated on or after Date.
	StopIncome
	// NewLoan pays the annuity installments of Amount at RateBP over Years,
	// the first one a month after Date. The capital inflow, if any, is a
	// separate OneOff.
	NewLoan
)

// Event is one hypothetical change applied on top of the base forecast.
type Event struct {
	Kind   EventKind
	Date   domain.Date
	Amount int64
	RateBP int64
	Years  int
}

// Flow is one dated base-forecast movement (signed, minor units).
type Flow struct {
	Date   domain.Date
	Amount int64
}

// MonthPoint is the end-of-month balance for base and scenario.
type MonthPoint struct {
	End      domain.Date
	Base     int64
	Scenario int64
}

// CashFlowResult compares the scenario with the base forecast.
type CashFlowResult struct {
	Months            []MonthPoint
	FirstBelowZero    domain.Date
	HasFirstBelowZero bool
	EndDelta          int64 // scenario minus base at the horizon
	// RunwayMonths is the count of full months with a non-negative scenario
	// balance before the first negative one; meaningful only if HasFirstBelowZero.
	RunwayMonths int
}

// CashFlow projects `months` monthly balances from start with and without events.
// Month i covers (start+i-1 months, start+i months]; flows on or before start
// are taken as already in opening, flows past the horizon are ignored.
func CashFlow(start domain.Date, opening int64, base []Flow, events []Event, months int) (CashFlowResult, error) {
	if start.IsZero() || months < 1 {
		return CashFlowResult{}, fmt.Errorf("%w: start and months >= 1 required", ErrInvalidParams)
	}
	ends := make([]domain.Date, months)
	for i := range ends {
		d, err := start.AddMonths(i + 1)
		if err != nil {
			return CashFlowResult{}, err
		}
		ends[i] = d
	}
	baseM, scenM := make([]int64, months), make([]int64, months)
	put := func(dst []int64, d domain.Date, amt int64) {
		if d.After(start) {
			for i, e := range ends {
				if !d.After(e) {
					dst[i] += amt
					return
				}
			}
		}
	}
	for _, f := range base {
		put(baseM, f.Date, f.Amount)
		if !stopped(f, events) {
			put(scenM, f.Date, f.Amount)
		}
	}
	for _, ev := range events {
		if err := applyEvent(ev, ends, scenM, put); err != nil {
			return CashFlowResult{}, err
		}
	}
	res := CashFlowResult{Months: make([]MonthPoint, months)}
	b, s := opening, opening
	for i := range ends {
		b += baseM[i]
		s += scenM[i]
		res.Months[i] = MonthPoint{End: ends[i], Base: b, Scenario: s}
		if s < 0 && !res.HasFirstBelowZero {
			res.HasFirstBelowZero, res.FirstBelowZero, res.RunwayMonths = true, ends[i], i
		}
	}
	res.EndDelta = s - b
	return res, nil
}

func stopped(f Flow, events []Event) bool {
	for _, ev := range events {
		if ev.Kind == StopIncome && f.Amount > 0 && !f.Date.Before(ev.Date) {
			return true
		}
	}
	return false
}

func applyEvent(ev Event, ends []domain.Date, scen []int64, put func([]int64, domain.Date, int64)) error {
	switch ev.Kind {
	case OneOff:
		put(scen, ev.Date, ev.Amount)
	case RecurringChange:
		for i, e := range ends {
			if !e.Before(ev.Date) {
				scen[i] += ev.Amount
			}
		}
	case StopIncome:
	case NewLoan:
		pay, err := annuity(ev.Amount, ev.RateBP, ev.Years)
		if err != nil {
			return err
		}
		for k := 1; k <= ev.Years*12; k++ {
			d, err := ev.Date.AddMonths(k)
			if err != nil {
				return err
			}
			put(scen, d, -pay)
		}
	default:
		return fmt.Errorf("%w: unknown event kind %d", ErrInvalidParams, ev.Kind)
	}
	return nil
}

// annuity is the constant monthly installment, rounded to the nearest minor unit.
func annuity(capital, rateBP int64, years int) (int64, error) {
	if capital <= 0 || years < 1 || rateBP < 0 {
		return 0, fmt.Errorf("%w: loan needs capital > 0, years >= 1, rate >= 0", ErrInvalidParams)
	}
	n := float64(years * 12)
	r := bp(rateBP) / 12
	if r == 0 {
		return round(float64(capital) / n), nil
	}
	return round(float64(capital) * r / (1 - math.Pow(1+r, -n))), nil
}
