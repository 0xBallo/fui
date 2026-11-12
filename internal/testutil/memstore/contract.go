package memstore

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

// RunContract checks that the stores made by newStore honour the repository contract of
// internal/app (ports.go). The in-memory store and the SQLite store both run it.
func RunContract(t *testing.T, newStore func(t *testing.T) app.Store) {
	t.Helper()
	for name, fn := range map[string]func(*testing.T, *fixture){
		"accounts":     contractAccounts,
		"categories":   contractCategories,
		"transactions": contractTransactions,
		"list":         contractList,
		"lookups":      contractLookups,
		"transfers":    contractTransfers,
		"fx rates":     contractFXRates,
		"imports":      contractImports,
		"unit of work": contractUnitOfWork,
	} {
		t.Run(name, func(t *testing.T) {
			t.Helper()
			fn(t, &fixture{store: newStore(t)})
		})
	}
}

type fixture struct{ store app.Store }

// do runs fn in a write transaction and fails the test on error.
func (f *fixture) do(t *testing.T, fn func(r app.Repos) error) {
	t.Helper()
	if err := f.store.WithTx(context.Background(), fn); err != nil {
		t.Fatalf("WithTx: %v", err)
	}
}

func mustEUR(t *testing.T) domain.Currency {
	t.Helper()
	c, err := domain.ParseCurrency("EUR")
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func mustMoney(t *testing.T, minor int64) domain.Money {
	t.Helper()
	m, err := domain.NewMoney(minor, mustEUR(t))
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func mustDate(t *testing.T, s string) domain.Date {
	t.Helper()
	d, err := domain.ParseDate(s)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func (f *fixture) account(t *testing.T, name string) domain.Account {
	t.Helper()
	var out domain.Account
	f.do(t, func(r app.Repos) (err error) {
		out, err = r.Accounts().Insert(context.Background(), domain.Account{
			Name: name, Kind: domain.AccountChecking, Currency: mustEUR(t),
			OpeningBalance: mustMoney(t, 0), OpenedOn: mustDate(t, "2026-01-01"),
		})
		return err
	})
	return out
}

func (f *fixture) tx(t *testing.T, base domain.Transaction) domain.Transaction {
	t.Helper()
	if base.Status == "" {
		base.Status, base.Kind = domain.StatusBooked, domain.KindPayment
	}
	var out domain.Transaction
	f.do(t, func(r app.Repos) (err error) {
		out, err = r.Transactions().Insert(context.Background(), base)
		return err
	})
	return out
}

func wantErr(t *testing.T, err error, target *domain.Error, what string) {
	t.Helper()
	if !errors.Is(err, target) {
		t.Errorf("%s: got error %v, want code %s", what, err, target.Code)
	}
}

func contractAccounts(t *testing.T, f *fixture) {
	ctx := context.Background()
	a := f.account(t, "Fineco")
	if a.ID == 0 {
		t.Fatal("Insert did not assign an ID")
	}
	b := f.account(t, "Cash")
	f.do(t, func(r app.Repos) error {
		got, err := r.Accounts().Get(ctx, a.ID)
		if err != nil || got != a {
			t.Errorf("Get = %+v, %v; want %+v", got, err, a)
		}
		_, err = r.Accounts().Insert(ctx, domain.Account{Name: "FINECO", Kind: a.Kind, Currency: a.Currency, OpeningBalance: a.OpeningBalance, OpenedOn: a.OpenedOn})
		wantErr(t, err, app.ErrAlreadyExists, "duplicate name (case-insensitive)")
		b.Name = "fineco"
		wantErr(t, r.Accounts().Update(ctx, b), app.ErrAlreadyExists, "rename onto an existing name")
		_, err = r.Accounts().Get(ctx, 999)
		wantErr(t, err, domain.ErrNotFound, "Get missing")
		wantErr(t, r.Accounts().Update(ctx, domain.Account{ID: 999, Name: "x"}), domain.ErrNotFound, "Update missing")
		return nil
	})
	a.Archived, a.Description = true, "old"
	f.do(t, func(r app.Repos) error {
		if err := r.Accounts().Update(ctx, a); err != nil {
			t.Fatalf("Update: %v", err)
		}
		all, err := r.Accounts().List(ctx)
		if err != nil || len(all) != 2 || all[0] != a || all[1].ID != b.ID {
			t.Errorf("List = %+v, %v; want both accounts (archived included) ordered by ID", all, err)
		}
		return nil
	})
	a.IBAN = "DE89370400440532013000"
	f.do(t, func(r app.Repos) error {
		if err := r.Accounts().Update(ctx, a); err != nil {
			t.Fatalf("Update with iban: %v", err)
		}
		got, err := r.Accounts().Get(ctx, a.ID)
		if err != nil || got != a {
			t.Errorf("Get after setting iban = %+v, %v; want %+v", got, err, a)
		}
		a.IBAN = ""
		if err := r.Accounts().Update(ctx, a); err != nil {
			t.Fatalf("Update clearing iban: %v", err)
		}
		if got, err := r.Accounts().Get(ctx, a.ID); err != nil || got != a {
			t.Errorf("Get after clearing iban = %+v, %v; want %+v", got, err, a)
		}
		return nil
	})
}

func contractCategories(t *testing.T, f *fixture) {
	ctx := context.Background()
	acct := f.account(t, "A")
	var food, other domain.Category
	f.do(t, func(r app.Repos) (err error) {
		if food, err = r.Categories().Insert(ctx, domain.Category{Name: "Food", Kind: domain.CategoryExpense}); err != nil {
			return err
		}
		other, err = r.Categories().Insert(ctx, domain.Category{Name: "Other", Kind: domain.CategoryExpense})
		return err
	})
	f.tx(t, domain.Transaction{AccountID: acct.ID, Date: mustDate(t, "2026-02-01"), Amount: mustMoney(t, -100), CategoryID: food.ID})
	f.tx(t, domain.Transaction{AccountID: acct.ID, Date: mustDate(t, "2026-02-02"), Amount: mustMoney(t, -200), CategoryID: food.ID})
	f.do(t, func(r app.Repos) error {
		_, err := r.Categories().Insert(ctx, domain.Category{Name: "food", Kind: domain.CategoryIncome})
		wantErr(t, err, app.ErrAlreadyExists, "duplicate name (case-insensitive)")
		if n, err := r.Categories().Usage(ctx, food.ID); err != nil || n != 2 {
			t.Errorf("Usage = %d, %v; want 2", n, err)
		}
		wantErr(t, r.Categories().Delete(ctx, food.ID), app.ErrInUse, "Delete of a used category")
		food.Name = "Groceries"
		if err := r.Categories().Update(ctx, food); err != nil {
			t.Errorf("Update: %v", err)
		}
		if got, err := r.Categories().Get(ctx, food.ID); err != nil || got.Name != "Groceries" {
			t.Errorf("Get after rename = %+v, %v", got, err)
		}
		if n, err := r.Categories().Reassign(ctx, food.ID, other.ID); err != nil || n != 2 {
			t.Errorf("Reassign = %d, %v; want 2", n, err)
		}
		if err := r.Categories().Delete(ctx, food.ID); err != nil {
			t.Errorf("Delete after reassign: %v", err)
		}
		_, err = r.Categories().Get(ctx, food.ID)
		wantErr(t, err, domain.ErrNotFound, "Get deleted")
		if all, _ := r.Categories().List(ctx); len(all) != 1 || all[0].ID != other.ID {
			t.Errorf("List = %+v, want only Other", all)
		}
		return nil
	})
}

func contractTransactions(t *testing.T, f *fixture) {
	ctx := context.Background()
	a, b := f.account(t, "A"), f.account(t, "B")
	tx := f.tx(t, domain.Transaction{
		AccountID: a.ID, Date: mustDate(t, "2026-03-01"), Amount: mustMoney(t, -1250),
		Description: "Coffee", ExternalID: "ext-1",
	})
	if tx.ID == 0 {
		t.Fatal("Insert did not assign an ID")
	}
	f.tx(t, domain.Transaction{AccountID: b.ID, Date: tx.Date, Amount: tx.Amount, ExternalID: "ext-1"}) // same id, other account: fine
	f.do(t, func(r app.Repos) error {
		got, err := r.Transactions().Get(ctx, tx.ID)
		if err != nil || got != tx {
			t.Errorf("Get = %+v, %v; want %+v", got, err, tx)
		}
		_, err = r.Transactions().Insert(ctx, domain.Transaction{
			AccountID: a.ID, Date: tx.Date, Amount: tx.Amount, Status: domain.StatusBooked, Kind: domain.KindPayment, ExternalID: "ext-1",
		})
		wantErr(t, err, app.ErrAlreadyExists, "duplicate external id on one account")
		tx.Description, tx.Status = "Espresso", domain.StatusReconciled
		if err := r.Transactions().Update(ctx, tx); err != nil {
			t.Fatalf("Update: %v", err)
		}
		if got, _ := r.Transactions().Get(ctx, tx.ID); got != tx {
			t.Errorf("Get after Update = %+v, want %+v", got, tx)
		}
		wantErr(t, r.Transactions().Update(ctx, domain.Transaction{ID: 999}), domain.ErrNotFound, "Update missing")
		if err := r.Transactions().Delete(ctx, tx.ID); err != nil {
			t.Fatalf("Delete: %v", err)
		}
		_, err = r.Transactions().Get(ctx, tx.ID)
		wantErr(t, err, domain.ErrNotFound, "Get deleted")
		wantErr(t, r.Transactions().Delete(ctx, tx.ID), domain.ErrNotFound, "Delete missing")
		return nil
	})
}

// seedList inserts 25 transactions on two accounts over 5 days and returns them newest first.
func seedList(t *testing.T, f *fixture) (a, b domain.Account, newest []domain.Transaction) {
	t.Helper()
	a, b = f.account(t, "A"), f.account(t, "B")
	var cat domain.Category
	f.do(t, func(r app.Repos) (err error) {
		cat, err = r.Categories().Insert(context.Background(), domain.Category{Name: "C", Kind: domain.CategoryExpense})
		return err
	})
	for i := range 25 {
		tx := domain.Transaction{
			AccountID: a.ID, Date: mustDate(t, fmt.Sprintf("2026-04-%02d", 1+i%5)),
			Amount: mustMoney(t, int64(-100*(i+1))), Description: fmt.Sprintf("Shop %d", i),
		}
		if i%2 == 1 {
			tx.AccountID, tx.CategoryID, tx.Status, tx.Kind = b.ID, cat.ID, domain.StatusPending, domain.KindPayment
		}
		newest = append([]domain.Transaction{f.tx(t, tx)}, newest...)
	}
	// Order is (date desc, id asc): the date sort of TxFilter.Less, descending.
	for i := range newest {
		for j := i + 1; j < len(newest); j++ {
			if newest[j].Date.After(newest[i].Date) || (newest[j].Date == newest[i].Date && newest[j].ID < newest[i].ID) {
				newest[i], newest[j] = newest[j], newest[i]
			}
		}
	}
	return a, b, newest
}

func filter(t *testing.T, spec domain.TxFilterSpec) domain.TxFilter {
	t.Helper()
	f, err := domain.NewTxFilter(spec)
	if err != nil {
		t.Fatalf("NewTxFilter: %v", err)
	}
	return f
}

func contractList(t *testing.T, f *fixture) {
	ctx := context.Background()
	a, b, newest := seedList(t, f)
	f.do(t, func(r app.Repos) error {
		var got []domain.Transaction
		var cur *app.TxCursor
		for pages := 0; ; pages++ {
			rows, err := r.Transactions().List(ctx, filter(t, domain.TxFilterSpec{Desc: true}), cur, 10)
			if err != nil {
				t.Fatalf("List: %v", err)
			}
			got = append(got, rows...)
			if len(rows) < 10 {
				break
			}
			last := rows[len(rows)-1]
			cur = &app.TxCursor{Date: last.Date, ID: last.ID}
			if pages > 5 {
				t.Fatal("pagination does not terminate")
			}
		}
		if len(got) != len(newest) {
			t.Fatalf("keyset pages returned %d rows, want %d", len(got), len(newest))
		}
		for i := range got {
			if got[i] != newest[i] {
				t.Fatalf("row %d = %+v, want %+v", i, got[i], newest[i])
			}
		}
		all, _ := r.Transactions().List(ctx, filter(t, domain.TxFilterSpec{}), nil, 0)
		if len(all) != 25 {
			t.Errorf("limit 0 returned %d rows, want all 25", len(all))
		}
		lo, hi := int64(300), int64(1000)
		cases := map[string]struct {
			f    domain.TxFilterSpec
			want int
		}{
			"period":   {domain.TxFilterSpec{From: mustDate(t, "2026-04-02"), To: mustDate(t, "2026-04-03")}, 10},
			"account":  {domain.TxFilterSpec{AccountIDs: []domain.AccountID{a.ID}}, 13},
			"accounts": {domain.TxFilterSpec{AccountIDs: []domain.AccountID{a.ID, b.ID}}, 25},
			"status":   {domain.TxFilterSpec{Statuses: []domain.TxStatus{domain.StatusPending}}, 12},
			"text":     {domain.TxFilterSpec{Text: "shop 1"}, 11}, // Shop 1, Shop 10..19
			"amount":   {domain.TxFilterSpec{Sign: domain.TxSignExpense, MinAbs: &lo, MaxAbs: &hi}, 8},
			"none":     {domain.TxFilterSpec{Text: "no such thing"}, 0},
		}
		for name, c := range cases {
			rows, err := r.Transactions().List(ctx, filter(t, c.f), nil, 0)
			if err != nil || len(rows) != c.want {
				t.Errorf("filter %s: %d rows, %v; want %d", name, len(rows), err, c.want)
			}
		}
		return nil
	})
}

func contractLookups(t *testing.T, f *fixture) {
	ctx := context.Background()
	a := f.account(t, "A")
	d := mustDate(t, "2026-05-10")
	one := f.tx(t, domain.Transaction{AccountID: a.ID, Date: d, Amount: mustMoney(t, -500), ExternalID: "x1", Fingerprint: "fp1", TransferGroupID: 7})
	f.tx(t, domain.Transaction{AccountID: a.ID, Date: mustDate(t, "2026-05-20"), Amount: mustMoney(t, -500), Fingerprint: "fp2"})
	f.tx(t, domain.Transaction{AccountID: a.ID, Date: d, Amount: mustMoney(t, 300)})
	f.do(t, func(r app.Repos) error {
		if got, _ := r.Transactions().ByExternalID(ctx, a.ID, "x1"); len(got) != 1 || got[0] != one {
			t.Errorf("ByExternalID = %+v", got)
		}
		if got, _ := r.Transactions().ByExternalID(ctx, a.ID, "nope"); len(got) != 0 {
			t.Errorf("ByExternalID(nope) = %+v, want none", got)
		}
		if got, _ := r.Transactions().ByFingerprint(ctx, "fp1"); len(got) != 1 || got[0] != one {
			t.Errorf("ByFingerprint = %+v", got)
		}
		if got, _ := r.Transactions().ByTransferGroup(ctx, 7); len(got) != 1 || got[0] != one {
			t.Errorf("ByTransferGroup = %+v", got)
		}
		near, _ := r.Transactions().NearAmount(ctx, a.ID, -500, mustDate(t, "2026-05-08"), mustDate(t, "2026-05-12"))
		if len(near) != 1 || near[0] != one {
			t.Errorf("NearAmount = %+v, want only the row within the window", near)
		}
		sum, err := r.Transactions().SumUpTo(ctx, a.ID, d)
		if err != nil || sum != -200 {
			t.Errorf("SumUpTo(d) = %d, %v; want -200", sum, err)
		}
		if sum, _ := r.Transactions().SumUpTo(ctx, a.ID, mustDate(t, "2026-12-31")); sum != -700 {
			t.Errorf("SumUpTo(end) = %d, want -700", sum)
		}
		if sum, _ := r.Transactions().SumUpTo(ctx, a.ID, mustDate(t, "2026-05-09")); sum != 0 {
			t.Errorf("SumUpTo(before) = %d, want 0", sum)
		}
		return nil
	})
}

func contractTransfers(t *testing.T, f *fixture) {
	ctx := context.Background()
	a, b := f.account(t, "A"), f.account(t, "B")
	var g domain.TransferGroup
	f.do(t, func(r app.Repos) (err error) {
		g, err = r.Transfers().Insert(ctx, domain.TransferGroup{Fee: mustMoney(t, 150)})
		return err
	})
	if g.ID == 0 {
		t.Fatal("Insert did not assign an ID")
	}
	out := f.tx(t, domain.Transaction{AccountID: a.ID, Date: mustDate(t, "2026-06-01"), Amount: mustMoney(t, -10150), Kind: domain.KindTransfer, TransferGroupID: g.ID})
	in := f.tx(t, domain.Transaction{AccountID: b.ID, Date: mustDate(t, "2026-06-01"), Amount: mustMoney(t, 10000), Kind: domain.KindTransfer, TransferGroupID: g.ID})
	f.do(t, func(r app.Repos) error {
		got, err := r.Transfers().Get(ctx, g.ID)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if got.Out.TransactionID != out.ID || got.In.TransactionID != in.ID || got.Out.AccountID != a.ID ||
			got.In.AccountID != b.ID || got.Out.Amount != out.Amount || got.In.Amount != in.Amount || got.Fee != g.Fee {
			t.Errorf("Get = %+v, want legs %d/%d and fee 150", got, out.ID, in.ID)
		}
		if err := r.Transfers().Delete(ctx, g.ID); err != nil {
			t.Errorf("Delete: %v", err)
		}
		_, err = r.Transfers().Get(ctx, g.ID)
		wantErr(t, err, domain.ErrNotFound, "Get deleted")
		wantErr(t, r.Transfers().Delete(ctx, g.ID), domain.ErrNotFound, "Delete missing")
		return nil
	})
}

func contractFXRates(t *testing.T, f *fixture) {
	ctx := context.Background()
	usd, err := domain.ParseCurrency("USD")
	if err != nil {
		t.Fatal(err)
	}
	gbp, _ := domain.ParseCurrency("GBP")
	rate := func(c domain.Currency, day string, scaled int64) domain.FXRate {
		r, err := domain.NewRate(scaled)
		if err != nil {
			t.Fatal(err)
		}
		fx, err := domain.NewFXRate(c, mustDate(t, day), r)
		if err != nil {
			t.Fatal(err)
		}
		return fx
	}
	f.do(t, func(r app.Repos) error {
		for _, fx := range []domain.FXRate{
			rate(usd, "2026-01-10", 900_000_000), rate(usd, "2026-02-10", 920_000_000), rate(gbp, "2026-01-05", 1_170_000_000),
			rate(usd, "2026-01-10", 910_000_000), // replaces the first
		} {
			if err := r.FXRates().Upsert(ctx, fx); err != nil {
				t.Fatalf("Upsert: %v", err)
			}
		}
		all, _ := r.FXRates().List(ctx, domain.Currency{})
		if len(all) != 3 || all[0].Currency != gbp || all[1].Rate.Scaled() != 910_000_000 || all[2].Date != mustDate(t, "2026-02-10") {
			t.Errorf("List(all) = %+v, want gbp, usd Jan (replaced), usd Feb", all)
		}
		if only, _ := r.FXRates().List(ctx, usd); len(only) != 2 {
			t.Errorf("List(usd) = %+v, want 2 rows", only)
		}
		for day, want := range map[string]int64{"2026-01-10": 910_000_000, "2026-02-09": 910_000_000, "2026-03-01": 920_000_000} {
			got, err := r.FXRates().OnOrBefore(ctx, usd, mustDate(t, day))
			if err != nil || got.Rate.Scaled() != want {
				t.Errorf("OnOrBefore(%s) = %+v, %v; want %d", day, got, err, want)
			}
		}
		_, err := r.FXRates().OnOrBefore(ctx, usd, mustDate(t, "2026-01-09"))
		wantErr(t, err, domain.ErrNotFound, "OnOrBefore before the first rate")
		return nil
	})
}

func contractImports(t *testing.T, f *fixture) {
	ctx := context.Background()
	a := f.account(t, "A")
	at := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
	var b1, b2 app.ImportBatch
	f.do(t, func(r app.Repos) (err error) {
		if b1, err = r.Imports().InsertBatch(ctx, app.ImportBatch{Connector: "csv", Source: "a.csv", SourceSHA256: "aa", ImportedAt: at}); err != nil {
			return err
		}
		b2, err = r.Imports().InsertBatch(ctx, app.ImportBatch{Connector: "csv", Source: "b.csv", SourceSHA256: "bb", ImportedAt: at.Add(time.Hour)})
		return err
	})
	tx := f.tx(t, domain.Transaction{AccountID: a.ID, Date: mustDate(t, "2026-07-01"), Amount: mustMoney(t, -1), ImportBatchID: b1.ID})
	f.do(t, func(r app.Repos) error {
		if got, err := r.Imports().Batch(ctx, b1.ID); err != nil || !got.ImportedAt.Equal(at) || got.Source != "a.csv" {
			t.Errorf("Batch = %+v, %v", got, err)
		}
		if got, err := r.Imports().BatchBySHA256(ctx, "bb"); err != nil || got.ID != b2.ID {
			t.Errorf("BatchBySHA256 = %+v, %v", got, err)
		}
		_, err := r.Imports().BatchBySHA256(ctx, "zz")
		wantErr(t, err, domain.ErrNotFound, "BatchBySHA256 unknown")
		if all, _ := r.Imports().Batches(ctx); len(all) != 2 || all[0].ID != b2.ID {
			t.Errorf("Batches = %+v, want most recent first", all)
		}
		if rows, _ := r.Transactions().List(ctx, filter(t, domain.TxFilterSpec{ImportBatchID: b1.ID}), nil, 0); len(rows) != 1 || rows[0].ID != tx.ID {
			t.Errorf("List by batch = %+v", rows)
		}
		_, err = r.Imports().AccountRef(ctx, "csv", "ref-1")
		wantErr(t, err, domain.ErrNotFound, "AccountRef on first use")
		if err := r.Imports().SetAccountRef(ctx, "csv", "ref-1", a.ID); err != nil {
			t.Fatalf("SetAccountRef: %v", err)
		}
		if id, err := r.Imports().AccountRef(ctx, "csv", "ref-1"); err != nil || id != a.ID {
			t.Errorf("AccountRef = %d, %v; want %d", id, err, a.ID)
		}
		if err := r.Imports().DeleteBatch(ctx, b1.ID); err != nil {
			t.Errorf("DeleteBatch: %v", err)
		}
		wantErr(t, r.Imports().DeleteBatch(ctx, b1.ID), domain.ErrNotFound, "DeleteBatch missing")
		return nil
	})
}

func contractUnitOfWork(t *testing.T, f *fixture) {
	ctx := context.Background()
	boom := errors.New("boom")
	err := f.store.WithTx(ctx, func(r app.Repos) error {
		if _, err := r.Categories().Insert(ctx, domain.Category{Name: "Gone", Kind: domain.CategoryExpense}); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("WithTx error = %v, want the callback's error", err)
	}
	f.do(t, func(r app.Repos) error {
		if cats, _ := r.Categories().List(ctx); len(cats) != 0 {
			t.Errorf("a failed WithTx left %+v behind", cats)
		}
		_, err := r.Categories().Insert(ctx, domain.Category{Name: "Kept", Kind: domain.CategoryExpense})
		return err
	})
	err = f.store.Read(ctx, func(r app.Repos) error {
		cats, _ := r.Categories().List(ctx)
		if len(cats) != 1 {
			t.Errorf("committed WithTx not visible to Read: %+v", cats)
		}
		_, err := r.Categories().Insert(ctx, domain.Category{Name: "Nope", Kind: domain.CategoryExpense})
		return err
	})
	if err == nil {
		t.Error("a write inside Read must fail")
	}
}
