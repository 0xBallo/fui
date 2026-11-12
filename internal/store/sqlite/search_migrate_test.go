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

// TestMigrateFromV2BackfillsSearchText also proves FTS5 with the trigram tokenizer is built into
// the modernc.org/sqlite version in go.mod (the virtual table is created by the migration).
func TestMigrateFromV2BackfillsSearchText(t *testing.T) {
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
	if _, err := provider.UpTo(ctx, 2); err != nil {
		t.Fatalf("migrate to v2: %v", err)
	}
	mustExec(t, db, `INSERT INTO accounts (name, kind, currency, opening_balance_minor, opened_on)
		VALUES ('Checking', 'checking', 'EUR', 0, '2026-01-01')`)
	mustExec(t, db, `INSERT INTO transactions (account_id, date, amount_minor, currency, description, notes, status, kind)
		VALUES (1, '2026-01-02', -500, 'EUR', 'Caffè  ÉCLAIR', 'Prima Colazione', 'booked', 'payment'),
		       (1, '2026-01-03', -700, 'EUR', '', '', 'booked', 'payment')`)
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

	var got string
	if err := store.writeDB.QueryRowContext(ctx, "SELECT search_text FROM transactions WHERE id = 1").Scan(&got); err != nil {
		t.Fatalf("scan search_text: %v", err)
	}
	if want := "caffe eclair prima colazione"; got != want {
		t.Fatalf("search_text = %q, want %q", got, want)
	}
	var id int64
	if err := store.writeDB.QueryRowContext(ctx,
		`SELECT rowid FROM transactions_fts WHERE transactions_fts MATCH '"clair pri"'`).Scan(&id); err != nil {
		t.Fatalf("fts match: %v", err)
	}
	if id != 1 {
		t.Fatalf("fts rowid = %d, want 1", id)
	}
}
