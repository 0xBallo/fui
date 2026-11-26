package domain

import (
	"testing"

	"pgregory.net/rapid"
)

func TestPeriodFor(t *testing.T) {
	type want struct{ from, to Date }
	cases := []struct {
		name   string
		period BudgetPeriod
		d      Date
		start  int
		want   want
	}{
		{"month s1", BudgetMonthly, bdate(t, 2026, 3, 17), 1, want{bdate(t, 2026, 3, 1), bdate(t, 2026, 3, 31)}},
		{"month s15 before", BudgetMonthly, bdate(t, 2026, 1, 10), 15, want{bdate(t, 2025, 12, 15), bdate(t, 2026, 1, 14)}},
		{"year s1", BudgetYearly, bdate(t, 2026, 12, 31), 1, want{bdate(t, 2026, 1, 1), bdate(t, 2026, 12, 31)}},
		{"year s1 jan", BudgetYearly, bdate(t, 2026, 1, 1), 1, want{bdate(t, 2026, 1, 1), bdate(t, 2026, 12, 31)}},
		{"year s15 jan early", BudgetYearly, bdate(t, 2026, 1, 10), 15, want{bdate(t, 2025, 1, 15), bdate(t, 2026, 1, 14)}},
		{"year s15 dec late", BudgetYearly, bdate(t, 2025, 12, 20), 15, want{bdate(t, 2025, 1, 15), bdate(t, 2026, 1, 14)}},
		{"year s15 boundary", BudgetYearly, bdate(t, 2026, 1, 15), 15, want{bdate(t, 2026, 1, 15), bdate(t, 2027, 1, 14)}},
		{"year s31 leap", BudgetYearly, bdate(t, 2024, 6, 1), 31, want{bdate(t, 2024, 1, 31), bdate(t, 2025, 1, 30)}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			b := testBudget(t, BudgetCap, 100)
			b.Period = c.period
			got, err := b.PeriodFor(c.d, c.start)
			if err != nil || got.From() != c.want.from || got.To() != c.want.to {
				t.Fatalf("got %v..%v, %v; want %v..%v", got.From(), got.To(), err, c.want.from, c.want.to)
			}
		})
	}
	b := testBudget(t, BudgetCap, 100)
	if _, err := b.PeriodFor(bdate(t, 2026, 1, 1), 0); err == nil {
		t.Fatal("start day 0 must fail")
	}
	b.Period = BudgetYearly
	if _, err := b.PeriodFor(bdate(t, 2026, 1, 1), 32); err == nil {
		t.Fatal("start day 32 must fail")
	}
}

func TestBudgetPeriodsFor(t *testing.T) {
	b := testBudget(t, BudgetCap, 100)
	b.Period = BudgetYearly
	ps, err := BudgetPeriodsFor(b, bdate(t, 2025, 6, 1), 2, 15)
	if err != nil || len(ps) != 2 || ps[0].From() != bdate(t, 2025, 1, 15) || ps[1].From() != bdate(t, 2026, 1, 15) ||
		ps[1].To() != bdate(t, 2027, 1, 14) {
		t.Fatalf("yearly: %v, %v", ps, err)
	}
	b.Period = BudgetMonthly
	ps, err = BudgetPeriodsFor(b, bdate(t, 2025, 12, 20), 3, 1)
	if err != nil || len(ps) != 3 || ps[1].From() != bdate(t, 2026, 1, 1) {
		t.Fatalf("monthly: %v, %v", ps, err)
	}
	if _, err := BudgetPeriodsFor(b, bdate(t, 2025, 1, 1), 1, 0); err == nil {
		t.Fatal("bad start day must fail")
	}
}

func TestSuggestedMonthly(t *testing.T) {
	year := func(sd int) Period {
		b := testBudget(t, BudgetCap, 1)
		b.Period = BudgetYearly
		p, err := b.PeriodFor(bdate(t, 2026, 6, 1), sd)
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	cases := []struct {
		name      string
		remaining int64
		asOf      Date
		start     int
		want      int64
	}{
		{"full year", 120000, bdate(t, 2026, 1, 5), 1, 10000},
		{"mid year 7 left", 70000, bdate(t, 2026, 6, 30), 1, 10000},
		{"last month", 5555, bdate(t, 2026, 12, 31), 1, 5555},
		{"after end", 5555, bdate(t, 2027, 3, 1), 1, 5555},
		{"before start", 1200, bdate(t, 2025, 6, 1), 1, 100},
		{"round half up", 5, bdate(t, 2026, 10, 1), 1, 2},
		{"round down", 10, bdate(t, 2026, 10, 1), 1, 3},
		{"negative half away", -5, bdate(t, 2026, 11, 1), 1, -3},
		{"negative", -9000, bdate(t, 2026, 10, 1), 1, -3000},
		{"zero", 0, bdate(t, 2026, 2, 1), 1, 0},
		{"start 15 budget month", 1200, bdate(t, 2026, 1, 10), 15, 100},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := SuggestedMonthly(money(t, c.remaining, "EUR"), year(c.start), c.asOf, c.start)
			if err != nil || got.Minor() != c.want || got.Currency().String() != "EUR" {
				t.Fatalf("got %v, %v; want %d", got, err, c.want)
			}
		})
	}
	if _, err := SuggestedMonthly(money(t, 1, "EUR"), year(1), bdate(t, 2026, 1, 1), 0); err == nil {
		t.Fatal("bad start day must fail")
	}
}

func TestNewBudgetThresholds(t *testing.T) {
	for _, th := range []BudgetThresholds{{80, 80}, {100, 80}, {0, 100}, {80, 0}, {-1, 50}} {
		b := Budget{Name: "x", Mode: BudgetCap, Amount: money(t, 1, "EUR"), Targets: []BudgetTarget{{CategoryID: 1}}, Thresholds: th}
		if _, err := NewBudget(b); err == nil {
			t.Fatalf("%+v must be invalid", th)
		}
	}
	b := Budget{Name: "x", Mode: BudgetCap, Amount: money(t, 1, "EUR"), Targets: []BudgetTarget{{CategoryID: 1}}, Thresholds: BudgetThresholds{50, 150}}
	if got, err := NewBudget(b); err != nil || got.Thresholds != (BudgetThresholds{50, 150}) {
		t.Fatalf("over>100 must be valid: %+v, %v", got.Thresholds, err)
	}
}

// A yearly cap over the year period spends exactly what the twelve monthly caps spend, and carry
// conserves money for both period lengths.
func TestYearlyCapRapid(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		start := rapid.IntRange(1, 28).Draw(t, "start")
		b := testBudget(t, BudgetCap, rapid.Int64Range(0, 1e6).Draw(t, "amount"))
		months, err := BudgetPeriodsFor(b, bdate(t, 2026, 1, 20), 12, start)
		if err != nil {
			t.Fatal(err)
		}
		yb := b
		yb.Period = BudgetYearly
		year, err := yb.PeriodFor(months[0].From(), start)
		if err != nil {
			t.Fatal(err)
		}
		// Anchor the 12 months on the year period so both cover the same rows.
		if months, err = BudgetPeriodsFor(b, year.From(), 12, start); err != nil {
			t.Fatal(err)
		}
		if months[0].From() != year.From() || months[11].To() != year.To() {
			t.Fatalf("year %v..%v vs months %v..%v", year.From(), year.To(), months[0].From(), months[11].To())
		}
		var rows []BudgetRow
		for range rapid.IntRange(0, 20).Draw(t, "rows") {
			p := months[rapid.IntRange(0, 11).Draw(t, "p")]
			rows = append(rows, spend(t, p.From(), rapid.Int64Range(0, 1e6).Draw(t, "spend"), StatusBooked))
		}
		opening := rapid.Int64Range(-1e6, 1e6).Draw(t, "opening")
		om := money(t, opening, "EUR")
		monthly, err := CapStatuses(b, months, rows, om)
		if err != nil {
			t.Fatal(err)
		}
		yearly, err := CapStatuses(yb, []Period{year}, rows, om)
		if err != nil {
			t.Fatal(err)
		}
		var sum, amounts int64
		for _, s := range monthly {
			sum += s.Spent.Minor()
			amounts += s.Amount.Minor()
		}
		if got := yearly[0].Spent.Minor(); got != sum {
			t.Fatalf("yearly spent %d != monthly sum %d", got, sum)
		}
		if want := opening + yearly[0].Amount.Minor() - sum; yearly[0].Remaining.Minor() != want {
			t.Fatalf("yearly remaining %d want %d", yearly[0].Remaining.Minor(), want)
		}
		if want := opening + amounts - sum; monthly[11].Remaining.Minor() != want {
			t.Fatalf("monthly remaining %d want %d", monthly[11].Remaining.Minor(), want)
		}
	})
}
