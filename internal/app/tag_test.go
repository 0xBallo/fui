package app_test

import (
	"testing"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

func TestTags(t *testing.T) {
	e := newEnv(t)
	ctx := t.Context()
	vacanze := e.tag(t, "vacanze")
	e.tag(t, "lavoro")

	for name, req := range map[string]app.AddTagRequest{
		"duplicate":  {Name: "VACANZE"},
		"empty name": {Name: " "},
	} {
		if _, err := e.svc.PlanAddTag(ctx, req); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}

	p, err := e.svc.PlanRenameTag(ctx, app.RenameTagRequest{Tag: "vac", NewName: "holidays"})
	wantNoErr(t, err)
	tg, err := e.svc.ApplyRenameTag(ctx, p)
	wantNoErr(t, err)
	if tg.ID != vacanze.ID || tg.Name != "holidays" {
		t.Errorf("renamed = %+v", tg)
	}
	_, err = e.svc.PlanRenameTag(ctx, app.RenameTagRequest{Tag: "holidays", NewName: "lavoro"})
	wantCode(t, err, app.ErrAlreadyExists)

	tags, err := e.svc.ListTags(ctx)
	wantNoErr(t, err)
	if len(tags) != 2 {
		t.Errorf("ListTags = %+v", tags)
	}
}

func tagEnv(t *testing.T) (env, domain.Transaction, domain.Transaction) {
	t.Helper()
	e := newEnv(t)
	e.account(t, "A", "checking", "EUR", "0")
	e.category(t, "Bar", "expense")
	e.tag(t, "vacanze")
	e.tag(t, "lavoro")
	tx1 := e.tx(t, app.AddTxRequest{Account: "A", Amount: "-1", Category: "Bar", Date: "2026-10-01"})
	tx2 := e.tx(t, app.AddTxRequest{Account: "A", Amount: "-2", Category: "Bar", Date: "2026-10-02"})
	ctx := t.Context()
	p, err := e.svc.PlanTagTransaction(ctx, app.TagTransactionRequest{ID: tx1.ID, Add: []string{"vacanze"}})
	wantNoErr(t, err)
	wantNoErr(t, e.svc.ApplyTagTransaction(ctx, p))
	return e, tx1, tx2
}

func TestTagTransaction(t *testing.T) {
	e, tx1, tx2 := tagEnv(t)
	ctx := t.Context()

	// adding the same tag twice is a no-op, no second change
	p, err := e.svc.PlanTagTransaction(ctx, app.TagTransactionRequest{ID: tx1.ID, Add: []string{"vacanze"}})
	wantNoErr(t, err)
	if len(p.Changes) != 0 {
		t.Errorf("re-adding an already set tag should produce no change, got %+v", p.Changes)
	}

	p, err = e.svc.PlanTagTransaction(ctx, app.TagTransactionRequest{ID: tx2.ID, Add: []string{"lavoro"}})
	wantNoErr(t, err)
	wantNoErr(t, e.svc.ApplyTagTransaction(ctx, p))

	page, err := e.svc.ListTx(ctx, app.ListTxRequest{Tags: []string{"vacanze"}})
	wantNoErr(t, err)
	if len(page.Items) != 1 || page.Items[0].Transaction.ID != tx1.ID {
		t.Errorf("tagged vacanze = %+v, want only tx1", page.Items)
	}

	// OR semantics: both tags match both transactions
	page, err = e.svc.ListTx(ctx, app.ListTxRequest{Tags: []string{"vacanze", "lavoro"}})
	wantNoErr(t, err)
	if len(page.Items) != 2 {
		t.Errorf("tagged vacanze or lavoro = %+v, want both", page.Items)
	}

	up, err := e.svc.PlanUntagTransaction(ctx, app.UntagTransactionRequest{ID: tx1.ID, Remove: []string{"vacanze"}})
	wantNoErr(t, err)
	wantNoErr(t, e.svc.ApplyUntagTransaction(ctx, up))
	page, err = e.svc.ListTx(ctx, app.ListTxRequest{Tags: []string{"vacanze"}})
	wantNoErr(t, err)
	if len(page.Items) != 0 {
		t.Errorf("after untagging, want no matches, got %+v", page.Items)
	}

	_, err = e.svc.PlanTagTransaction(ctx, app.TagTransactionRequest{ID: 999, Add: []string{"vacanze"}})
	wantCode(t, err, domain.ErrNotFound)
	_, err = e.svc.PlanTagTransaction(ctx, app.TagTransactionRequest{ID: tx1.ID, Add: []string{"zzz"}})
	wantCode(t, err, domain.ErrNotFound)
}

func TestRemoveTag(t *testing.T) {
	e, tx1, _ := tagEnv(t)
	ctx := t.Context()

	p, err := e.svc.PlanRemoveTag(ctx, app.RemoveTagRequest{Tag: "vacanze"})
	wantNoErr(t, err)
	if !p.Bulk || p.Detached != 1 {
		t.Fatalf("plan = %+v, want a bulk plan detaching 1 transaction", p)
	}
	wantNoErr(t, e.svc.ApplyRemoveTag(ctx, p))
	if len(e.backup.reasons) != 1 || e.backup.reasons[0] != app.BackupReasonBulk {
		t.Errorf("backups = %v, want one with reason bulk", e.backup.reasons)
	}
	tags, err := e.svc.ListTags(ctx)
	wantNoErr(t, err)
	if len(tags) != 1 {
		t.Errorf("tags left = %+v, want only lavoro", tags)
	}

	page, err := e.svc.ListTx(ctx, app.ListTxRequest{})
	wantNoErr(t, err)
	if len(page.Items) != 2 {
		t.Fatalf("expected both transactions still present, got %+v", page.Items)
	}
	_ = tx1

	p2, err := e.svc.PlanRemoveTag(ctx, app.RemoveTagRequest{Tag: "lavoro"})
	wantNoErr(t, err)
	if p2.Bulk || p2.Detached != 0 {
		t.Errorf("plan = %+v, want a non-bulk plan", p2)
	}
}

func TestMergeTag(t *testing.T) {
	e, tx1, tx2 := tagEnv(t)
	ctx := t.Context()
	p2, err := e.svc.PlanTagTransaction(ctx, app.TagTransactionRequest{ID: tx2.ID, Add: []string{"lavoro", "vacanze"}})
	wantNoErr(t, err)
	wantNoErr(t, e.svc.ApplyTagTransaction(ctx, p2))

	p, err := e.svc.PlanMergeTag(ctx, app.MergeTagRequest{Source: "vacanze", Target: "lavoro"})
	wantNoErr(t, err)
	if !p.Bulk || p.Moved != 2 || len(p.Changes) != 2 {
		t.Fatalf("plan = %+v, want a bulk plan moving 2 transactions then deleting", p)
	}
	wantNoErr(t, e.svc.ApplyMergeTag(ctx, p))

	tags, err := e.svc.ListTags(ctx)
	wantNoErr(t, err)
	if len(tags) != 1 || tags[0].Name != "lavoro" {
		t.Errorf("tags left = %+v, want only lavoro", tags)
	}
	page, err := e.svc.ListTx(ctx, app.ListTxRequest{Tags: []string{"lavoro"}})
	wantNoErr(t, err)
	if len(page.Items) != 2 {
		t.Errorf("tagged lavoro = %+v, want both transactions", page.Items)
	}
	if page.Items[0].Transaction.ID != tx2.ID {
		t.Fatalf("tx2 not among the merged set: %+v", page.Items)
	}
	_ = tx1

	_, err = e.svc.PlanMergeTag(ctx, app.MergeTagRequest{Source: "lavoro", Target: "lavoro"})
	wantCode(t, err, domain.ErrInvalidValue)
}
