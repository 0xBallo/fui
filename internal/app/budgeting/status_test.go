package budgeting_test

import (
	"context"
	"testing"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

func insertBudget(t tb, e *env, b domain.Budget) domain.Budget {
	t.Helper()
	ctx := context.Background()
	b = must(domain.NewBudget(b))
	err := e.store.WithTx(ctx, func(r app.Repos) (err error) {
		b, err = r.Budgets().Insert(ctx, b)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func assign(t tb, e *env, id domain.BudgetID, date string, minor int64) {
	t.Helper()
	ctx := context.Background()
	err := e.store.WithTx(ctx, func(r app.Repos) error {
		return r.Budgets().AddMove(ctx, domain.BudgetMove{Date: day(date), To: id, Amount: money(minor)})
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestYearlyCapStatus(t *testing.T) {
	tests := []struct {
		name      string
		carry     bool
		month     string
		spent     int64
		carryIn   int64
		periodEnd string
	}{
		{"summed over the year", false, "2026-04", 30000, 0, "2026-12-31"},
		{"next year without carry", false, "2027-02", 5000, 0, "2027-12-31"},
		{"next year with carry", true, "2027-02", 5000, 120000 - 10000 - 20000 - 9999, "2027-12-31"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t)
			s := svc(e, domain.BudgetCap)
			insertBudget(t, e, domain.Budget{
				Name: "Food", Mode: domain.BudgetCap, Period: domain.BudgetYearly, Carry: tc.carry,
				Targets: []domain.BudgetTarget{{CategoryID: e.food.ID}}, Amount: money(120000),
			})
			e.spend(t, "2026-01-10", 10000, e.food, domain.StatusBooked)
			e.spend(t, "2026-03-10", 20000, e.food, domain.StatusBooked)
			e.spend(t, "2027-02-03", 5000, e.food, domain.StatusBooked)
			e.spend(t, "2026-05-01", 9999, e.food, domain.StatusBooked) // after the shown April
			st, err := s.Status(context.Background(), tc.month)
			if err != nil {
				t.Fatal(err)
			}
			c := st.Caps[0]
			if tc.month == "2026-04" {
				// Jan 100 + Mar 200 = 300; May is beyond the shown month.
				if c.Spent.Minor() != tc.spent {
					t.Errorf("spent = %d, want %d", c.Spent.Minor(), tc.spent)
				}
			} else {
				if c.Spent.Minor() != tc.spent || c.CarryIn.Minor() != tc.carryIn {
					t.Errorf("spent/carry = %d/%d, want %d/%d", c.Spent.Minor(), c.CarryIn.Minor(), tc.spent, tc.carryIn)
				}
			}
			if c.Period.To() != day(tc.periodEnd) || st.Period.From().Year() != c.Period.From().Year() {
				t.Errorf("period = %v, status %v", c.Period, st.Period)
			}
		})
	}
}

func TestYearlyEnvelopeSuggestedMonthly(t *testing.T) {
	e := newEnv(t)
	s := svc(e, domain.BudgetEnvelope)
	yearly := insertBudget(t, e, domain.Budget{
		Name: "Insurance", Mode: domain.BudgetEnvelope, Period: domain.BudgetYearly,
		Targets: []domain.BudgetTarget{{CategoryID: e.home.ID}}, Amount: money(0),
	})
	monthly := insertBudget(t, e, domain.Budget{
		Name: "Food", Mode: domain.BudgetEnvelope,
		Targets: []domain.BudgetTarget{{CategoryID: e.food.ID}}, Amount: money(0),
	})
	assign(t, e, yearly.ID, "2026-01-02", 120000)
	assign(t, e, monthly.ID, "2026-04-02", 10000)
	e.spend(t, "2026-01-15", 10000, e.home, domain.StatusBooked)
	e.spend(t, "2026-02-15", 10000, e.home, domain.StatusBooked)
	e.spend(t, "2026-03-15", 10000, e.home, domain.StatusBooked)
	e.spend(t, "2026-04-05", 2500, e.food, domain.StatusBooked)
	e.spend(t, "2026-05-05", 7777, e.home, domain.StatusBooked) // after the shown month

	st, err := s.Status(context.Background(), "2026-04")
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range st.Envelopes {
		switch l.Budget.Name {
		case "Insurance":
			// 1200 - 300 = 900 left, spread over Apr..Dec = 100 a month.
			if l.Balance.Minor() != 90000 || l.SuggestedMonthly.Minor() != 10000 {
				t.Errorf("yearly balance/suggested = %d/%d", l.Balance.Minor(), l.SuggestedMonthly.Minor())
			}
			if l.Period.From() != day("2026-01-01") || l.Period.To() != day("2026-12-31") {
				t.Errorf("yearly period = %v", l.Period)
			}
		case "Food":
			if l.Balance.Minor() != 7500 || !l.SuggestedMonthly.Currency().IsZero() {
				t.Errorf("monthly balance/suggested = %d/%v", l.Balance.Minor(), l.SuggestedMonthly)
			}
			if l.Period != st.Period {
				t.Errorf("monthly period = %v", l.Period)
			}
		}
	}
	u := st.ToAssign
	sum, err := u.Envelopes.Add(u.ToAssign)
	if err != nil {
		t.Fatal(err)
	}
	if sum != u.Liquidity || u.Envelopes.Minor() != 90000+7500 {
		t.Errorf("envelopes %s + to-assign %s vs liquidity %s", u.Envelopes, u.ToAssign, u.Liquidity)
	}
}
