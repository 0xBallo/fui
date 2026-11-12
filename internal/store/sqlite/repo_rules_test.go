package sqlite

import (
	"context"
	"errors"
	"testing"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

func TestRuleRepoCRUDAndEnabledOrder(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()

	alpha := domain.Rule{
		Name:       "Alpha",
		Priority:   20,
		Enabled:    true,
		Conditions: domain.RuleConditions{DescriptionContains: "coffee"},
		Actions:    domain.RuleActions{CategoryID: 1},
	}
	beta := domain.Rule{
		Name:       "beta",
		Priority:   10,
		Enabled:    true,
		Conditions: domain.RuleConditions{DescriptionContains: "rent"},
		Actions:    domain.RuleActions{CategoryID: 2},
	}

	if err := store.WithTx(ctx, func(r app.Repos) error {
		if _, err := r.Rules().Insert(ctx, alpha); err != nil {
			return err
		}
		if _, err := r.Rules().Insert(ctx, beta); err != nil {
			return err
		}
		return nil
	}); err != nil {
		t.Fatalf("insert rules: %v", err)
	}

	if err := store.Read(ctx, func(r app.Repos) error {
		all, err := r.Rules().List(ctx)
		if err != nil {
			return err
		}
		if len(all) != 2 || all[0].Name != "Alpha" || all[1].Name != "beta" {
			t.Fatalf("List = %+v, want Alpha,beta", all)
		}
		enabled, err := r.Rules().ListEnabledByPriority(ctx)
		if err != nil {
			return err
		}
		if len(enabled) != 2 || enabled[0].Name != "beta" || enabled[1].Name != "Alpha" {
			t.Fatalf("ListEnabledByPriority = %+v, want beta,Alpha", enabled)
		}
		return nil
	}); err != nil {
		t.Fatalf("read rules: %v", err)
	}

	alpha.Enabled = false
	if err := store.WithTx(ctx, func(r app.Repos) error {
		return r.Rules().Update(ctx, alpha)
	}); err != nil {
		t.Fatalf("update rule: %v", err)
	}
	if err := store.Read(ctx, func(r app.Repos) error {
		enabled, err := r.Rules().ListEnabledByPriority(ctx)
		if err != nil {
			return err
		}
		if len(enabled) != 1 || enabled[0].Name != "beta" {
			t.Fatalf("enabled after disable = %+v, want only beta", enabled)
		}
		return nil
	}); err != nil {
		t.Fatalf("read enabled after update: %v", err)
	}
}

func TestRuleRepoValidationAndNotFound(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()

	err := store.WithTx(ctx, func(r app.Repos) error {
		_, err := r.Rules().Insert(ctx, domain.Rule{
			Name:       "NoAction",
			Priority:   1,
			Enabled:    true,
			Conditions: domain.RuleConditions{},
			Actions:    domain.RuleActions{},
		})
		return err
	})
	if !errors.Is(err, domain.ErrMissingField) {
		t.Fatalf("insert invalid rule err = %v, want missing_field", err)
	}

	err = store.WithTx(ctx, func(r app.Repos) error {
		return r.Rules().Update(ctx, domain.Rule{
			Name:       "Missing",
			Priority:   1,
			Enabled:    true,
			Conditions: domain.RuleConditions{DescriptionContains: "x"},
			Actions:    domain.RuleActions{CategoryID: 1},
		})
	})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("update missing rule err = %v, want not_found", err)
	}

	err = store.WithTx(ctx, func(r app.Repos) error {
		return r.Rules().Delete(ctx, "missing")
	})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("delete missing rule err = %v, want not_found", err)
	}
	if err := store.Read(ctx, func(r app.Repos) error {
		_, err := r.Rules().Get(ctx, "missing")
		if !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("get missing rule err = %v, want not_found", err)
		}
		return nil
	}); err != nil {
		t.Fatalf("read get missing: %v", err)
	}
}

func TestRuleRepoRejectsInvalidStoredJSON(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()

	if _, err := store.writeDB.ExecContext(ctx,
		`INSERT INTO rules (name, priority, enabled, conditions, actions) VALUES (?, ?, ?, ?, ?)`,
		"raw", 1, 1, `{}`, `{"unknown":1}`); err != nil {
		t.Fatalf("insert raw malformed rule: %v", err)
	}

	err := store.Read(ctx, func(r app.Repos) error {
		_, err := r.Rules().Get(ctx, "raw")
		return err
	})
	if err == nil {
		t.Fatal("Get on malformed stored rule succeeded, want error")
	}
}
