package sqlite

import (
	"context"
	"fmt"
	"math/rand"
	"path/filepath"
	"testing"
	"time"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

const syntheticTxCount = 50_000

// searchTxCount and searchTarget are the spec's search goal: filter + text over 100 000
// transactions in under 200 ms. BenchmarkSearch100k is not part of `make test`.
const (
	searchTxCount = 100_000
	searchTarget  = 200 * time.Millisecond
)

var benchDescriptions = []string{
	"Esselunga spesa", "Caffè bar Roma", "Netflix abbonamento", "Affitto ottobre", "Trenitalia biglietto",
	"Amazon ordine", "Farmacia", "Stipendio", "bench", "Ristorante da Mario",
}

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
				Description: benchDescriptions[i%len(benchDescriptions)],
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

func BenchmarkSearch100k(b *testing.B) {
	store, accountIDs, _ := seedSyntheticStore(b, searchTxCount)
	f, err := domain.NewTxFilter(domain.TxFilterSpec{
		AccountIDs: accountIDs[:2], From: mustBenchDate(b, "2025-01-01"), To: mustBenchDate(b, "2025-12-31"),
		Sign: domain.TxSignExpense, Text: "esselun", Desc: true, Limit: 100,
	})
	if err != nil {
		b.Fatalf("NewTxFilter: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := store.Read(context.Background(), func(r app.Repos) error {
			_, err := r.Transactions().List(context.Background(), f, nil, 0)
			return err
		}); err != nil {
			b.Fatalf("Read: %v", err)
		}
	}
	b.StopTimer()
	if per := time.Duration(b.Elapsed().Nanoseconds() / int64(b.N)); per > searchTarget {
		b.Logf("search took %v per query, above the %v target", per, searchTarget)
	}
}

func mustBenchDate(tb testing.TB, s string) domain.Date {
	tb.Helper()
	d, err := domain.ParseDate(s)
	if err != nil {
		tb.Fatalf("ParseDate(%q): %v", s, err)
	}
	return d
}
