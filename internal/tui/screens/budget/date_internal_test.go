package budget

import (
	"testing"

	"github.com/0xballo/fui/internal/domain"
)

func TestMoveDate(t *testing.T) {
	d := func(y, m, day int) domain.Date {
		t.Helper()
		v, err := domain.NewDate(y, m, day)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	tests := []struct {
		name     string
		today    domain.Date
		monthIn  domain.Date
		startDay int
		want     string
	}{
		{"today inside", d(2026, 6, 15), d(2026, 6, 1), 1, "2026-06-15"},
		{"month on screen is past", d(2026, 6, 15), d(2026, 5, 1), 1, "2026-05-01"},
		{"month on screen is future", d(2026, 6, 15), d(2026, 7, 1), 1, "2026-07-01"},
		{"period edge first day", d(2026, 6, 1), d(2026, 6, 1), 1, "2026-06-01"},
		{"period edge last day", d(2026, 6, 30), d(2026, 6, 1), 1, "2026-06-30"},
		{"start day 15, today inside", d(2026, 6, 20), d(2026, 6, 20), 15, "2026-06-20"},
		{"start day 15, today before period", d(2026, 6, 10), d(2026, 6, 20), 15, "2026-06-15"},
		{"start day 15, today after period", d(2026, 7, 15), d(2026, 6, 20), 15, "2026-06-15"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := domain.MonthPeriod(tt.monthIn, tt.startDay)
			if err != nil {
				t.Fatal(err)
			}
			if got := moveDate(tt.today, p); got != tt.want {
				t.Errorf("moveDate = %s, want %s", got, tt.want)
			}
		})
	}
}
