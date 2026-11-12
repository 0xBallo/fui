package importing_test

import (
	"context"
	"testing"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

func TestImportAppliesRules(t *testing.T) {
	e := newEnv(nil)
	acct := e.seedAccount(t, "Main", "bank", "main")
	ctx := context.Background()
	var food domain.Category
	var shop domain.Payee
	var tag domain.Tag
	err := e.store.WithTx(ctx, func(r app.Repos) (err error) {
		if food, err = r.Categories().Insert(ctx, must(domain.NewCategory("Food", domain.CategoryExpense))); err != nil {
			return err
		}
		if shop, err = r.Payees().Insert(ctx, must(domain.NewPayee("Shop"))); err != nil {
			return err
		}
		if tag, err = r.Tags().Insert(ctx, must(domain.NewTag("weekly"))); err != nil {
			return err
		}
		_, err = r.Rules().Insert(ctx, domain.Rule{
			Name: "bar", Enabled: true, Conditions: domain.RuleConditions{DescriptionContains: "bar"},
			Actions: domain.RuleActions{CategoryID: food.ID, PayeeID: shop.ID, TagIDs: []domain.TagID{tag.ID}},
		})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	e.seedTx(t, domain.Transaction{
		AccountID: acct.ID, Date: date("2026-10-01"), Amount: money(-500), Description: "Bar old",
		Status: domain.StatusBooked, Kind: domain.KindPayment, ExternalID: "dup",
	})
	src := stream("bank", "sha",
		account("main", "Main", "EUR"),
		txRec("main", "a", "2026-10-10", -300, "BAR Roma"),
		txRec("main", "b", "2026-10-11", -400, "Bar Milano", category("Other")),
		txRec("main", "c", "2026-10-12", -600, "Cinema"),
		txRec("main", "dup", "2026-10-01", -500, "Bar old"),
	)
	plan := e.plan(t, src)
	if r := plan.Rows[0]; r.Category != "Food" || r.Tx.PayeeID != shop.ID || r.Rules.TagsRule != "bar" {
		t.Fatalf("row 0 = %+v", r)
	}
	if r := plan.Rows[1]; r.Category != "Other" || r.Rules.CategoryRule != "" || r.Tx.PayeeID != shop.ID {
		t.Fatalf("row 1 keeps source category: %+v", r)
	}
	if r := plan.Rows[3]; r.Rules.TagsRule != "" || r.Rules.CategoryRule != "" {
		t.Fatalf("duplicate row got rules: %+v", r.Rules)
	}

	if _, err := e.svc.ApplyImport(ctx, plan); err != nil {
		t.Fatal(err)
	}
	var tags []domain.Tag
	var txID domain.TransactionID
	for _, tx := range e.txs(t) {
		if tx.Description == "BAR Roma" {
			txID = tx.ID
			if tx.CategoryID != food.ID || tx.PayeeID != shop.ID {
				t.Fatalf("stored tx = %+v", tx)
			}
		}
	}
	_ = e.store.Read(ctx, func(r app.Repos) (err error) { tags, err = r.Tags().TagsOf(ctx, txID); return err })
	if len(tags) != 1 || tags[0].ID != tag.ID {
		t.Fatalf("tags = %v", tags)
	}
}
