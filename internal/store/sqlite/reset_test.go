package sqlite

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"

	"github.com/google/go-cmp/cmp"
	"pgregory.net/rapid"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/backup"
	"github.com/0xballo/fui/internal/domain"
)

// tb is the subset of testing.TB that *rapid.T also implements.
type tb interface {
	Helper()
	Errorf(format string, args ...any)
	Fatalf(format string, args ...any)
}

func count(t tb, s *Store, table string) int {
	t.Helper()
	var n int
	if err := s.readDB.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}

func categoryNames(t tb, s *Store) []string {
	t.Helper()
	var out []string
	err := s.Read(context.Background(), func(r app.Repos) error {
		cats, err := r.Categories().List(context.Background())
		for _, c := range cats {
			out = append(out, string(c.Kind)+":"+c.Name)
		}
		return err
	})
	if err != nil {
		t.Fatalf("list categories: %v", err)
	}
	slices.Sort(out)
	return out
}

// seed fills every table a reset must empty, through raw SQL so rapid.T can call it too.
func seed(t tb, s *Store, accounts int) {
	t.Helper()
	ctx := context.Background()
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := s.writeDB.ExecContext(ctx, q, args...); err != nil {
			t.Fatalf("seed %q: %v", q, err)
		}
	}
	for i := 1; i <= accounts; i++ {
		exec(`INSERT INTO accounts (id, name, kind, currency, opening_balance_minor, opened_on) VALUES (?, ?, 'checking', 'EUR', 0, '2026-01-01')`, i, "Acc"+strconv.Itoa(i))
		exec(`INSERT INTO transactions (account_id, date, amount_minor, currency, status, kind) VALUES (?, '2026-01-05', -500, 'EUR', 'booked', 'payment')`, i)
		exec(`INSERT INTO connector_account_refs (connector, ref, account_id) VALUES ('c', ?, ?)`, "r"+strconv.Itoa(i), i)
	}
	exec(`INSERT INTO fx_rates (currency, date, rate_scaled) VALUES ('USD', '2026-01-01', 1000000)`)
	exec(`INSERT INTO import_batches (connector, source, source_sha256, imported_at) VALUES ('c', 's', 'x', '2026-01-01T00:00:00Z')`)
	exec(`INSERT INTO transfer_groups (fee_minor) VALUES (0)`)
}

func assertEmptyExceptCategories(t tb, s *Store) {
	t.Helper()
	for _, table := range []string{"accounts", "transactions", "transfer_groups", "import_batches", "fx_rates", "connector_account_refs"} {
		if n := count(t, s, table); n != 0 {
			t.Errorf("table %s has %d rows after reset, want 0", table, n)
		}
	}
}

func TestResetFull(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()
	if err := s.WithTx(ctx, func(r app.Repos) error {
		_, err := r.Categories().Insert(ctx, domain.Category{Name: "Custom", Kind: domain.CategoryExpense})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	seed(t, s, 2)

	if err := s.Reset(ctx, app.ResetOptions{}); err != nil {
		t.Fatalf("Reset: %v", err)
	}

	assertEmptyExceptCategories(t, s)
	var want []string
	for _, c := range domain.DefaultCategories() {
		want = append(want, string(c.Kind)+":"+c.Name)
	}
	slices.Sort(want)
	if diff := cmp.Diff(want, categoryNames(t, s)); diff != "" {
		t.Errorf("categories mismatch (-want +got):\n%s", diff)
	}

	files, err := backup.List(s.backupDir)
	if err != nil || len(files) != 1 || files[len(files)-1].Reason != backup.ReasonReset {
		t.Fatalf("backups = %+v, err %v; want one reset backup", files, err)
	}
	// The store stays usable after the swap.
	insertAccountForTest(t, s, "After")
}

func TestResetCrashSafety(t *testing.T) {
	s, _ := newTestStore(t)
	seed(t, s, 1)
	// A regular file where the backup dir should be makes the backup step fail, after the fresh
	// file was built and before the rename.
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	s.backupDir = filepath.Join(blocker, "backups")

	if err := s.Reset(context.Background(), app.ResetOptions{}); err == nil {
		t.Fatal("Reset succeeded, want backup failure")
	}

	if n := count(t, s, "accounts"); n != 1 {
		t.Errorf("accounts = %d, want 1 (old archive intact)", n)
	}
	if n := count(t, s, "transactions"); n != 1 {
		t.Errorf("transactions = %d, want 1", n)
	}
	left, _ := filepath.Glob(filepath.Join(filepath.Dir(s.path), ".fui-reset-*"))
	if len(left) != 0 {
		t.Errorf("temp files left behind: %v", left)
	}
}

// TestResetKeepSettingsProperty: whatever categories and data exist, --keep-settings keeps the
// categories exactly and empties every other table.
func TestResetKeepSettingsProperty(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()
	iter := 0
	rapid.Check(t, func(rt *rapid.T) {
		iter++
		n := rapid.IntRange(0, 5).Draw(rt, "categories")
		for i := range n {
			kind := rapid.SampledFrom([]domain.CategoryKind{domain.CategoryIncome, domain.CategoryExpense}).Draw(rt, "kind")
			name := "Cat" + strconv.Itoa(iter) + "_" + strconv.Itoa(i)
			if err := s.WithTx(ctx, func(r app.Repos) error {
				_, err := r.Categories().Insert(ctx, domain.Category{Name: name, Kind: kind})
				return err
			}); err != nil {
				rt.Fatalf("insert category: %v", err)
			}
		}
		seed(rt, s, rapid.IntRange(0, 4).Draw(rt, "accounts"))
		before := categoryNames(rt, s)

		if err := s.Reset(ctx, app.ResetOptions{KeepSettings: true}); err != nil {
			rt.Fatalf("Reset: %v", err)
		}

		assertEmptyExceptCategories(rt, s)
		if diff := cmp.Diff(before, categoryNames(rt, s)); diff != "" {
			rt.Fatalf("categories changed (-before +after):\n%s", diff)
		}
	})
}
