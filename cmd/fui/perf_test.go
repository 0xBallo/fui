package main

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"path/filepath"
	"testing"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/store/sqlite"
)

// perfSyntheticTxCount matches the acceptance criterion in docs/tasks/T11-integration.md: startup
// of `fui account list` on a 50 000-transaction archive, target < 100 ms.
const perfSyntheticTxCount = 50_000

// BenchmarkAccountListStartup50k seeds a 50 000-transaction archive directly through
// internal/store/sqlite (the same real store the composition root opens), then times
// `fui account list --json` end to end through run(), the way a user would invoke it. It is a
// benchmark, not a Test, like internal/store/sqlite's own 50k benchmarks: run it explicitly with
// `go test -bench=BenchmarkAccountListStartup50k -benchtime=1x -run=^$ ./cmd/fui` and report the
// per-op time in .task-report.md (the <100ms target is documentation, not an assertion, so
// ordinary `go test ./...` runs stay fast and unaffected by machine-dependent timing).
func BenchmarkAccountListStartup50k(b *testing.B) {
	dir := b.TempDir()
	b.Setenv("HOME", dir)
	b.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "config"))
	dataDir := filepath.Join(dir, "data")
	b.Setenv("XDG_DATA_HOME", dataDir)
	b.Setenv("XDG_STATE_HOME", filepath.Join(dir, "state"))
	b.Setenv("FUI_TEST_TODAY", "2026-01-15")

	seedPerfArchive(b, filepath.Join(dataDir, "fui", "fui.db"), perfSyntheticTxCount)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var out, errOut bytes.Buffer
		code := run(context.Background(), []string{"account", "list", "--json"}, bytes.NewReader(nil), &out, &errOut)
		if code != 0 {
			b.Fatalf("account list: code=%d stderr=%s", code, errOut.String())
		}
	}
}

// seedPerfArchive creates a fresh SQLite archive at path with five accounts and total
// transactions spread pseudo-randomly across them and three years, mirroring
// internal/store/sqlite's own BenchmarkAccountListBalances50k seed shape.
func seedPerfArchive(tb testing.TB, path string, total int) {
	tb.Helper()
	ctx := context.Background()
	store, err := sqlite.Open(ctx, sqlite.Options{
		Path:       path,
		BackupDir:  filepath.Join(filepath.Dir(path), "backups"),
		BackupKeep: 10,
	})
	if err != nil {
		tb.Fatalf("open archive for seeding: %v", err)
	}
	defer func() { _ = store.Close() }()

	rng := rand.New(rand.NewSource(42))
	eur, err := domain.ParseCurrency("EUR")
	if err != nil {
		tb.Fatalf("parse currency: %v", err)
	}
	start, err := domain.ParseDate("2024-01-01")
	if err != nil {
		tb.Fatalf("parse start date: %v", err)
	}
	opening, err := domain.NewMoney(0, eur)
	if err != nil {
		tb.Fatalf("new money: %v", err)
	}

	var accountIDs []domain.AccountID
	if err := store.WithTx(ctx, func(r app.Repos) error {
		for i := range 5 {
			account, err := r.Accounts().Insert(ctx, domain.Account{
				Name: fmt.Sprintf("Perf-%d", i+1), Kind: domain.AccountChecking,
				Currency: eur, OpeningBalance: opening, OpenedOn: start,
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

	if err := store.WithTx(ctx, func(r app.Repos) error {
		for i := range total {
			date, err := start.AddDays(rng.Intn(365 * 3))
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
			if _, err := r.Transactions().Insert(ctx, domain.Transaction{
				AccountID:   accountIDs[rng.Intn(len(accountIDs))],
				Date:        date,
				Amount:      amount,
				Description: "perf",
				Status:      domain.StatusBooked,
				Kind:        domain.KindPayment,
			}); err != nil {
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
}
