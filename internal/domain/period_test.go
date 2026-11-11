package domain

import (
	"errors"
	"testing"

	"pgregory.net/rapid" //nolint:depguard // test-only dependency allowed by the technical spec
)

func TestNewPeriod(t *testing.T) {
	a, b := date(t, 2026, 1, 1), date(t, 2026, 1, 31)
	p, err := NewPeriod(a, b)
	if err != nil || p.From() != a || p.To() != b {
		t.Fatalf("NewPeriod = %v, %v", p, err)
	}
	if _, err := NewPeriod(a, a); err != nil {
		t.Errorf("single-day period: %v", err)
	}
	if _, err := NewPeriod(b, a); !errors.Is(err, ErrInvalidDate) {
		t.Errorf("reversed err = %v", err)
	}
	if _, err := NewPeriod(Date{}, a); !errors.Is(err, ErrMissingField) {
		t.Errorf("zero err = %v", err)
	}
	for _, tt := range []struct {
		d    Date
		want bool
	}{{a, true}, {b, true}, {date(t, 2026, 1, 15), true}, {date(t, 2025, 12, 31), false}, {date(t, 2026, 2, 1), false}} {
		if p.Contains(tt.d) != tt.want {
			t.Errorf("Contains(%v) = %v", tt.d, !tt.want)
		}
	}
}

func TestMonthPeriod(t *testing.T) {
	tests := []struct {
		name     string
		d        Date
		start    int
		from, to Date
	}{
		{"calendar month", date(t, 2026, 11, 11), 1, date(t, 2026, 11, 1), date(t, 2026, 11, 30)},
		{"calendar month feb leap", date(t, 2024, 2, 10), 1, date(t, 2024, 2, 1), date(t, 2024, 2, 29)},
		{"after start day", date(t, 2026, 11, 28), 27, date(t, 2026, 11, 27), date(t, 2026, 12, 26)},
		{"on start day", date(t, 2026, 11, 27), 27, date(t, 2026, 11, 27), date(t, 2026, 12, 26)},
		{"before start day", date(t, 2026, 11, 26), 27, date(t, 2026, 10, 27), date(t, 2026, 11, 26)},
		{"before start across year", date(t, 2026, 1, 5), 27, date(t, 2025, 12, 27), date(t, 2026, 1, 26)},
		{"after start across year", date(t, 2026, 12, 30), 27, date(t, 2026, 12, 27), date(t, 2027, 1, 26)},
		{"31 clamps in feb", date(t, 2026, 2, 10), 31, date(t, 2026, 1, 31), date(t, 2026, 2, 27)},
		{"31 on clamped day", date(t, 2026, 2, 28), 31, date(t, 2026, 2, 28), date(t, 2026, 3, 30)},
		{"31 in 30-day month", date(t, 2026, 4, 30), 31, date(t, 2026, 4, 30), date(t, 2026, 5, 30)},
		{"31 leap feb", date(t, 2024, 2, 29), 31, date(t, 2024, 2, 29), date(t, 2024, 3, 30)},
		{"29 non-leap", date(t, 2026, 2, 28), 29, date(t, 2026, 2, 28), date(t, 2026, 3, 28)},
		{"29 before clamp", date(t, 2026, 2, 27), 29, date(t, 2026, 1, 29), date(t, 2026, 2, 27)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := MonthPeriod(tt.d, tt.start)
			if err != nil || p.From() != tt.from || p.To() != tt.to {
				t.Fatalf("MonthPeriod(%v, %d) = [%v, %v], %v; want [%v, %v]", tt.d, tt.start, p.From(), p.To(), err, tt.from, tt.to)
			}
		})
	}
}

func TestMonthPeriodErrors(t *testing.T) {
	d := date(t, 2026, 1, 1)
	for _, start := range []int{0, 32, -1} {
		if _, err := MonthPeriod(d, start); !errors.Is(err, ErrInvalidValue) {
			t.Errorf("start %d err = %v", start, err)
		}
	}
	if _, err := MonthPeriod(Date{}, 1); !errors.Is(err, ErrInvalidDate) {
		t.Errorf("zero date err = %v", err)
	}
	if _, err := MonthPeriod(date(t, 9999, 12, 31), 1); !errors.Is(err, ErrInvalidDate) {
		t.Errorf("year-end overflow err = %v", err)
	}
	if _, err := MonthPeriod(date(t, 1, 1, 1), 5); !errors.Is(err, ErrInvalidDate) {
		t.Errorf("year-start underflow err = %v", err)
	}
}

func TestMonthPeriodPartitionProperty(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		d := genDate(t)
		start := rapid.IntRange(1, 31).Draw(t, "start")
		p, err := MonthPeriod(d, start)
		if err != nil {
			t.Skip("edge of supported range")
		}
		if !p.Contains(d) {
			t.Fatalf("%v not in %v..%v", d, p.From(), p.To())
		}
		// Consecutive periods tile the calendar: the next one starts the day after this one ends.
		nextDay, err := p.To().AddDays(1)
		if err != nil {
			t.Skip("edge of supported range")
		}
		next, err := MonthPeriod(nextDay, start)
		if err != nil {
			t.Skip("edge of supported range")
		}
		if next.From() != nextDay {
			t.Fatalf("next period from %v starts at %v", nextDay, next.From())
		}
	})
}
