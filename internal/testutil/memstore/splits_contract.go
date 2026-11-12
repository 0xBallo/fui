package memstore

import (
	"context"
	"testing"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

func contractTags(t *testing.T, f *fixture) {
	ctx := context.Background()
	acct := f.account(t, "A")
	tx := f.tx(t, domain.Transaction{AccountID: acct.ID, Date: mustDate(t, "2026-02-01"), Amount: mustMoney(t, -100)})
	var trip, work domain.Tag
	f.do(t, func(r app.Repos) (err error) {
		if trip, err = r.Tags().Insert(ctx, domain.Tag{Name: "trip"}); err != nil {
			return err
		}
		work, err = r.Tags().Insert(ctx, domain.Tag{Name: "work"})
		return err
	})
	f.do(t, func(r app.Repos) error {
		tagErrors(t, r, tx.ID)
		if got, err := r.Tags().TagsOf(ctx, tx.ID); err != nil || len(got) != 0 {
			t.Errorf("TagsOf untagged = %v, %v; want empty", got, err)
		}
		if err := r.Tags().SetTags(ctx, tx.ID, []domain.TagID{work.ID, trip.ID, work.ID}); err != nil {
			t.Fatalf("SetTags: %v", err)
		}
		if got, _ := r.Tags().TagsOf(ctx, tx.ID); len(got) != 2 || got[0].ID != trip.ID || got[1].ID != work.ID {
			t.Errorf("TagsOf = %+v, want trip, work by ID", got)
		}
		if err := r.Tags().SetTags(ctx, tx.ID, []domain.TagID{work.ID}); err != nil {
			t.Fatalf("SetTags replace: %v", err)
		}
		if got, _ := r.Tags().TagsOf(ctx, tx.ID); len(got) != 1 || got[0].ID != work.ID {
			t.Errorf("TagsOf after replace = %+v, want work", got)
		}
		tagRename(t, r, tx.ID, trip, work)
		return nil
	})
	tagsCascade(t, f, tx.ID, trip.ID)
}

func contractPayees(t *testing.T, f *fixture) {
	ctx := context.Background()
	acct := f.account(t, "A")
	var coop, other domain.Payee
	f.do(t, func(r app.Repos) (err error) {
		if coop, err = r.Payees().Insert(ctx, domain.Payee{Name: "Coop", Aliases: []string{"COOP SOC", "Coop Lombardia"}}); err != nil {
			return err
		}
		other, err = r.Payees().Insert(ctx, domain.Payee{Name: "Other"})
		return err
	})
	f.do(t, func(r app.Repos) error {
		_, err := r.Payees().Insert(ctx, domain.Payee{Name: "coop"})
		wantErr(t, err, app.ErrAlreadyExists, "duplicate payee name (case-insensitive)")
		return nil
	})
	// A failing insert may leave a partial row in the transaction, which the use case rolls back.
	err := f.store.WithTx(ctx, func(r app.Repos) error {
		_, err := r.Payees().Insert(ctx, domain.Payee{Name: "Third", Aliases: []string{"coop soc"}})
		return err
	})
	wantErr(t, err, app.ErrAlreadyExists, "duplicate alias (case-insensitive)")
	f.do(t, func(r app.Repos) error {
		got, err := r.Payees().Get(ctx, coop.ID)
		if err != nil || got.Name != "Coop" || len(got.Aliases) != 2 || got.Aliases[0] != "COOP SOC" {
			t.Errorf("Get = %+v, %v", got, err)
		}
		_, err = r.Payees().Get(ctx, 99)
		wantErr(t, err, domain.ErrNotFound, "Get missing payee")
		payeeErrors(t, r, other.ID)
		coop.Aliases = []string{"Coop Italia"}
		if err := r.Payees().Update(ctx, coop); err != nil {
			t.Errorf("Update: %v", err)
		}
		if got, _ := r.Payees().Get(ctx, coop.ID); len(got.Aliases) != 1 || got.Aliases[0] != "Coop Italia" {
			t.Errorf("aliases after update = %v, want replaced", got.Aliases)
		}
		if all, _ := r.Payees().List(ctx); len(all) != 2 || all[1].ID != other.ID || len(all[1].Aliases) != 0 {
			t.Errorf("List = %+v", all)
		}
		return nil
	})
	// Transactions refer to payees: unknown payee is rejected, a used payee cannot be deleted.
	f.do(t, func(r app.Repos) error {
		_, err := r.Transactions().Insert(ctx, domain.Transaction{
			AccountID: acct.ID, Date: mustDate(t, "2026-02-01"), Amount: mustMoney(t, -100), PayeeID: 99,
			Status: domain.StatusBooked, Kind: domain.KindPayment,
		})
		wantErr(t, err, domain.ErrNotFound, "Insert with unknown payee")
		return nil
	})
	tx := f.tx(t, domain.Transaction{AccountID: acct.ID, Date: mustDate(t, "2026-02-01"), Amount: mustMoney(t, -100), PayeeID: coop.ID})
	f.do(t, func(r app.Repos) error {
		wantErr(t, r.Payees().Delete(ctx, coop.ID), app.ErrInUse, "Delete of a used payee")
		if err := r.Transactions().Delete(ctx, tx.ID); err != nil {
			return err
		}
		if err := r.Payees().Delete(ctx, coop.ID); err != nil {
			t.Errorf("Delete unused: %v", err)
		}
		return nil
	})
}

func contractSplits(t *testing.T, f *fixture) {
	ctx := context.Background()
	acct := f.account(t, "A")
	var food, home domain.Category
	f.do(t, func(r app.Repos) (err error) {
		if food, err = r.Categories().Insert(ctx, domain.Category{Name: "Food", Kind: domain.CategoryExpense}); err != nil {
			return err
		}
		home, err = r.Categories().Insert(ctx, domain.Category{Name: "Home", Kind: domain.CategoryExpense})
		return err
	})
	tx := f.tx(t, domain.Transaction{AccountID: acct.ID, Date: mustDate(t, "2026-02-01"), Amount: mustMoney(t, -1000)})
	row := func(c domain.Category, minor int64, note string) domain.Split {
		return domain.Split{Category: c.ID, Amount: mustMoney(t, minor), Note: note}
	}
	f.do(t, func(r app.Repos) error {
		if got, err := r.Splits().SplitsOf(ctx, tx.ID); err != nil || len(got) != 0 {
			t.Errorf("SplitsOf unsplit = %v, %v; want none", got, err)
		}
		_, err := r.Splits().SplitsOf(ctx, 99)
		wantErr(t, err, domain.ErrNotFound, "SplitsOf missing transaction")
		wantErr(t, r.Splits().ReplaceSplits(ctx, 99, nil), domain.ErrNotFound, "ReplaceSplits missing transaction")
		invalidSplits(t, r, tx.ID, row(food, -400, ""), row(home, -600, ""))
		return nil
	})
	f.do(t, func(r app.Repos) error {
		rows := []domain.Split{row(home, -600, "rent"), row(food, -400, "")}
		if err := r.Splits().ReplaceSplits(ctx, tx.ID, rows); err != nil {
			t.Fatalf("ReplaceSplits: %v", err)
		}
		got, err := r.Splits().SplitsOf(ctx, tx.ID)
		if err != nil || len(got) != 2 || got[0] != rows[0] || got[1] != rows[1] {
			t.Errorf("SplitsOf = %+v, %v; want %+v in order", got, err, rows)
		}
		if err := r.Splits().ReplaceSplits(ctx, tx.ID, []domain.Split{row(food, -700, ""), row(home, -300, "")}); err != nil {
			t.Fatalf("ReplaceSplits again: %v", err)
		}
		if got, _ := r.Splits().SplitsOf(ctx, tx.ID); len(got) != 2 || got[0].Category != food.ID {
			t.Errorf("SplitsOf after replace = %+v", got)
		}
		wantErr(t, r.Categories().Delete(ctx, home.ID), app.ErrInUse, "Delete of a category used by a split row")
		if err := r.Splits().ReplaceSplits(ctx, tx.ID, nil); err != nil {
			t.Errorf("unsplit: %v", err)
		}
		if got, _ := r.Splits().SplitsOf(ctx, tx.ID); len(got) != 0 {
			t.Errorf("SplitsOf after unsplit = %+v", got)
		}
		return nil
	})
	splitsCascade(t, f, tx.ID, row(food, -700, ""), row(home, -300, ""))
}

func tagErrors(t *testing.T, r app.Repos, tx domain.TransactionID) {
	ctx := context.Background()
	_, err := r.Tags().Insert(ctx, domain.Tag{Name: "TRIP"})
	wantErr(t, err, app.ErrAlreadyExists, "duplicate tag (case-insensitive)")
	_, err = r.Tags().Get(ctx, 99)
	wantErr(t, err, domain.ErrNotFound, "Get missing tag")
	wantErr(t, r.Tags().Update(ctx, domain.Tag{ID: 99, Name: "x"}), domain.ErrNotFound, "Update missing tag")
	wantErr(t, r.Tags().Delete(ctx, 99), domain.ErrNotFound, "Delete missing tag")
	wantErr(t, r.Tags().SetTags(ctx, 99, nil), domain.ErrNotFound, "SetTags on missing transaction")
	wantErr(t, r.Tags().SetTags(ctx, tx, []domain.TagID{99}), domain.ErrNotFound, "SetTags with missing tag")
}

func payeeErrors(t *testing.T, r app.Repos, other domain.PayeeID) {
	ctx := context.Background()
	wantErr(t, r.Payees().Update(ctx, domain.Payee{ID: 99, Name: "x"}), domain.ErrNotFound, "Update missing payee")
	wantErr(t, r.Payees().Delete(ctx, 99), domain.ErrNotFound, "Delete missing payee")
	wantErr(t, r.Payees().Update(ctx, domain.Payee{ID: other, Name: "COOP"}), app.ErrAlreadyExists, "rename to an existing name")
}

// invalidSplits expects food+home to be a valid split of the transaction (-1000).
func invalidSplits(t *testing.T, r app.Repos, tx domain.TransactionID, food, home domain.Split) {
	ctx := context.Background()
	short, err := domain.NewMoney(-500, home.Amount.Currency())
	if err != nil {
		t.Fatal(err)
	}
	wantErr(t, r.Splits().ReplaceSplits(ctx, tx, []domain.Split{food, {Category: home.Category, Amount: short}}), domain.ErrInvalidValue, "rows not summing to the amount")
	wantErr(t, r.Splits().ReplaceSplits(ctx, tx, []domain.Split{food}), domain.ErrInvalidValue, "single row")
	wantErr(t, r.Splits().ReplaceSplits(ctx, tx, []domain.Split{food, {Category: 99, Amount: home.Amount}}), domain.ErrNotFound, "unknown category")
}

func tagsCascade(t *testing.T, f *fixture, tx domain.TransactionID, trip domain.TagID) {
	ctx := context.Background()
	// Deleting a transaction drops its tags.
	f.do(t, func(r app.Repos) error {
		if err := r.Tags().SetTags(ctx, tx, []domain.TagID{trip}); err != nil {
			return err
		}
		return r.Transactions().Delete(ctx, tx)
	})
	f.do(t, func(r app.Repos) error {
		wantErr(t, r.Tags().SetTags(ctx, tx, nil), domain.ErrNotFound, "SetTags after transaction delete")
		return nil
	})
}

func splitsCascade(t *testing.T, f *fixture, tx domain.TransactionID, food, home domain.Split) {
	ctx := context.Background()
	f.do(t, func(r app.Repos) error {
		if err := r.Splits().ReplaceSplits(ctx, tx, []domain.Split{food, home}); err != nil {
			return err
		}
		return r.Transactions().Delete(ctx, tx)
	})
	f.do(t, func(r app.Repos) error {
		if err := r.Categories().Delete(ctx, home.Category); err != nil {
			t.Errorf("Delete category after its transaction was deleted: %v", err)
		}
		return nil
	})
}

func tagRename(t *testing.T, r app.Repos, tx domain.TransactionID, trip, work domain.Tag) {
	ctx := context.Background()
	work.Name = "job"
	if err := r.Tags().Update(ctx, work); err != nil {
		t.Errorf("Update: %v", err)
	}
	if err := r.Tags().Delete(ctx, work.ID); err != nil {
		t.Errorf("Delete: %v", err)
	}
	if got, _ := r.Tags().TagsOf(ctx, tx); len(got) != 0 {
		t.Errorf("TagsOf after tag delete = %+v, want none", got)
	}
	if all, _ := r.Tags().List(ctx); len(all) != 1 || all[0].ID != trip.ID {
		t.Errorf("List = %+v, want only trip", all)
	}
}
