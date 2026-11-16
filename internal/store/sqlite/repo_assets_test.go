package sqlite

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/pressly/goose/v3"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/clock"
	"github.com/0xballo/fui/internal/domain"
)

func withAssets(t *testing.T, s *Store, fn func(ctx context.Context, repo app.AssetRepo) error) error {
	t.Helper()
	ctx := context.Background()
	return s.WithTx(ctx, func(r app.Repos) error { return fn(ctx, r.Assets()) })
}

func seedLoan(t *testing.T, s *Store) {
	t.Helper()
	mustExec(t, s.writeDB, `INSERT INTO accounts (id, name, kind, currency, opening_balance_minor, opened_on)
		VALUES (1, 'Checking', 'checking', 'EUR', 0, '2026-01-01')`)
	mustExec(t, s.writeDB, `INSERT INTO loans (id, name, kind, principal_minor, currency, disbursed_on, term_months, rate_type, debit_account_id)
		VALUES (1, 'Mortgage', 'mortgage', 100000, 'EUR', '2026-01-01', 24, 'fixed', 1)`)
}

func TestAssetRepo(t *testing.T) {
	s, _ := newTestStore(t)
	eur := mustCurrency(t, "EUR")
	seedLoan(t, s)

	var home domain.Asset
	err := withAssets(t, s, func(ctx context.Context, repo app.AssetRepo) error {
		var err error
		home, err = repo.Insert(ctx, domain.Asset{
			Name: " Home ", Kind: domain.AssetKindProperty, PurchaseDate: mustDate(t, "2026-01-01"),
			PurchaseValue: mustMoney(t, 250_000_00, eur), LoanID: 1,
		})
		if err != nil {
			return err
		}
		if home.ID == 0 || home.Name != "Home" || home.LoanID != 1 {
			t.Fatalf("inserted = %+v", home)
		}
		if _, err := repo.Insert(ctx, domain.Asset{
			Name: "HOME", Kind: domain.AssetKindOther, PurchaseDate: home.PurchaseDate, PurchaseValue: home.PurchaseValue,
		}); !errors.Is(err, app.ErrAlreadyExists) {
			t.Fatalf("duplicate name err = %v", err)
		}
		if _, err := repo.Insert(ctx, domain.Asset{
			Name: "Ghost", Kind: domain.AssetKindOther, PurchaseDate: home.PurchaseDate, PurchaseValue: home.PurchaseValue, LoanID: 99,
		}); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("missing loan err = %v", err)
		}
		if _, err := repo.Insert(ctx, domain.Asset{Name: "x"}); !errors.Is(err, domain.ErrMissingField) {
			t.Fatalf("invalid asset err = %v", err)
		}
		car, err := repo.Insert(ctx, domain.Asset{
			Name: "Car", Kind: domain.AssetKindVehicle, PurchaseDate: home.PurchaseDate, PurchaseValue: mustMoney(t, 10_000_00, eur),
		})
		if err != nil {
			return err
		}
		car.Name = "Old car"
		if err := repo.Update(ctx, car); err != nil {
			return err
		}
		if err := repo.Update(ctx, domain.Asset{
			ID: 77, Name: "n", Kind: domain.AssetKindOther, PurchaseDate: home.PurchaseDate, PurchaseValue: home.PurchaseValue,
		}); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("update missing err = %v", err)
		}
		got, err := repo.Get(ctx, car.ID)
		if err != nil {
			return err
		}
		if diff := cmp.Diff(car, got, cmp.AllowUnexported(domain.Money{}, domain.Currency{}, domain.Date{})); diff != "" {
			t.Fatalf("get mismatch (-want +got):\n%s", diff)
		}
		list, err := repo.List(ctx)
		if err != nil {
			return err
		}
		if len(list) != 2 || list[0].ID != home.ID {
			t.Fatalf("list = %+v", list)
		}
		if _, err := repo.Get(ctx, 99); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("get missing err = %v", err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	// Deleting the loan unlinks the asset instead of blocking or removing it.
	mustExec(t, s.writeDB, `DELETE FROM loans WHERE id = 1`)
	err = withAssets(t, s, func(ctx context.Context, repo app.AssetRepo) error {
		got, err := repo.Get(ctx, home.ID)
		if err != nil {
			return err
		}
		if got.LoanID != 0 {
			t.Fatalf("loan id after loan delete = %d", got.LoanID)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestAssetValuations(t *testing.T) {
	s, _ := newTestStore(t)
	eur := mustCurrency(t, "EUR")
	err := withAssets(t, s, func(ctx context.Context, repo app.AssetRepo) error {
		a, err := repo.Insert(ctx, domain.Asset{
			Name: "Home", Kind: domain.AssetKindProperty, PurchaseDate: mustDate(t, "2026-01-01"),
			PurchaseValue: mustMoney(t, 100, eur),
		})
		if err != nil {
			return err
		}
		set := func(date string, minor int64) error {
			return repo.SetValuation(ctx, domain.Valuation{AssetID: a.ID, Date: mustDate(t, date), Value: mustMoney(t, minor, eur)})
		}
		for _, v := range []struct {
			date  string
			minor int64
		}{{"2026-06-01", 120}, {"2026-03-01", 110}, {"2026-06-01", 130}} {
			if err := set(v.date, v.minor); err != nil {
				return err
			}
		}
		vals, err := repo.ListValuations(ctx, a.ID)
		if err != nil {
			return err
		}
		if len(vals) != 2 || vals[0].Date.String() != "2026-03-01" || vals[1].Value.Minor() != 130 {
			t.Fatalf("valuations = %+v", vals)
		}
		if err := repo.SetValuation(ctx, domain.Valuation{AssetID: 99, Date: mustDate(t, "2026-01-01"), Value: mustMoney(t, 1, eur)}); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("missing asset err = %v", err)
		}
		usd := mustCurrency(t, "USD")
		if err := repo.SetValuation(ctx, domain.Valuation{AssetID: a.ID, Date: mustDate(t, "2026-01-01"), Value: mustMoney(t, 1, usd)}); !errors.Is(err, domain.ErrCurrencyMismatch) {
			t.Fatalf("currency err = %v", err)
		}
		if err := repo.SetValuation(ctx, domain.Valuation{AssetID: a.ID}); !errors.Is(err, domain.ErrMissingField) {
			t.Fatalf("invalid valuation err = %v", err)
		}
		if err := repo.DeleteValuation(ctx, a.ID, mustDate(t, "2026-03-01")); err != nil {
			return err
		}
		if err := repo.DeleteValuation(ctx, a.ID, mustDate(t, "2026-03-01")); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("delete missing err = %v", err)
		}
		if err := repo.Delete(ctx, a.ID); err != nil {
			return err
		}
		if err := repo.Delete(ctx, a.ID); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("delete missing asset err = %v", err)
		}
		if n := count(t, s, "asset_valuations"); n != 0 {
			t.Fatalf("valuations left after cascade: %d", n)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestAssetReadOnlyRejectsWrites(t *testing.T) {
	s, _ := newTestStore(t)
	err := s.Read(context.Background(), func(r app.Repos) error {
		_, err := r.Assets().Insert(context.Background(), domain.Asset{})
		return err
	})
	if err == nil {
		t.Fatal("read-only insert succeeded")
	}
}

func TestAssetsSurviveBackupRestoreAndResetDropsThem(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()
	mustExec(t, s.writeDB, `INSERT INTO assets (id, name, kind, purchase_date, purchase_minor, currency)
		VALUES (1, 'Car', 'vehicle', '2026-01-01', 500, 'EUR')`)
	mustExec(t, s.writeDB, `INSERT INTO asset_valuations (asset_id, date, value_minor, currency)
		VALUES (1, '2026-02-01', 400, 'EUR')`)
	path, err := s.Backup(ctx, "manual")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Reset(ctx, app.ResetOptions{KeepSettings: true}); err != nil {
		t.Fatal(err)
	}
	if n, v := count(t, s, "assets"), count(t, s, "asset_valuations"); n != 0 || v != 0 {
		t.Fatalf("after reset assets=%d valuations=%d", n, v)
	}
	if err := s.Restore(ctx, path); err != nil {
		t.Fatal(err)
	}
	if n, v := count(t, s, "assets"), count(t, s, "asset_valuations"); n != 1 || v != 1 {
		t.Fatalf("after restore assets=%d valuations=%d", n, v)
	}
}

func TestMigrateFromV11AddsAssetTables(t *testing.T) {
	root := t.TempDir()
	dbPath := filepath.Join(root, "data", "fui.db")
	ctx := context.Background()

	db := mustOpenRawDB(t, dbPath)
	migrationsFS, err := fs.Sub(migrationFS, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, db, migrationsFS)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.UpTo(ctx, 11); err != nil {
		t.Fatalf("migrate to v11: %v", err)
	}
	mustExec(t, db, `INSERT INTO accounts (name, kind, currency, opening_balance_minor, opened_on)
		VALUES ('Checking', 'checking', 'EUR', 0, '2026-01-01')`)
	mustClose(t, db)

	s, err := Open(ctx, Options{
		Path: dbPath, BackupDir: filepath.Join(root, "data", "backups"), BackupKeep: 10,
		Clock: clock.NewFixed(time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC)),
	})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	if v, err := pragmaUserVersion(ctx, s.writeDB); err != nil || v != currentSchemaVersion {
		t.Fatalf("user_version = %d (err %v), want %d", v, err, currentSchemaVersion)
	}
	if n := count(t, s, "accounts"); n != 1 {
		t.Fatalf("accounts after migration = %d", n)
	}
	mustExec(t, s.writeDB, `INSERT INTO assets (name, kind, purchase_date, purchase_minor, currency)
		VALUES ('Car', 'vehicle', '2026-01-01', 500, 'EUR')`)
	mustExec(t, s.writeDB, `INSERT INTO asset_valuations (asset_id, date, value_minor, currency)
		VALUES (1, '2026-02-01', 400, 'EUR')`)
	if _, err := s.writeDB.ExecContext(ctx, `INSERT INTO asset_valuations (asset_id, date, value_minor, currency)
		VALUES (1, '2026-02-01', 300, 'EUR')`); err == nil {
		t.Fatal("duplicate (asset_id, date) accepted")
	}
	if _, err := s.writeDB.ExecContext(ctx, `INSERT INTO assets (name, kind, purchase_date, purchase_minor, currency, loan_id)
		VALUES ('Bad', 'other', '2026-01-01', 1, 'EUR', 42)`); err == nil {
		t.Fatal("dangling loan_id accepted")
	}
}
