package importing_test

import (
	"bytes"
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"

	"pgregory.net/rapid"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/importing"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/pkg/fuiproto"
)

var (
	words      = []string{"coffee", "rent", "salary", "market", "fuel", "gift"}
	catNames   = []string{"Food", "Fun", "Pay"}
	payeeNames = []string{"Market Co", "Landlord", "Employer"}
	tagNames   = []string{"vacation", "recurring", "shared"}
	accKinds   = []fuiproto.AccountKind{"checking", "card", "savings", "investment", "cash", "loan"}
	plainKinds = []fuiproto.TransactionKind{"payment", "fee", "interest", "refund", "other"}
)

// drawStream draws a valid stream over 1–3 accounts (refs r0.., names Acct0..) with unique
// external ids, some transfer pairs and, optionally, categories.
func drawStream(t *rapid.T, withCategories bool) (sliceSource, int) {
	nAcct := rapid.IntRange(1, 3).Draw(t, "accounts")
	recs := make([]fuiproto.Record, 0, 40)
	for i := range nAcct {
		kind := rapid.SampledFrom(accKinds).Draw(t, "account kind")
		recs = append(recs, account(fmt.Sprintf("r%d", i), fmt.Sprintf("Acct%d", i), "EUR",
			func(a *fuiproto.AccountRecord) { a.Kind = kind }))
	}
	n := 0
	ext := func() string { n++; return fmt.Sprintf("x%d", n) }
	day := rapid.IntRange(1, 28)
	for range rapid.IntRange(0, 25).Draw(t, "transactions") {
		desc := strings.Join(rapid.SliceOfN(rapid.SampledFrom(words), 1, 3).Draw(t, "words"), " ")
		cat, payee, txTags := "", "", []string(nil)
		if withCategories {
			cat = rapid.SampledFrom(append([]string{""}, catNames...)).Draw(t, "category")
			payee = rapid.SampledFrom(append([]string{""}, payeeNames...)).Draw(t, "payee")
			txTags = rapid.SliceOfN(rapid.SampledFrom(tagNames), 0, len(tagNames)).Draw(t, "tags")
		}
		recs = append(recs, txRec(fmt.Sprintf("r%d", rapid.IntRange(0, nAcct-1).Draw(t, "account")), ext(),
			fmt.Sprintf("2026-09-%02d", day.Draw(t, "day")), rapid.Int64Range(-100000, 100000).Draw(t, "amount"), desc,
			kind(rapid.SampledFrom(plainKinds).Draw(t, "kind")), category(cat), payeeName(payee), withTags(txTags...),
			func(r *fuiproto.TransactionRecord) {
				r.Notes = rapid.SampledFrom([]string{"", "note"}).Draw(t, "notes")
				r.Status = rapid.SampledFrom([]fuiproto.TransactionStatus{"booked", "pending"}).Draw(t, "status")
			}))
	}
	if nAcct >= 2 {
		for i := range rapid.IntRange(0, 3).Draw(t, "transfers") {
			from := rapid.IntRange(0, nAcct-1).Draw(t, "from")
			to := (from + rapid.IntRange(1, nAcct-1).Draw(t, "shift")) % nAcct
			amount := rapid.Int64Range(1, 100000).Draw(t, "transfer amount")
			fee := rapid.Int64Range(0, 500).Draw(t, "fee")
			l, d := label(fmt.Sprintf("t%d", i)), fmt.Sprintf("2026-09-%02d", day.Draw(t, "transfer day"))
			recs = append(recs,
				txRec(fmt.Sprintf("r%d", from), ext(), d, -(amount+fee), "transfer out", kind("transfer"), l),
				txRec(fmt.Sprintf("r%d", to), ext(), d, amount, "transfer in", kind("transfer"), l))
		}
	}
	return stream("test", "", recs...), nAcct
}

func TestPropertyImportIsIdempotent(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		src, _ := drawStream(t, true)
		e := newEnv(nil)
		first := e.importAll(t, src)
		snap := e.store.Snapshot()
		second := e.plan(t, src, createAccounts)
		if c := second.Counts(); c.Included != 0 || c.Duplicate != first.Transactions {
			t.Fatalf("second plan counts = %+v after importing %d", c, first.Transactions)
		}
		res := must(e.svc.ApplyImport(context.Background(), second))
		if res.Batch.ID != 0 || e.store.Snapshot() != snap {
			t.Fatalf("importing the same file twice changed the archive: %+v", res)
		}
	})
}

func TestPropertyUndoRestoresState(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		src, nAcct := drawStream(t, false)
		e := newEnv(nil)
		for i := range nAcct {
			e.seedAccount(t, fmt.Sprintf("Acct%d", i), "test", fmt.Sprintf("r%d", i))
		}
		before := snapshot(e.store)
		res := must(e.svc.ApplyImport(context.Background(), e.plan(t, src)))
		if res.Transactions == 0 {
			return
		}
		plan := must(e.svc.PlanUndo(context.Background(), res.Batch.ID))
		if err := e.svc.ApplyUndo(context.Background(), plan); err != nil {
			t.Fatalf("undo: %v", err)
		}
		if after := snapshot(e.store); after != before {
			t.Fatalf("undo did not restore the previous state\nbefore %s\nafter  %s", before, after)
		}
	})
}

func TestPropertyFUIExportRoundTrips(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		src, _ := drawStream(t, true)
		a := newEnv(nil)
		a.importAll(t, src)
		var buf bytes.Buffer
		n := must(a.svc.Export(context.Background(), importing.ExportRequest{Format: importing.FormatFUI, Filter: domain.TxFilter{}}, &buf))

		b := newEnv(nil)
		file := fuiSource(t, buf.Bytes())
		res := must(b.svc.ApplyImport(context.Background(), b.plan(t, file, createAccounts)))
		if res.Transactions != n {
			t.Fatalf("reimported %d of %d exported transactions", res.Transactions, n)
		}
		if got, want := b.view(t), a.view(t); !slices.Equal(got, want) {
			t.Fatalf("round trip differs\n got: %q\nwant: %q", got, want)
		}
		// The same file into the archive it came from creates nothing.
		again := must(a.svc.ApplyImport(context.Background(), a.plan(t, file)))
		if again.Transactions != 0 {
			t.Fatalf("reimport into the source archive created %d transactions", again.Transactions)
		}
	})
}

func fuiSource(t tb, data []byte) importing.Source {
	t.Helper()
	var recs []fuiproto.Record
	for r, err := range fuiproto.DecodeJSONL(bytes.NewReader(data)) {
		if err != nil {
			t.Fatalf("decode export: %v", err)
		}
		recs = append(recs, r)
	}
	return sliceSource{recs: recs}
}

// view lists the accounts with transactions (with balance), the categories in use and the transactions with the
// transfer they belong to, in a form that does not depend on IDs.
func (e env) view(t tb) []string {
	t.Helper()
	var out []string
	err := e.store.Read(context.Background(), func(r app.Repos) error {
		ctx := context.Background()
		accts := must(r.Accounts().List(ctx))
		cats := must(r.Categories().List(ctx))
		txs := must(r.Transactions().List(ctx, must(domain.NewTxFilter(domain.TxFilterSpec{Desc: true})), nil, 0))
		name := map[domain.AccountID]string{}
		used := map[domain.AccountID]bool{}
		for _, tx := range txs {
			used[tx.AccountID] = true
		}
		for _, a := range accts {
			name[a.ID] = a.Name
			if !used[a.ID] {
				continue // a FUI export carries the accounts that have transactions
			}
			sum := must(r.Transactions().SumUpTo(ctx, a.ID, date("2026-11-11")))
			out = append(out, fmt.Sprintf("account %s %s %s balance=%d", a.Name, a.Kind, a.Currency, a.OpeningBalance.Minor()+sum))
		}
		catName := map[domain.CategoryID]string{0: ""}
		for _, c := range cats {
			catName[c.ID] = c.Name
		}
		payees := must(r.Payees().List(ctx))
		payeeOf := map[domain.PayeeID]string{0: ""}
		for _, p := range payees {
			payeeOf[p.ID] = p.Name
		}
		lines := map[domain.TransactionID]string{}
		legs := map[domain.TransferGroupID][]string{}
		for _, tx := range txs {
			tagsOf := must(r.Tags().TagsOf(ctx, tx.ID))
			tagList := make([]string, len(tagsOf))
			for i, tg := range tagsOf {
				tagList[i] = tg.Name
			}
			slices.Sort(tagList)
			lines[tx.ID] = fmt.Sprintf("tx %s %s %d %q %q %q %s %s %s payee=%q tags=%q", name[tx.AccountID], tx.Date, tx.Amount.Minor(),
				tx.Description, catName[tx.CategoryID], tx.Notes, tx.Status, tx.Kind, tx.ExternalID,
				payeeOf[tx.PayeeID], strings.Join(tagList, ","))
			if tx.TransferGroupID != 0 {
				legs[tx.TransferGroupID] = append(legs[tx.TransferGroupID], lines[tx.ID])
			}
		}
		for _, tx := range txs {
			l := lines[tx.ID]
			if g := legs[tx.TransferGroupID]; tx.TransferGroupID != 0 {
				sorted := slices.Sorted(slices.Values(g))
				l += " linked=" + strings.Join(sorted, " ~ ")
			}
			out = append(out, l)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("view: %v", err)
	}
	slices.Sort(out)
	return out
}
