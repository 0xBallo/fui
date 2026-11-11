package main

import (
	"reflect"
	"strings"
	"testing"

	"github.com/0xballo/fui/pkg/fuiproto"
)

func TestParseMinorUnits(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  int64
	}{
		{name: "integer", value: "42", want: 4200},
		{name: "two decimals", value: "-12.34", want: -1234},
		{name: "one decimal", value: "1.2", want: 120},
		{name: "round up", value: "1.235", want: 124},
		{name: "round down", value: "-1.234", want: -123},
		{name: "half away from zero", value: "-1.235", want: -124},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseMinorUnits(tc.value)
			if err != nil {
				t.Fatalf("parseMinorUnits returned error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("parseMinorUnits(%q) = %d, want %d", tc.value, got, tc.want)
			}
		})
	}
}

func TestParseRevolutCSVErrors(t *testing.T) {
	input := strings.NewReader("Type,Product,Started Date,Completed Date,Description,Amount,Fee,Currency,State,Balance\nCard Payment,Current,2026-01-01,2026-01-01,Invalid,abc,0,EUR,COMPLETED,100")

	result, err := parseRevolutCSV(input, nil)
	if err != nil {
		t.Fatalf("parseRevolutCSV returned error: %v", err)
	}
	if len(result.errors) != 1 {
		t.Fatalf("errors = %d, want 1", len(result.errors))
	}
}

func TestRevolutFeeNormalization(t *testing.T) {
	input := strings.NewReader(
		"Type,Product,Started Date,Completed Date,Description,Amount,Fee,Currency,State,Balance\n" +
			"Transfer,Current,2026-01-01 10:00:00,2026-01-01 11:00:00,Transfer to Example,-10.00,0.50,EUR,COMPLETED,100.00\n",
	)

	result, err := parseRevolutCSV(input, nil)
	if err != nil {
		t.Fatalf("parseRevolutCSV returned error: %v", err)
	}
	if len(result.transactions) != 2 {
		t.Fatalf("transactions = %d, want 2", len(result.transactions))
	}

	mainTx := result.transactions[0].Transaction
	feeTx := result.transactions[1].Transaction

	if mainTx.BalanceAfter == nil {
		t.Fatalf("main balance_after is nil")
	}
	if *mainTx.BalanceAfter != 10050 {
		t.Fatalf("main balance_after = %d, want 10050", *mainTx.BalanceAfter)
	}
	if feeTx.Amount != -50 {
		t.Fatalf("fee amount = %d, want -50", feeTx.Amount)
	}
}

func TestParseRevolutCSVMissingColumn(t *testing.T) {
	input := strings.NewReader("Type,Product,Started Date,Completed Date,Description,Amount,Fee,Currency,State\nCard Payment,Current,2026-01-01,2026-01-01,Invalid,-1.00,0,EUR,COMPLETED")
	_, err := parseRevolutCSV(input, nil)
	if err == nil {
		t.Fatalf("expected missing column error")
	}
}

func TestParseRevolutState(t *testing.T) {
	tests := []struct {
		name      string
		rawState  string
		wantState string
		wantKeep  bool
		wantErr   bool
	}{
		{name: "completed", rawState: "COMPLETED", wantState: "COMPLETED", wantKeep: true},
		{name: "pending", rawState: "PENDING", wantState: "PENDING", wantKeep: true},
		{name: "reverted", rawState: "REVERTED", wantKeep: false},
		{name: "unsupported", rawState: "WHATEVER", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			state, keep, errRecord := parseRevolutState(10, map[string]string{"state": tc.rawState})
			if tc.wantErr != (errRecord != nil) {
				t.Fatalf("error presence mismatch: got err=%v", errRecord != nil)
			}
			if keep != tc.wantKeep {
				t.Fatalf("keep = %v, want %v", keep, tc.wantKeep)
			}
			if state != tc.wantState {
				t.Fatalf("state = %q, want %q", state, tc.wantState)
			}
		})
	}
}

func TestParseRevolutAmounts(t *testing.T) {
	amount, fee, balance, errRecord := parseRevolutAmounts(3, map[string]string{
		"amount":  "-1.23",
		"fee":     "-0.10",
		"balance": "100.00",
	})
	if errRecord != nil {
		t.Fatalf("unexpected error: %+v", errRecord)
	}
	if amount != -123 || fee != 10 || balance == nil || *balance != 10000 {
		t.Fatalf("unexpected parsed amounts: amount=%d fee=%d balance=%v", amount, fee, balance)
	}

	_, _, _, errRecord = parseRevolutAmounts(3, map[string]string{
		"amount": "x",
		"fee":    "0",
	})
	if errRecord == nil {
		t.Fatalf("expected invalid amount error")
	}
}

func TestRevolutAccountKind(t *testing.T) {
	if got := revolutAccountKind("Savings"); got != fuiproto.AccountKindSavings {
		t.Fatalf("kind = %q, want %q", got, fuiproto.AccountKindSavings)
	}
	if got := revolutAccountKind("Current"); got != fuiproto.AccountKindChecking {
		t.Fatalf("kind = %q, want %q", got, fuiproto.AccountKindChecking)
	}
}

func TestFallbackHelpers(t *testing.T) {
	if got := mustDate("2026-01-02").Format("2006-01-02"); got != "2026-01-02" {
		t.Fatalf("mustDate returned %q", got)
	}

	row := revolutRawRow([]string{"a", "b"}, map[string]int{"x": 0, "y": 3})
	if !reflect.DeepEqual(row, map[string]string{"x": "a", "y": ""}) {
		t.Fatalf("unexpected raw row: %#v", row)
	}
}
