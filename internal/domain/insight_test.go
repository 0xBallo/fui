package domain

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"pgregory.net/rapid"
)

func expense(t tb, id int64, d Date, minor int64, payee PayeeID, cat CategoryID) Transaction {
	return Transaction{
		ID: TransactionID(id), AccountID: 1, Date: d, Amount: money(t, -minor, "EUR"),
		PayeeID: payee, CategoryID: cat, Status: StatusBooked, Kind: KindPayment,
	}
}

func monthly(t tb, n int, start Date, minor func(i int) int64, payee PayeeID) []Transaction {
	var out []Transaction
	for i := range n {
		d, err := start.AddMonths(i)
		if err != nil {
			t.Fatalf("AddMonths: %v", err)
		}
		out = append(out, expense(t, int64(100+i), d, minor(i), payee, 5))
	}
	return out
}

func TestDetectAnomalies(t *testing.T) {
	cfg := DefaultInsightConfig()
	per, _ := NewPeriod(date(t, 2026, 6, 1), date(t, 2026, 6, 30))
	hist := func() []Transaction {
		return []Transaction{
			expense(t, 1, date(t, 2026, 3, 1), 1000, 7, 5),
			expense(t, 2, date(t, 2026, 4, 1), 1000, 7, 5),
			expense(t, 3, date(t, 2026, 5, 1), 1000, 7, 5),
		}
	}
	tests := []struct {
		name  string
		extra []Transaction
		want  int
	}{
		{"at threshold", []Transaction{expense(t, 9, date(t, 2026, 6, 10), 2500, 7, 5)}, 1},
		{"below threshold", []Transaction{expense(t, 9, date(t, 2026, 6, 10), 2499, 7, 5)}, 0},
		{"transfer ignored", []Transaction{func() Transaction {
			x := expense(t, 9, date(t, 2026, 6, 10), 9000, 7, 5)
			x.Kind = KindTransfer
			return x
		}()}, 0},
		{"outside period", []Transaction{expense(t, 9, date(t, 2026, 7, 10), 9000, 7, 5)}, 0},
		{"no subject", []Transaction{expense(t, 9, date(t, 2026, 6, 10), 9000, 0, 0)}, 0},
		{"income ignored", []Transaction{{ID: 9, Date: date(t, 2026, 6, 10), Amount: money(t, 9000, "EUR"), PayeeID: 7}}, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := DetectAnomalies(cfg, per, append(hist(), tc.extra...))
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != tc.want {
				t.Fatalf("got %d insights, want %d: %+v", len(got), tc.want, got)
			}
		})
	}
	t.Run("short history", func(t *testing.T) {
		got, err := DetectAnomalies(cfg, per, append(hist()[:2], expense(t, 9, date(t, 2026, 6, 10), 9000, 7, 5)))
		if err != nil || len(got) != 0 {
			t.Fatalf("got %v, %v", got, err)
		}
	})
	t.Run("history outside window", func(t *testing.T) {
		old := []Transaction{
			expense(t, 1, date(t, 2025, 1, 1), 1000, 7, 5),
			expense(t, 2, date(t, 2025, 2, 1), 1000, 7, 5),
			expense(t, 3, date(t, 2025, 3, 1), 1000, 7, 5),
			expense(t, 9, date(t, 2026, 6, 10), 9000, 7, 5),
		}
		got, _ := DetectAnomalies(cfg, per, old)
		if len(got) != 0 {
			t.Fatalf("got %v", got)
		}
	})
}

func TestDetectSubscriptions(t *testing.T) {
	cfg := DefaultInsightConfig()
	start := date(t, 2026, 1, 15)
	fixed := func(int) int64 { return 999 }
	tests := []struct {
		name string
		txs  []Transaction
		want []InsightKind
	}{
		{"monthly", monthly(t, 3, start, fixed, 3), []InsightKind{InsightSubscript}},
		{"two occurrences", monthly(t, 2, start, fixed, 3), nil},
		{"within tolerance", monthly(t, 3, start, func(i int) int64 { return []int64{1000, 1050, 950}[i] }, 3), []InsightKind{InsightSubscript}},
		{"last beyond tolerance, unknown", monthly(t, 4, start, func(i int) int64 { return []int64{1000, 1000, 1000, 1051}[i] }, 3), nil},
		{"irregular", []Transaction{
			expense(t, 1, date(t, 2026, 1, 1), 1000, 3, 5),
			expense(t, 2, date(t, 2026, 1, 20), 1000, 3, 5),
			expense(t, 3, date(t, 2026, 3, 9), 1000, 3, 5),
		}, nil},
		{"weekly", []Transaction{
			expense(t, 1, date(t, 2026, 1, 1), 500, 3, 5),
			expense(t, 2, date(t, 2026, 1, 8), 500, 3, 5),
			expense(t, 3, date(t, 2026, 1, 15), 500, 3, 5),
		}, []InsightKind{InsightSubscript}},
		{"yearly", []Transaction{
			expense(t, 1, date(t, 2023, 5, 1), 5000, 3, 5),
			expense(t, 2, date(t, 2024, 5, 1), 5000, 3, 5),
			expense(t, 3, date(t, 2025, 5, 1), 5000, 3, 5),
		}, []InsightKind{InsightSubscript}},
		{"mixed intervals", []Transaction{
			expense(t, 1, date(t, 2026, 1, 1), 500, 3, 5),
			expense(t, 2, date(t, 2026, 1, 8), 500, 3, 5),
			expense(t, 3, date(t, 2026, 2, 8), 500, 3, 5),
		}, nil},
		{"same day", []Transaction{
			expense(t, 1, date(t, 2026, 1, 1), 500, 3, 5),
			expense(t, 2, date(t, 2026, 1, 1), 500, 3, 5),
			expense(t, 3, date(t, 2026, 2, 1), 500, 3, 5),
		}, nil},
		{"unstable history", monthly(t, 3, start, func(i int) int64 { return []int64{1000, 2000, 1000}[i] }, 3), nil},
		{"known, same price", withRecurrence(monthly(t, 3, start, fixed, 3)), nil},
		{"known, price change", withRecurrence(monthly(t, 4, start, func(i int) int64 { return []int64{1000, 1000, 1000, 1200}[i] }, 3)), []InsightKind{InsightPriceChange}},
		{"description subject", func() []Transaction {
			txs := monthly(t, 3, start, fixed, 0)
			for i := range txs {
				txs[i].CategoryID = 0
				txs[i].Description = " Netflix "
			}
			return txs
		}(), []InsightKind{InsightSubscript}},
		{"no subject", func() []Transaction {
			txs := monthly(t, 3, start, fixed, 0)
			for i := range txs {
				txs[i].CategoryID = 0
			}
			return txs
		}(), nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := DetectSubscriptions(cfg, tc.txs)
			if err != nil {
				t.Fatal(err)
			}
			var kinds []InsightKind
			for _, g := range got {
				kinds = append(kinds, g.Kind)
			}
			if diff := cmp.Diff(tc.want, kinds); diff != "" {
				t.Fatalf("kinds mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func withRecurrence(txs []Transaction) []Transaction {
	txs[0].RecurrenceID = 1
	return txs
}

func TestDetectTrends(t *testing.T) {
	cfg := DefaultInsightConfig()
	cur, _ := NewPeriod(date(t, 2026, 6, 1), date(t, 2026, 6, 30))
	prev := func(each int64) []Transaction {
		return []Transaction{
			expense(t, 1, date(t, 2026, 3, 5), each, 0, 5),
			expense(t, 2, date(t, 2026, 4, 5), each, 0, 5),
			expense(t, 3, date(t, 2026, 5, 5), each, 0, 5),
		}
	}
	tests := []struct {
		name    string
		prev    int64
		current int64
		want    string
	}{
		{"up at threshold", 10000, 12500, "up"},
		{"up below threshold", 10000, 12499, ""},
		{"down at threshold", 10000, 7500, "down"},
		{"down below threshold", 10000, 7501, ""},
		{"below minimum", 1000, 1250, ""},
		{"new spending", 0, 6000, "up"},
		{"vanished spending", 6000, 0, "down"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			txs := prev(tc.prev)
			if tc.current > 0 {
				txs = append(txs, expense(t, 9, date(t, 2026, 6, 3), tc.current, 0, 5))
			}
			if tc.prev == 0 {
				txs = txs[:0]
				txs = append(txs, expense(t, 9, date(t, 2026, 6, 3), tc.current, 0, 5))
			}
			got, err := DetectTrends(cfg, cur, txs)
			if err != nil {
				t.Fatal(err)
			}
			if tc.want == "" {
				if len(got) != 0 {
					t.Fatalf("got %+v", got)
				}
				return
			}
			if len(got) != 1 || got[0].Kind != InsightTrend || !strings.Contains(got[0].Detail, "direction="+tc.want) {
				t.Fatalf("got %+v", got)
			}
		})
	}
}

func TestComparePeriods(t *testing.T) {
	a, _ := NewPeriod(date(t, 2026, 5, 1), date(t, 2026, 5, 31))
	b, _ := NewPeriod(date(t, 2026, 6, 1), date(t, 2026, 6, 30))
	txs := []Transaction{
		expense(t, 1, date(t, 2026, 5, 2), 1000, 0, 1),
		expense(t, 2, date(t, 2026, 6, 2), 1500, 0, 1),
		expense(t, 3, date(t, 2026, 6, 3), 700, 0, 2),
		expense(t, 4, date(t, 2026, 6, 3), 300, 0, 0),
		{ID: 5, Date: date(t, 2026, 6, 3), Amount: money(t, -999, "EUR"), Kind: KindTransfer},
	}
	got, err := ComparePeriods(a, b, txs, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Rows) != 3 || !got.Rows[0].Uncategorized {
		t.Fatalf("rows = %+v", got.Rows)
	}
	r1 := got.Rows[1]
	if r1.Delta.Minor() != 500 || !r1.HasPct || r1.DeltaPermille != 500 {
		t.Fatalf("row1 = %+v", r1)
	}
	if got.Rows[2].HasPct {
		t.Fatalf("row2 must have no pct: %+v", got.Rows[2])
	}
	if got.Total.A.Minor() != 1000 || got.Total.B.Minor() != 2500 || got.Total.DeltaPermille != 1500 {
		t.Fatalf("total = %+v", got.Total)
	}
	flat, _ := ComparePeriods(a, b, txs, false)
	if len(flat.Rows) != 0 || flat.Total != got.Total {
		t.Fatalf("flat = %+v", flat)
	}
	empty, err := ComparePeriods(a, b, nil, true)
	if err != nil || len(empty.Rows) != 0 {
		t.Fatalf("empty = %+v, %v", empty, err)
	}
	mixed := append(slices.Clone(txs), Transaction{ID: 6, Date: date(t, 2026, 6, 4), Amount: money(t, -1, "USD")})
	if _, err := ComparePeriods(a, b, mixed, true); !errors.Is(err, ErrCurrencyMismatch) {
		t.Fatalf("mixed err = %v", err)
	}
}

func genExpenses(t *rapid.T) []Transaction {
	n := rapid.IntRange(0, 25).Draw(t, "n")
	out := make([]Transaction, n)
	for i := range out {
		out[i] = expense(t, int64(i+1),
			date(t, 2026, rapid.IntRange(1, 6).Draw(t, "m"), rapid.IntRange(1, 28).Draw(t, "d")),
			rapid.Int64Range(1, 20000).Draw(t, "amt"),
			PayeeID(rapid.IntRange(1, 3).Draw(t, "p")), CategoryID(rapid.IntRange(1, 3).Draw(t, "c")))
	}
	return out
}

func TestInsightsOrderIndependentProperty(t *testing.T) {
	cfg := DefaultInsightConfig()
	cfg.TrendMinMinor = 1
	per, _ := NewPeriod(date(t, 2026, 6, 1), date(t, 2026, 6, 30))
	rapid.Check(t, func(rt *rapid.T) {
		txs := genExpenses(rt)
		shuffled := rapid.Permutation(txs).Draw(rt, "perm")
		for name, run := range map[string]func([]Transaction) ([]Insight, error){
			"anomalies":     func(x []Transaction) ([]Insight, error) { return DetectAnomalies(cfg, per, x) },
			"subscriptions": func(x []Transaction) ([]Insight, error) { return DetectSubscriptions(cfg, x) },
			"trends":        func(x []Transaction) ([]Insight, error) { return DetectTrends(cfg, per, x) },
		} {
			a, err1 := run(txs)
			b, err2 := run(shuffled)
			if err1 != nil || err2 != nil {
				rt.Fatalf("%s: %v %v", name, err1, err2)
			}
			if diff := cmp.Diff(a, b, cmp.AllowUnexported(Money{}, Currency{})); diff != "" {
				rt.Fatalf("%s order dependent:\n%s", name, diff)
			}
			seen := map[string]bool{}
			for _, in := range a {
				if in.Key == "" || seen[in.Key] {
					rt.Fatalf("%s: empty or duplicate key %q", name, in.Key)
				}
				seen[in.Key] = true
			}
		}
		x, _ := ComparePeriods(per, per, txs, true)
		y, _ := ComparePeriods(per, per, shuffled, true)
		if diff := cmp.Diff(x, y, cmp.AllowUnexported(Money{}, Currency{})); diff != "" {
			rt.Fatalf("compare order dependent:\n%s", diff)
		}
	})
}
