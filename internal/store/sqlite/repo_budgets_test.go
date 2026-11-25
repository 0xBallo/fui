package sqlite

import (
	"context"
	"io/fs"
	"path/filepath"
	"testing"
	"time"

	"github.com/pressly/goose/v3"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/clock"
	"github.com/0xballo/fui/internal/domain"
)

// TestMigrateFromV4AddsBudgetTables runs 0006 on a populated v4 archive (0005 belongs to a
// parallel task and does not exist on this branch): the new budgets tables must appear and
// accept rows that satisfy their constraints.
func TestMigrateFromV4AddsBudgetTables(t *testing.T) {
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
	mustExec(t, db, `INSERT INTO categories (id, name, kind) VALUES (1, 'Food', 'expense')`)
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

	mustExec(t, store.writeDB, `INSERT INTO budgets (id, name, mode, amount_minor, currency) VALUES (1, 'Food', 'cap', 10000, 'EUR')`)
	mustExec(t, store.writeDB, `INSERT INTO budget_targets (budget_id, category_id) VALUES (1, 1)`)
	mustExec(t, store.writeDB, `INSERT INTO budget_amounts (budget_id, year, month, amount_minor) VALUES (1, 2026, 1, 5000)`)
	mustExec(t, store.writeDB, `INSERT INTO budget_moves (date, from_budget_id, to_budget_id, amount_minor, currency) VALUES ('2026-01-05', NULL, 1, 100, 'EUR')`)

	if _, err := store.writeDB.ExecContext(ctx, `INSERT INTO budget_targets (budget_id, category_id) VALUES (1, 1)`); err == nil {
		t.Error("a second budget on the same category was accepted, want a unique constraint violation")
	}
	if _, err := store.writeDB.ExecContext(ctx, `INSERT INTO budget_moves (date, amount_minor, currency) VALUES ('2026-01-06', 100, 'EUR')`); err == nil {
		t.Error("a move with both endpoints NULL was accepted, want a CHECK violation")
	}
	if _, err := store.writeDB.ExecContext(ctx, `DELETE FROM budgets WHERE id = 1`); err != nil {
		t.Fatalf("delete budget: %v", err)
	}
	var targets, amounts, moves int
	_ = store.writeDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM budget_targets").Scan(&targets)
	_ = store.writeDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM budget_amounts").Scan(&amounts)
	_ = store.writeDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM budget_moves").Scan(&moves)
	if targets != 0 || amounts != 0 || moves != 0 {
		t.Errorf("deleting a budget left targets=%d amounts=%d moves=%d, want all cascaded to zero", targets, amounts, moves)
	}
}

// TestMigrateBudgetPeriodKeepsBudgets runs the budget-period migration on a populated archive
// (budgets with carry 0/1, non-default thresholds, overrides and moves): every value survives and
// existing budgets become monthly.
func TestMigrateBudgetPeriodKeepsBudgets(t *testing.T) {
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
	if _, err := provider.UpTo(ctx, budgetPeriodMigration-1); err != nil {
		t.Fatalf("migrate to v%d: %v", budgetPeriodMigration-1, err)
	}
	mustExec(t, db, `INSERT INTO categories (id, name, kind) VALUES (1, 'Food', 'expense'), (2, 'Home', 'expense')`)
	mustExec(t, db, `INSERT INTO budgets (id, name, mode, amount_minor, currency, carry, threshold_warn, threshold_over)
		VALUES (1, 'Food', 'cap', 10000, 'EUR', 0, 80, 100), (2, 'Home', 'envelope', 20000, 'EUR', 1, 70, 120)`)
	mustExec(t, db, `INSERT INTO budget_targets (budget_id, category_id) VALUES (1, 1), (2, 2)`)
	mustExec(t, db, `INSERT INTO budget_amounts (budget_id, year, month, amount_minor) VALUES (2, 2026, 1, 5000)`)
	mustExec(t, db, `INSERT INTO budget_moves (date, from_budget_id, to_budget_id, amount_minor, currency)
		VALUES ('2026-01-05', NULL, 2, 100, 'EUR')`)
	mustClose(t, db)

	store, err := Open(ctx, Options{
		Path: dbPath, BackupDir: filepath.Join(root, "data", "backups"), BackupKeep: 10,
		Clock: clock.NewFixed(time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC)),
	})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	var got []domain.Budget
	if err := store.WithTx(ctx, func(r app.Repos) (err error) {
		got, err = r.Budgets().List(ctx)
		return err
	}); err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("List = %d budgets, want 2", len(got))
	}
	food, home := got[0], got[1]
	if food.Period != domain.BudgetMonthly || home.Period != domain.BudgetMonthly {
		t.Errorf("periods = %q, %q; want both monthly", food.Period, home.Period)
	}
	if food.Carry || !home.Carry || food.Amount.Minor() != 10000 || home.Amount.Minor() != 20000 ||
		food.Mode != domain.BudgetCap || home.Mode != domain.BudgetEnvelope {
		t.Errorf("budgets changed by migration: %+v %+v", food, home)
	}
	if home.Thresholds != (domain.BudgetThresholds{Warn: 70, Over: 120}) ||
		food.Thresholds != (domain.BudgetThresholds{Warn: 80, Over: 100}) {
		t.Errorf("thresholds = %+v, %+v; want 80/100 and 70/120", food.Thresholds, home.Thresholds)
	}
	if o := home.Overrides[domain.MonthKey{Year: 2026, Month: 1}]; o.Minor() != 5000 {
		t.Errorf("override = %v, want 5000", o)
	}
	var moves int
	_ = store.writeDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM budget_moves WHERE to_budget_id = 2").Scan(&moves)
	if moves != 1 {
		t.Errorf("moves = %d, want 1", moves)
	}
	if _, err := store.writeDB.ExecContext(ctx, `UPDATE budgets SET period = 'week' WHERE id = 1`); err == nil {
		t.Error("period 'week' accepted, want a CHECK violation")
	}
}

// budgetPeriodMigration is the migration number of the budget period column; change it here if
// the migration is renumbered.
const budgetPeriodMigration = 17

// TestResetWipesBudgets checks that Reset (with or without KeepSettings) never carries budgets,
// their targets, overrides or envelope moves into the fresh archive.
func TestResetWipesBudgets(t *testing.T) {
	for _, keep := range []bool{false, true} {
		t.Run(map[bool]string{false: "full", true: "keep settings"}[keep], func(t *testing.T) {
			store, _ := newTestStore(t)
			ctx := context.Background()

			var catID int64
			if err := store.WithTx(ctx, func(r app.Repos) error {
				cat, err := r.Categories().Insert(ctx, domain.Category{Name: "Food", Kind: domain.CategoryExpense})
				if err != nil {
					return err
				}
				catID = int64(cat.ID)
				return nil
			}); err != nil {
				t.Fatalf("insert category: %v", err)
			}

			mustExec(t, store.writeDB, `INSERT INTO budgets (id, name, mode, amount_minor, currency) VALUES (1, 'Food', 'cap', 10000, 'EUR')`)
			if _, err := store.writeDB.ExecContext(ctx, `INSERT INTO budget_targets (budget_id, category_id) VALUES (1, ?)`, catID); err != nil {
				t.Fatalf("insert target: %v", err)
			}
			mustExec(t, store.writeDB, `INSERT INTO budget_moves (date, from_budget_id, to_budget_id, amount_minor, currency) VALUES ('2026-01-05', NULL, 1, 100, 'EUR')`)

			if err := store.Reset(ctx, app.ResetOptions{KeepSettings: keep}); err != nil {
				t.Fatalf("Reset: %v", err)
			}

			var budgets, targets, moves int
			if err := store.writeDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM budgets").Scan(&budgets); err != nil {
				t.Fatalf("count budgets: %v", err)
			}
			if err := store.writeDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM budget_targets").Scan(&targets); err != nil {
				t.Fatalf("count budget_targets: %v", err)
			}
			if err := store.writeDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM budget_moves").Scan(&moves); err != nil {
				t.Fatalf("count budget_moves: %v", err)
			}
			if budgets != 0 || targets != 0 || moves != 0 {
				t.Errorf("Reset(KeepSettings=%v) left budgets=%d targets=%d moves=%d, want all zero", keep, budgets, targets, moves)
			}
		})
	}
}
