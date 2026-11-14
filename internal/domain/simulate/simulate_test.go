package simulate

import (
	"errors"
	"reflect"
	"testing"

	"pgregory.net/rapid"

	"github.com/0xballo/fui/internal/domain"
)

func date(t testing.TB, y, m, d int) domain.Date {
	t.Helper()
	v, err := domain.NewDate(y, m, d)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestInvestmentHandComputed(t *testing.T) {
	tests := []struct {
		name string
		p    Params
		want Result
	}{
		{
			"annual 5% 2y",
			Params{Initial: 100000, ReturnBP: 500, Years: 2, Compounding: CompoundAnnual},
			Result{Contributions: 100000, Nominal: 110250, Real: 110250, GrossGain: 10250, NetGain: 10250},
		},
		{
			"tax 26% + cost 1%",
			Params{Initial: 100000, ReturnBP: 600, CostBP: 100, TaxBP: 2600, Years: 1, Compounding: CompoundAnnual},
			Result{Contributions: 100000, Nominal: 105000, Real: 105000, GrossGain: 5000, Tax: 1300, NetGain: 3700},
		},
		{
			"inflation 10% 1y",
			Params{Initial: 110000, InflationBP: 1000, Years: 1},
			Result{Contributions: 110000, Nominal: 110000, Real: 100000},
		},
		{
			"monthly 12%",
			Params{Initial: 100000, ReturnBP: 1200, Years: 1},
			Result{Contributions: 100000, Nominal: 112683, Real: 112683, GrossGain: 12683, NetGain: 12683},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Investment(tc.p)
			if err != nil {
				t.Fatal(err)
			}
			got.Rows = nil
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %+v want %+v", got, tc.want)
			}
		})
	}
}

func TestPACHandComputed(t *testing.T) {
	// 100/month, 0% return, 2 years, contribution grows 10%/year: 1200 + 1320.
	got, err := PAC(Params{Contribution: 10000, Frequency: FrequencyMonthly, ContributionGrow: 1000, Years: 2})
	if err != nil {
		t.Fatal(err)
	}
	if got.Contributions != 252000 || got.Nominal != 252000 || len(got.Rows) != 2 || got.Rows[0].Contributions != 120000 {
		t.Fatalf("%+v", got)
	}
	// Yearly contribution of 1000 at start of year, 10% annual: 1000*1.1 + 1000 → (1100+1000)*1.1 = 2310.
	got, _ = PAC(Params{Contribution: 100000, Frequency: FrequencyYearly, ReturnBP: 1000, Years: 2, Compounding: CompoundAnnual})
	if got.Nominal != 231000 {
		t.Fatalf("yearly: %d", got.Nominal)
	}
	// Quarterly: 4 payments/year.
	got, _ = PAC(Params{Contribution: 100, Frequency: FrequencyQuarterly, Years: 1})
	if got.Contributions != 400 {
		t.Fatalf("quarterly: %d", got.Contributions)
	}
}

func TestCompare(t *testing.T) {
	tr, err := Compare(Params{Initial: 100000, Years: 5}, Rates{0, 300, 600})
	if err != nil {
		t.Fatal(err)
	}
	if tr.Pessimistic.Nominal != 100000 || tr.Base.Nominal >= tr.Optimistic.Nominal || tr.Base.Nominal <= tr.Pessimistic.Nominal {
		t.Fatalf("%+v", tr)
	}
	if _, err := Compare(Params{}, Rates{}); !errors.Is(err, ErrInvalidParams) {
		t.Fatal(err)
	}
}

func TestInvalidParams(t *testing.T) {
	ok := Params{Initial: 1, Years: 1}
	bad := []Params{
		{Years: 0},
		{Initial: -1, Years: 1},
		{Years: 1, TaxBP: 10001},
		{Years: 1, CostBP: -1},
		{Years: 1, Frequency: 5},
		{Years: 1, Compounding: 9},
		{Years: 1, ReturnBP: -10000},
	}
	for i, p := range bad {
		if _, err := PAC(p); !errors.Is(err, ErrInvalidParams) {
			t.Errorf("case %d: %v", i, err)
		}
	}
	if _, err := Investment(ok); err != nil {
		t.Fatal(err)
	}
}

func genParams(t *rapid.T) Params {
	return Params{
		Initial:          rapid.Int64Range(0, 1e9).Draw(t, "init"),
		Contribution:     rapid.Int64Range(0, 1e7).Draw(t, "contrib"),
		Frequency:        rapid.SampledFrom([]Frequency{FrequencyNone, FrequencyMonthly, FrequencyQuarterly, FrequencyYearly}).Draw(t, "freq"),
		ContributionGrow: rapid.Int64Range(0, 2000).Draw(t, "grow"),
		Years:            rapid.IntRange(1, 40).Draw(t, "years"),
		Compounding:      rapid.SampledFrom([]Compounding{CompoundMonthly, CompoundAnnual}).Draw(t, "comp"),
	}
}

func TestPropertyZeroReturnIsContributions(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		res, err := PAC(genParams(t))
		if err != nil {
			t.Fatal(err)
		}
		if d := res.Nominal - res.Contributions; d != 0 || res.GrossGain != 0 {
			t.Fatalf("%+v", res)
		}
	})
}

func TestPropertyMonotonicInRate(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		p := genParams(t)
		lo := rapid.Int64Range(0, 2000).Draw(t, "lo")
		hi := lo + rapid.Int64Range(0, 2000).Draw(t, "d")
		tr, err := Compare(p, Rates{lo, lo, hi})
		if err != nil {
			t.Fatal(err)
		}
		if tr.Optimistic.Nominal < tr.Pessimistic.Nominal {
			t.Fatalf("hi rate lower: %d < %d", tr.Optimistic.Nominal, tr.Pessimistic.Nominal)
		}
	})
}

func TestPropertyPACZeroContributionIsInvestment(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		p := genParams(t)
		p.ReturnBP = rapid.Int64Range(0, 2000).Draw(t, "ret")
		p.Contribution = 0
		a, _ := PAC(p)
		b, _ := Investment(p)
		if a.Nominal != b.Nominal || a.Real != b.Real || a.NetGain != b.NetGain {
			t.Fatalf("%+v vs %+v", a, b)
		}
	})
}

func TestCashFlow(t *testing.T) {
	start := date(t, 2026, 1, 31)
	base := []Flow{
		{date(t, 2026, 2, 10), 300000}, // salary month 1
		{date(t, 2026, 3, 10), 300000}, // salary month 2
		{date(t, 2026, 4, 10), 300000},
		{date(t, 2026, 1, 15), 999}, // before start: ignored
		{date(t, 2030, 1, 1), 999},  // past horizon: ignored
		{date(t, 2026, 2, 20), -100000},
	}
	t.Run("no events", func(t *testing.T) {
		r, err := CashFlow(start, 50000, base, nil, 3)
		if err != nil {
			t.Fatal(err)
		}
		if r.Months[0].Base != 250000 || r.Months[2].Scenario != 850000 || r.EndDelta != 0 || r.HasFirstBelowZero {
			t.Fatalf("%+v", r)
		}
	})
	t.Run("stop income + oneoff + recurring", func(t *testing.T) {
		ev := []Event{
			{Kind: StopIncome, Date: date(t, 2026, 3, 1)},
			{Kind: OneOff, Date: date(t, 2026, 2, 5), Amount: -100000},
			{Kind: RecurringChange, Date: date(t, 2026, 3, 1), Amount: -50000},
		}
		r, err := CashFlow(start, 50000, base, ev, 4)
		if err != nil {
			t.Fatal(err)
		}
		// m1: 50000+300000-100000-100000=150000; m2: 150000-50000=100000;
		// m3: 100000-50000=50000 (salary stopped); m4: 0 -> not below zero.
		want := []int64{150000, 100000, 50000, 0}
		for i, w := range want {
			if r.Months[i].Scenario != w {
				t.Fatalf("m%d=%d want %d", i+1, r.Months[i].Scenario, w)
			}
		}
		if r.HasFirstBelowZero || r.EndDelta != -850000 {
			t.Fatalf("%+v", r)
		}
	})
	t.Run("loan, below zero, runway", func(t *testing.T) {
		ev := []Event{{Kind: NewLoan, Date: start, Amount: 120000, RateBP: 0, Years: 1}} // 10000/month
		r, err := CashFlow(start, 25000, nil, ev, 4)
		if err != nil {
			t.Fatal(err)
		}
		// credit 120000 in month 1: 145000-10000, then -10000/month, never below zero in 4 months.
		if r.Months[0].Scenario != 135000 || r.HasFirstBelowZero {
			t.Fatalf("%+v", r)
		}
		// opening -130000: credit lifts month 1 to -20000 -> below zero immediately.
		r, err = CashFlow(start, -130000, nil, ev, 4)
		if err != nil || !r.HasFirstBelowZero || r.RunwayMonths != 0 {
			t.Fatalf("%+v %v", r, err)
		}
		// credit dated later lands in its own month (2); installments start a month later.
		ev[0].Date = date(t, 2026, 3, 15)
		r, _ = CashFlow(start, 25000, nil, ev, 4)
		if r.Months[0].Scenario != 25000 || r.Months[1].Scenario != 25000+120000 {
			t.Fatalf("%+v", r)
		}
	})
}

func TestCashFlowErrors(t *testing.T) {
	s := date(t, 2026, 1, 1)
	for name, f := range map[string]func() error{
		"months": func() error { _, e := CashFlow(s, 0, nil, nil, 0); return e },
		"start":  func() error { _, e := CashFlow(domain.Date{}, 0, nil, nil, 1); return e },
		"kind":   func() error { _, e := CashFlow(s, 0, nil, []Event{{Kind: 99}}, 1); return e },
		"loan":   func() error { _, e := CashFlow(s, 0, nil, []Event{{Kind: NewLoan}}, 1); return e },
	} {
		if err := f(); !errors.Is(err, ErrInvalidParams) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestAnnuity(t *testing.T) {
	// 100000 at 6%/12y... known: 10000.00 over 1y at 12% => 888.49/month.
	got, err := annuity(1000000, 1200, 1)
	if err != nil || got != 88849 {
		t.Fatalf("%d %v", got, err)
	}
}
