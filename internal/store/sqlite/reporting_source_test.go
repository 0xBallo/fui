package sqlite

import (
	"context"
	"testing"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/reporting"
	"github.com/0xballo/fui/internal/domain"
)

func TestReportingSourceBasics(t *testing.T) {
	store, _ := newTestStore(t)

	eur := mustCurrency(t, "EUR")
	usd := mustCurrency(t, "USD")
	opened := mustDate(t, "2026-01-01")
	d1 := mustDate(t, "2026-02-01")
	d2 := mustDate(t, "2026-02-10")

	var eurAccount, usdAccount domain.Account
	var groceries domain.Category
	err := store.WithTx(context.Background(), func(r app.Repos) error {
		var txErr error
		eurAccount, txErr = r.Accounts().Insert(context.Background(), domain.Account{
			Name:           "EUR",
			Kind:           domain.AccountChecking,
			Currency:       eur,
			OpeningBalance: mustMoney(t, 1000, eur),
			OpenedOn:       opened,
		})
		if txErr != nil {
			return txErr
		}
		usdAccount, txErr = r.Accounts().Insert(context.Background(), domain.Account{
			Name:           "USD",
			Kind:           domain.AccountChecking,
			Currency:       usd,
			OpeningBalance: mustMoney(t, 0, usd),
			OpenedOn:       opened,
		})
		if txErr != nil {
			return txErr
		}
		groceries, txErr = r.Categories().Insert(context.Background(), domain.Category{
			Name: "Groceries",
			Kind: domain.CategoryExpense,
		})
		if txErr != nil {
			return txErr
		}
		_, txErr = r.Transactions().Insert(context.Background(), domain.Transaction{
			AccountID:   eurAccount.ID,
			Date:        d1,
			Amount:      mustMoney(t, -300, eur),
			Description: "food",
			CategoryID:  groceries.ID,
			Status:      domain.StatusBooked,
			Kind:        domain.KindPayment,
		})
		if txErr != nil {
			return txErr
		}
		_, txErr = r.Transactions().Insert(context.Background(), domain.Transaction{
			AccountID:   usdAccount.ID,
			Date:        d2,
			Amount:      mustMoney(t, 2000, usd),
			Description: "salary",
			Status:      domain.StatusBooked,
			Kind:        domain.KindPayment,
		})
		if txErr != nil {
			return txErr
		}
		rate, rateErr := domain.NewRate(900_000_000) // 1 USD = 0.9 EUR
		if rateErr != nil {
			return rateErr
		}
		fx, fxErr := domain.NewFXRate(usd, d2, rate)
		if fxErr != nil {
			return fxErr
		}
		return r.FXRates().Upsert(context.Background(), fx)
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	accounts, err := store.ListAccounts(context.Background(), reporting.AccountFilter{
		AccountIDs:      []domain.AccountID{eurAccount.ID},
		IncludeArchived: false,
	})
	if err != nil {
		t.Fatalf("ListAccounts: %v", err)
	}
	if len(accounts) != 1 || accounts[0].ID != eurAccount.ID {
		t.Fatalf("accounts = %+v, want only EUR account", accounts)
	}

	period, err := domain.NewPeriod(d1, d2)
	if err != nil {
		t.Fatalf("NewPeriod: %v", err)
	}
	txs, err := store.ListTransactions(context.Background(), reporting.TransactionFilter{
		Period: period,
	})
	if err != nil {
		t.Fatalf("ListTransactions: %v", err)
	}
	if len(txs) != 2 {
		t.Fatalf("transactions = %d, want 2", len(txs))
	}

	balance, err := store.BalanceAt(context.Background(), eurAccount.ID, d2)
	if err != nil {
		t.Fatalf("BalanceAt: %v", err)
	}
	if got := balance.Minor(); got != 700 {
		t.Fatalf("balance minor = %d, want 700", got)
	}

	fxRate, err := store.FXRateOnOrBefore(context.Background(), usd, eur, d2)
	if err != nil {
		t.Fatalf("FXRateOnOrBefore: %v", err)
	}
	if fxRate.Scaled() != 900_000_000 {
		t.Fatalf("fx rate = %d, want 900000000", fxRate.Scaled())
	}
}
