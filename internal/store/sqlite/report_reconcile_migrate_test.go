package sqlite

import (
	"context"
	"io/fs"
	"path/filepath"
	"testing"
	"time"

	"github.com/pressly/goose/v3"

	"github.com/0xballo/fui/internal/clock"
)

func TestMigrateFromV8AddsSavedReportsAndReconciliations(t *testing.T) {
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
	if _, err := provider.UpTo(ctx, 8); err != nil {
		t.Fatalf("migrate to v8: %v", err)
	}
	mustExec(t, db, `INSERT INTO accounts (id, name, kind, currency, opening_balance_minor, opened_on)
		VALUES (1, 'Checking', 'checking', 'EUR', 0, '2026-01-01')`)
	mustExec(t, db, `INSERT INTO transactions (id, account_id, date, amount_minor, currency, description, status, kind)
		VALUES (7, 1, '2026-01-10', -1200, 'EUR', 'before v9', 'booked', 'payment')`)
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

	var accountCount, txCount int
	if err := store.writeDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM accounts").Scan(&accountCount); err != nil {
		t.Fatalf("count accounts: %v", err)
	}
	if err := store.writeDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM transactions").Scan(&txCount); err != nil {
		t.Fatalf("count transactions: %v", err)
	}
	if accountCount != 1 || txCount != 1 {
		t.Fatalf("counts after migration = (%d, %d), want (1, 1)", accountCount, txCount)
	}

	if _, err := store.writeDB.ExecContext(ctx, `INSERT INTO saved_reports (name, kind, params)
		VALUES ('Main', 'cashflow', '{"period":{"kind":"last_months","value":3}}')`); err != nil {
		t.Fatalf("insert saved report: %v", err)
	}
	if _, err := store.writeDB.ExecContext(ctx, `INSERT INTO saved_reports (name, kind, params)
		VALUES ('main', 'cashflow', '{}')`); err == nil {
		t.Fatal("duplicate saved report name (NOCASE) accepted, want unique violation")
	}
	if _, err := store.writeDB.ExecContext(ctx, `INSERT INTO saved_reports (name, kind, params)
		VALUES ('Broken', 'cashflow', '{')`); err == nil {
		t.Fatal("invalid saved report params JSON accepted, want check violation")
	}

	if _, err := store.writeDB.ExecContext(ctx, `INSERT INTO reconciliations
		(account_id, date, statement_balance_minor, currency)
		VALUES (1, '2026-01-31', 123400, 'EUR')`); err != nil {
		t.Fatalf("insert reconciliation: %v", err)
	}
	if _, err := store.writeDB.ExecContext(ctx, `INSERT INTO reconciliations
		(account_id, date, statement_balance_minor, currency)
		VALUES (1, '2026-01-31', 123400, 'EUR')`); err == nil {
		t.Fatal("duplicate reconciliation (account_id, date) accepted, want unique violation")
	}
	if _, err := store.writeDB.ExecContext(ctx, `INSERT INTO reconciliations
		(account_id, date, statement_balance_minor, currency)
		VALUES (999, '2026-01-31', 123400, 'EUR')`); err == nil {
		t.Fatal("reconciliation with missing account accepted, want foreign key violation")
	}
	if _, err := store.writeDB.ExecContext(ctx, `UPDATE reconciliations
		SET closed_at = '2026-02-01T09:10:11.123456Z'
		WHERE account_id = 1 AND date = '2026-01-31'`); err != nil {
		t.Fatalf("close reconciliation: %v", err)
	}
}
