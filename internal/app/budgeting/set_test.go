package budgeting_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/0xballo/fui/internal/app/budgeting"
	"github.com/0xballo/fui/internal/domain"
)

func ptr[T any](v T) *T { return &v }

func TestSetDefaultsAndFlags(t *testing.T) {
	e := newEnv(t)
	s := svc(e, domain.BudgetCap)
	b := set(t, s, budgeting.SetRequest{Name: "Food", Categories: []string{"Food"}, Amount: "100"})
	if !b.Carry || b.Period != domain.BudgetMonthly || b.Thresholds != domain.DefaultBudgetThresholds {
		t.Errorf("new budget = carry %v period %q th %+v", b.Carry, b.Period, b.Thresholds)
	}

	// Update keeps carry, period and thresholds when not given.
	b = set(t, s, budgeting.SetRequest{Name: "Food", Amount: "120", Warn: ptr(70)})
	if !b.Carry || b.Thresholds != (domain.BudgetThresholds{Warn: 70, Over: 100}) || b.Amount.Minor() != 12000 {
		t.Errorf("updated = %+v", b)
	}
	b = set(t, s, budgeting.SetRequest{Name: "Food", Over: ptr(110), Carry: ptr(false), Period: "year"})
	if b.Carry || b.Period != domain.BudgetYearly || b.Thresholds != (domain.BudgetThresholds{Warn: 70, Over: 110}) {
		t.Errorf("updated = %+v", b)
	}
	b = set(t, s, budgeting.SetRequest{Name: "Food", Amount: "10"})
	if b.Carry || b.Period != domain.BudgetYearly {
		t.Errorf("unchanged flags lost: %+v", b)
	}

	b = set(t, s, budgeting.SetRequest{Name: "Home", Categories: []string{"Home"}, Amount: "5", Carry: ptr(false), Warn: ptr(50), Over: ptr(90)})
	if b.Carry || b.Thresholds != (domain.BudgetThresholds{Warn: 50, Over: 90}) {
		t.Errorf("new with flags = %+v", b)
	}
}

func TestSetInvalid(t *testing.T) {
	e := newEnv(t)
	s := svc(e, domain.BudgetCap)
	for name, req := range map[string]budgeting.SetRequest{
		"warn>=over":   {Name: "A", Categories: []string{"Food"}, Amount: "1", Warn: ptr(90), Over: ptr(90)},
		"warn zero":    {Name: "A", Categories: []string{"Food"}, Amount: "1", Warn: ptr(0)},
		"over<=80":     {Name: "A", Categories: []string{"Food"}, Amount: "1", Over: ptr(50)},
		"bad period":   {Name: "A", Categories: []string{"Food"}, Amount: "1", Period: "week"},
		"new no amt":   {Name: "Food"},
		"month no amt": {Name: "Food", Month: "2026-03"},
	} {
		if _, err := s.PlanSet(context.Background(), req); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
}

func TestSetCreatesFromCategoryName(t *testing.T) {
	e := newEnv(t)
	s := svc(e, domain.BudgetCap)
	b := set(t, s, budgeting.SetRequest{Name: "food", Amount: "400"})
	if len(b.Targets) != 1 || b.Targets[0].CategoryID != e.food.ID {
		t.Errorf("targets = %+v", b.Targets)
	}

	_, err := s.PlanSet(context.Background(), budgeting.SetRequest{Name: "Nothing", Amount: "4"})
	var de *domain.Error
	if !errors.As(err, &de) || de.Code != domain.CodeNotFound ||
		!strings.Contains(de.Message, `budget "Nothing" does not exist and no category has that name; pass --category NAME or --tag NAME`) {
		t.Errorf("err = %v", err)
	}
	if _, err := s.PlanSet(context.Background(), budgeting.SetRequest{Name: "Home"}); !errors.Is(err, domain.ErrMissingField) {
		t.Errorf("missing amount: %v", err)
	}
}

func TestSetMonthOverride(t *testing.T) {
	e := newEnv(t)
	s := svc(e, domain.BudgetCap)
	b := set(t, s, budgeting.SetRequest{Name: "Food", Amount: "400", Month: "2026-11"})
	if b.Amount.Minor() != 40000 || b.Overrides[domain.MonthKey{Year: 2026, Month: 11}].Minor() != 40000 {
		t.Errorf("new with month = %+v", b)
	}

	set(t, s, budgeting.SetRequest{Name: "Home", Categories: []string{"Home"}, Amount: "1000", Period: "year"})
	y := set(t, s, budgeting.SetRequest{Name: "Home", Amount: "900", Month: "2026-07"})
	if y.Overrides[domain.MonthKey{Year: 2026, Month: 1}].Minor() != 90000 || len(y.Overrides) != 1 || y.Amount.Minor() != 100000 {
		t.Errorf("yearly override = %+v", y.Overrides)
	}
}
