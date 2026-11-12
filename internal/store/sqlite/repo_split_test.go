package sqlite

import (
	"context"
	"errors"
	"testing"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

// TestPayeeRenameRefreshesSearchText: the payee name is indexed, so renaming must re-index its rows.
func TestPayeeRenameRefreshesSearchText(t *testing.T) {
	s, _ := newTestStore(t)
	seedSearchRefs(t, s)
	ctx := context.Background()
	eur := mustCurrency(t, "EUR")
	var txID domain.TransactionID
	err := s.WithTx(ctx, func(r app.Repos) error {
		// seedSearchRefs created payee 1 = "Esselunga".
		tx, err := r.Transactions().Insert(ctx, domain.Transaction{
			AccountID: 1, Date: mustDate(t, "2026-03-01"), Amount: mustMoney(t, -100, eur), PayeeID: 1,
			Description: "groceries", Status: domain.StatusBooked, Kind: domain.KindPayment,
		})
		txID = tx.ID
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	find := func(text string) int {
		var n int
		err := s.Read(ctx, func(r app.Repos) error {
			rows, err := r.Transactions().List(ctx, mustFilter(t, domain.TxFilterSpec{Text: text}), nil, 0)
			n = len(rows)
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
		return n
	}
	if find("esselunga") != 1 {
		t.Fatal("payee name not searchable after insert")
	}
	if err := s.WithTx(ctx, func(r app.Repos) error {
		return r.Payees().Update(ctx, domain.Payee{ID: 1, Name: "Coop", Aliases: []string{"COOP SOC"}})
	}); err != nil {
		t.Fatal(err)
	}
	if find("esselunga") != 0 || find("coop") != 1 {
		t.Fatalf("search after rename: esselunga=%d coop=%d, want 0 and 1", find("esselunga"), find("coop"))
	}
	// Edits that change the transaction re-read the payee name.
	if err := s.WithTx(ctx, func(r app.Repos) error {
		tx, err := r.Transactions().Get(ctx, txID)
		if err != nil {
			return err
		}
		tx.Notes = "weekly"
		return r.Transactions().Update(ctx, tx)
	}); err != nil {
		t.Fatal(err)
	}
	if find("coop") != 1 || find("weekly") != 1 {
		t.Fatal("search text lost the payee name on transaction update")
	}
}

// TestSplitSumIntegrity: whatever is stored through ReplaceSplits sums to the transaction amount, and
// invalid sets leave the previous rows untouched.
func TestSplitSumIntegrity(t *testing.T) {
	s, _ := newTestStore(t)
	seedSearchRefs(t, s)
	ctx := context.Background()
	eur := mustCurrency(t, "EUR")
	row := func(cat domain.CategoryID, minor int64) domain.Split {
		return domain.Split{Category: cat, Amount: mustMoney(t, minor, eur)}
	}
	var id domain.TransactionID
	if err := s.WithTx(ctx, func(r app.Repos) error {
		tx, err := r.Transactions().Insert(ctx, domain.Transaction{
			AccountID: 1, Date: mustDate(t, "2026-03-01"), Amount: mustMoney(t, -1000, eur),
			Status: domain.StatusBooked, Kind: domain.KindPayment,
		})
		id = tx.ID
		if err != nil {
			return err
		}
		return r.Splits().ReplaceSplits(ctx, id, []domain.Split{row(1, -400), row(2, -600)})
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.WithTx(ctx, func(r app.Repos) error {
		return r.Splits().ReplaceSplits(ctx, id, []domain.Split{row(1, -400), row(2, -500)})
	}); !errors.Is(err, domain.ErrInvalidValue) {
		t.Fatalf("unbalanced split error = %v, want ErrInvalidValue", err)
	}
	var sum int64
	if err := s.readDB.QueryRow("SELECT SUM(amount_minor) FROM transaction_splits WHERE transaction_id = ?", id).Scan(&sum); err != nil || sum != -1000 {
		t.Fatalf("stored split sum = %d, err %v; want -1000", sum, err)
	}
	// A used category cannot be deleted (RESTRICT), and deleting the transaction cascades.
	if err := s.WithTx(ctx, func(r app.Repos) error { return r.Categories().Delete(ctx, 2) }); !errors.Is(err, app.ErrInUse) {
		t.Fatalf("delete split category error = %v, want ErrInUse", err)
	}
	if err := s.WithTx(ctx, func(r app.Repos) error { return r.Transactions().Delete(ctx, id) }); err != nil {
		t.Fatal(err)
	}
	if n := count(t, s, "transaction_splits"); n != 0 {
		t.Fatalf("splits left after transaction delete = %d", n)
	}
}
