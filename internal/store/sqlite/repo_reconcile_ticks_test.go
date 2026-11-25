package sqlite

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/pressly/goose/v3"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/clock"
	"github.com/0xballo/fui/internal/domain"
)

func TestMigrateFromV16AddsReconcileTicks(t *testing.T) {
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
	if _, err := provider.UpTo(ctx, 16); err != nil {
		t.Fatalf("migrate to v16: %v", err)
	}
	mustExec(t, db, `INSERT INTO accounts (name, kind, currency, opening_balance_minor, opened_on)
		VALUES ('Checking', 'checking', 'EUR', 1000, '2026-01-01'), ('Savings', 'savings', 'EUR', 0, '2026-01-01')`)
	mustExec(t, db, `INSERT INTO transactions (account_id, date, amount_minor, currency, description, status, kind)
		VALUES (1, '2026-01-02', -500, 'EUR', 'Coffee', 'booked', 'payment'),
		       (1, '2026-01-03', -200, 'EUR', 'Lunch', 'booked', 'payment'),
		       (2, '2026-01-03', 100, 'EUR', 'Gift', 'booked', 'payment')`)
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
	if err != nil || version != currentSchemaVersion {
		t.Fatalf("user_version = %d, %v; want %d", version, err, currentSchemaVersion)
	}

	if err := store.WithTx(ctx, func(r app.Repos) error {
		rr := r.Reconciliations()
		if got, err := rr.Ticks(ctx, 1); err != nil || len(got) != 0 {
			t.Fatalf("Ticks on upgraded archive = %v, %v", got, err)
		}
		if err := rr.SetTicks(ctx, 1, []domain.TransactionID{2, 1, 1}, true); err != nil {
			t.Fatalf("SetTicks add: %v", err)
		}
		if err := rr.SetTicks(ctx, 1, []domain.TransactionID{1}, true); err != nil {
			t.Fatalf("SetTicks idempotent add: %v", err)
		}
		if got, err := rr.Ticks(ctx, 1); err != nil || !slices.Equal(got, []domain.TransactionID{1, 2}) {
			t.Fatalf("Ticks = %v, %v; want [1 2]", got, err)
		}
		if got, err := rr.Ticks(ctx, 2); err != nil || len(got) != 0 {
			t.Fatalf("Ticks other account = %v, %v", got, err)
		}
		if err := rr.SetTicks(ctx, 1, []domain.TransactionID{3}, true); !errors.Is(err, domain.ErrInvalidValue) {
			t.Fatalf("SetTicks foreign tx err = %v, want invalid_value", err)
		}
		if err := rr.SetTicks(ctx, 1, []domain.TransactionID{99}, true); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("SetTicks unknown tx err = %v, want not_found", err)
		}
		if err := rr.SetTicks(ctx, 99, []domain.TransactionID{1}, true); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("SetTicks unknown account err = %v, want not_found", err)
		}
		if err := rr.SetTicks(ctx, 0, nil, true); !errors.Is(err, domain.ErrMissingField) {
			t.Fatalf("SetTicks zero account err = %v, want missing_field", err)
		}
		if err := rr.SetTicks(ctx, 1, []domain.TransactionID{1, 1}, false); err != nil {
			t.Fatalf("SetTicks remove: %v", err)
		}
		if err := rr.SetTicks(ctx, 1, []domain.TransactionID{1}, false); err != nil {
			t.Fatalf("SetTicks idempotent remove: %v", err)
		}
		if got, err := rr.Ticks(ctx, 1); err != nil || !slices.Equal(got, []domain.TransactionID{2}) {
			t.Fatalf("Ticks after remove = %v, %v; want [2]", got, err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := store.writeDB.ExecContext(ctx, `DELETE FROM transactions WHERE id = 2`); err != nil {
		t.Fatalf("delete tx: %v", err)
	}
	var n int
	if err := store.writeDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM reconcile_ticks`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("ticks after tx delete = %d, %v; want cascade to 0", n, err)
	}
}

func TestReconcileTicksReadOnly(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	err := store.Read(ctx, func(r app.Repos) error {
		return r.Reconciliations().SetTicks(ctx, 1, nil, true)
	})
	if err == nil {
		t.Fatal("SetTicks in read-only tx succeeded")
	}
}
