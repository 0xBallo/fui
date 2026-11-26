package sqlite

import (
	"context"
	"testing"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/reporting"
	"github.com/0xballo/fui/internal/domain"
)

func TestReportingSourceExpandsSplits(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	eur := mustCurrency(t, "EUR")
	day := mustDate(t, "2026-03-05")

	var food, home domain.Category
	var split domain.Transaction
	err := store.WithTx(ctx, func(r app.Repos) error {
		acct, err := r.Accounts().Insert(ctx, domain.Account{
			Name: "Main", Kind: domain.AccountChecking, Currency: eur,
			OpeningBalance: mustMoney(t, 0, eur), OpenedOn: mustDate(t, "2026-01-01"),
		})
		if err != nil {
			return err
		}
		if food, err = r.Categories().Insert(ctx, domain.Category{Name: "Food", Kind: domain.CategoryExpense}); err != nil {
			return err
		}
		if home, err = r.Categories().Insert(ctx, domain.Category{Name: "Home", Kind: domain.CategoryExpense}); err != nil {
			return err
		}
		if split, err = r.Transactions().Insert(ctx, domain.Transaction{
			AccountID: acct.ID, Date: day, Amount: mustMoney(t, -100, eur),
			Status: domain.StatusBooked, Kind: domain.KindPayment,
		}); err != nil {
			return err
		}
		if _, err = r.Transactions().Insert(ctx, domain.Transaction{
			AccountID: acct.ID, Date: day, Amount: mustMoney(t, -7, eur),
			CategoryID: food.ID, Status: domain.StatusBooked, Kind: domain.KindPayment,
		}); err != nil {
			return err
		}
		return r.Splits().ReplaceSplits(ctx, split.ID, []domain.Split{
			{Category: food.ID, Amount: mustMoney(t, -60, eur)},
			{Category: home.ID, Amount: mustMoney(t, -40, eur)},
		})
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	period, err := domain.NewPeriod(mustDate(t, "2026-03-01"), mustDate(t, "2026-03-31"))
	if err != nil {
		t.Fatal(err)
	}
	txs, err := store.ListTransactions(ctx, reporting.TransactionFilter{Period: period})
	if err != nil {
		t.Fatalf("ListTransactions: %v", err)
	}
	if len(txs) != 3 {
		t.Fatalf("transactions = %d, want 3 (2 split rows + 1 plain)", len(txs))
	}
	for _, tx := range txs {
		if tx.ID == split.ID && tx.Amount.Minor() == -100 {
			t.Fatalf("split parent returned: %+v", tx)
		}
	}

	rates := func(domain.Currency, domain.Currency, domain.Date) (domain.Rate, error) {
		return domain.NewRate(domain.RateScale)
	}
	totals, total, err := domain.SpendingByCategory(period, txs, eur, rates)
	if err != nil {
		t.Fatalf("SpendingByCategory: %v", err)
	}
	got := map[domain.CategoryID]int64{}
	for _, ct := range totals {
		got[ct.CategoryID] = ct.Total.Minor()
	}
	if got[food.ID] != 67 || got[home.ID] != 40 || total.Minor() != 107 {
		t.Fatalf("food=%d home=%d total=%d, want 67/40/107", got[food.ID], got[home.ID], total.Minor())
	}
}
