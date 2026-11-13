package simulate

import (
	"errors"
	"fmt"
	"math"
)

// Frequency is how often a PAC contribution is paid.
type Frequency int

// Contribution frequencies, expressed as months between payments.
const (
	FrequencyNone      Frequency = 0
	FrequencyMonthly   Frequency = 1
	FrequencyQuarterly Frequency = 3
	FrequencyYearly    Frequency = 12
)

// Compounding is when accrued interest is capitalised.
type Compounding int

// Compounding modes.
const (
	CompoundMonthly Compounding = iota
	CompoundAnnual
)

// ErrInvalidParams is wrapped by every parameter validation failure.
var ErrInvalidParams = errors.New("simulate: invalid params")

// Params drives the shared investment/PAC engine. Money is in minor units;
// rates are basis points per year.
type Params struct {
	Initial          int64
	Contribution     int64 // paid at the start of each period
	Frequency        Frequency
	ContributionGrow int64 // annual growth of Contribution, bp
	ReturnBP         int64
	CostBP           int64 // annual costs, subtracted from the return
	TaxBP            int64 // rate on the final gain; a parameter, not a tax calculation
	InflationBP      int64
	Years            int
	Compounding      Compounding
}

// YearRow is the state at the end of one year.
type YearRow struct {
	Year          int
	Contributions int64 // cumulative, initial capital included
	Nominal       int64
	Real          int64 // Nominal deflated by inflation
	GrossGain     int64
}

// Result is the outcome of one run.
type Result struct {
	Rows          []YearRow
	Contributions int64
	Nominal       int64
	Real          int64
	GrossGain     int64
	Tax           int64
	NetGain       int64
}

// Rates are the three annual returns compared side by side, in basis points.
type Rates struct{ Pessimistic, Base, Optimistic int64 }

// Triple holds the outcome for each of the three rates.
type Triple struct{ Pessimistic, Base, Optimistic Result }

func (f Frequency) valid() bool {
	return f == FrequencyNone || f == FrequencyMonthly || f == FrequencyQuarterly || f == FrequencyYearly
}

func (p Params) validate() error {
	switch {
	case p.Years < 1 || p.Years > 100:
		return fmt.Errorf("%w: years must be 1..100", ErrInvalidParams)
	case p.InflationBP <= -10000 || p.ContributionGrow <= -10000:
		return fmt.Errorf("%w: inflation and contribution growth must be above -100%%", ErrInvalidParams)
	case p.Initial < 0 || p.Contribution < 0:
		return fmt.Errorf("%w: amounts must not be negative", ErrInvalidParams)
	case p.TaxBP < 0 || p.TaxBP > 10000 || p.CostBP < 0:
		return fmt.Errorf("%w: tax must be 0..10000 bp and cost >= 0", ErrInvalidParams)
	case !p.Frequency.valid():
		return fmt.Errorf("%w: unknown frequency %d", ErrInvalidParams, p.Frequency)
	case p.Compounding != CompoundMonthly && p.Compounding != CompoundAnnual:
		return fmt.Errorf("%w: unknown compounding", ErrInvalidParams)
	}
	return nil
}

// Investment runs the engine for a lump sum: contributions are ignored.
func Investment(p Params) (Result, error) {
	p.Contribution, p.Frequency, p.ContributionGrow = 0, FrequencyNone, 0
	return run(p)
}

// PAC runs the engine with periodic contributions.
func PAC(p Params) (Result, error) { return run(p) }

// Compare runs PAC (Investment when there are no contributions) for the three rates.
func Compare(p Params, r Rates) (Triple, error) {
	var t Triple
	for _, c := range []struct {
		bp  int64
		out *Result
	}{{r.Pessimistic, &t.Pessimistic}, {r.Base, &t.Base}, {r.Optimistic, &t.Optimistic}} {
		q := p
		q.ReturnBP = c.bp
		res, err := run(q)
		if err != nil {
			return Triple{}, err
		}
		*c.out = res
	}
	return t, nil
}

func bp(v int64) float64 { return float64(v) / 10000 }

func run(p Params) (Result, error) {
	if err := p.validate(); err != nil {
		return Result{}, err
	}
	net := bp(p.ReturnBP - p.CostBP)
	if net <= -1 {
		return Result{}, fmt.Errorf("%w: net return must be above -100%%", ErrInvalidParams)
	}
	value, paid, accrued := float64(p.Initial), float64(p.Initial), 0.0
	res := Result{Rows: make([]YearRow, 0, p.Years)}
	for m := 0; m < p.Years*12; m++ {
		if p.Frequency != FrequencyNone && m%int(p.Frequency) == 0 {
			c := float64(p.Contribution) * math.Pow(1+bp(p.ContributionGrow), float64(m/12))
			value += c
			paid += c
		}
		if p.Compounding == CompoundMonthly {
			value *= 1 + net/12
		} else {
			// Annual: simple interest accrues on the capitalised balance, added at year end.
			accrued += value * net / 12
			if m%12 == 11 {
				value += accrued
				accrued = 0
			}
		}
		if m%12 == 11 {
			y := m/12 + 1
			res.Rows = append(res.Rows, YearRow{
				Year:          y,
				Contributions: round(paid),
				Nominal:       round(value),
				Real:          round(value / math.Pow(1+bp(p.InflationBP), float64(y))),
				GrossGain:     round(value) - round(paid),
			})
		}
	}
	last := res.Rows[len(res.Rows)-1]
	res.Contributions, res.Nominal, res.Real, res.GrossGain = last.Contributions, last.Nominal, last.Real, last.GrossGain
	if res.GrossGain > 0 {
		res.Tax = round(float64(res.GrossGain) * bp(p.TaxBP))
	}
	res.NetGain = res.GrossGain - res.Tax
	return res, nil
}

func round(f float64) int64 { return int64(math.Round(f)) }
