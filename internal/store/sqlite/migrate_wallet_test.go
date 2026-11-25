package sqlite

import (
	"context"
	"database/sql"
	"io/fs"
	"path/filepath"
	"testing"
	"time"

	"github.com/pressly/goose/v3"

	"github.com/0xballo/fui/internal/clock"
)

func TestMigrateV18ToWalletAndPriceScale(t *testing.T) {
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
	if _, err := provider.UpTo(ctx, 18); err != nil {
		t.Fatalf("migrate to v18: %v", err)
	}

	for i, kind := range []string{"cash", "checking", "credit_card", "savings", "investment", "loan"} {
		mustExec(t, db, `INSERT INTO accounts (name, kind, currency, opening_balance_minor, opened_on)
			VALUES ('`+kind+`', '`+kind+`', 'EUR', `+string(rune('1'+i))+`00, '2026-01-01')`)
	}
	mustExec(t, db, `INSERT INTO transfer_groups (fee_minor) VALUES (0)`)
	mustExec(t, db, `INSERT INTO transactions (account_id, date, amount_minor, currency, description, status, kind, transfer_group_id)
		VALUES (1, '2026-01-10', -1200, 'EUR', 'out', 'booked', 'transfer', 1),
		       (2, '2026-01-10', 1200, 'EUR', 'in', 'booked', 'transfer', 1),
		       (3, '2026-01-11', -300, 'EUR', 'card', 'booked', 'payment', NULL)`)
	mustExec(t, db, `INSERT INTO recurrences (account_id, amount_minor, currency, every, unit, start_date, next_date)
		VALUES (2, -5000, 'EUR', 1, 'months', '2026-01-01', '2026-02-01')`)
	mustExec(t, db, `UPDATE transactions SET recurrence_id = 1 WHERE id = 3`)
	mustExec(t, db, `INSERT INTO loans (kind, principal_minor, currency, disbursed_on, term_months, rate_type, debit_account_id, name, liability_account_id)
		VALUES ('loan', 100000, 'EUR', '2026-01-01', 24, 'fixed', 2, 'L1', 6)`)
	mustExec(t, db, `INSERT INTO budgets (name, mode, amount_minor, currency) VALUES ('B', 'cap', 1000, 'EUR')`)
	mustExec(t, db, `INSERT INTO reconciliations (account_id, date, statement_balance_minor, currency)
		VALUES (2, '2026-01-31', 500, 'EUR')`)
	mustExec(t, db, `INSERT INTO reconcile_ticks (account_id, transaction_id) VALUES (2, 2)`)
	mustExec(t, db, `INSERT INTO securities (name, ticker, kind, currency, quantity_scale) VALUES
		('Stock', 'STK', 'stock', 'EUR', 0),
		('Coin', 'BTC', 'crypto', 'USD', 8),
		('Yen Coin', 'JPC', 'crypto', 'JPY', 8),
		('Yen Stock', 'JST', 'stock', 'JPY', 0)`)
	mustExec(t, db, `INSERT INTO security_prices (security_id, date, price_minor, currency) VALUES
		(1, '2026-01-02', 12345, 'EUR'),
		(2, '2026-01-02', 4200000, 'USD'),
		(3, '2026-01-02', 5000, 'JPY'),
		(4, '2026-01-02', 777, 'JPY')`)
	mustExec(t, db, `INSERT INTO security_ops
		(date, account_id, security_id, kind, quantity_units, quantity_scale, unit_price_minor, unit_price_currency, fees_minor, fees_currency)
		VALUES
		('2026-01-03', 5, 1, 'buy', 2, 0, 12000, 'EUR', 150, 'EUR'),
		('2026-01-03', 5, 2, 'buy', 100000000, 8, 4100000, 'USD', 99, 'USD'),
		('2026-01-03', 5, 3, 'buy', 100000000, 8, 4900, 'JPY', 7, 'JPY'),
		('2026-01-03', 5, 4, 'buy', 1, 0, 770, 'JPY', 0, 'JPY')`)
	counts := map[string]int{}
	tables := []string{
		"accounts", "transactions", "transfer_groups", "recurrences", "loans", "budgets", "reconciliations",
		"reconcile_ticks", "securities", "security_prices", "security_ops",
	}
	for _, tbl := range tables {
		counts[tbl] = rowCount(t, db, tbl)
	}
	mustClose(t, db)

	store, err := Open(ctx, Options{
		Path: dbPath, BackupDir: filepath.Join(root, "data", "backups"), BackupKeep: 10,
		Clock: clock.NewFixed(time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC)),
	})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	conn := store.writeDB

	if v, err := pragmaUserVersion(ctx, conn); err != nil || v != currentSchemaVersion {
		t.Fatalf("user_version = %d, %v; want %d", v, err, currentSchemaVersion)
	}
	for _, tbl := range tables {
		if got := rowCount(t, conn, tbl); got != counts[tbl] {
			t.Errorf("%s rows = %d, want %d", tbl, got, counts[tbl])
		}
	}

	rows, err := conn.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		t.Fatalf("foreign_key_check: %v", err)
	}
	if rows.Next() {
		t.Errorf("foreign_key_check reports violations")
	}
	_ = rows.Close()

	// Foreign keys must still be live and enforced against the rebuilt table.
	if _, err := conn.ExecContext(ctx, "DELETE FROM accounts WHERE id = 1"); err == nil {
		t.Errorf("deleting an account with transactions must fail")
	}
	var recurrenceLinks int
	if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM transactions WHERE recurrence_id IS NOT NULL").Scan(&recurrenceLinks); err != nil || recurrenceLinks != 1 {
		t.Errorf("recurrence links = %d, %v; want 1", recurrenceLinks, err)
	}

	if _, err := conn.ExecContext(ctx, `INSERT INTO accounts (name, kind, currency, opening_balance_minor, opened_on)
		VALUES ('Wallet', 'wallet', 'USD', 0, '2026-01-01')`); err != nil {
		t.Errorf("wallet insert: %v", err)
	}
	if _, err := conn.ExecContext(ctx, `INSERT INTO accounts (name, kind, currency, opening_balance_minor, opened_on)
		VALUES ('Bogus', 'bogus', 'USD', 0, '2026-01-01')`); err == nil {
		t.Errorf("unknown account kind must be rejected")
	}
	if _, err := conn.ExecContext(ctx, `INSERT INTO securities (name, ticker, kind, currency, quantity_scale, price_scale)
		VALUES ('Bad', 'BAD', 'stock', 'EUR', 0, 9)`); err == nil {
		t.Errorf("price_scale 9 must be rejected")
	}

	scales := map[int]int{1: 2, 2: 8, 3: 8, 4: 0}
	for id, want := range scales {
		var got int
		if err := conn.QueryRowContext(ctx, "SELECT price_scale FROM securities WHERE id = ?", id).Scan(&got); err != nil || got != want {
			t.Errorf("security %d price_scale = %d, %v; want %d", id, got, err, want)
		}
	}
	wantPrices := map[int]int64{1: 12345, 2: 4200000 * 1_000_000, 3: 5000 * 100_000_000, 4: 777}
	for id, want := range wantPrices {
		var got int64
		if err := conn.QueryRowContext(ctx, "SELECT price_minor FROM security_prices WHERE security_id = ?", id).Scan(&got); err != nil || got != want {
			t.Errorf("security %d price_minor = %d, %v; want %d", id, got, err, want)
		}
	}
	wantOps := map[int]int64{1: 12000, 2: 4100000 * 1_000_000, 3: 4900 * 100_000_000, 4: 770}
	for id, want := range wantOps {
		var got, fees int64
		if err := conn.QueryRowContext(ctx, "SELECT unit_price_minor, fees_minor FROM security_ops WHERE security_id = ?", id).Scan(&got, &fees); err != nil || got != want {
			t.Errorf("security %d unit_price_minor = %d, %v; want %d", id, got, err, want)
		}
		wantFees := map[int]int64{1: 150, 2: 99, 3: 7, 4: 0}[id]
		if fees != wantFees {
			t.Errorf("security %d fees_minor = %d, want %d (fees stay in currency minor units)", id, fees, wantFees)
		}
	}
}

func rowCount(t *testing.T, db *sql.DB, table string) int {
	t.Helper()
	var n int
	if err := db.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM "+table).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}
