package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/pressly/goose/v3"

	"github.com/0xballo/fui/internal/backup"
	"github.com/0xballo/fui/internal/clock"
)

func TestOpenMigratesEmptyDatabase(t *testing.T) {
	store, _ := newMigrateTestStore(t)

	version, err := pragmaUserVersion(context.Background(), store.writeDB)
	if err != nil {
		t.Fatalf("pragmaUserVersion: %v", err)
	}
	if version != currentSchemaVersion {
		t.Fatalf("user_version = %d, want %d", version, currentSchemaVersion)
	}
}

func TestSchemaGolden(t *testing.T) {
	store, _ := newMigrateTestStore(t)

	got, err := collectSchemaDump(context.Background(), store.writeDB)
	if err != nil {
		t.Fatalf("collectSchemaDump: %v", err)
	}
	wantBytes, err := os.ReadFile(filepath.Join("testdata", "schema.golden"))
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	want := string(wantBytes)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("schema dump mismatch (-want +got):\n%s", diff)
	}
}

func TestMigrateFromV1AddsIBANColumn(t *testing.T) {
	root := t.TempDir()
	dbPath := filepath.Join(root, "data", "fui.db")
	backupDir := filepath.Join(root, "data", "backups")
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
	if _, err := provider.UpTo(ctx, 1); err != nil {
		t.Fatalf("migrate to v1: %v", err)
	}
	mustExec(t, db, `INSERT INTO accounts
		(name, kind, currency, opening_balance_minor, opened_on)
		VALUES ('Checking', 'checking', 'EUR', 1000, '2026-01-01')`)
	mustClose(t, db)

	clk := clock.NewFixed(time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC))
	store, err := Open(ctx, Options{
		Path:       dbPath,
		BackupDir:  backupDir,
		BackupKeep: 10,
		Clock:      clk,
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

	var name string
	var iban sql.NullString
	row := store.writeDB.QueryRowContext(ctx, "SELECT name, iban FROM accounts WHERE name = 'Checking'")
	if err := row.Scan(&name, &iban); err != nil {
		t.Fatalf("scan migrated account: %v", err)
	}
	if name != "Checking" || iban.Valid {
		t.Fatalf("migrated account = (%q, %v), want (%q, invalid)", name, iban, "Checking")
	}
}

func TestOpenRejectsNewerDatabase(t *testing.T) {
	root := t.TempDir()
	dbPath := filepath.Join(root, "data", "fui.db")
	backupDir := filepath.Join(root, "data", "backups")
	clk := clock.NewFixed(time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC))

	db := mustOpenRawDB(t, dbPath)
	t.Cleanup(func() { _ = db.Close() })
	mustExec(t, db, "PRAGMA user_version = 999")
	mustClose(t, db)

	_, err := Open(context.Background(), Options{
		Path:       dbPath,
		BackupDir:  backupDir,
		BackupKeep: 10,
		Clock:      clk,
	})
	if !errors.Is(err, errDBNewerThanBinary) {
		t.Fatalf("Open error = %v, want db_newer_than_binary", err)
	}
}

func TestOpenCreatesMigrateBackupForExistingDatabase(t *testing.T) {
	root := t.TempDir()
	dbPath := filepath.Join(root, "data", "fui.db")
	backupDir := filepath.Join(root, "data", "backups")
	clk := clock.NewFixed(time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC))

	db := mustOpenRawDB(t, dbPath)
	mustExec(t, db, "CREATE TABLE legacy (id INTEGER PRIMARY KEY, name TEXT)")
	mustExec(t, db, "INSERT INTO legacy (name) VALUES ('x')")
	mustClose(t, db)

	store, err := Open(context.Background(), Options{
		Path:       dbPath,
		BackupDir:  backupDir,
		BackupKeep: 10,
		Clock:      clk,
	})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	files, err := backup.List(backupDir)
	if err != nil {
		t.Fatalf("backup.List: %v", err)
	}
	if len(files) != 1 || files[0].Reason != backup.ReasonMigrate {
		t.Fatalf("backups = %+v, want one migrate backup", files)
	}
}

func TestOpenMapsMigrationFailure(t *testing.T) {
	root := t.TempDir()
	dbPath := filepath.Join(root, "data", "fui.db")
	backupDir := filepath.Join(root, "data", "backups")
	clk := clock.NewFixed(time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC))

	db := mustOpenRawDB(t, dbPath)
	mustExec(t, db, "CREATE TABLE accounts (id INTEGER PRIMARY KEY)")
	mustClose(t, db)

	_, err := Open(context.Background(), Options{
		Path:       dbPath,
		BackupDir:  backupDir,
		BackupKeep: 10,
		Clock:      clk,
	})
	if !errors.Is(err, errMigrationFailed) {
		t.Fatalf("Open error = %v, want migration_failed", err)
	}
}

func mustOpenRawDB(t *testing.T, path string) *sql.DB {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	db, err := openDB(path, 1, false)
	if err != nil {
		t.Fatalf("openDB: %v", err)
	}
	return db
}

func mustExec(t *testing.T, db *sql.DB, stmt string) {
	t.Helper()
	if _, err := db.ExecContext(context.Background(), stmt); err != nil {
		t.Fatalf("exec %q: %v", stmt, err)
	}
}

func mustClose(t *testing.T, db *sql.DB) {
	t.Helper()
	if err := db.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}
}

func newMigrateTestStore(t *testing.T) (*Store, *clock.Fixed) {
	t.Helper()

	baseTime := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	clk := clock.NewFixed(baseTime)

	root := t.TempDir()
	store, err := Open(context.Background(), Options{
		Path:         filepath.Join(root, "data", "fui.db"),
		BackupDir:    filepath.Join(root, "data", "backups"),
		BackupKeep:   10,
		ReadPoolSize: 4,
		Clock:        clk,
	})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := store.Close(); closeErr != nil {
			t.Fatalf("Close: %v", closeErr)
		}
	})
	return store, clk
}

// TestMigrateFromV3KeepsTransactions runs 0004 on a populated v3 archive: the transactions table is
// rebuilt (payee FK), so rows, ids and the FTS index must survive and the new constraints must hold.
func TestMigrateFromV3KeepsTransactions(t *testing.T) {
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
	mustExec(t, db, `INSERT INTO accounts (name, kind, currency, opening_balance_minor, opened_on)
		VALUES ('Checking', 'checking', 'EUR', 0, '2026-01-01')`)
	mustExec(t, db, `INSERT INTO transactions (id, account_id, date, amount_minor, currency, payee_id, description, status, kind, search_text)
		VALUES (7, 1, '2026-01-05', -500, 'EUR', 99, 'Coffee beans', 'booked', 'payment', 'coffee beans')`)
	mustClose(t, db)

	store, err := Open(ctx, Options{
		Path: dbPath, BackupDir: filepath.Join(root, "data", "backups"), BackupKeep: 10,
		Clock: clock.NewFixed(time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC)),
	})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	var id, amount int64
	var payee sql.NullInt64
	var searchText string
	if err := store.writeDB.QueryRowContext(ctx,
		"SELECT id, amount_minor, payee_id, search_text FROM transactions").Scan(&id, &amount, &payee, &searchText); err != nil {
		t.Fatalf("scan migrated transaction: %v", err)
	}
	if id != 7 || amount != -500 || payee.Valid || searchText != "coffee beans" {
		t.Errorf("migrated row = (%d, %d, %v, %q), want (7, -500, NULL, %q)", id, amount, payee, searchText, "coffee beans")
	}
	var hits int
	if err := store.writeDB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM transactions WHERE id IN (SELECT rowid FROM transactions_fts WHERE search_text MATCH '"coffee"')`).Scan(&hits); err != nil || hits != 1 {
		t.Errorf("fts hits after migration = %d, err %v; want 1", hits, err)
	}
	if _, err := store.writeDB.ExecContext(ctx, "UPDATE transactions SET payee_id = 99 WHERE id = 7"); err == nil {
		t.Error("payee_id referencing a missing payee was accepted, want foreign key failure")
	}
}

// TestMigrateFromV4AddsRecurrenceColumns migrates a populated v4 archive to 0007: existing
// transactions survive, recurrence columns are added, and FTS triggers keep working.
func TestMigrateFromV4AddsRecurrenceColumns(t *testing.T) {
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
	if _, err := provider.UpTo(ctx, 4); err != nil {
		t.Fatalf("migrate to v4: %v", err)
	}
	mustExec(t, db, `INSERT INTO accounts (name, kind, currency, opening_balance_minor, opened_on)
		VALUES ('Checking', 'checking', 'EUR', 0, '2026-01-01')`)
	mustExec(t, db, `INSERT INTO categories (name, kind) VALUES ('Rent', 'expense')`)
	mustExec(t, db, `INSERT INTO transactions (id, account_id, date, amount_minor, currency, description, status, kind, category_id, search_text)
		VALUES (7, 1, '2026-01-05', -50000, 'EUR', 'Rent', 'pending', 'payment', 1, 'rent')`)
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

	var gotID, gotMinor int64
	var recID sql.NullInt64
	var dueDate sql.NullString
	if err := store.writeDB.QueryRowContext(ctx,
		"SELECT id, amount_minor, recurrence_id, due_date FROM transactions WHERE id = 7").
		Scan(&gotID, &gotMinor, &recID, &dueDate); err != nil {
		t.Fatalf("scan migrated transaction: %v", err)
	}
	if gotID != 7 || gotMinor != -50000 || recID.Valid || dueDate.Valid {
		t.Fatalf("migrated row = (%d, %d, %v, %v), want (7, -50000, NULL, NULL)", gotID, gotMinor, recID, dueDate)
	}

	var hits int
	if err := store.writeDB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM transactions WHERE id IN (SELECT rowid FROM transactions_fts WHERE search_text MATCH '"rent"')`).Scan(&hits); err != nil || hits != 1 {
		t.Fatalf("fts before update = %d, err %v; want 1", hits, err)
	}
	if _, err := store.writeDB.ExecContext(ctx, "UPDATE transactions SET search_text = 'rent monthly' WHERE id = 7"); err != nil {
		t.Fatalf("update search_text: %v", err)
	}
	if err := store.writeDB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM transactions WHERE id IN (SELECT rowid FROM transactions_fts WHERE search_text MATCH '"monthly"')`).Scan(&hits); err != nil || hits != 1 {
		t.Fatalf("fts after update = %d, err %v; want 1", hits, err)
	}

	res, err := store.writeDB.ExecContext(ctx, `INSERT INTO recurrences (
		account_id, category_id, description, amount_minor, currency, every, unit, day_of_month, start_date, next_date
	) VALUES (1, 1, 'Rent', -50000, 'EUR', 1, 'months', 0, '2026-01-01', '2026-02-01')`)
	if err != nil {
		t.Fatalf("insert recurrence: %v", err)
	}
	rid, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("LastInsertId: %v", err)
	}
	if _, err := store.writeDB.ExecContext(ctx,
		"UPDATE transactions SET recurrence_id = ?, due_date = '2026-02-01' WHERE id = 7", rid); err != nil {
		t.Fatalf("set recurrence_id/due_date: %v", err)
	}
	if _, err := store.writeDB.ExecContext(ctx, "DELETE FROM recurrences WHERE id = ?", rid); err != nil {
		t.Fatalf("delete recurrence: %v", err)
	}
	if err := store.writeDB.QueryRowContext(ctx,
		"SELECT recurrence_id, due_date FROM transactions WHERE id = 7").Scan(&recID, &dueDate); err != nil {
		t.Fatalf("scan after recurrence delete: %v", err)
	}
	if recID.Valid || !dueDate.Valid || dueDate.String != "2026-02-01" {
		t.Fatalf("after recurrence delete = (recurrence_id=%v due_date=%v), want (NULL, 2026-02-01)", recID, dueDate)
	}
}
