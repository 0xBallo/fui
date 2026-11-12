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

func TestMigrateFromV5AddsLoanTables(t *testing.T) {
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
	if _, err := provider.UpTo(ctx, 5); err != nil {
		t.Fatalf("migrate to v5: %v", err)
	}
	mustExec(t, db, `INSERT INTO accounts (name, kind, currency, opening_balance_minor, opened_on)
		VALUES ('Checking', 'checking', 'EUR', 0, '2026-01-01')`)
	mustExec(t, db, `INSERT INTO transactions (account_id, date, amount_minor, currency, description, status, kind)
		VALUES (1, '2026-01-10', -1200, 'EUR', 'before v8', 'booked', 'payment')`)
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

	// The newly added schema must accept rows and enforce its key constraints.
	if _, err := store.writeDB.ExecContext(ctx, `INSERT INTO loans
		(kind, principal_minor, currency, disbursed_on, term_months, rate_type, debit_account_id)
		VALUES ('loan', 100000, 'EUR', '2026-01-01', 24, 'fixed', 1)`); err != nil {
		t.Fatalf("insert loan: %v", err)
	}
	if _, err := store.writeDB.ExecContext(ctx, `INSERT INTO loan_schedules
		(loan_id, valid_from, origin, rate_basis_points)
		VALUES (1, '2026-01-01', 'computed', 350)`); err != nil {
		t.Fatalf("insert loan schedule: %v", err)
	}
	if _, err := store.writeDB.ExecContext(ctx, `INSERT INTO loan_schedules
		(loan_id, valid_from, origin, rate_basis_points)
		VALUES (1, '2026-01-01', 'computed', 351)`); err == nil {
		t.Fatal("duplicate loan schedule valid_from accepted, want unique violation")
	}
	if _, err := store.writeDB.ExecContext(ctx, `INSERT INTO loan_installments
		(schedule_id, due_date, principal_minor, interest_minor, fees_minor, remaining_minor, transaction_id)
		VALUES (1, '2026-02-01', 1000, 100, 0, 99000, 1)`); err != nil {
		t.Fatalf("insert loan installment: %v", err)
	}
	if _, err := store.writeDB.ExecContext(ctx, `INSERT INTO loan_events
		(loan_id, kind, event_date, new_rate_basis_points)
		VALUES (1, 'rate_change', '2026-02-10', 300)`); err != nil {
		t.Fatalf("insert loan event: %v", err)
	}

	var linkedTx sql.NullInt64
	if err := store.writeDB.QueryRowContext(ctx, "SELECT transaction_id FROM loan_installments WHERE id = 1").Scan(&linkedTx); err != nil {
		t.Fatalf("scan linked installment: %v", err)
	}
	if !linkedTx.Valid || linkedTx.Int64 != 1 {
		t.Fatalf("loan_installments.transaction_id = %v, want 1", linkedTx)
	}
}

func TestMigrateFromV10BackfillsLoanIdentity(t *testing.T) {
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
	if _, err := provider.UpTo(ctx, 10); err != nil {
		t.Fatalf("migrate to v10: %v", err)
	}
	mustExec(t, db, `INSERT INTO accounts (name, kind, currency, opening_balance_minor, opened_on)
		VALUES ('Checking', 'checking', 'EUR', 0, '2026-01-01')`)
	mustExec(t, db, `INSERT INTO loans
		(kind, principal_minor, currency, disbursed_on, term_months, rate_type, debit_account_id)
		VALUES ('mortgage', 100000, 'EUR', '2026-01-01', 24, 'fixed', 1),
		       ('loan', 5000, 'EUR', '2026-02-01', 6, 'fixed', 1)`)
	mustClose(t, db)

	store, err := Open(ctx, Options{
		Path: dbPath, BackupDir: filepath.Join(root, "data", "backups"), BackupKeep: 10,
		Clock: clock.NewFixed(time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC)),
	})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	rows, err := store.writeDB.QueryContext(ctx, `SELECT l.name, a.name, a.kind, a.opening_balance_minor, a.opened_on
		FROM loans AS l JOIN accounts AS a ON a.id = l.liability_account_id ORDER BY l.id`)
	if err != nil {
		t.Fatalf("query loans: %v", err)
	}
	defer func() { _ = rows.Close() }()
	want := [][5]any{
		{"Loan 1", "Loan 1", "loan", int64(-100000), "2026-01-01"},
		{"Loan 2", "Loan 2", "loan", int64(-5000), "2026-02-01"},
	}
	for i := 0; rows.Next(); i++ {
		var got [5]any
		var n1, n2, kind, opened string
		var bal int64
		if err := rows.Scan(&n1, &n2, &kind, &bal, &opened); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got = [5]any{n1, n2, kind, bal, opened}
		if i >= len(want) || got != want[i] {
			t.Fatalf("loan %d = %v, want %v", i, got, want)
		}
	}
	if _, err := store.writeDB.ExecContext(ctx, `UPDATE loans SET name = 'loan 1' WHERE id = 2`); err == nil {
		t.Fatal("duplicate loan name accepted, want unique violation")
	}
	if _, err := store.writeDB.ExecContext(ctx, `UPDATE loans SET liability_account_id = 2 WHERE id = 2`); err == nil {
		t.Fatal("liability account shared by two loans, want unique violation")
	}
}
