package sqlite

import (
	"context"
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

func TestTransactionOriginalAndRawRoundTrip(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	eur, usd := mustCurrency(t, "EUR"), mustCurrency(t, "USD")
	orig := mustMoney(t, -1999, usd)

	tests := []struct {
		name     string
		original *domain.Money
		raw      string
	}{
		{name: "none"},
		{name: "original only", original: &orig},
		{name: "raw only", raw: `{"a":1}`},
		{name: "both", original: &orig, raw: `{"memo":"x"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := store.WithTx(ctx, func(r app.Repos) error {
				acc, err := r.Accounts().Insert(ctx, domain.Account{
					Name: "Acc " + tt.name, Kind: domain.AccountChecking, Currency: eur,
					OpeningBalance: mustMoney(t, 0, eur), OpenedOn: mustDate(t, "2026-01-01"),
				})
				if err != nil {
					return err
				}
				in := domain.Transaction{
					AccountID: acc.ID, Date: mustDate(t, "2026-02-01"), Amount: mustMoney(t, -1800, eur),
					Status: domain.StatusBooked, Kind: domain.KindPayment, Original: tt.original, Raw: tt.raw,
				}
				ins, err := r.Transactions().Insert(ctx, in)
				if err != nil {
					return err
				}
				got, err := r.Transactions().Get(ctx, ins.ID)
				if err != nil {
					return err
				}
				if diff := cmp.Diff(in.Original, got.Original, cmp.AllowUnexported(domain.Money{}, domain.Currency{})); diff != "" {
					t.Errorf("original (-want +got):\n%s", diff)
				}
				if got.Raw != tt.raw {
					t.Errorf("raw = %q, want %q", got.Raw, tt.raw)
				}

				// Update flips the presence of both fields.
				if tt.original == nil {
					got.Original, got.Raw = &orig, `{"u":true}`
				} else {
					got.Original, got.Raw = nil, ""
				}
				if err := r.Transactions().Update(ctx, got); err != nil {
					return err
				}
				again, err := r.Transactions().Get(ctx, ins.ID)
				if err != nil {
					return err
				}
				if (again.Original == nil) != (got.Original == nil) || again.Raw != got.Raw {
					t.Errorf("after update original=%v raw=%q, want original=%v raw=%q", again.Original, again.Raw, got.Original, got.Raw)
				}
				return nil
			}); err != nil {
				t.Fatalf("round trip: %v", err)
			}
		})
	}
}

func TestTransactionOriginalRawConstraints(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	mustExec(t, store.writeDB, `INSERT INTO accounts (id, name, kind, currency, opening_balance_minor, opened_on)
		VALUES (1, 'A', 'checking', 'EUR', 0, '2026-01-01')`)
	const prefix = `INSERT INTO transactions (account_id, date, amount_minor, currency, status, kind, `
	const suffix = ` VALUES (1, '2026-01-02', 1, 'EUR', 'booked', 'other', `
	for name, q := range map[string]string{
		"amount without currency": prefix + `original_amount_minor)` + suffix + `5)`,
		"currency without amount": prefix + `original_currency)` + suffix + `'USD')`,
		"bad currency length":     prefix + `original_amount_minor, original_currency)` + suffix + `5, 'US')`,
		"invalid json":            prefix + `raw)` + suffix + `'{nope')`,
	} {
		if _, err := store.writeDB.ExecContext(ctx, q); err == nil {
			t.Errorf("%s: insert succeeded, want constraint error", name)
		}
	}
}

func TestSecurityOpImportBatch(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	eur := mustCurrency(t, "EUR")
	qty, err := domain.NewQuantity(10, 0)
	if err != nil {
		t.Fatal(err)
	}

	var batch app.ImportBatch
	var opID domain.SecurityOpID
	if err := store.WithTx(ctx, func(r app.Repos) error {
		acc, err := r.Accounts().Insert(ctx, domain.Account{
			Name: "Broker", Kind: domain.AccountInvestment, Currency: eur,
			OpeningBalance: mustMoney(t, 0, eur), OpenedOn: mustDate(t, "2026-01-01"),
		})
		if err != nil {
			return err
		}
		sec, err := mustSecurityRepo(t, r).Insert(ctx, domain.Security{Name: "ETF", Ticker: "etf", Kind: domain.SecurityETF, Currency: eur})
		if err != nil {
			return err
		}
		batch, err = r.Imports().InsertBatch(ctx, app.ImportBatch{Connector: "x", Source: "s", SourceSHA256: "h", ImportedAt: time.Unix(0, 0).UTC()})
		if err != nil {
			return err
		}
		mk := func(day string, b domain.ImportBatchID) domain.SecurityOp {
			return domain.SecurityOp{
				Date: mustDate(t, day), Account: acc.ID, Security: sec.ID, Kind: domain.OpBuy, Quantity: qty,
				UnitPrice: mustPrice(t, 100, eur), Fees: mustMoney(t, 0, eur), ImportBatchID: b,
			}
		}
		second, err := mustSecurityRepo(t, r).InsertOp(ctx, mk("2026-03-02", batch.ID))
		if err != nil {
			return err
		}
		if _, err := mustSecurityRepo(t, r).InsertOp(ctx, mk("2026-03-01", batch.ID)); err != nil {
			return err
		}
		if _, err := mustSecurityRepo(t, r).InsertOp(ctx, mk("2026-03-03", 0)); err != nil {
			return err
		}
		opID = second.ID
		return nil
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	err = store.WithTx(ctx, func(r app.Repos) error {
		sr := mustSecurityRepo(t, r)
		ops, err := sr.ListOpsByBatch(ctx, batch.ID)
		if err != nil {
			return err
		}
		if len(ops) != 2 || ops[0].Date.String() != "2026-03-01" || ops[1].ID != opID || ops[1].ImportBatchID != batch.ID {
			t.Errorf("ListOpsByBatch = %+v", ops)
		}
		// Update keeps the batch link.
		ops[1].Fees = mustMoney(t, 5, eur)
		if err := sr.UpdateOp(ctx, ops[1]); err != nil {
			return err
		}
		got, err := sr.GetOp(ctx, opID)
		if err != nil {
			return err
		}
		if got.ImportBatchID != batch.ID || got.Fees.Minor() != 5 {
			t.Errorf("after update = %+v", got)
		}
		none, err := sr.ListOpsByBatch(ctx, 0)
		if err != nil {
			return err
		}
		if len(none) != 0 {
			t.Errorf("batch 0 returned %d ops, want 0", len(none))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	mustExec(t, store.writeDB, `DELETE FROM import_batches`)
	err = store.WithTx(ctx, func(r app.Repos) error {
		ops, err := mustSecurityRepo(t, r).ListOpsByBatch(ctx, batch.ID)
		if err != nil {
			return err
		}
		if len(ops) != 0 {
			t.Errorf("ops after batch delete = %d, want 0", len(ops))
		}
		got, err := mustSecurityRepo(t, r).GetOp(ctx, opID)
		if err != nil {
			return err
		}
		if got.ImportBatchID != 0 {
			t.Errorf("ImportBatchID after batch delete = %d, want 0", got.ImportBatchID)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestMigrateFromV15AddsImportColumns(t *testing.T) {
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
	if _, err := provider.UpTo(ctx, 15); err != nil {
		t.Fatalf("migrate to v15: %v", err)
	}
	mustExec(t, db, `INSERT INTO accounts (id, name, kind, currency, opening_balance_minor, opened_on)
		VALUES (1, 'Broker', 'investment', 'EUR', 0, '2026-01-01')`)
	mustExec(t, db, `INSERT INTO transactions (id, account_id, date, amount_minor, currency, status, kind, description)
		VALUES (1, 1, '2026-04-01', -12000, 'EUR', 'booked', 'investment', 'seed tx')`)
	mustExec(t, db, `INSERT INTO securities (id, name, ticker, kind, currency, quantity_scale)
		VALUES (1, 'VWCE', 'VWCE', 'etf', 'EUR', 4)`)
	mustExec(t, db, `INSERT INTO security_ops (date, account_id, security_id, kind, quantity_units, quantity_scale,
			unit_price_minor, unit_price_currency, fees_minor, fees_currency, cash_tx_id)
		VALUES ('2026-04-01', 1, 1, 'buy', 10000, 4, 1200, 'EUR', 10, 'EUR', 1)`)
	mustClose(t, db)

	store, err := Open(ctx, Options{
		Path: dbPath, BackupDir: filepath.Join(root, "data", "backups"), BackupKeep: 10,
		Clock: clock.NewFixed(time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC)),
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
	if err := store.WithTx(ctx, func(r app.Repos) error {
		tx, err := r.Transactions().Get(ctx, 1)
		if err != nil {
			return err
		}
		if tx.Original != nil || tx.Raw != "" || tx.Amount.Minor() != -12000 {
			t.Errorf("migrated tx = %+v", tx)
		}
		op, err := mustSecurityRepo(t, r).GetOp(ctx, 1)
		if err != nil {
			return err
		}
		if op.ImportBatchID != 0 || op.CashTx != 1 {
			t.Errorf("migrated op = %+v", op)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
