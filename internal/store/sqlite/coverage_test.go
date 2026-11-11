package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/reporting"
	"github.com/0xballo/fui/internal/domain"
)

func TestOpenValidatesOptions(t *testing.T) {
	_, err := Open(context.Background(), Options{})
	if err == nil {
		t.Fatal("Open should fail on missing options")
	}
	_, err = Open(context.Background(), Options{Path: "x", BackupDir: "y", BackupKeep: 0})
	if err == nil {
		t.Fatal("Open should fail on invalid backup keep")
	}
}

func TestEnsureDBFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data", "fui.db")
	existing, err := ensureDBFile(path)
	if err != nil {
		t.Fatalf("ensureDBFile create: %v", err)
	}
	if existing {
		t.Fatal("first ensureDBFile should report existing=false")
	}
	existing, err = ensureDBFile(path)
	if err != nil {
		t.Fatalf("ensureDBFile existing: %v", err)
	}
	if !existing {
		t.Fatal("second ensureDBFile should report existing=true")
	}
}

func TestMapNotFound(t *testing.T) {
	err := mapNotFound(sql.ErrNoRows, "x")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("mapNotFound = %v, want not_found", err)
	}
}

func TestDomainErrorf(t *testing.T) {
	err := &domainErrorf{
		sentinel: errMigrationFailed,
		format:   "failed %d",
		args:     []any{3},
	}
	if err.Error() != "failed 3" {
		t.Fatalf("Error() = %q", err.Error())
	}
	if !errors.Is(err, errMigrationFailed) {
		t.Fatalf("errors.Is(%v, migration_failed) = false", err)
	}
}

func TestReportingSourceAdditionalBranches(t *testing.T) {
	store, _ := newTestStore(t)
	eur := mustCurrency(t, "EUR")
	usd := mustCurrency(t, "USD")
	gbp := mustCurrency(t, "GBP")
	jpy := mustCurrency(t, "JPY")
	chf := mustCurrency(t, "CHF")
	opened := mustDate(t, "2026-01-01")
	d := mustDate(t, "2026-02-10")

	var category domain.Category
	err := store.WithTx(context.Background(), func(r app.Repos) error {
		var txErr error
		_, txErr = r.Accounts().Insert(context.Background(), domain.Account{
			Name:           "A",
			Kind:           domain.AccountChecking,
			Currency:       eur,
			OpeningBalance: mustMoney(t, 0, eur),
			OpenedOn:       opened,
		})
		if txErr != nil {
			return txErr
		}
		category, txErr = r.Categories().Insert(context.Background(), domain.Category{
			Name: "Food",
			Kind: domain.CategoryExpense,
		})
		if txErr != nil {
			return txErr
		}
		usdRate, _ := domain.NewRate(900_000_000)   // 1 USD = 0.9 EUR
		gbpRate, _ := domain.NewRate(1_200_000_000) // 1 GBP = 1.2 EUR
		fxUSD, _ := domain.NewFXRate(usd, d, usdRate)
		fxGBP, _ := domain.NewFXRate(gbp, d, gbpRate)
		if txErr = r.FXRates().Upsert(context.Background(), fxUSD); txErr != nil {
			return txErr
		}
		return r.FXRates().Upsert(context.Background(), fxGBP)
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	categories, err := store.ListCategories(context.Background(), []domain.CategoryID{category.ID})
	if err != nil {
		t.Fatalf("ListCategories: %v", err)
	}
	if len(categories) != 1 || categories[0].ID != category.ID {
		t.Fatalf("categories = %+v", categories)
	}
	categories, err = store.ListCategories(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListCategories empty: %v", err)
	}
	if len(categories) != 0 {
		t.Fatalf("categories(empty) = %+v, want none", categories)
	}

	rate, err := store.FXRateOnOrBefore(context.Background(), usd, gbp, d)
	if err != nil {
		t.Fatalf("FX USD->GBP: %v", err)
	}
	if rate.Scaled() <= 0 {
		t.Fatalf("rate USD->GBP = %d, want > 0", rate.Scaled())
	}

	inverted, err := store.FXRateOnOrBefore(context.Background(), eur, usd, d)
	if err != nil {
		t.Fatalf("FX EUR->USD: %v", err)
	}
	if inverted.Scaled() <= 0 {
		t.Fatalf("rate EUR->USD = %d, want > 0", inverted.Scaled())
	}

	fromOnly, err := store.FXRateOnOrBefore(context.Background(), usd, eur, d)
	if err != nil {
		t.Fatalf("FX USD->EUR: %v", err)
	}
	if fromOnly.Scaled() != 900_000_000 {
		t.Fatalf("rate USD->EUR = %d, want 900000000", fromOnly.Scaled())
	}

	same, err := store.FXRateOnOrBefore(context.Background(), eur, eur, d)
	if err != nil {
		t.Fatalf("FX EUR->EUR: %v", err)
	}
	if same.Scaled() != domain.RateScale {
		t.Fatalf("rate EUR->EUR = %d, want %d", same.Scaled(), domain.RateScale)
	}

	_, err = store.FXRateOnOrBefore(context.Background(), jpy, chf, d)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("FX JPY->CHF err = %v, want not_found", err)
	}
}

func TestMapSQLErrorNil(t *testing.T) {
	if mapSQLError(nil) != nil {
		t.Fatal("mapSQLError(nil) should return nil")
	}
	plain := errors.New("plain")
	if got := mapSQLError(plain); !errors.Is(got, plain) {
		t.Fatalf("mapSQLError(plain) changed error: %v", got)
	}
}

func TestReportingAdaptersCompileAgainstInterface(t *testing.T) {
	store, _ := newTestStore(t)
	var _ reporting.Source = store
}

func TestBackupAndRestoreValidationBranches(t *testing.T) {
	store, _ := newTestStore(t)

	if _, err := store.Backup(context.Background(), "invalid"); err == nil {
		t.Fatal("Backup with invalid reason should fail")
	}
	if err := store.Restore(context.Background(), ""); err == nil {
		t.Fatal("Restore with empty path should fail")
	}

	candidate := filepath.Join(t.TempDir(), "candidate.db")
	db := mustOpenRawDB(t, candidate)
	mustExec(t, db, "PRAGMA user_version = 999")
	mustClose(t, db)

	err := store.Restore(context.Background(), candidate)
	if !errors.Is(err, errDBNewerThanBinary) {
		t.Fatalf("Restore error = %v, want db_newer_than_binary", err)
	}

	if got := quoteString("a'b"); got != "'a''b'" {
		t.Fatalf("quoteString = %q, want 'a''b'", got)
	}
	if got := escapeSingleQuotes("a'b'c"); got != "a''b''c" {
		t.Fatalf("escapeSingleQuotes = %q, want a''b''c", got)
	}
}

func TestRepoExtraErrorBranches(t *testing.T) {
	store, _ := newTestStore(t)
	eur := mustCurrency(t, "EUR")
	date := mustDate(t, "2026-01-01")
	amount := mustMoney(t, 10, eur)

	err := store.WithTx(context.Background(), func(r app.Repos) error {
		_, txErr := r.Transactions().Insert(context.Background(), domain.Transaction{
			AccountID:   999,
			Date:        date,
			Amount:      amount,
			Description: "x",
			Status:      domain.StatusBooked,
			Kind:        domain.KindPayment,
		})
		return txErr
	})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("insert tx missing account err = %v, want not_found", err)
	}

	err = store.WithTx(context.Background(), func(r app.Repos) error {
		_, txErr := r.Categories().Insert(context.Background(), domain.Category{
			Name: "A", Kind: domain.CategoryExpense,
		})
		if txErr != nil {
			return txErr
		}
		_, txErr = r.Categories().Reassign(context.Background(), 1, 999)
		return txErr
	})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("reassign to missing category err = %v, want not_found", err)
	}

	err = store.WithTx(context.Background(), func(r app.Repos) error {
		return r.Imports().SetAccountRef(context.Background(), "csv", "x", 999)
	})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("set account ref missing account err = %v, want not_found", err)
	}
}

func TestValidateRestoreCandidateCorrupted(t *testing.T) {
	badPath := filepath.Join(t.TempDir(), "bad.db")
	if err := os.WriteFile(badPath, []byte("bad"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	err := validateRestoreCandidate(context.Background(), badPath)
	if err == nil {
		t.Fatal("validateRestoreCandidate on bad file should fail")
	}
}

func TestMapConstraintHelper(t *testing.T) {
	if mapConstraint(nil) != nil {
		t.Fatal("mapConstraint(nil) should return nil")
	}
	if !errors.Is(mapConstraint(app.ErrAlreadyExists), app.ErrAlreadyExists) {
		t.Fatal("mapConstraint should keep already_exists")
	}
	if !errors.Is(mapConstraint(app.ErrInUse), app.ErrInUse) {
		t.Fatal("mapConstraint should keep in_use")
	}
	plain := errors.New("plain")
	if got := mapConstraint(plain); !errors.Is(got, plain) {
		t.Fatalf("mapConstraint(plain) changed error: %v", got)
	}
}

func TestMapSQLErrorConstraintCodes(t *testing.T) {
	db := mustOpenRawDB(t, filepath.Join(t.TempDir(), "constraints.db"))
	defer mustClose(t, db)
	mustExec(t, db, "PRAGMA foreign_keys = ON")
	mustExec(t, db, "CREATE TABLE unique_t (name TEXT UNIQUE)")
	mustExec(t, db, "INSERT INTO unique_t(name) VALUES ('x')")
	_, err := db.ExecContext(context.Background(), "INSERT INTO unique_t(name) VALUES ('x')")
	if !errors.Is(mapSQLError(err), app.ErrAlreadyExists) {
		t.Fatalf("mapSQLError(unique) = %v, want already_exists", mapSQLError(err))
	}

	mustExec(t, db, "CREATE TABLE parent_t (id INTEGER PRIMARY KEY)")
	mustExec(t, db, "CREATE TABLE child_t (pid INTEGER REFERENCES parent_t(id))")
	_, err = db.ExecContext(context.Background(), "INSERT INTO child_t(pid) VALUES (99)")
	if !errors.Is(mapSQLError(err), app.ErrInUse) {
		t.Fatalf("mapSQLError(fk) = %v, want in_use", mapSQLError(err))
	}
}

func TestBackupHelpersErrorBranches(t *testing.T) {
	store, _ := newTestStore(t)
	_, err := store.copyCandidateToTemp(filepath.Join(t.TempDir(), "missing.db"))
	if err == nil {
		t.Fatal("copyCandidateToTemp with missing file should fail")
	}

	store.path = t.TempDir()
	if err := store.reopenLocked(); err == nil {
		t.Fatal("reopenLocked with directory path should fail")
	}
}

func TestCategoryRepoNotFoundBranches(t *testing.T) {
	store, _ := newTestStore(t)
	err := store.WithTx(context.Background(), func(r app.Repos) error {
		return r.Categories().Update(context.Background(), domain.Category{
			ID:   999,
			Name: "missing",
			Kind: domain.CategoryExpense,
		})
	})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("update missing category err = %v, want not_found", err)
	}

	err = store.WithTx(context.Background(), func(r app.Repos) error {
		return r.Categories().Delete(context.Background(), 999)
	})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("delete missing category err = %v, want not_found", err)
	}
}
