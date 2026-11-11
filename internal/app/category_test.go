package app_test

import (
	"testing"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

func TestCategories(t *testing.T) {
	e := newEnv(t)
	ctx := t.Context()
	food := e.category(t, "Food", "")
	if food.Kind != domain.CategoryExpense {
		t.Errorf("default kind = %s, want expense", food.Kind)
	}
	e.category(t, "Salary", "income")

	for name, req := range map[string]app.AddCategoryRequest{
		"duplicate":    {Name: "FOOD"},
		"empty name":   {Name: " "},
		"unknown kind": {Name: "X", Kind: "x"},
	} {
		if _, err := e.svc.PlanAddCategory(ctx, req); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
	_, err := e.svc.PlanAddCategory(ctx, app.AddCategoryRequest{Name: "food"})
	wantCode(t, err, app.ErrAlreadyExists)

	p, err := e.svc.PlanRenameCategory(ctx, app.RenameCategoryRequest{Category: "foo", NewName: "Groceries"})
	wantNoErr(t, err)
	c, err := e.svc.ApplyRenameCategory(ctx, p)
	wantNoErr(t, err)
	if c.ID != food.ID || c.Name != "Groceries" || c.Kind != domain.CategoryExpense {
		t.Errorf("renamed = %+v", c)
	}
	_, err = e.svc.PlanRenameCategory(ctx, app.RenameCategoryRequest{Category: "groc", NewName: "salary"})
	wantCode(t, err, app.ErrAlreadyExists)
	_, err = e.svc.PlanRenameCategory(ctx, app.RenameCategoryRequest{Category: "groc", NewName: " "})
	wantCode(t, err, domain.ErrMissingField)
	pSame, err := e.svc.PlanRenameCategory(ctx, app.RenameCategoryRequest{Category: "groc", NewName: "GROCERIES"})
	wantNoErr(t, err) // changing only the case of its own name is allowed
	_, err = e.svc.ApplyRenameCategory(ctx, pSame)
	wantNoErr(t, err)

	cats, err := e.svc.ListCategories(ctx)
	wantNoErr(t, err)
	if len(cats) != 2 || cats[0].Name != "GROCERIES" {
		t.Errorf("ListCategories = %+v", cats)
	}
}

func categoryEnv(t *testing.T) env {
	t.Helper()
	e := newEnv(t)
	e.account(t, "A", "checking", "EUR", "0")
	e.category(t, "Bar", "expense")
	e.category(t, "Dining", "expense")
	e.category(t, "Unused", "expense")
	for _, d := range []string{"1", "2", "3"} {
		e.tx(t, app.AddTxRequest{Account: "A", Amount: "-" + d, Category: "Bar", Date: "2026-10-0" + d})
	}
	e.tx(t, app.AddTxRequest{Account: "A", Amount: "-9", Category: "Dining"})
	return e
}

func TestMergeCategory(t *testing.T) {
	e := categoryEnv(t)
	ctx := t.Context()
	p, err := e.svc.PlanMergeCategory(ctx, app.MergeCategoryRequest{Source: "bar", Target: "dining"})
	wantNoErr(t, err)
	if !p.Bulk || p.Moved != 3 || len(p.Changes) != 2 {
		t.Fatalf("plan = %+v, want a bulk plan moving 3 transactions then deleting", p)
	}
	if len(e.backup.reasons) != 0 {
		t.Fatal("planning must not take a backup")
	}
	res, err := e.svc.ApplyMergeCategory(ctx, p)
	wantNoErr(t, err)
	if res.Reassigned != 3 {
		t.Errorf("Reassigned = %d, want 3", res.Reassigned)
	}
	if len(e.backup.reasons) != 1 || e.backup.reasons[0] != app.BackupReasonBulk {
		t.Errorf("backups = %v, want one with reason bulk", e.backup.reasons)
	}
	page, err := e.svc.ListTx(ctx, app.ListTxRequest{Categories: []string{"dining"}})
	wantNoErr(t, err)
	if len(page.Items) != 4 {
		t.Errorf("dining has %d transactions, want 4", len(page.Items))
	}
	_, err = e.svc.PlanRemoveCategory(ctx, app.RemoveCategoryRequest{Category: "bar"})
	wantCode(t, err, domain.ErrNotFound)
}

func TestRemoveCategory(t *testing.T) {
	tests := []struct {
		name     string
		req      app.RemoveCategoryRequest
		wantErr  *domain.Error
		wantBulk bool
	}{
		{name: "unused", req: app.RemoveCategoryRequest{Category: "unused"}},
		{name: "used without a target", req: app.RemoveCategoryRequest{Category: "bar"}, wantErr: app.ErrInUse},
		{name: "used with a target", req: app.RemoveCategoryRequest{Category: "bar", ReassignTo: "dining"}, wantBulk: true},
		{name: "unused with a target", req: app.RemoveCategoryRequest{Category: "unused", ReassignTo: "dining"}},
		{name: "target is itself", req: app.RemoveCategoryRequest{Category: "bar", ReassignTo: "bar"}, wantErr: domain.ErrInvalidValue},
		{name: "unknown target", req: app.RemoveCategoryRequest{Category: "bar", ReassignTo: "zzz"}, wantErr: domain.ErrNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := categoryEnv(t)
			p, err := e.svc.PlanRemoveCategory(t.Context(), tt.req)
			if tt.wantErr != nil {
				wantCode(t, err, tt.wantErr)
				return
			}
			wantNoErr(t, err)
			if p.Bulk != tt.wantBulk {
				t.Errorf("Bulk = %v, want %v", p.Bulk, tt.wantBulk)
			}
			_, err = e.svc.ApplyRemoveCategory(t.Context(), p)
			wantNoErr(t, err)
			if wantBackups := map[bool]int{false: 0, true: 1}[tt.wantBulk]; len(e.backup.reasons) != wantBackups {
				t.Errorf("backups = %v, want %d", e.backup.reasons, wantBackups)
			}
			cats, _ := e.svc.ListCategories(t.Context())
			if len(cats) != 2 {
				t.Errorf("%d categories left, want 2", len(cats))
			}
		})
	}
}
