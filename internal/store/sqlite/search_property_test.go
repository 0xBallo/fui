package sqlite

import (
	"context"
	"math"
	"slices"
	"strconv"
	"testing"
	"time"

	"pgregory.net/rapid"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/testutil/memstore"
	"github.com/0xballo/fui/internal/textnorm"
)

var (
	searchWords  = []string{"Caffè", "ÉCLAIR", "shop", "Esselunga", "pizza", " Rent ", "à", "ab", "a_b", "50%", `say "hi"`, "Zürich"}
	searchKinds  = []domain.TxKind{domain.KindPayment, domain.KindTransfer, domain.KindFee, domain.KindRefund, domain.KindOther}
	searchStatus = []domain.TxStatus{domain.StatusPending, domain.StatusBooked, domain.StatusReconciled}
)

// seedSearchRefs creates the rows transactions refer to; both stores assign the same IDs.
func seedSearchRefs(t *testing.T, s app.Store) {
	t.Helper()
	ctx := context.Background()
	eur := mustCurrency(t, "EUR")
	err := s.WithTx(ctx, func(r app.Repos) error {
		for _, n := range []string{"A", "B"} {
			if _, err := r.Accounts().Insert(ctx, domain.Account{
				Name: n, Kind: domain.AccountChecking, Currency: eur,
				OpeningBalance: mustMoney(t, 0, eur), OpenedOn: mustDate(t, "2026-01-01"),
			}); err != nil {
				return err
			}
			if _, err := r.Categories().Insert(ctx, domain.Category{Name: "cat" + n, Kind: domain.CategoryExpense}); err != nil {
				return err
			}
			if _, err := r.Imports().InsertBatch(ctx, app.ImportBatch{Connector: "x", Source: n, SourceSHA256: n, ImportedAt: time.Unix(0, 0).UTC()}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("seed refs: %v", err)
	}
}

func drawTxs(t *rapid.T, eur domain.Currency) []domain.Transaction {
	day := baseDay()
	amount := rapid.OneOf(rapid.Int64Range(-2000, 2000), rapid.SampledFrom([]int64{0, math.MinInt64, math.MaxInt64, -1, 1}))
	text := rapid.SliceOfN(rapid.SampledFrom(searchWords), 1, 3)
	return rapid.SliceOfN(rapid.Custom(func(t *rapid.T) domain.Transaction {
		d, _ := day.AddDays(rapid.IntRange(0, 6).Draw(t, "day"))
		m, err := domain.NewMoney(amount.Draw(t, "amount"), eur)
		if err != nil {
			t.Fatalf("NewMoney: %v", err)
		}
		tx := domain.Transaction{
			AccountID: domain.AccountID(rapid.IntRange(1, 2).Draw(t, "account")),
			Date:      d, Amount: m,
			Description: joinWords(text.Draw(t, "description")), Notes: joinWords(text.Draw(t, "notes")),
			PayeeID: domain.PayeeID(rapid.IntRange(0, 2).Draw(t, "payee")),
			Status:  rapid.SampledFrom(searchStatus).Draw(t, "status"),
			Kind:    rapid.SampledFrom(searchKinds).Draw(t, "kind"),
		}
		if c := rapid.IntRange(0, 2).Draw(t, "category"); c > 0 {
			tx.CategoryID = domain.CategoryID(c)
		}
		if b := rapid.IntRange(0, 2).Draw(t, "batch"); b > 0 {
			tx.ImportBatchID = domain.ImportBatchID(b)
		}
		return tx
	}), 0, 40).Draw(t, "txs")
}

func joinWords(w []string) string {
	out := ""
	for i, s := range w {
		if i > 0 {
			out += "  "
		}
		out += s
	}
	return out
}

// maybe is true about a third of the time, so most filters constrain only a few dimensions and
// results stay non-empty.
func maybe(t *rapid.T, label string) bool { return rapid.IntRange(0, 2).Draw(t, label) == 0 }

func subset[T any](t *rapid.T, label string, all []T) []T {
	if !maybe(t, label) {
		return nil
	}
	var out []T
	for i, v := range all {
		if rapid.Bool().Draw(t, label+strconv.Itoa(i)) {
			out = append(out, v)
		}
	}
	return out
}

func drawFilter(t *rapid.T, txs []domain.Transaction) domain.TxFilterSpec {
	day := baseDay()
	spec := domain.TxFilterSpec{
		Uncategorized: maybe(t, "uncategorized"),
		AccountIDs:    subset(t, "acct", []domain.AccountID{1, 2}),
		CategoryIDs:   subset(t, "cat", []domain.CategoryID{1, 2}),
		PayeeIDs:      subset(t, "payee", []domain.PayeeID{1, 2}),
		Statuses:      subset(t, "status", searchStatus),
		Kinds:         subset(t, "kind", searchKinds),
		Sort:          rapid.SampledFrom([]domain.TxSort{"", domain.TxSortDate, domain.TxSortAmount, domain.TxSortPayee}).Draw(t, "sort"),
		Desc:          rapid.Bool().Draw(t, "desc"),
		Limit:         rapid.SampledFrom([]int{0, 0, 0, 1, 3, 7}).Draw(t, "limit"),
	}
	if maybe(t, "sign") {
		spec.Sign = rapid.SampledFrom([]domain.TxSign{"", domain.TxSignAny, domain.TxSignIncome, domain.TxSignExpense}).Draw(t, "signValue")
	}
	if maybe(t, "batch") {
		spec.ImportBatchID = domain.ImportBatchID(rapid.IntRange(1, 2).Draw(t, "batchFilter"))
	}
	if maybe(t, "period") {
		spec.From, _ = day.AddDays(rapid.IntRange(0, 6).Draw(t, "from"))
		spec.To, _ = spec.From.AddDays(rapid.IntRange(0, 6).Draw(t, "span"))
	}
	bound := rapid.OneOf(rapid.Int64Range(0, 2500), rapid.SampledFrom([]int64{math.MaxInt64, 1}))
	if maybe(t, "hasMin") {
		v := bound.Draw(t, "min")
		spec.MinAbs = &v
	}
	if maybe(t, "hasMax") {
		v := bound.Draw(t, "max")
		if spec.MinAbs != nil && v < *spec.MinAbs {
			v = *spec.MinAbs
		}
		spec.MaxAbs = &v
	}
	spec.Text = drawSearchText(t, txs)
	return spec
}

// drawSearchText mixes substrings of real rows (hits, 1–6 runes, so both the instr and the FTS
// paths) with arbitrary text over an alphabet of FTS and LIKE metacharacters (usually misses).
func drawSearchText(t *rapid.T, txs []domain.Transaction) string {
	if rapid.Bool().Draw(t, "noText") || len(txs) == 0 {
		return ""
	}
	var raw string
	if rapid.IntRange(0, 3).Draw(t, "fromRow") > 0 {
		tx := rapid.SampledFrom(txs).Draw(t, "row")
		runes := []rune(textnorm.SearchText(tx.Description, "", tx.Notes))
		if len(runes) == 0 {
			return ""
		}
		start := rapid.IntRange(0, len(runes)-1).Draw(t, "start")
		raw = string(runes[start:min(len(runes), start+rapid.IntRange(1, 6).Draw(t, "len"))])
	} else {
		raw = rapid.StringOfN(rapid.SampledFrom([]rune(`ab_%" *-é`)), 1, 5, -1).Draw(t, "raw")
	}
	return textnorm.Normalize(raw) // the app layer normalizes the typed text the same way
}

func fingerprints(rows []domain.Transaction) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.Fingerprint
	}
	return out
}

// TestPropertySearchMatchesOracle checks the SQL translation of TxFilter against memstore, which
// runs TxFilter.Match and TxFilter.Less: same rows, same order, same keyset pages.
func TestPropertySearchMatchesOracle(t *testing.T) {
	store, _ := newTestStore(t)
	seedSearchRefs(t, store)
	eur := mustCurrency(t, "EUR")
	ctx := context.Background()

	rapid.Check(t, func(rt *rapid.T) {
		txs := drawTxs(rt, eur)
		oracle := memstore.New()
		seedSearchRefs(t, oracle)
		for i := range txs {
			txs[i].Fingerprint = strconv.Itoa(i) // insertion order, to compare rows across stores
		}
		load := func(s app.Store) {
			if err := s.WithTx(ctx, func(r app.Repos) error {
				old, err := r.Transactions().List(ctx, mustFilter(t, domain.TxFilterSpec{}), nil, 0)
				if err != nil {
					return err
				}
				for _, o := range old {
					if err := r.Transactions().Delete(ctx, o.ID); err != nil {
						return err
					}
				}
				for _, tx := range txs {
					if _, err := r.Transactions().Insert(ctx, tx); err != nil {
						return err
					}
				}
				return nil
			}); err != nil {
				rt.Fatalf("load: %v", err)
			}
		}
		load(store)
		load(oracle)

		spec := drawFilter(rt, txs)
		f, err := domain.NewTxFilter(spec)
		if err != nil {
			rt.Fatalf("NewTxFilter(%+v): %v", spec, err)
		}
		list := func(s app.Store, after *app.TxCursor) []domain.Transaction {
			var out []domain.Transaction
			if err := s.Read(ctx, func(r app.Repos) (err error) {
				out, err = r.Transactions().List(ctx, f, after, 0)
				return err
			}); err != nil {
				rt.Fatalf("List: %v", err)
			}
			return out
		}
		want, got := list(oracle, nil), list(store, nil)
		if !slices.Equal(fingerprints(got), fingerprints(want)) {
			rt.Fatalf("filter %+v\n sql:    %v\n oracle: %v", spec, fingerprints(got), fingerprints(want))
		}
		if len(want) > 1 && f.Sort() == domain.TxSortDate {
			cur := &app.TxCursor{Date: want[0].Date, ID: want[0].ID}
			// IDs differ across stores only if insertion differs; they do not, but use each store's own row.
			wantPage := list(oracle, cur)
			gotCur := &app.TxCursor{Date: got[0].Date, ID: got[0].ID}
			if g := list(store, gotCur); !slices.Equal(fingerprints(g), fingerprints(wantPage)) {
				rt.Fatalf("filter %+v after %v\n sql:    %v\n oracle: %v", spec, cur, fingerprints(g), fingerprints(wantPage))
			}
		}
	})
}

func mustFilter(t *testing.T, spec domain.TxFilterSpec) domain.TxFilter {
	t.Helper()
	f, err := domain.NewTxFilter(spec)
	if err != nil {
		t.Fatalf("NewTxFilter: %v", err)
	}
	return f
}

func baseDay() domain.Date {
	d, err := domain.ParseDate("2026-03-01")
	if err != nil {
		panic(err) // constant input
	}
	return d
}
