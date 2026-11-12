package sqlite

import (
	"context"
	"fmt"
	"math/rand"
	"path/filepath"
	"testing"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

const syntheticTxCount = 50_000

func BenchmarkAccountListBalances50k(b *testing.B) {
	store, accountIDs, at := seedSyntheticStore(b, syntheticTxCount)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := store.Read(context.Background(), func(r app.Repos) error {
			for _, accountID := range accountIDs {
				if _, err := r.Transactions().SumUpTo(context.Background(), accountID, at); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			b.Fatalf("Read: %v", err)
		}
	}
}

func BenchmarkTransactionKeysetPage50k(b *testing.B) {
	store, _, _ := seedSyntheticStore(b, syntheticTxCount)
	limit := 200

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := store.Read(context.Background(), func(r app.Repos) error {
			f, err := domain.NewTxFilter(domain.TxFilterSpec{Desc: true})
			if err != nil {
				return err
			}
			_, err = r.Transactions().List(context.Background(), f, nil, limit)
			return err
		}); err != nil {
			b.Fatalf("Read: %v", err)
		}
	}
}

func seedSyntheticStore(tb testing.TB, total int) (*Store, []domain.AccountID, domain.Date) {
	tb.Helper()

	root := tb.TempDir()
	store, err := Open(context.Background(), Options{
		Path:       filepath.Join(root, "data", "fui.db"),
		BackupDir:  filepath.Join(root, "data", "backups"),
		BackupKeep: 10,
	})
	if err != nil {
		tb.Fatalf("Open: %v", err)
	}
	tb.Cleanup(func() { _ = store.Close() })
	rng := rand.New(rand.NewSource(42))
	eur, _ := domain.ParseCurrency("EUR")
	start, _ := domain.ParseDate("2024-01-01")
	end, _ := domain.ParseDate("2026-12-31")
	opening, _ := domain.NewMoney(0, eur)

	var accountIDs []domain.AccountID
	if err := store.WithTx(context.Background(), func(r app.Repos) error {
		for i := range 5 {
			account, err := r.Accounts().Insert(context.Background(), domain.Account{
				Name:           fmt.Sprintf("Bench-%d", i+1),
				Kind:           domain.AccountChecking,
				Currency:       eur,
				OpeningBalance: opening,
				OpenedOn:       start,
			})
			if err != nil {
				return err
			}
			accountIDs = append(accountIDs, account.ID)
		}
		return nil
	}); err != nil {
		tb.Fatalf("seed accounts: %v", err)
	}

	if err := store.WithTx(context.Background(), func(r app.Repos) error {
		for i := range total {
			dayOffset := rng.Intn(365 * 3)
			date, err := start.AddDays(dayOffset)
			if err != nil {
				return err
			}
			minor := int64(rng.Intn(50_000) - 25_000)
			if minor == 0 {
				minor = 1
			}
			amount, err := domain.NewMoney(minor, eur)
			if err != nil {
				return err
			}
			accountID := accountIDs[rng.Intn(len(accountIDs))]
			_, err = r.Transactions().Insert(context.Background(), domain.Transaction{
				AccountID:   accountID,
				Date:        date,
				Amount:      amount,
				Description: "bench",
				Status:      domain.StatusBooked,
				Kind:        domain.KindPayment,
			})
			if err != nil {
				return err
			}
			if i%10_000 == 0 {
				tb.Logf("seeded %d transactions", i)
			}
		}
		return nil
	}); err != nil {
		tb.Fatalf("seed transactions: %v", err)
	}

	return store, accountIDs, end
}
