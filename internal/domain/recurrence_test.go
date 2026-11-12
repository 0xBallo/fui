package domain

import (
	"errors"
	"testing"

	"pgregory.net/rapid"
)

func recurrenceForTest(t tb, start Date, rule RecurrenceRule) Recurrence {
	t.Helper()
	r, err := NewRecurrence(Recurrence{
		AccountID:  1,
		CategoryID: 2,
		Amount:     money(t, -1_000, "EUR"),
		Rule:       rule,
		Start:      start,
		NextDate:   start,
	})
	if err != nil {
		t.Fatalf("NewRecurrence: %v", err)
	}
	return r
}

func TestNewRecurrenceRule(t *testing.T) {
	_, err := NewRecurrenceRule(0, RecurrenceDays, 0)
	if !errors.Is(err, ErrInvalidValue) {
		t.Fatalf("every=0 err = %v", err)
	}
	_, err = NewRecurrenceRule(1, RecurrenceUnit("x"), 0)
	if !errors.Is(err, ErrInvalidValue) && !errors.Is(err, ErrMissingField) {
		t.Fatalf("invalid unit err = %v", err)
	}
	_, err = NewRecurrenceRule(1, RecurrenceWeeks, 12)
	if !errors.Is(err, ErrInvalidValue) {
		t.Fatalf("day_of_month for weekly err = %v", err)
	}
	_, err = NewRecurrenceRule(1, RecurrenceMonths, 32)
	if !errors.Is(err, ErrInvalidValue) {
		t.Fatalf("day_of_month overflow err = %v", err)
	}
}

func TestAdvanceMonthEndClamp(t *testing.T) {
	tests := []struct {
		name  string
		start Date
		want  []Date
	}{
		{
			name:  "non leap year",
			start: date(t, 2025, 1, 31),
			want:  []Date{date(t, 2025, 2, 28), date(t, 2025, 3, 31), date(t, 2025, 4, 30)},
		},
		{
			name:  "leap year",
			start: date(t, 2024, 1, 31),
			want:  []Date{date(t, 2024, 2, 29), date(t, 2024, 3, 31), date(t, 2024, 4, 30)},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule, err := NewRecurrenceRule(1, RecurrenceMonths, 31)
			if err != nil {
				t.Fatal(err)
			}
			r := recurrenceForTest(t, tt.start, rule)
			for i := range tt.want {
				r, err = Advance(r)
				if err != nil {
					t.Fatal(err)
				}
				if r.NextDate != tt.want[i] {
					t.Fatalf("step %d: next=%v want=%v", i, r.NextDate, tt.want[i])
				}
			}
		})
	}
}

func TestExpandInclusiveWindow(t *testing.T) {
	rule, err := NewRecurrenceRule(1, RecurrenceMonths, 31)
	if err != nil {
		t.Fatal(err)
	}
	r := recurrenceForTest(t, date(t, 2024, 1, 31), rule)
	got := Expand(r, date(t, 2024, 2, 1), date(t, 2024, 3, 31))
	want := []Date{date(t, 2024, 2, 29), date(t, 2024, 3, 31)}
	if !sameDates(got, want) {
		t.Fatalf("Expand = %v, want %v", got, want)
	}
}

func TestExpandStopsAtEnd(t *testing.T) {
	rule, err := NewRecurrenceRule(1, RecurrenceMonths, 0)
	if err != nil {
		t.Fatal(err)
	}
	r, err := NewRecurrence(Recurrence{
		AccountID: 1,
		Amount:    money(t, -1_000, "EUR"),
		Rule:      rule,
		Start:     date(t, 2026, 1, 31),
		NextDate:  date(t, 2026, 1, 31),
		End:       date(t, 2026, 3, 31),
	})
	if err != nil {
		t.Fatal(err)
	}
	got := Expand(r, date(t, 2026, 1, 1), date(t, 2026, 12, 31))
	want := []Date{date(t, 2026, 1, 31), date(t, 2026, 2, 28), date(t, 2026, 3, 31)}
	if !sameDates(got, want) {
		t.Fatalf("Expand = %v, want %v", got, want)
	}
}

func genRule(t *rapid.T) RecurrenceRule {
	unit := rapid.SampledFrom([]RecurrenceUnit{RecurrenceDays, RecurrenceWeeks, RecurrenceMonths}).Draw(t, "unit")
	every := rapid.IntRange(1, 6).Draw(t, "every")
	dayOfMonth := 0
	if unit == RecurrenceMonths {
		dayOfMonth = rapid.IntRange(0, 31).Draw(t, "day_of_month")
	}
	rule, err := NewRecurrenceRule(every, unit, dayOfMonth)
	if err != nil {
		t.Fatal(err)
	}
	return rule
}

func genRecurrence(t *rapid.T) Recurrence {
	start := date(t,
		rapid.IntRange(1900, 2100).Draw(t, "year"),
		rapid.IntRange(1, 12).Draw(t, "month"),
		rapid.IntRange(1, 28).Draw(t, "day"),
	)
	rule := genRule(t)
	r := recurrenceForTest(t, start, rule)
	limited := rapid.Bool().Draw(t, "with_end")
	if !limited {
		return r
	}
	maxSteps := rapid.IntRange(1, 36).Draw(t, "steps")
	end := start
	for range maxSteps {
		next, ok, err := nextRecurrenceDate(r, end)
		if err != nil || !ok {
			break
		}
		end = next
	}
	r.End = end
	return r
}

func TestExpandMonotonicProperty(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		r := genRecurrence(t)
		from, err := r.Start.AddDays(rapid.IntRange(0, 90).Draw(t, "from_offset"))
		if err != nil {
			t.Skip("out of range")
		}
		until, err := from.AddDays(rapid.IntRange(0, 120).Draw(t, "window"))
		if err != nil {
			t.Skip("out of range")
		}
		got := Expand(r, from, until)
		for i := 1; i < len(got); i++ {
			if !got[i].After(got[i-1]) {
				t.Fatalf("non-monotonic at %d: %v", i, got)
			}
		}
	})
}

func TestExpandIdempotentWindowProperty(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		r := genRecurrence(t)
		from, err := r.Start.AddDays(rapid.IntRange(0, 90).Draw(t, "from_offset"))
		if err != nil {
			t.Skip("out of range")
		}
		until, err := from.AddDays(rapid.IntRange(0, 120).Draw(t, "window"))
		if err != nil {
			t.Skip("out of range")
		}
		first := Expand(r, from, until)
		second := Expand(r, from, until)
		if !sameDates(first, second) {
			t.Fatalf("idempotence failed: %v vs %v", first, second)
		}
	})
}

func TestExpandWindowPartitionProperty(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		r := genRecurrence(t)
		from, err := r.Start.AddDays(rapid.IntRange(0, 90).Draw(t, "from_offset"))
		if err != nil {
			t.Skip("out of range")
		}
		mid, err := from.AddDays(rapid.IntRange(0, 60).Draw(t, "mid_offset"))
		if err != nil {
			t.Skip("out of range")
		}
		until, err := mid.AddDays(rapid.IntRange(0, 60).Draw(t, "until_offset"))
		if err != nil {
			t.Skip("out of range")
		}
		left := Expand(r, from, mid)
		nextDay, err := mid.AddDays(1)
		if err != nil {
			t.Skip("out of range")
		}
		right := Expand(r, nextDay, until)
		all := Expand(r, from, until)
		joined := append(append([]Date(nil), left...), right...)
		if !sameDates(all, joined) {
			t.Fatalf("partition failed: all=%v joined=%v", all, joined)
		}
	})
}

func sameDates(left, right []Date) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
