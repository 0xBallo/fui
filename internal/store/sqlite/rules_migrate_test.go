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

func TestMigrateFromV3AddsRulesTable(t *testing.T) {
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
	if _, err := provider.UpTo(ctx, 3); err != nil {
		t.Fatalf("migrate to v3: %v", err)
	}
	mustExec(t, db, `INSERT INTO accounts
		(name, kind, currency, opening_balance_minor, opened_on)
		VALUES ('Checking', 'checking', 'EUR', 1000, '2026-01-01')`)
	mustExec(t, db, `INSERT INTO transactions (account_id, date, amount_minor, currency, description, status, kind)
		VALUES (1, '2026-01-02', -500, 'EUR', 'Coffee', 'booked', 'payment')`)
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

	var n int
	if err := store.writeDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM accounts").Scan(&n); err != nil {
		t.Fatalf("count accounts: %v", err)
	}
	if n != 1 {
		t.Fatalf("accounts count = %d, want 1", n)
	}

	const validRule = `{"category_id":1}`
	if _, err := store.writeDB.ExecContext(ctx,
		`INSERT INTO rules (name, priority, enabled, conditions, actions) VALUES (?, ?, ?, ?, ?)`,
		"Rule A", 10, 1, `{}`, validRule); err != nil {
		t.Fatalf("insert valid rule: %v", err)
	}
	if _, err := store.writeDB.ExecContext(ctx,
		`INSERT INTO rules (name, priority, enabled, conditions, actions) VALUES (?, ?, ?, ?, ?)`,
		"Rule B", 20, 1, `{"bad_json":`, validRule); err == nil {
		t.Fatal("insert invalid JSON conditions succeeded, want failure")
	}
}
