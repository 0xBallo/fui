package simulating

import (
	"errors"
	"testing"

	"github.com/0xballo/fui/internal/domain"
)

func TestParseEvent(t *testing.T) {
	cur := eur(t)
	tests := []struct {
		name, kind, entry string
		want              CashFlowEvent
		wantErr           bool
	}{
		{
			name: "one off", kind: EventOneOff, entry: "2027-03-01:-50,00",
			want: CashFlowEvent{Kind: "one_off", Date: "2027-03-01", AmountMinor: -5000},
		},
		{
			name: "recurring change", kind: EventRecurringChange, entry: " 2027-03-01 : 10 ",
			want: CashFlowEvent{Kind: "recurring_change", Date: "2027-03-01", AmountMinor: 1000},
		},
		{
			name: "stop income", kind: EventStopIncome, entry: "2027-01-01",
			want: CashFlowEvent{Kind: "stop_income", Date: "2027-01-01"},
		},
		{
			name: "loan", kind: EventNewLoan, entry: "2027-02-01:200:4,5:5",
			want: CashFlowEvent{Kind: "new_loan", Date: "2027-02-01", AmountMinor: 20000, RateBP: 450, Years: 5},
		},
		{name: "unknown kind", kind: "nope", entry: "2027-01-01", wantErr: true},
		{name: "wrong part count", kind: EventOneOff, entry: "2027-03-01", wantErr: true},
		{name: "bad date", kind: EventStopIncome, entry: "tomorrow", wantErr: true},
		{name: "bad amount", kind: EventOneOff, entry: "2027-03-01:abc", wantErr: true},
		{name: "bad rate", kind: EventNewLoan, entry: "2027-02-01:200:x:5", wantErr: true},
		{name: "bad years", kind: EventNewLoan, entry: "2027-02-01:200:4:0", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseEvent(tc.kind, tc.entry, domain.LocaleITIT, cur)
			if tc.wantErr {
				var de *domain.Error
				if !errors.As(err, &de) || de.Code != domain.CodeInvalidValue {
					t.Fatalf("ParseEvent() error = %v, want invalid_value", err)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("ParseEvent() = %+v, %v; want %+v", got, err, tc.want)
			}
		})
	}
}

func TestParsePercentBP(t *testing.T) {
	for in, want := range map[string]int64{"4,5": 450, "4.25%": 425, "-1": -100, "+0.5": 50, ".5": 50, "7": 700} {
		if got, err := ParsePercentBP(in); err != nil || got != want {
			t.Errorf("ParsePercentBP(%q) = %d, %v; want %d", in, got, err, want)
		}
	}
	for _, in := range []string{"", "x", "1.234", "--1", "1.-5"} {
		if _, err := ParsePercentBP(in); err == nil {
			t.Errorf("ParsePercentBP(%q) accepted", in)
		}
	}
}
