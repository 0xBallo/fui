package domain

import (
	"errors"
	"testing"

	"pgregory.net/rapid"
)

func TestNormalizeDescription(t *testing.T) {
	got := NormalizeDescription("  AcMe\tMarket   SPA  ")
	if got != "acme market spa" {
		t.Fatalf("got %q", got)
	}
}

func TestFingerprint(t *testing.T) {
	tx := Transaction{
		AccountID:   7,
		Date:        date(t, 2026, 11, 11),
		Amount:      money(t, -1234, "EUR"),
		Description: "  ACME   MARKET ",
	}
	a, err := Fingerprint(tx.AccountID, tx.Date, tx.Amount, tx.Description)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Fingerprint(tx.AccountID, tx.Date, tx.Amount, "acme market")
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Fatalf("fingerprints differ: %q vs %q", a, b)
	}
}

func TestClassifyDedup(t *testing.T) {
	incoming := Transaction{
		ID:          99,
		AccountID:   1,
		Date:        date(t, 2026, 11, 10),
		Amount:      money(t, -1000, "EUR"),
		Description: "Coffee bar",
		ExternalID:  "x-1",
	}
	existing := []Transaction{
		{ID: 1, AccountID: 1, Date: date(t, 2026, 11, 10), Amount: money(t, -1000, "EUR"), Description: "Coffee bar", ExternalID: "x-1"},
		{ID: 2, AccountID: 1, Date: date(t, 2026, 11, 11), Amount: money(t, -1000, "EUR"), Description: "Coffee bar"},
		{ID: 3, AccountID: 1, Date: date(t, 2026, 11, 12), Amount: money(t, -1000, "EUR"), Description: "Other"},
	}
	got, err := ClassifyDedup(incoming, existing)
	if err != nil {
		t.Fatal(err)
	}
	if got.Class != DedupDuplicate || len(got.Matched) != 1 || got.Matched[0] != 1 {
		t.Fatalf("duplicate match = %+v", got)
	}
	incoming.ExternalID = ""
	incoming.Date = date(t, 2026, 11, 11)
	got, err = ClassifyDedup(incoming, existing[1:])
	if err != nil {
		t.Fatal(err)
	}
	if got.Class != DedupDuplicate || len(got.Matched) != 1 || got.Matched[0] != 2 {
		t.Fatalf("fingerprint duplicate = %+v", got)
	}
	incoming.Description = "new text"
	got, err = ClassifyDedup(incoming, existing[2:])
	if err != nil {
		t.Fatal(err)
	}
	if got.Class != DedupDoubtful || len(got.Matched) != 1 || got.Matched[0] != 3 {
		t.Fatalf("doubtful = %+v", got)
	}
	incoming.Date = date(t, 2026, 11, 20)
	got, err = ClassifyDedup(incoming, existing)
	if err != nil {
		t.Fatal(err)
	}
	if got.Class != DedupNew {
		t.Fatalf("new = %+v", got)
	}
}

func TestClassifyDedupErrors(t *testing.T) {
	incoming := Transaction{AccountID: 1, Date: date(t, 2026, 1, 1), Amount: money(t, -1, "EUR")}
	if _, err := ClassifyDedup(Transaction{}, nil); !errors.Is(err, ErrMissingField) {
		t.Fatalf("missing account err = %v", err)
	}
	if _, err := ClassifyDedup(Transaction{AccountID: 1}, nil); !errors.Is(err, ErrMissingField) {
		t.Fatalf("missing date err = %v", err)
	}
	if _, err := ClassifyDedup(Transaction{AccountID: 1, Date: date(t, 2026, 1, 1)}, nil); !errors.Is(err, ErrMissingField) {
		t.Fatalf("missing amount err = %v", err)
	}
	if _, err := ClassifyDedup(incoming, []Transaction{{AccountID: 1, Amount: money(t, -1, "EUR")}}); !errors.Is(err, ErrMissingField) {
		t.Fatalf("existing missing date err = %v", err)
	}
}

func TestDedupIdempotenceProperty(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		count := rapid.IntRange(1, 40).Draw(t, "count")
		txs := make([]Transaction, 0, count)
		for i := 0; i < count; i++ {
			account := rapid.Int64Range(1, 5).Draw(t, "account")
			day := rapid.IntRange(1, 28).Draw(t, "day")
			amount := rapid.Int64Range(-10_000, 10_000).Draw(t, "amount")
			if amount == 0 {
				amount = 1
			}
			txs = append(txs, Transaction{
				ID:          TransactionID(i + 1),
				AccountID:   AccountID(account),
				Date:        date(t, 2026, 11, day),
				Amount:      money(t, amount, "EUR"),
				Description: "merchant",
			})
		}
		for _, tx := range txs {
			got, err := ClassifyDedup(tx, txs)
			if err != nil {
				t.Fatal(err)
			}
			if got.Class != DedupDuplicate {
				t.Fatalf("tx %d classified as %q", tx.ID, got.Class)
			}
		}
	})
}
