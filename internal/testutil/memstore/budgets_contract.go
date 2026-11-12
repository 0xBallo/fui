package memstore

import (
	"context"
	"testing"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

func mustBudget(t *testing.T, name string, targets []domain.BudgetTarget) domain.Budget {
	t.Helper()
	b, err := domain.NewBudget(domain.Budget{
		Name: name, Mode: domain.BudgetCap, Targets: targets, Amount: mustMoney(t, 10000),
		Thresholds: domain.BudgetThresholds{Warn: 80, Over: 100},
	})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func contractBudgets(t *testing.T, f *fixture) {
	ctx := context.Background()
	food, home, trip := budgetFixtures(t, f)
	foodBudget, tripBudget := insertBudgets(t, f, food, trip)
	budgetDuplicateCategory(t, f, food)
	f.do(t, func(r app.Repos) error {
		got, err := r.Budgets().Get(ctx, foodBudget.ID)
		if err != nil || got.Name != "Food" || len(got.Targets) != 1 || got.Targets[0].CategoryID != food.ID {
			t.Errorf("Get = %+v, %v; want %+v", got, err, foodBudget)
		}
		budgetErrors(t, r)
		return nil
	})
	jan := domain.MonthKey{Year: 2026, Month: 1}
	feb := domain.MonthKey{Year: 2026, Month: 2}
	foodBudget.Overrides = map[domain.MonthKey]domain.Money{jan: mustMoney(t, 5000)}
	foodBudget.Targets = []domain.BudgetTarget{{CategoryID: home.ID}}
	f.do(t, func(r app.Repos) error {
		if err := r.Budgets().Update(ctx, foodBudget); err != nil {
			t.Fatalf("Update: %v", err)
		}
		budgetListing(t, r, foodBudget, tripBudget, home, jan, feb)
		return nil
	})
	budgetMoves(t, f, foodBudget.ID, tripBudget.ID)
	budgetDeleteCascade(t, f, foodBudget.ID)
}

// budgetFixtures inserts the categories and tag that the budgets below target.
func budgetFixtures(t *testing.T, f *fixture) (food, home domain.Category, trip domain.Tag) {
	ctx := context.Background()
	f.do(t, func(r app.Repos) (err error) {
		if food, err = r.Categories().Insert(ctx, domain.Category{Name: "Food", Kind: domain.CategoryExpense}); err != nil {
			return err
		}
		if home, err = r.Categories().Insert(ctx, domain.Category{Name: "Home", Kind: domain.CategoryExpense}); err != nil {
			return err
		}
		trip, err = r.Tags().Insert(ctx, domain.Tag{Name: "trip"})
		return err
	})
	return food, home, trip
}

// insertBudgets inserts a category-targeted and a tag-targeted budget.
func insertBudgets(t *testing.T, f *fixture, food domain.Category, trip domain.Tag) (foodBudget, tripBudget domain.Budget) {
	ctx := context.Background()
	f.do(t, func(r app.Repos) (err error) {
		foodBudget, err = r.Budgets().Insert(ctx, mustBudget(t, "Food", []domain.BudgetTarget{{CategoryID: food.ID}}))
		return err
	})
	if foodBudget.ID == 0 {
		t.Fatal("Insert did not assign an ID")
	}
	f.do(t, func(r app.Repos) (err error) {
		tripBudget, err = r.Budgets().Insert(ctx, mustBudget(t, "Trip", []domain.BudgetTarget{{TagID: trip.ID}}))
		return err
	})
	return foodBudget, tripBudget
}

// budgetDuplicateCategory checks that a category can be the target of only one budget. A failing
// insert may leave a partial row in the transaction, which the use case rolls back.
func budgetDuplicateCategory(t *testing.T, f *fixture, food domain.Category) {
	ctx := context.Background()
	err := f.store.WithTx(ctx, func(r app.Repos) error {
		_, err := r.Budgets().Insert(ctx, mustBudget(t, "Food again", []domain.BudgetTarget{{CategoryID: food.ID}}))
		return err
	})
	wantErr(t, err, app.ErrAlreadyExists, "a second budget on the same category")
}

// budgetDeleteCascade checks that deleting a budget also drops the envelope moves that touch it.
func budgetDeleteCascade(t *testing.T, f *fixture, id domain.BudgetID) {
	ctx := context.Background()
	f.do(t, func(r app.Repos) error {
		if err := r.Budgets().Delete(ctx, id); err != nil {
			t.Fatalf("Delete: %v", err)
		}
		_, err := r.Budgets().Get(ctx, id)
		wantErr(t, err, domain.ErrNotFound, "Get deleted budget")
		moves, err := r.Budgets().Moves(ctx, mustDate(t, "2026-01-01"), mustDate(t, "2026-12-31"))
		if err != nil {
			t.Fatalf("Moves after delete: %v", err)
		}
		for _, m := range moves {
			if m.From == id || m.To == id {
				t.Errorf("Moves after Delete still reference the deleted budget: %+v", m)
			}
		}
		return nil
	})
}

// budgetErrors exercises the not-found paths shared by Insert/Get/Update/Delete.
func budgetErrors(t *testing.T, r app.Repos) {
	ctx := context.Background()
	_, err := r.Budgets().Insert(ctx, mustBudget(t, "Unknown category", []domain.BudgetTarget{{CategoryID: 999}}))
	wantErr(t, err, domain.ErrNotFound, "Insert with unknown category target")
	_, err = r.Budgets().Insert(ctx, mustBudget(t, "Unknown tag", []domain.BudgetTarget{{TagID: 999}}))
	wantErr(t, err, domain.ErrNotFound, "Insert with unknown tag target")
	_, err = r.Budgets().Get(ctx, 999)
	wantErr(t, err, domain.ErrNotFound, "Get missing budget")
	wantErr(t, r.Budgets().Update(ctx, domain.Budget{ID: 999, Name: "x"}), domain.ErrNotFound, "Update missing budget")
	wantErr(t, r.Budgets().Delete(ctx, 999), domain.ErrNotFound, "Delete missing budget")
}

// budgetListing checks Get, List and ForMonth once food has been updated to the home category
// target with a jan override, alongside trip (untouched).
func budgetListing(t *testing.T, r app.Repos, food, trip domain.Budget, home domain.Category, jan, feb domain.MonthKey) {
	ctx := context.Background()
	got, err := r.Budgets().Get(ctx, food.ID)
	if err != nil || got.Targets[0].CategoryID != home.ID || got.Overrides[jan].Minor() != 5000 {
		t.Errorf("Get after Update = %+v, %v; want home target and the jan override", got, err)
	}
	all, err := r.Budgets().List(ctx)
	if err != nil || len(all) != 2 || all[0].ID != food.ID || all[1].ID != trip.ID {
		t.Errorf("List = %+v, %v; want both budgets ordered by ID", all, err)
	}
	month, err := r.Budgets().ForMonth(ctx, jan)
	if err != nil || len(month) != 2 || month[0].Overrides[jan].Minor() != 5000 {
		t.Errorf("ForMonth(jan) = %+v, %v; want the jan override only", month, err)
	}
	if len(month[1].Overrides) != 0 {
		t.Errorf("ForMonth(jan) trip budget = %+v, want no overrides", month[1])
	}
	other, err := r.Budgets().ForMonth(ctx, feb)
	if err != nil || len(other[0].Overrides) != 0 {
		t.Errorf("ForMonth(feb) = %+v, %v; want no override for an empty month", other, err)
	}
}

// budgetMoves checks the append-only envelope ledger: AddMove validates the endpoints, 0 means
// the unassigned pool, and Moves returns rows within range ordered by date.
func budgetMoves(t *testing.T, f *fixture, food, trip domain.BudgetID) {
	ctx := context.Background()
	f.do(t, func(r app.Repos) error {
		err := r.Budgets().AddMove(ctx, domain.BudgetMove{Date: mustDate(t, "2026-01-05"), From: 999, To: food, Amount: mustMoney(t, 1000)})
		wantErr(t, err, domain.ErrNotFound, "AddMove with unknown From")
		err = r.Budgets().AddMove(ctx, domain.BudgetMove{Date: mustDate(t, "2026-01-05"), From: food, To: 999, Amount: mustMoney(t, 1000)})
		wantErr(t, err, domain.ErrNotFound, "AddMove with unknown To")
		if err := r.Budgets().AddMove(ctx, domain.BudgetMove{Date: mustDate(t, "2026-01-05"), From: 0, To: food, Amount: mustMoney(t, 1000)}); err != nil {
			t.Fatalf("AddMove from pool: %v", err)
		}
		if err := r.Budgets().AddMove(ctx, domain.BudgetMove{Date: mustDate(t, "2026-01-10"), From: food, To: trip, Amount: mustMoney(t, 400)}); err != nil {
			t.Fatalf("AddMove between budgets: %v", err)
		}
		if err := r.Budgets().AddMove(ctx, domain.BudgetMove{Date: mustDate(t, "2026-02-01"), From: trip, To: 0, Amount: mustMoney(t, 100)}); err != nil {
			t.Fatalf("AddMove to pool: %v", err)
		}
		moves, err := r.Budgets().Moves(ctx, mustDate(t, "2026-01-01"), mustDate(t, "2026-01-31"))
		if err != nil || len(moves) != 2 || moves[0].To != food || moves[1].From != food {
			t.Errorf("Moves(jan) = %+v, %v; want the two January moves in date order", moves, err)
		}
		all, err := r.Budgets().Moves(ctx, mustDate(t, "2026-01-01"), mustDate(t, "2026-12-31"))
		if err != nil || len(all) != 3 {
			t.Errorf("Moves(year) = %+v, %v; want 3 rows", all, err)
		}
		return nil
	})
}
