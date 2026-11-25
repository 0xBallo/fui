package sqlite

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

func TestRemapRule(t *testing.T) {
	m := newIDRemap()
	m.categories[1] = 10
	m.payees[2] = 20
	m.tags[3] = 30

	tests := []struct {
		name     string
		in       domain.RuleActions
		accounts []domain.AccountID
		want     domain.RuleActions
		warns    int
		ok       bool
	}{
		{
			"all hit",
			domain.RuleActions{CategoryID: 1, PayeeID: 2, TagIDs: []domain.TagID{3}},
			nil,
			domain.RuleActions{CategoryID: 10, PayeeID: 20, TagIDs: []domain.TagID{30}},
			0, true,
		},
		{
			"category miss",
			domain.RuleActions{CategoryID: 9, PayeeID: 2},
			nil,
			domain.RuleActions{PayeeID: 20},
			1, true,
		},
		{
			"payee miss",
			domain.RuleActions{CategoryID: 1, PayeeID: 9},
			nil,
			domain.RuleActions{CategoryID: 10},
			1, true,
		},
		{
			"tag miss",
			domain.RuleActions{CategoryID: 1, TagIDs: []domain.TagID{3, 9}},
			nil,
			domain.RuleActions{CategoryID: 10, TagIDs: []domain.TagID{30}},
			1, true,
		},
		{
			"account always dropped",
			domain.RuleActions{CategoryID: 1},
			[]domain.AccountID{1, 2},
			domain.RuleActions{CategoryID: 10},
			2, true,
		},
		{
			"nothing left",
			domain.RuleActions{CategoryID: 9},
			nil,
			domain.RuleActions{},
			2, false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rule := domain.Rule{Name: "R", Actions: tc.in, Conditions: domain.RuleConditions{AccountIDs: tc.accounts}}
			got, warns, ok := m.remapRule(rule)
			if ok != tc.ok || len(warns) != tc.warns {
				t.Fatalf("ok=%v warns=%v, want ok=%v with %d warnings", ok, warns, tc.ok, tc.warns)
			}
			if diff := cmp.Diff(tc.want, got.Actions); diff != "" {
				t.Errorf("actions (-want +got):\n%s", diff)
			}
			if len(got.Conditions.AccountIDs) != 0 {
				t.Errorf("account ids kept: %v", got.Conditions.AccountIDs)
			}
		})
	}
}

// TestResetKeepSettingsRemapsRulesAndRelinksRefs: ids change when the settings are re-inserted,
// rules follow them, and a kept connector ref binds to the account of the same name at next use.
func TestResetKeepSettingsRemapsRulesAndRelinksRefs(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := s.writeDB.ExecContext(ctx, q, args...); err != nil {
			t.Fatalf("%q: %v", q, err)
		}
	}
	// The deleted first category makes the kept ones get lower ids after the reset.
	exec(`INSERT INTO categories (name, kind) VALUES ('Old', 'expense'), ('Food', 'expense')`)
	exec(`DELETE FROM categories WHERE name = 'Old'`)
	exec(`INSERT INTO payees (name) VALUES ('Old'), ('Coop')`)
	exec(`DELETE FROM payees WHERE name = 'Old'`)
	exec(`INSERT INTO accounts (name, kind, currency, opening_balance_minor, opened_on)
		VALUES ('Main', 'checking', 'EUR', 0, '2026-01-01')`)
	exec(`INSERT INTO connector_account_refs (connector, ref, account_id) VALUES ('c', 'IT01', 1)`)
	var foodID, coopID int64
	if err := s.readDB.QueryRowContext(ctx, `SELECT id FROM categories WHERE name = 'Food'`).Scan(&foodID); err != nil {
		t.Fatal(err)
	}
	if err := s.readDB.QueryRowContext(ctx, `SELECT id FROM payees WHERE name = 'Coop'`).Scan(&coopID); err != nil {
		t.Fatal(err)
	}
	wantErr := s.WithTx(ctx, func(r app.Repos) error {
		_, err := r.Rules().Insert(ctx, domain.Rule{
			Name: "R", Enabled: true,
			Conditions: domain.RuleConditions{AccountIDs: []domain.AccountID{1}},
			Actions:    domain.RuleActions{CategoryID: domain.CategoryID(foodID), PayeeID: domain.PayeeID(coopID)},
		})
		return err
	})
	if wantErr != nil {
		t.Fatalf("insert rule: %v", wantErr)
	}

	var warnings []string
	opts := app.ResetOptions{KeepSettings: true, OnWarning: func(w string) { warnings = append(warnings, w) }}
	if err := s.Reset(ctx, opts); err != nil {
		t.Fatalf("Reset: %v", err)
	}
	if len(warnings) != 1 {
		t.Errorf("warnings = %v, want 1 (account condition)", warnings)
	}

	var newFood, newCoop int64
	if err := s.readDB.QueryRowContext(ctx, `SELECT id FROM categories WHERE name = 'Food'`).Scan(&newFood); err != nil {
		t.Fatal(err)
	}
	if err := s.readDB.QueryRowContext(ctx, `SELECT id FROM payees WHERE name = 'Coop'`).Scan(&newCoop); err != nil {
		t.Fatal(err)
	}
	if newFood == foodID && newCoop == coopID {
		t.Fatal("test setup: ids did not change")
	}
	rules := rulesList(t, s)
	if len(rules) != 1 {
		t.Fatalf("rules = %d, want 1", len(rules))
	}
	if got := rules[0]; got.Actions.CategoryID != domain.CategoryID(newFood) ||
		got.Actions.PayeeID != domain.PayeeID(newCoop) || len(got.Conditions.AccountIDs) != 0 {
		t.Errorf("rule = %+v, want category %d payee %d and no account ids", got, newFood, newCoop)
	}
	if n := count(t, s, "connector_account_refs"); n != 0 {
		t.Errorf("connector_account_refs = %d, want 0", n)
	}
	if n := count(t, s, "connector_ref_pending"); n != 1 {
		t.Fatalf("connector_ref_pending = %d, want 1", n)
	}

	// No account named Main yet: still unresolved. Once created, the ref binds and is consumed.
	err := s.WithTx(ctx, func(r app.Repos) error {
		_, err := r.Imports().AccountRef(ctx, "c", "IT01")
		return err
	})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("AccountRef before account exists = %v, want not found", err)
	}
	exec(`INSERT INTO accounts (name, kind, currency, opening_balance_minor, opened_on)
		VALUES ('main', 'checking', 'EUR', 0, '2026-01-01')`)
	var got domain.AccountID
	err = s.WithTx(ctx, func(r app.Repos) (err error) {
		got, err = r.Imports().AccountRef(ctx, "c", "IT01")
		return err
	})
	if err != nil || got != 1 {
		t.Fatalf("AccountRef after account exists = %d, %v; want 1", got, err)
	}
	if n := count(t, s, "connector_account_refs"); n != 1 {
		t.Errorf("connector_account_refs = %d, want 1 after relink", n)
	}
	if n := count(t, s, "connector_ref_pending"); n != 0 {
		t.Errorf("connector_ref_pending = %d, want 0 after consume", n)
	}
}
