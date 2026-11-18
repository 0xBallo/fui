package dashboard

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/0xballo/fui/internal/app/reporting"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/tui/screens"
	"github.com/0xballo/fui/internal/tui/theme"
)

func mustDate(t *testing.T, s string) domain.Date {
	t.Helper()
	d, err := domain.ParseDate(s)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestCountdown(t *testing.T) {
	tests := []struct {
		name, from, to string
		y, m, d        int
	}{
		{"same day", "2026-05-10", "2026-05-10", 0, 0, 0},
		{"before", "2026-05-10", "2026-05-01", 0, 0, 0},
		{"mixed", "2025-11-18", "2027-03-30", 1, 4, 12},
		{"month end 31 to 28", "2027-01-31", "2027-02-28", 0, 0, 28},
		{"month end clamp", "2027-01-31", "2027-03-31", 0, 2, 0},
		{"leap day", "2024-02-29", "2025-02-28", 0, 11, 30},
		{"leap day year", "2024-02-29", "2028-02-29", 4, 0, 0},
		{"over ten years", "2026-01-15", "2037-03-20", 11, 2, 5},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			y, m, d := countdown(mustDate(t, tc.from), mustDate(t, tc.to))
			if y != tc.y || m != tc.m || d != tc.d {
				t.Errorf("countdown = %dy %dm %dd, want %dy %dm %dd", y, m, d, tc.y, tc.m, tc.d)
			}
		})
	}
}

type runwayStub struct {
	UseCases
	res reporting.RunwayResult
	err error
}

func (s runwayStub) Runway(_ context.Context, _ reporting.RunwayParams) (reporting.RunwayResult, error) {
	return s.res, s.err
}

func TestRunwaySection(t *testing.T) {
	asOf := mustDate(t, "2025-11-18")
	p := sectionParams{AsOf: asOf, Base: domain.Currency{}, MonthStartDay: 1}
	th := theme.Default()

	s := &runwaySection{uc: runwayStub{res: reporting.RunwayResult{ZeroDate: mustDate(t, "2027-03-30")}}, th: th}
	if got := s.view(80); got != "" {
		t.Errorf("view before load = %q, want empty", got)
	}
	msg := s.load(p)()
	if cmd, ok := s.update(msg); !ok || cmd != nil {
		t.Fatalf("update(runwayMsg) = %v, %v; want nil, true", cmd, ok)
	}
	if v := s.view(80); !strings.Contains(v, "1y 4m 12d") || !strings.Contains(v, "zero on 2027-03-30") {
		t.Errorf("view = %q", v)
	}

	s = &runwaySection{uc: runwayStub{err: domain.ErrNotEnoughData}, th: th}
	s.update(s.load(p)())
	if v := s.view(80); !strings.Contains(v, "not enough data (needs 3 complete months)") {
		t.Errorf("view = %q", v)
	}

	s = &runwaySection{uc: runwayStub{err: errors.New("boom")}, th: th}
	msg = s.load(p)()
	if _, ok := msg.(screens.ErrorMsg); !ok {
		t.Fatalf("msg = %T, want screens.ErrorMsg", msg)
	}
	if _, ok := s.update(msg); ok {
		t.Error("runway section must not consume foreign messages")
	}
}
