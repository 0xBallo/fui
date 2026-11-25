package domain

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"pgregory.net/rapid"
)

func TestDetectSubscriptionsKnown(t *testing.T) {
	cfg := DefaultInsightConfig()
	start := date(t, 2026, 1, 15)
	eur := money(t, 1, "EUR").Currency()
	usd := money(t, 1, "USD").Currency()
	byPayee := monthly(t, 3, start, func(int) int64 { return 1000 }, 3)
	byDesc := monthly(t, 3, start, func(int) int64 { return 1000 }, 0)
	for i := range byDesc {
		byDesc[i].CategoryID = 0
		byDesc[i].Description = "Netflix"
	}
	tests := []struct {
		name  string
		txs   []Transaction
		known KnownRecurrence
		want  int
	}{
		{"payee match", byPayee, KnownRecurrence{[]string{"payee:3"}, 1000, eur, "monthly"}, 0},
		{"description match", byDesc, KnownRecurrence{[]string{"x", "description:netflix"}, 1000, eur, "monthly"}, 0},
		{"within tolerance", byPayee, KnownRecurrence{[]string{"payee:3"}, 1040, eur, "monthly"}, 0},
		{"amount outside tolerance", byPayee, KnownRecurrence{[]string{"payee:3"}, 1200, eur, "monthly"}, 1},
		{"interval mismatch", byPayee, KnownRecurrence{[]string{"payee:3"}, 1000, eur, "weekly"}, 1},
		{"other currency", byPayee, KnownRecurrence{[]string{"payee:3"}, 1000, usd, "monthly"}, 1},
		{"other subject", byPayee, KnownRecurrence{[]string{"payee:4"}, 1000, eur, "monthly"}, 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := DetectSubscriptions(cfg, tc.txs, []KnownRecurrence{tc.known})
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != tc.want {
				t.Fatalf("got %d insights, want %d: %v", len(got), tc.want, got)
			}
		})
	}
}

func TestDetectSubscriptionsKnownKeepsPriceChange(t *testing.T) {
	cfg := DefaultInsightConfig()
	txs := withRecurrence(monthly(t, 4, date(t, 2026, 1, 15), func(i int) int64 { return []int64{1000, 1000, 1000, 1200}[i] }, 3))
	known := []KnownRecurrence{{[]string{"payee:3"}, 1000, money(t, 1, "EUR").Currency(), "monthly"}}
	got, err := DetectSubscriptions(cfg, txs, known)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Kind != InsightPriceChange {
		t.Fatalf("got %v", got)
	}
}

func TestDetectSubscriptionsKnownProperty(t *testing.T) {
	cfg := DefaultInsightConfig()
	rapid.Check(t, func(rt *rapid.T) {
		txs := genExpenses(rt)
		a, err1 := DetectSubscriptions(cfg, txs, nil)
		b, err2 := DetectSubscriptions(cfg, txs, []KnownRecurrence{})
		if err1 != nil || err2 != nil {
			rt.Fatal(err1, err2)
		}
		if diff := cmp.Diff(a, b, cmp.AllowUnexported(Money{}, Currency{})); diff != "" {
			rt.Fatalf("nil vs empty known differ:\n%s", diff)
		}
		// Known entries built from the results suppress exactly the subscriptions.
		var known []KnownRecurrence
		for _, in := range a {
			if in.Kind != InsightSubscript {
				continue
			}
			known = append(known, KnownRecurrence{
				Subjects: []string{in.Subject}, Amount: in.Amount.Minor(),
				Currency: in.Amount.Currency(), Interval: intervalOf(in.Detail),
			})
		}
		c, err := DetectSubscriptions(cfg, txs, known)
		if err != nil {
			rt.Fatal(err)
		}
		for _, in := range c {
			if in.Kind == InsightSubscript {
				rt.Fatalf("subscription not suppressed: %v", in)
			}
		}
	})
}

func intervalOf(detail string) string {
	for _, u := range []string{"weekly", "monthly", "yearly"} {
		if len(detail) >= 9+len(u) && detail[9:9+len(u)] == u {
			return u
		}
	}
	return ""
}
