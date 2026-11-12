package domain

import (
	"errors"
	"testing"
)

func TestNewTxFilterValidation(t *testing.T) {
	minAbs := int64(10)
	maxAbs := int64(5)
	tests := []TxFilterSpec{
		{From: date(t, 2026, 11, 2), To: date(t, 2026, 11, 1)},
		{From: date(t, 2026, 1, 1), Month: date(t, 2026, 11, 9)},
		{To: date(t, 2026, 12, 31), Year: 2026},
		{Month: date(t, 2026, 11, 1), Year: 2026},
		{Year: 10_000},
		{AccountIDs: []AccountID{0}},
		{CategoryIDs: []CategoryID{-1}},
		{PayeeIDs: []PayeeID{0}},
		{Statuses: []TxStatus{"done"}},
		{Kinds: []TxKind{"gift"}},
		{ImportBatchID: ImportBatchID(-2)},
		{MinAbs: int64ptr(-1)},
		{MaxAbs: int64ptr(-1)},
		{MinAbs: &minAbs, MaxAbs: &maxAbs},
		{Sign: TxSign("other")},
		{Sort: TxSort("id")},
		{Limit: -1},
	}
	for i, spec := range tests {
		_, err := NewTxFilter(spec)
		if !errors.Is(err, ErrInvalidFilter) {
			t.Fatalf("case %d: err = %v, want ErrInvalidFilter", i, err)
		}
	}
}

func TestNewTxFilterDefaultsAndGetters(t *testing.T) {
	minAbs := int64(10)
	maxAbs := int64(100)
	spec := TxFilterSpec{
		Month:         date(t, 2026, 2, 22),
		AccountIDs:    []AccountID{1, 2, 1},
		CategoryIDs:   []CategoryID{5, 6, 5},
		Uncategorized: true,
		PayeeIDs:      []PayeeID{7, 8, 7},
		Statuses:      []TxStatus{StatusBooked, StatusPending, StatusBooked},
		Kinds:         []TxKind{KindPayment, KindRefund, KindPayment},
		ImportBatchID: 11,
		MinAbs:        &minAbs,
		MaxAbs:        &maxAbs,
		Text:          "  amazon prime  ",
		Desc:          true,
		Limit:         30,
	}
	f, err := NewTxFilter(spec)
	if err != nil {
		t.Fatalf("NewTxFilter: %v", err)
	}
	if f.From() != date(t, 2026, 2, 1) || f.To() != date(t, 2026, 2, 28) {
		t.Fatalf("period = [%s,%s]", f.From(), f.To())
	}
	if got := f.AccountIDs(); len(got) != 2 || got[0] != 1 || got[1] != 2 {
		t.Fatalf("account ids = %v", got)
	}
	if got := f.CategoryIDs(); len(got) != 2 || got[0] != 5 || got[1] != 6 {
		t.Fatalf("category ids = %v", got)
	}
	if got := f.PayeeIDs(); len(got) != 2 || got[0] != 7 || got[1] != 8 {
		t.Fatalf("payee ids = %v", got)
	}
	if got := f.Statuses(); len(got) != 2 || got[0] != StatusBooked || got[1] != StatusPending {
		t.Fatalf("statuses = %v", got)
	}
	if got := f.Kinds(); len(got) != 2 || got[0] != KindPayment || got[1] != KindRefund {
		t.Fatalf("kinds = %v", got)
	}
	if !f.Uncategorized() || f.ImportBatchID() != 11 || f.Sign() != TxSignAny || f.Sort() != TxSortDate || !f.Desc() || f.Limit() != 30 {
		t.Fatalf("unexpected scalars: uncategorized=%v batch=%d sign=%q sort=%q desc=%v limit=%d", f.Uncategorized(), f.ImportBatchID(), f.Sign(), f.Sort(), f.Desc(), f.Limit())
	}
	if f.Text() != "amazon prime" {
		t.Fatalf("text = %q", f.Text())
	}
	if got := f.MinAbs(); got == nil || *got != 10 {
		t.Fatalf("min = %v", got)
	}
	if got := f.MaxAbs(); got == nil || *got != 100 {
		t.Fatalf("max = %v", got)
	}

	spec.AccountIDs[0] = 99
	spec.CategoryIDs[0] = 99
	spec.PayeeIDs[0] = 99
	spec.Statuses[0] = StatusReconciled
	spec.Kinds[0] = KindOther
	*spec.MinAbs = 999
	*spec.MaxAbs = 999
	a := f.AccountIDs()
	a[0] = 99
	if f.AccountIDs()[0] != 1 || f.CategoryIDs()[0] != 5 || f.PayeeIDs()[0] != 7 || f.Statuses()[0] != StatusBooked || f.Kinds()[0] != KindPayment {
		t.Fatal("filter slices are not immutable")
	}
	if *f.MinAbs() != 10 || *f.MaxAbs() != 100 {
		t.Fatal("filter pointers are not immutable")
	}
}

func TestNewTxFilterYearAndOpenBounds(t *testing.T) {
	year, err := NewTxFilter(TxFilterSpec{Year: 2024})
	if err != nil {
		t.Fatalf("year filter: %v", err)
	}
	if year.From() != date(t, 2024, 1, 1) || year.To() != date(t, 2024, 12, 31) {
		t.Fatalf("year period = [%s,%s]", year.From(), year.To())
	}

	fromOnly, err := NewTxFilter(TxFilterSpec{From: date(t, 2026, 1, 1)})
	if err != nil {
		t.Fatalf("from-only: %v", err)
	}
	if fromOnly.From() != date(t, 2026, 1, 1) || !fromOnly.To().IsZero() {
		t.Fatalf("from-only = [%s,%s]", fromOnly.From(), fromOnly.To())
	}

	toOnly, err := NewTxFilter(TxFilterSpec{To: date(t, 2026, 12, 31)})
	if err != nil {
		t.Fatalf("to-only: %v", err)
	}
	if !toOnly.From().IsZero() || toOnly.To() != date(t, 2026, 12, 31) {
		t.Fatalf("to-only = [%s,%s]", toOnly.From(), toOnly.To())
	}
}

func TestTxFilterMatch(t *testing.T) {
	f, err := NewTxFilter(TxFilterSpec{
		From:          date(t, 2026, 11, 1),
		To:            date(t, 2026, 11, 30),
		AccountIDs:    []AccountID{1, 2},
		CategoryIDs:   []CategoryID{10},
		Uncategorized: true,
		PayeeIDs:      []PayeeID{20, 21},
		Statuses:      []TxStatus{StatusBooked},
		Kinds:         []TxKind{KindPayment},
		ImportBatchID: 30,
		MinAbs:        int64ptr(100),
		MaxAbs:        int64ptr(600),
		Sign:          TxSignExpense,
		Text:          "prime",
	})
	if err != nil {
		t.Fatalf("NewTxFilter: %v", err)
	}
	tx := Transaction{
		ID:            1,
		AccountID:     1,
		Date:          date(t, 2026, 11, 15),
		Amount:        money(t, -500, "EUR"),
		CategoryID:    10,
		PayeeID:       20,
		Status:        StatusBooked,
		Kind:          KindPayment,
		ImportBatchID: 30,
	}
	if !f.Match(tx, "amazon prime membership") {
		t.Fatal("expected tx to match")
	}

	reject := func(name string, mutate func(*Transaction), text string) {
		t.Helper()
		m := tx
		mutate(&m)
		if f.Match(m, text) {
			t.Fatalf("expected mismatch for %s", name)
		}
	}
	reject("date before", func(m *Transaction) { m.Date = date(t, 2026, 10, 31) }, "amazon prime membership")
	reject("date after", func(m *Transaction) { m.Date = date(t, 2026, 12, 1) }, "amazon prime membership")
	reject("account", func(m *Transaction) { m.AccountID = 9 }, "amazon prime membership")
	reject("category", func(m *Transaction) { m.CategoryID = 9 }, "amazon prime membership")
	reject("payee", func(m *Transaction) { m.PayeeID = 99 }, "amazon prime membership")
	reject("status", func(m *Transaction) { m.Status = StatusPending }, "amazon prime membership")
	reject("kind", func(m *Transaction) { m.Kind = KindFee }, "amazon prime membership")
	reject("batch", func(m *Transaction) { m.ImportBatchID = 999 }, "amazon prime membership")
	reject("abs min", func(m *Transaction) { m.Amount = money(t, -90, "EUR") }, "amazon prime membership")
	reject("abs max", func(m *Transaction) { m.Amount = money(t, -700, "EUR") }, "amazon prime membership")
	reject("sign", func(m *Transaction) { m.Amount = money(t, 500, "EUR") }, "amazon prime membership")
	reject("text", func(*Transaction) {}, "something else")

	uncat, err := NewTxFilter(TxFilterSpec{CategoryIDs: []CategoryID{10}, Uncategorized: true})
	if err != nil {
		t.Fatalf("uncategorized filter: %v", err)
	}
	tx.CategoryID = 0
	if !uncat.Match(tx, "") {
		t.Fatal("uncategorized OR category did not match uncategorized transaction")
	}
}

func TestTxFilterMatchIncomeAndAny(t *testing.T) {
	income, err := NewTxFilter(TxFilterSpec{Sign: TxSignIncome})
	if err != nil {
		t.Fatalf("income filter: %v", err)
	}
	tx := Transaction{Amount: money(t, 1, "EUR")}
	if !income.Match(tx, "") {
		t.Fatal("income sign should match positive")
	}
	tx.Amount = money(t, -1, "EUR")
	if income.Match(tx, "") {
		t.Fatal("income sign must reject negative")
	}

	anySign, err := NewTxFilter(TxFilterSpec{})
	if err != nil {
		t.Fatalf("any filter: %v", err)
	}
	if !anySign.Match(tx, "") {
		t.Fatal("any sign should match any amount")
	}
}

func TestTxFilterLess(t *testing.T) {
	a := Transaction{ID: 1, Date: date(t, 2026, 1, 1), Amount: money(t, -100, "EUR")}
	b := Transaction{ID: 2, Date: date(t, 2026, 1, 2), Amount: money(t, 50, "EUR")}

	dateAsc, _ := NewTxFilter(TxFilterSpec{})
	if !dateAsc.Less(a, b, "", "") || dateAsc.Less(b, a, "", "") {
		t.Fatal("date ascending order mismatch")
	}
	dateDesc, _ := NewTxFilter(TxFilterSpec{Desc: true})
	if !dateDesc.Less(b, a, "", "") || dateDesc.Less(a, b, "", "") {
		t.Fatal("date descending order mismatch")
	}

	amountAsc, _ := NewTxFilter(TxFilterSpec{Sort: TxSortAmount})
	if !amountAsc.Less(a, b, "", "") || amountAsc.Less(b, a, "", "") {
		t.Fatal("amount ascending order mismatch")
	}
	amountDesc, _ := NewTxFilter(TxFilterSpec{Sort: TxSortAmount, Desc: true})
	if !amountDesc.Less(b, a, "", "") || amountDesc.Less(a, b, "", "") {
		t.Fatal("amount descending order mismatch")
	}

	payeeAsc, _ := NewTxFilter(TxFilterSpec{Sort: TxSortPayee})
	if !payeeAsc.Less(a, b, "alice", "Bob") || payeeAsc.Less(b, a, "Bob", "alice") {
		t.Fatal("payee ascending order mismatch")
	}
	payeeDesc, _ := NewTxFilter(TxFilterSpec{Sort: TxSortPayee, Desc: true})
	if !payeeDesc.Less(b, a, "Bob", "alice") || payeeDesc.Less(a, b, "alice", "Bob") {
		t.Fatal("payee descending order mismatch")
	}

	sameKeyA := Transaction{ID: 1, Date: date(t, 2026, 1, 1), Amount: money(t, 100, "EUR")}
	sameKeyB := Transaction{ID: 2, Date: date(t, 2026, 1, 1), Amount: money(t, 100, "EUR")}
	if !dateAsc.Less(sameKeyA, sameKeyB, "same", "same") || dateAsc.Less(sameKeyB, sameKeyA, "same", "same") {
		t.Fatal("id tie-breaker is not deterministic")
	}
}

func int64ptr(v int64) *int64 { return &v }
