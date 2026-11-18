package domain

import (
	"errors"
	"testing"

	"pgregory.net/rapid"
)

func TestParseCycleNext(t *testing.T) {
	c := CycleMonth
	for _, want := range []Cycle{CycleQuarter, CycleYear, CycleLifetime, CycleMonth} {
		c = c.Next()
		if c != want {
			t.Fatalf("Next = %s, want %s", c, want)
		}
		if got, err := ParseCycle(string(c)); err != nil || got != c {
			t.Fatalf("ParseCycle(%s) = %v, %v", c, got, err)
		}
	}
	if _, err := ParseCycle("week"); !errors.Is(err, ErrInvalidValue) {
		t.Errorf("err = %v", err)
	}
	if _, err := CyclePeriod("x", date(t, 2026, 1, 1), 1, Date{}); !errors.Is(err, ErrInvalidValue) {
		t.Errorf("CyclePeriod err = %v", err)
	}
	if _, err := ShiftCycle("x", date(t, 2026, 1, 1), 1, 1); !errors.Is(err, ErrInvalidValue) {
		t.Errorf("ShiftCycle err = %v", err)
	}
}

func TestCyclePeriod(t *testing.T) {
	tests := []struct {
		name     string
		c        Cycle
		anchor   Date
		startDay int
		since    Date
		from, to string
	}{
		{"month day1", CycleMonth, date(t, 2026, 2, 10), 1, Date{}, "2026-02-01", "2026-02-28"},
		{"month leap", CycleMonth, date(t, 2024, 2, 10), 1, Date{}, "2024-02-01", "2024-02-29"},
		{"month day15 before", CycleMonth, date(t, 2026, 3, 10), 15, Date{}, "2026-02-15", "2026-03-14"},
		{"month day31", CycleMonth, date(t, 2026, 3, 10), 31, Date{}, "2026-02-28", "2026-03-30"},
		{"quarter start", CycleQuarter, date(t, 2026, 1, 1), 1, Date{}, "2026-01-01", "2026-03-31"},
		{"quarter end", CycleQuarter, date(t, 2026, 6, 30), 1, Date{}, "2026-04-01", "2026-06-30"},
		{"quarter q4", CycleQuarter, date(t, 2026, 12, 31), 1, Date{}, "2026-10-01", "2026-12-31"},
		{"year start", CycleYear, date(t, 2026, 1, 1), 1, Date{}, "2026-01-01", "2026-12-31"},
		{"year end", CycleYear, date(t, 2026, 12, 31), 1, Date{}, "2026-01-01", "2026-12-31"},
		{"lifetime", CycleLifetime, date(t, 2026, 5, 5), 1, date(t, 2020, 3, 1), "2020-03-01", "2026-05-05"},
		{"lifetime no since", CycleLifetime, date(t, 2026, 5, 5), 1, Date{}, "2026-05-05", "2026-05-05"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := CyclePeriod(tt.c, tt.anchor, tt.startDay, tt.since)
			if err != nil {
				t.Fatal(err)
			}
			if p.From().String() != tt.from || p.To().String() != tt.to {
				t.Errorf("got %s..%s, want %s..%s", p.From(), p.To(), tt.from, tt.to)
			}
		})
	}
	if _, err := CyclePeriod(CycleMonth, date(t, 2026, 1, 1), 0, Date{}); err == nil {
		t.Error("expected error for startDay 0")
	}
}

func TestShiftCycle(t *testing.T) {
	tests := []struct {
		name     string
		c        Cycle
		anchor   Date
		n        int
		startDay int
		want     string
	}{
		{"month +1", CycleMonth, date(t, 2026, 1, 20), 1, 1, "2026-02-01"},
		{"month -1 jan", CycleMonth, date(t, 2026, 1, 20), -1, 1, "2025-12-01"},
		{"month +13", CycleMonth, date(t, 2026, 1, 20), 13, 1, "2027-02-01"},
		{"month -13", CycleMonth, date(t, 2026, 1, 20), -13, 1, "2024-12-01"},
		{"month 15 +1", CycleMonth, date(t, 2026, 1, 20), 1, 15, "2026-02-15"},
		{"month 31 +1", CycleMonth, date(t, 2026, 1, 31), 1, 31, "2026-02-28"},
		{"month 31 leap", CycleMonth, date(t, 2024, 1, 31), 1, 31, "2024-02-29"},
		{"quarter +1", CycleQuarter, date(t, 2026, 11, 5), 1, 1, "2027-01-01"},
		{"quarter -1", CycleQuarter, date(t, 2026, 1, 5), -1, 1, "2025-10-01"},
		{"quarter +13", CycleQuarter, date(t, 2026, 5, 5), 13, 1, "2029-07-01"},
		{"year +1", CycleYear, date(t, 2026, 7, 5), 1, 1, "2027-01-01"},
		{"year -13", CycleYear, date(t, 2026, 7, 5), -13, 1, "2013-01-01"},
		{"lifetime", CycleLifetime, date(t, 2026, 7, 5), 5, 1, "2026-07-05"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ShiftCycle(tt.c, tt.anchor, tt.n, tt.startDay)
			if err != nil {
				t.Fatal(err)
			}
			if got.String() != tt.want {
				t.Errorf("got %s, want %s", got, tt.want)
			}
		})
	}
	if _, err := ShiftCycle(CycleMonth, date(t, 2026, 1, 1), 1, 40); err == nil {
		t.Error("expected error for startDay 40")
	}
}

func drawCycleInput(t *rapid.T) (Cycle, Date, int) {
	c := rapid.SampledFrom([]Cycle{CycleMonth, CycleQuarter, CycleYear}).Draw(t, "cycle")
	d, err := NewDate(rapid.IntRange(1990, 2100).Draw(t, "y"), rapid.IntRange(1, 12).Draw(t, "m"), 1)
	if err != nil {
		t.Fatal(err)
	}
	if d, err = d.AddDays(rapid.IntRange(0, 27).Draw(t, "d")); err != nil {
		t.Fatal(err)
	}
	return c, d, rapid.SampledFrom([]int{1, 15, 28, 31}).Draw(t, "start")
}

func TestCycleProperties(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		c, d, sd := drawCycleInput(t)
		n := rapid.IntRange(-30, 30).Draw(t, "n")
		p, err := CyclePeriod(c, d, sd, Date{})
		if err != nil || !p.Contains(d) {
			t.Fatalf("period %v does not contain %v (%v)", p, d, err)
		}
		fwd, err := ShiftCycle(c, d, n, sd)
		if err != nil {
			t.Fatal(err)
		}
		back, err := ShiftCycle(c, fwd, -n, sd)
		if err != nil {
			t.Fatal(err)
		}
		if q, _ := CyclePeriod(c, back, sd, Date{}); q != p {
			t.Fatalf("round trip %v -> %v, want %v", q, back, p)
		}
		next, err := ShiftCycle(c, d, 1, sd)
		if err != nil {
			t.Fatal(err)
		}
		np, err := CyclePeriod(c, next, sd, Date{})
		if err != nil {
			t.Fatal(err)
		}
		if day, _ := p.To().AddDays(1); day != np.From() {
			t.Fatalf("gap/overlap: %v then %v", p, np)
		}
	})
}
