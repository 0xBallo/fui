package sqlite

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
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

func rulesList(t tb, s *Store) []domain.Rule {
	t.Helper()
	var out []domain.Rule
	err := s.Read(context.Background(), func(r app.Repos) error {
		var err error
		out, err = r.Rules().List(context.Background())
		return err
	})
	if err != nil {
		t.Fatalf("list rules: %v", err)
	}
	return out
}

// seed fills every table a reset must empty, through raw SQL so rapid.T can call it too.
func seed(t tb, s *Store, accounts, salt int) {
	t.Helper()
	ctx := context.Background()
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := s.writeDB.ExecContext(ctx, q, args...); err != nil {
			t.Fatalf("seed %q: %v", q, err)
		}
	}
	for i := 1; i <= accounts; i++ {
		name := fmt.Sprintf("Acc%d_%d", salt, i)
		exec(`INSERT INTO accounts (name, kind, currency, opening_balance_minor, opened_on) VALUES (?, 'checking', 'EUR', 0, '2026-01-01')`, name)
		exec(`INSERT INTO transactions (account_id, date, amount_minor, currency, status, kind)
			VALUES ((SELECT id FROM accounts WHERE name = ?), '2026-01-05', -500, 'EUR', 'booked', 'payment')`, name)
		exec(`INSERT INTO connector_account_refs (connector, ref, account_id)
			VALUES ('c', ?, (SELECT id FROM accounts WHERE name = ?))`, fmt.Sprintf("r%d_%d", salt, i), name)
	}
	exec(`INSERT OR IGNORE INTO fx_rates (currency, date, rate_scaled) VALUES ('USD', '2026-01-01', 1000000)`)
	exec(`INSERT INTO import_batches (connector, source, source_sha256, imported_at) VALUES ('c', 's', 'x', '2026-01-01T00:00:00Z')`)
	exec(`INSERT INTO transfer_groups (fee_minor) VALUES (0)`)
	exec(`INSERT OR IGNORE INTO tags (name) VALUES ('trip')`)
	exec(`INSERT OR IGNORE INTO payees (name) VALUES ('Esselunga')`)
	exec(`INSERT OR IGNORE INTO payee_aliases (payee_id, alias) SELECT id, 'ESSELUNGA SPA' FROM payees WHERE name = 'Esselunga'`)
	exec(`INSERT INTO transaction_tags (transaction_id, tag_id) SELECT t.id, g.id FROM transactions t, tags g WHERE g.name = 'trip' LIMIT 1`)
	exec(`INSERT INTO transaction_splits (transaction_id, category_id, amount_minor) SELECT t.id, c.id, -500 FROM transactions t, categories c LIMIT 1`)
	exec(`INSERT INTO rules (name, priority, enabled, conditions, actions)
		VALUES (?, 10, 1, '{}', '{"category_id":1}')`, fmt.Sprintf("Rule%d", salt))
	exec(`INSERT INTO saved_reports (name, kind, params)
		VALUES (?, 'cashflow', '{"period":{"kind":"last_months","value":3}}')`, fmt.Sprintf("Report%d", salt))
	exec(`INSERT INTO saved_simulations (name, kind, params)
		VALUES (?, 'investment', '{"years":20}')`, fmt.Sprintf("Simulation%d", salt))
	exec(`INSERT INTO dismissed_insights (key, kind, dismissed_at)
		VALUES (?, 'subscription', '2026-01-01T00:00:00Z')`, fmt.Sprintf("insight-%d", salt))
	if accounts > 0 {
		exec(`INSERT INTO recurrences (
			account_id, category_id, description, amount_minor, currency, every, unit, day_of_month, start_date, next_date
		) VALUES (
			1,
			(SELECT id FROM categories ORDER BY id LIMIT 1),
			'Rent',
			-50000,
			'EUR',
			1,
			'months',
			0,
			'2026-01-01',
			'2026-02-01'
		)`)
		exec(`UPDATE transactions
			SET recurrence_id = (SELECT id FROM recurrences LIMIT 1), due_date = '2026-01-05'
			WHERE id = (SELECT id FROM transactions ORDER BY id LIMIT 1)`)
		exec(`INSERT INTO loans (kind, principal_minor, currency, disbursed_on, term_months, rate_type, debit_account_id)
			VALUES ('loan', 100000, 'EUR', '2026-01-01', 24, 'fixed',
				(SELECT id FROM accounts WHERE name = ? LIMIT 1))`, fmt.Sprintf("Acc%d_%d", salt, 1))
		exec(`INSERT INTO loan_schedules (loan_id, valid_from, origin, rate_basis_points)
			VALUES ((SELECT id FROM loans ORDER BY id DESC LIMIT 1), '2026-01-01', 'computed', 350)`)
		exec(`INSERT INTO loan_installments (schedule_id, due_date, principal_minor, interest_minor, fees_minor, remaining_minor, transaction_id)
			VALUES ((SELECT id FROM loan_schedules ORDER BY id DESC LIMIT 1), '2026-02-01', 1000, 100, 0, 99000,
				(SELECT id FROM transactions ORDER BY id LIMIT 1))`)
		exec(`INSERT INTO loan_events (loan_id, kind, event_date, new_rate_basis_points)
			VALUES ((SELECT id FROM loans ORDER BY id DESC LIMIT 1), 'rate_change', '2026-02-15', 320)`)
		exec(`INSERT INTO reconciliations (account_id, date, statement_balance_minor, currency, closed_at)
			VALUES (1, '2026-01-31', 100000, 'EUR', '2026-02-01T00:00:00Z')`)
	}
}

func settingsDump(t tb, s *Store) []string {
	t.Helper()
	var out []string
	err := s.Read(context.Background(), func(r app.Repos) error {
		tags, err := r.Tags().List(context.Background())
		if err != nil {
			return err
		}
		for _, tag := range tags {
			out = append(out, "tag:"+tag.Name)
		}
		payees, err := r.Payees().List(context.Background())
		if err != nil {
			return err
		}
		for _, p := range payees {
			out = append(out, "payee:"+p.Name+"="+strings.Join(p.Aliases, "|"))
		}
		reports, err := r.SavedReports().List(context.Background())
		if err != nil {
			return err
		}
		for _, rep := range reports {
			out = append(out, "saved_report:"+rep.Name+"="+rep.Kind+"="+rep.ParamsJSON)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("list settings: %v", err)
	}
	return out
}

func assertEmptyExceptCategories(t tb, s *Store) {
	t.Helper()
	for _, table := range []string{
		"accounts", "transactions", "transfer_groups", "import_batches", "fx_rates",
		"connector_account_refs", "transaction_tags", "transaction_splits", "rules",
		"reconciliations",
		"loans", "loan_schedules", "loan_installments", "loan_events",
		"recurrences",
		"saved_simulations", "dismissed_insights",
	} {
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
	seed(t, s, 2, 0)

	if err := s.Reset(ctx, app.ResetOptions{}); err != nil {
		t.Fatalf("Reset: %v", err)
	}

	assertEmptyExceptCategories(t, s)
	for _, table := range []string{"tags", "payees", "payee_aliases", "saved_reports"} {
		if n := count(t, s, table); n != 0 {
			t.Errorf("table %s has %d rows after full reset, want 0", table, n)
		}
	}
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
	seed(t, s, 1, 0)
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

func TestReopenAfterReturnsOriginalCauseWhenReopenSucceeds(t *testing.T) {
	s, _ := newTestStore(t)
	cause := errors.New("cause")
	err := s.reopenAfter(cause)
	if !errors.Is(err, cause) {
		t.Fatalf("reopenAfter error = %v, want cause", err)
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
		seed(rt, s, rapid.IntRange(0, 4).Draw(rt, "accounts"), iter)
		before := categoryNames(rt, s)
		beforeSettings := settingsDump(rt, s)
		beforeRules := rulesList(rt, s)

		if err := s.Reset(ctx, app.ResetOptions{KeepSettings: true}); err != nil {
			rt.Fatalf("Reset: %v", err)
		}

		for _, table := range []string{
			"accounts", "transactions", "transfer_groups", "import_batches", "fx_rates", "connector_account_refs",
			"reconciliations",
			"loans", "loan_schedules", "loan_installments", "loan_events",
			"recurrences",
			"saved_simulations", "dismissed_insights",
		} {
			if n := count(rt, s, table); n != 0 {
				rt.Fatalf("table %s has %d rows after reset, want 0", table, n)
			}
		}
		if diff := cmp.Diff(before, categoryNames(rt, s)); diff != "" {
			rt.Fatalf("categories changed (-before +after):\n%s", diff)
		}
		if diff := cmp.Diff(beforeSettings, settingsDump(rt, s)); diff != "" {
			rt.Fatalf("tags/payees changed (-before +after):\n%s", diff)
		}
		if diff := cmp.Diff(beforeRules, rulesList(rt, s)); diff != "" {
			rt.Fatalf("rules changed (-before +after):\n%s", diff)
		}
	})
}
