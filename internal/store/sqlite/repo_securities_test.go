package sqlite

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"testing"
	"time"

	"github.com/pressly/goose/v3"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/clock"
	"github.com/0xballo/fui/internal/domain"
)

type securitiesAccess interface {
	Securities() app.SecurityRepo
}

func mustSecurityRepo(t *testing.T, r app.Repos) app.SecurityRepo {
	t.Helper()
	access, ok := r.(securitiesAccess)
	if !ok {
		t.Fatal("repos does not expose Securities")
	}
	return access.Securities()
}

func TestSecurityRepoCRUD(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	eur := mustCurrency(t, "EUR")

	var account domain.Account
	if err := store.WithTx(ctx, func(r app.Repos) error {
		var err error
		account, err = r.Accounts().Insert(ctx, domain.Account{
			Name:           "Broker",
			Kind:           domain.AccountInvestment,
			Currency:       eur,
			OpeningBalance: mustMoney(t, 0, eur),
			OpenedOn:       mustDate(t, "2026-01-01"),
		})
		if err != nil {
			return err
		}
		securities := mustSecurityRepo(t, r)
		_, err = securities.Insert(ctx, domain.Security{
			Name:          "Bitcoin",
			Ticker:        "btc",
			Kind:          domain.SecurityCrypto,
			Currency:      eur,
			QuantityScale: 8,
		})
		return err
	}); err != nil {
		t.Fatalf("seed account/security: %v", err)
	}

	var inserted domain.Security
	if err := store.WithTx(ctx, func(r app.Repos) error {
		securities := mustSecurityRepo(t, r)
		var err error
		inserted, err = securities.Insert(ctx, domain.Security{
			Name:          "VWCE",
			Ticker:        "vwce",
			ISIN:          "IE00BK5BQT80",
			Kind:          domain.SecurityETF,
			Currency:      eur,
			QuantityScale: 4,
		})
		if err != nil {
			return err
		}
		_, err = securities.Insert(ctx, domain.Security{
			Name:          "Dupe",
			Ticker:        "VWCE",
			Kind:          domain.SecurityETF,
			Currency:      eur,
			QuantityScale: 4,
		})
		if !errors.Is(err, app.ErrAlreadyExists) {
			t.Fatalf("duplicate ticker error = %v, want ErrAlreadyExists", err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if inserted.ID == 0 || inserted.Ticker != "VWCE" {
		t.Fatalf("inserted security = %+v", inserted)
	}

	if err := store.WithTx(ctx, func(r app.Repos) error {
		securities := mustSecurityRepo(t, r)
		inserted.Name = "VWCE UCITS"
		if err := securities.Update(ctx, inserted); err != nil {
			return err
		}
		return nil
	}); err != nil {
		t.Fatalf("update security: %v", err)
	}

	if err := store.Read(ctx, func(r app.Repos) error {
		securities := mustSecurityRepo(t, r)
		got, err := securities.Get(ctx, inserted.ID)
		if err != nil {
			return err
		}
		if got.Name != "VWCE UCITS" {
			t.Fatalf("Get name = %q, want %q", got.Name, "VWCE UCITS")
		}
		list, err := securities.List(ctx)
		if err != nil {
			return err
		}
		if len(list) != 2 {
			t.Fatalf("List len = %d, want 2", len(list))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	var cashTx domain.Transaction
	if err := store.WithTx(ctx, func(r app.Repos) error {
		var err error
		cashTx, err = r.Transactions().Insert(ctx, domain.Transaction{
			AccountID:   account.ID,
			Date:        mustDate(t, "2026-04-01"),
			Amount:      mustMoney(t, -12345, eur),
			Description: "Security buy",
			Status:      domain.StatusBooked,
			Kind:        domain.KindInvestment,
		})
		if err != nil {
			return err
		}
		securities := mustSecurityRepo(t, r)
		if err := securities.UpsertPrice(ctx, domain.SecurityPrice{
			Security: inserted.ID,
			Date:     mustDate(t, "2026-04-01"),
			Price:    mustMoney(t, 1250, eur),
		}); err != nil {
			return err
		}
		if err := securities.UpsertPrice(ctx, domain.SecurityPrice{
			Security: inserted.ID,
			Date:     mustDate(t, "2026-04-01"),
			Price:    mustMoney(t, 1300, eur),
		}); err != nil {
			return err
		}
		qty, err := domain.NewQuantity(30000, 4)
		if err != nil {
			return err
		}
		op, err := securities.InsertOp(ctx, domain.SecurityOp{
			Date:      mustDate(t, "2026-04-01"),
			Account:   account.ID,
			Security:  inserted.ID,
			Kind:      domain.OpBuy,
			Quantity:  qty,
			UnitPrice: mustMoney(t, 1200, eur),
			Fees:      mustMoney(t, 45, eur),
			CashTx:    cashTx.ID,
		})
		if err != nil {
			return err
		}
		op.Kind = domain.OpSell
		if err := securities.UpdateOp(ctx, op); err != nil {
			return err
		}
		got, err := securities.GetOp(ctx, op.ID)
		if err != nil {
			return err
		}
		if got.Kind != domain.OpSell {
			t.Fatalf("GetOp kind = %q, want %q", got.Kind, domain.OpSell)
		}
		if err := securities.DeleteOp(ctx, op.ID); err != nil {
			return err
		}
		if err := securities.DeleteOp(ctx, op.ID); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("DeleteOp missing err = %v, want ErrNotFound", err)
		}
		return nil
	}); err != nil {
		t.Fatalf("price/op round-trip: %v", err)
	}

	if err := store.Read(ctx, func(r app.Repos) error {
		securities := mustSecurityRepo(t, r)
		price, err := securities.PriceOnOrBefore(ctx, inserted.ID, mustDate(t, "2026-04-30"))
		if err != nil {
			return err
		}
		if price.Price.Minor() != 1300 {
			t.Fatalf("PriceOnOrBefore amount = %d, want 1300", price.Price.Minor())
		}
		prices, err := securities.ListPrices(ctx, inserted.ID)
		if err != nil {
			return err
		}
		if len(prices) != 1 {
			t.Fatalf("ListPrices len = %d, want 1", len(prices))
		}
		ops, err := securities.ListOpsBySecurity(ctx, inserted.ID, mustDate(t, "2026-01-01"), mustDate(t, "2026-12-31"))
		if err != nil {
			return err
		}
		if len(ops) != 0 {
			t.Fatalf("ListOpsBySecurity len = %d, want 0", len(ops))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := store.WithTx(ctx, func(r app.Repos) error {
		securities := mustSecurityRepo(t, r)
		qty, err := domain.NewQuantity(10, 2)
		if err != nil {
			return err
		}
		_, err = securities.InsertOp(ctx, domain.SecurityOp{
			Date:      mustDate(t, "2026-04-02"),
			Account:   account.ID,
			Security:  inserted.ID,
			Kind:      domain.OpBuy,
			Quantity:  qty,
			UnitPrice: mustMoney(t, 1000, eur),
			Fees:      mustMoney(t, 0, eur),
			CashTx:    cashTx.ID,
		})
		if !errors.Is(err, domain.ErrInvalidValue) {
			t.Fatalf("insert op scale mismatch err = %v, want ErrInvalidValue", err)
		}
		if err := securities.DeletePrice(ctx, inserted.ID, mustDate(t, "2026-04-01")); err != nil {
			return err
		}
		if err := securities.DeletePrice(ctx, inserted.ID, mustDate(t, "2026-04-01")); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("DeletePrice missing err = %v, want ErrNotFound", err)
		}
		if err := securities.Delete(ctx, inserted.ID); err != nil {
			return err
		}
		if err := securities.Delete(ctx, inserted.ID); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("Delete missing err = %v, want ErrNotFound", err)
		}
		return nil
	}); err != nil {
		t.Fatalf("delete security: %v", err)
	}
}

func TestSecurityFKRestrict(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	eur := mustCurrency(t, "EUR")

	var account domain.Account
	var security domain.Security
	if err := store.WithTx(ctx, func(r app.Repos) error {
		var err error
		account, err = r.Accounts().Insert(ctx, domain.Account{
			Name:           "Broker",
			Kind:           domain.AccountInvestment,
			Currency:       eur,
			OpeningBalance: mustMoney(t, 0, eur),
			OpenedOn:       mustDate(t, "2026-01-01"),
		})
		if err != nil {
			return err
		}
		securities := mustSecurityRepo(t, r)
		security, err = securities.Insert(ctx, domain.Security{
			Name:          "VWCE",
			Ticker:        "VWCE",
			ISIN:          "IE00BK5BQT80",
			Kind:          domain.SecurityETF,
			Currency:      eur,
			QuantityScale: 4,
		})
		if err != nil {
			return err
		}
		if err := securities.UpsertPrice(ctx, domain.SecurityPrice{
			Security: security.ID,
			Date:     mustDate(t, "2026-05-01"),
			Price:    mustMoney(t, 1234, eur),
		}); err != nil {
			return err
		}
		qty, err := domain.NewQuantity(5000, 4)
		if err != nil {
			return err
		}
		_, err = securities.InsertOp(ctx, domain.SecurityOp{
			Date:      mustDate(t, "2026-05-01"),
			Account:   account.ID,
			Security:  security.ID,
			Kind:      domain.OpBuy,
			Quantity:  qty,
			UnitPrice: mustMoney(t, 1230, eur),
			Fees:      mustMoney(t, 10, eur),
		})
		return err
	}); err != nil {
		t.Fatal(err)
	}

	if err := store.WithTx(ctx, func(r app.Repos) error {
		err := mustSecurityRepo(t, r).Delete(ctx, security.ID)
		if !errors.Is(err, app.ErrInUse) {
			t.Fatalf("Delete in-use security err = %v, want ErrInUse", err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestMigrateFromV11AddsSecuritiesTables(t *testing.T) {
	root := t.TempDir()
	dbPath := filepath.Join(root, "data", "fui.db")
	ctx := context.Background()

	db := mustOpenRawDB(t, dbPath)
	migrationsFS, err := fs.Sub(migrationFS, "migrations")
	if err != nil {
		t.Fatalf("fs.Sub: %v", err)
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, db, migrationsFS)
	if err != nil {
		t.Fatalf("goose.NewProvider: %v", err)
	}
	if _, err := provider.UpTo(ctx, 11); err != nil {
		t.Fatalf("migrate to v11: %v", err)
	}
	mustExec(t, db, `INSERT INTO accounts (id, name, kind, currency, opening_balance_minor, opened_on)
		VALUES (1, 'Broker', 'investment', 'EUR', 0, '2026-01-01')`)
	mustExec(t, db, `INSERT INTO transactions (id, account_id, date, amount_minor, currency, status, kind, description)
		VALUES (1, 1, '2026-04-01', -12000, 'EUR', 'booked', 'investment', 'seed tx')`)
	mustClose(t, db)

	store, err := Open(ctx, Options{
		Path:       dbPath,
		BackupDir:  filepath.Join(root, "data", "backups"),
		BackupKeep: 10,
		Clock:      clock.NewFixed(time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC)),
	})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	version, err := pragmaUserVersion(ctx, store.writeDB)
	if err != nil {
		t.Fatalf("pragmaUserVersion: %v", err)
	}
	if version != currentSchemaVersion {
		t.Fatalf("user_version = %d, want %d", version, currentSchemaVersion)
	}

	mustExec(t, store.writeDB, `INSERT INTO securities (id, name, ticker, kind, currency, quantity_scale)
		VALUES (1, 'VWCE', 'VWCE', 'etf', 'EUR', 4)`)
	mustExec(t, store.writeDB, `INSERT INTO security_prices (security_id, date, price_minor, currency)
		VALUES (1, '2026-04-01', 1234, 'EUR')`)
	mustExec(t, store.writeDB, `INSERT INTO security_ops (
			date, account_id, security_id, kind, quantity_units, quantity_scale, unit_price_minor,
			unit_price_currency, fees_minor, fees_currency, cash_tx_id
		) VALUES (
			'2026-04-01', 1, 1, 'buy', 10000, 4, 1200, 'EUR', 10, 'EUR', 1
		)`)
	if _, err := store.writeDB.ExecContext(ctx, `DELETE FROM securities WHERE id = 1`); !errors.Is(mapSQLError(err), app.ErrInUse) {
		t.Fatalf("delete in-use security err = %v, want ErrInUse", err)
	}
}

func TestResetWipesSecurities(t *testing.T) {
	for _, keep := range []bool{false, true} {
		t.Run(map[bool]string{false: "full", true: "keep settings"}[keep], func(t *testing.T) {
			store, _ := newTestStore(t)
			ctx := context.Background()
			eur := mustCurrency(t, "EUR")

			if err := store.WithTx(ctx, func(r app.Repos) error {
				account, err := r.Accounts().Insert(ctx, domain.Account{
					Name:           "Broker",
					Kind:           domain.AccountInvestment,
					Currency:       eur,
					OpeningBalance: mustMoney(t, 0, eur),
					OpenedOn:       mustDate(t, "2026-01-01"),
				})
				if err != nil {
					return err
				}
				securities := mustSecurityRepo(t, r)
				security, err := securities.Insert(ctx, domain.Security{
					Name:          "VWCE",
					Ticker:        "VWCE",
					ISIN:          "IE00BK5BQT80",
					Kind:          domain.SecurityETF,
					Currency:      eur,
					QuantityScale: 4,
				})
				if err != nil {
					return err
				}
				if err := securities.UpsertPrice(ctx, domain.SecurityPrice{
					Security: security.ID,
					Date:     mustDate(t, "2026-04-01"),
					Price:    mustMoney(t, 1300, eur),
				}); err != nil {
					return err
				}
				qty, err := domain.NewQuantity(10000, 4)
				if err != nil {
					return err
				}
				_, err = securities.InsertOp(ctx, domain.SecurityOp{
					Date:      mustDate(t, "2026-04-01"),
					Account:   account.ID,
					Security:  security.ID,
					Kind:      domain.OpBuy,
					Quantity:  qty,
					UnitPrice: mustMoney(t, 1200, eur),
					Fees:      mustMoney(t, 10, eur),
				})
				return err
			}); err != nil {
				t.Fatal(err)
			}

			if err := store.Reset(ctx, app.ResetOptions{KeepSettings: keep}); err != nil {
				t.Fatalf("Reset: %v", err)
			}
			for _, table := range []string{"securities", "security_prices", "security_ops"} {
				var n int
				if err := store.writeDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table).Scan(&n); err != nil {
					t.Fatalf("count %s: %v", table, err)
				}
				if n != 0 {
					t.Fatalf("table %s has %d rows after reset, want 0", table, n)
				}
			}
		})
	}
}

func TestBackupRestoreRoundTripSecurities(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	eur := mustCurrency(t, "EUR")

	if err := store.WithTx(ctx, func(r app.Repos) error {
		securities := mustSecurityRepo(t, r)
		_, err := securities.Insert(ctx, domain.Security{
			Name:          "VWCE",
			Ticker:        "VWCE",
			ISIN:          "IE00BK5BQT80",
			Kind:          domain.SecurityETF,
			Currency:      eur,
			QuantityScale: 4,
		})
		return err
	}); err != nil {
		t.Fatal(err)
	}

	backupPath, err := store.Backup(ctx, app.BackupReasonManual)
	if err != nil {
		t.Fatalf("Backup: %v", err)
	}

	if err := store.WithTx(ctx, func(r app.Repos) error {
		securities := mustSecurityRepo(t, r)
		_, err := securities.Insert(ctx, domain.Security{
			Name:          "Bitcoin",
			Ticker:        "BTC",
			Kind:          domain.SecurityCrypto,
			Currency:      eur,
			QuantityScale: 8,
		})
		return err
	}); err != nil {
		t.Fatal(err)
	}

	if err := store.Restore(ctx, backupPath); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	if err := store.Read(ctx, func(r app.Repos) error {
		list, err := mustSecurityRepo(t, r).List(ctx)
		if err != nil {
			return err
		}
		if len(list) != 1 || list[0].Name != "VWCE" {
			t.Fatalf("securities after restore = %+v, want only VWCE", list)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestSecurityRepoValidationAndReadOnly(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	eur := mustCurrency(t, "EUR")
	usd := mustCurrency(t, "USD")

	var account domain.Account
	var security domain.Security
	if err := store.WithTx(ctx, func(r app.Repos) error {
		var err error
		account, err = r.Accounts().Insert(ctx, domain.Account{
			Name:           "Broker",
			Kind:           domain.AccountInvestment,
			Currency:       eur,
			OpeningBalance: mustMoney(t, 0, eur),
			OpenedOn:       mustDate(t, "2026-01-01"),
		})
		if err != nil {
			return err
		}
		security, err = mustSecurityRepo(t, r).Insert(ctx, domain.Security{
			Name:          "VWCE",
			Ticker:        "VWCE",
			Kind:          domain.SecurityETF,
			Currency:      eur,
			QuantityScale: 4,
		})
		return err
	}); err != nil {
		t.Fatal(err)
	}

	if err := store.Read(ctx, func(r app.Repos) error {
		repo := mustSecurityRepo(t, r)
		if _, err := repo.Insert(ctx, domain.Security{Name: "x", Ticker: "X", Kind: domain.SecurityStock, Currency: eur, QuantityScale: 0}); !errors.Is(err, errReadOnly) {
			t.Fatalf("Insert read-only err = %v, want errReadOnly", err)
		}
		if err := repo.Update(ctx, security); !errors.Is(err, errReadOnly) {
			t.Fatalf("Update read-only err = %v, want errReadOnly", err)
		}
		if err := repo.Delete(ctx, security.ID); !errors.Is(err, errReadOnly) {
			t.Fatalf("Delete read-only err = %v, want errReadOnly", err)
		}
		if err := repo.UpsertPrice(ctx, domain.SecurityPrice{Security: security.ID, Date: mustDate(t, "2026-01-01"), Price: mustMoney(t, 1, eur)}); !errors.Is(err, errReadOnly) {
			t.Fatalf("UpsertPrice read-only err = %v, want errReadOnly", err)
		}
		if err := repo.DeletePrice(ctx, security.ID, mustDate(t, "2026-01-01")); !errors.Is(err, errReadOnly) {
			t.Fatalf("DeletePrice read-only err = %v, want errReadOnly", err)
		}
		qty, qErr := domain.NewQuantity(1, 4)
		if qErr != nil {
			t.Fatal(qErr)
		}
		if _, err := repo.InsertOp(ctx, domain.SecurityOp{
			Date:      mustDate(t, "2026-01-01"),
			Account:   account.ID,
			Security:  security.ID,
			Kind:      domain.OpBuy,
			Quantity:  qty,
			UnitPrice: mustMoney(t, 1, eur),
			Fees:      mustMoney(t, 0, eur),
		}); !errors.Is(err, errReadOnly) {
			t.Fatalf("InsertOp read-only err = %v, want errReadOnly", err)
		}
		if err := repo.UpdateOp(ctx, domain.SecurityOp{ID: 1}); !errors.Is(err, errReadOnly) {
			t.Fatalf("UpdateOp read-only err = %v, want errReadOnly", err)
		}
		if err := repo.DeleteOp(ctx, 1); !errors.Is(err, errReadOnly) {
			t.Fatalf("DeleteOp read-only err = %v, want errReadOnly", err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := store.WithTx(ctx, func(r app.Repos) error {
		repo := mustSecurityRepo(t, r)
		if err := repo.UpsertPrice(ctx, domain.SecurityPrice{Security: security.ID, Date: mustDate(t, "2026-01-01"), Price: mustMoney(t, 10, usd)}); !errors.Is(err, domain.ErrCurrencyMismatch) {
			t.Fatalf("UpsertPrice currency mismatch err = %v, want ErrCurrencyMismatch", err)
		}
		if err := repo.UpsertPrice(ctx, domain.SecurityPrice{Security: 0, Date: mustDate(t, "2026-01-01"), Price: mustMoney(t, 10, eur)}); !errors.Is(err, domain.ErrMissingField) {
			t.Fatalf("UpsertPrice missing security err = %v, want ErrMissingField", err)
		}
		if err := repo.UpsertPrice(ctx, domain.SecurityPrice{Security: security.ID, Date: mustDate(t, "2026-01-01"), Price: mustMoney(t, 0, eur)}); !errors.Is(err, domain.ErrInvalidValue) {
			t.Fatalf("UpsertPrice zero price err = %v, want ErrInvalidValue", err)
		}
		if _, err := repo.PriceOnOrBefore(ctx, security.ID, domain.Date{}); !errors.Is(err, domain.ErrMissingField) {
			t.Fatalf("PriceOnOrBefore zero date err = %v, want ErrMissingField", err)
		}
		if _, err := repo.PriceOnOrBefore(ctx, security.ID, mustDate(t, "2026-01-01")); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("PriceOnOrBefore missing err = %v, want ErrNotFound", err)
		}
		if _, err := repo.ListPrices(ctx, 0); !errors.Is(err, domain.ErrMissingField) {
			t.Fatalf("ListPrices missing security err = %v, want ErrMissingField", err)
		}
		if _, err := repo.ListOpsBySecurity(ctx, security.ID, mustDate(t, "2026-02-01"), mustDate(t, "2026-01-01")); !errors.Is(err, domain.ErrInvalidValue) {
			t.Fatalf("ListOpsBySecurity bad period err = %v, want ErrInvalidValue", err)
		}
		if _, err := repo.ListOpsBySecurity(ctx, 0, domain.Date{}, domain.Date{}); !errors.Is(err, domain.ErrMissingField) {
			t.Fatalf("ListOpsBySecurity missing security err = %v, want ErrMissingField", err)
		}
		qty, err := domain.NewQuantity(1000, 4)
		if err != nil {
			return err
		}
		_, err = repo.InsertOp(ctx, domain.SecurityOp{
			Date:      mustDate(t, "2026-01-01"),
			Account:   999,
			Security:  security.ID,
			Kind:      domain.OpBuy,
			Quantity:  qty,
			UnitPrice: mustMoney(t, 10, eur),
			Fees:      mustMoney(t, 0, eur),
		})
		if !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("InsertOp missing account err = %v, want ErrNotFound", err)
		}
		_, err = repo.InsertOp(ctx, domain.SecurityOp{
			Date:      mustDate(t, "2026-01-01"),
			Account:   account.ID,
			Security:  security.ID,
			Kind:      domain.OpBuy,
			Quantity:  qty,
			UnitPrice: mustMoney(t, 10, usd),
			Fees:      mustMoney(t, 0, usd),
		})
		if !errors.Is(err, domain.ErrCurrencyMismatch) {
			t.Fatalf("InsertOp currency mismatch err = %v, want ErrCurrencyMismatch", err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
