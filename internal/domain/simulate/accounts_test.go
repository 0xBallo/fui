package simulate

import (
	"errors"
	"testing"

	"pgregory.net/rapid"

	"github.com/0xballo/fui/internal/domain"
)

func TestCashFlowByAccount(t *testing.T) {
	start := date(t, 2026, 1, 1)
	a := AccountFlows{ID: 1, Opening: 100, Flows: []Flow{{Date: date(t, 2026, 1, 10), Amount: 50}, {Date: date(t, 2026, 2, 10), Amount: -20}}}
	b := AccountFlows{ID: 2, Opening: 10, Flows: []Flow{{Date: date(t, 2026, 1, 15), Amount: 30}}}
	stop := Event{Kind: StopIncome, Date: date(t, 2026, 1, 12)}
	oneOff := Event{Kind: OneOff, Date: date(t, 2026, 1, 20), Amount: -5}

	tests := []struct {
		name   string
		accts  []AccountFlows
		events []Event
		series int
		end    [2]int64 // total base/scenario at month 2
	}{
		{"one account", []AccountFlows{a}, nil, 2, [2]int64{130, 130}},
		{"two accounts", []AccountFlows{a, b}, nil, 3, [2]int64{170, 170}},
		{"stop income across accounts", []AccountFlows{a, b}, []Event{stop}, 3, [2]int64{170, 140}},
		{"one-off goes to events", []AccountFlows{a}, []Event{oneOff}, 2, [2]int64{130, 125}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			total, series, err := CashFlowByAccount(start, tc.accts, tc.events, 2)
			if err != nil {
				t.Fatal(err)
			}
			if len(series) != tc.series {
				t.Fatalf("series = %d, want %d", len(series), tc.series)
			}
			if series[len(series)-1].ID != 0 {
				t.Fatalf("last series ID = %d, want 0", series[len(series)-1].ID)
			}
			if got := [2]int64{total.Months[1].Base, total.Months[1].Scenario}; got != tc.end {
				t.Fatalf("end = %v, want %v", got, tc.end)
			}
		})
	}
	if _, _, err := CashFlowByAccount(start, []AccountFlows{a}, nil, 0); !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("months 0 err = %v", err)
	}
	bad := []Event{{Kind: NewLoan, Date: start, Amount: -1, Years: 1}}
	if _, _, err := CashFlowByAccount(start, []AccountFlows{a}, bad, 2); !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("bad loan err = %v", err)
	}
}

func TestPropertyAccountSeriesSumToTotal(t *testing.T) {
	start, _ := domain.NewDate(2026, 1, 1)
	rapid.Check(t, func(t *rapid.T) {
		months := rapid.IntRange(1, 12).Draw(t, "months")
		flow := rapid.Custom(func(t *rapid.T) Flow {
			d, _ := start.AddDays(rapid.IntRange(-10, 400).Draw(t, "day"))
			return Flow{Date: d, Amount: rapid.Int64Range(-1e6, 1e6).Draw(t, "amt")}
		})
		acct := rapid.Custom(func(t *rapid.T) AccountFlows {
			return AccountFlows{
				ID:      domain.AccountID(rapid.IntRange(1, 9).Draw(t, "id")),
				Opening: rapid.Int64Range(-1e6, 1e6).Draw(t, "open"),
				Flows:   rapid.SliceOfN(flow, 0, 8).Draw(t, "flows"),
			}
		})
		event := rapid.Custom(func(t *rapid.T) Event {
			d, _ := start.AddDays(rapid.IntRange(-10, 400).Draw(t, "eday"))
			ev := Event{Kind: rapid.SampledFrom([]EventKind{OneOff, RecurringChange, StopIncome, NewLoan}).Draw(t, "kind"), Date: d}
			ev.Amount = rapid.Int64Range(1, 1e6).Draw(t, "eamt")
			ev.Years, ev.RateBP = rapid.IntRange(1, 3).Draw(t, "years"), rapid.Int64Range(0, 1000).Draw(t, "rate")
			return ev
		})
		accts := rapid.SliceOfN(acct, 1, 4).Draw(t, "accts")
		events := rapid.SliceOfN(event, 0, 4).Draw(t, "events")
		total, series, err := CashFlowByAccount(start, accts, events, months)
		if err != nil {
			t.Fatal(err)
		}
		for i, m := range total.Months {
			var base, scen int64
			for _, s := range series {
				base += s.Months[i].Base
				scen += s.Months[i].Scenario
			}
			if base != m.Base || scen != m.Scenario {
				t.Fatalf("month %d: sum base/scen = %d/%d, total %d/%d", i, base, scen, m.Base, m.Scenario)
			}
		}
	})
}
