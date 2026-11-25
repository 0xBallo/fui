package cli

import (
	"strings"
	"testing"

	"github.com/0xballo/fui/internal/domain"
)

func TestRecurringEditRmAndLoanEventFlags(t *testing.T) {
	h := newLendingStandalone(t)
	h.seedAccount(t, "Checking")
	if err := h.run(newRecurringCmd(h.deps), "recurring", "add", "--account", "Checking",
		"--description", "Netflix", "--amount", "-9.99", "--every", "1", "--unit", "months",
		"--start", "2099-01-01", "--json"); err != nil {
		t.Fatalf("recurring add: %v", err)
	}
	if err := h.run(newLoanCmd(h.deps), "loan", "add", "Mutuo", "--kind", "mortgage", "--principal", "12000",
		"--term-months", "12", "--rate-type", "variable", "--rate", "3.00", "--debit", "Checking",
		"--disbursed", "2026-01-01", "--json"); err != nil {
		t.Fatalf("loan add: %v", err)
	}

	tests := []struct {
		name    string
		args    []string
		wantOut string
		wantErr string
	}{
		{"edit amount and every", []string{"recurring", "edit", "1", "--amount", "-12", "--every", "2", "--json"}, `"every":2`, ""},
		{"edit clears end", []string{"recurring", "edit", "1", "--end", "", "--json"}, `"description":"Netflix"`, ""},
		{"edit day-of-month", []string{"recurring", "edit", "1", "--day-of-month", "5", "--json"}, `"day_of_month":5`, ""},
		{"edit nothing", []string{"recurring", "edit", "1"}, "", "nothing to change"},
		{"edit bad id", []string{"recurring", "edit", "x", "--every", "1"}, "", "id"},
		{"rm bad id", []string{"recurring", "rm", "x"}, "", "id"},
		{"rm json", []string{"recurring", "rm", "1", "--yes", "--json"}, `"removed":1`, ""},
		{"loan event alias", []string{"loan", "event", "Mutuo", "--rate", "2.5", "--from", "2026-03-01", "--json"}, "", ""},
		{"loan event same date and from", []string{"loan", "event", "Mutuo", "--rate", "2", "--date", "2026-04-01", "--from", "2026-04-01", "--json"}, "", ""},
		{"loan event conflicting", []string{"loan", "event", "Mutuo", "--rate", "2", "--date", "2026-04-01", "--from", "2026-05-01"}, "", "differ"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newRecurringCmd(h.deps)
			if tt.args[0] == "loan" {
				cmd = newLoanCmd(h.deps)
			}
			err := h.run(cmd, tt.args...)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("%v (stderr=%q)", err, h.err.String())
			}
			if !strings.Contains(h.out.String(), tt.wantOut) {
				t.Fatalf("out = %q, want %q", h.out.String(), tt.wantOut)
			}
		})
	}
}

func TestRuleTestLenient(t *testing.T) {
	h := newStandalone(t, domain.LocaleENUS, "us")
	h.seedAccount(t, "Checking")
	h.seedCategory(t, "Food")
	if err := h.run(newRuleCmd(h.deps), "rule", "add", "bar", "--contains", "bar", "--category", "Food",
		"--min", "100", "--account", "Checking", "--json"); err != nil {
		t.Fatalf("rule add: %v", err)
	}
	tests := []struct {
		name    string
		args    []string
		wantErr string
		note    string
	}{
		{"text only", []string{"rule", "test", "Bar Roma", "--json"}, "", "account/amount conditions not evaluated"},
		{"account only", []string{"rule", "test", "Bar Roma", "--account", "Checking", "--json"}, "", "amount conditions not evaluated"},
		{"amount without account", []string{"rule", "test", "Bar Roma", "--amount", "-3", "--json"}, "account", ""},
		{"unknown account", []string{"rule", "test", "Bar Roma", "--account", "Nope", "--json"}, "account", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h.out.Reset()
			h.err.Reset()
			err := h.run(newRuleCmd(h.deps), tt.args...)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("%v", err)
			}
			if !strings.Contains(h.out.String(), `"category_rule":"bar"`) || !strings.Contains(h.err.String(), tt.note) {
				t.Fatalf("out=%q err=%q", h.out.String(), h.err.String())
			}
		})
	}
}
