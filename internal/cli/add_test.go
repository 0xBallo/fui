package cli_test

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestAddSpecExample is the exact example from the functional spec ("Uso con agenti AI"):
// fui add -12,50 Esselunga -c Spesa -a Contanti. It must parse "-12,50" as the amount, not as an
// unknown shorthand flag chain, and create an outflow transaction on Contanti/Spesa.
func TestAddSpecExample(t *testing.T) {
	h := newHarness(t)
	h.useItalianLocale()
	h.account("Contanti", "cash", "EUR", "0")
	h.category("Spesa", "expense")

	out, _ := h.mustRun("add", "-12,50", "Esselunga", "-c", "Spesa", "-a", "Contanti", "--json")

	var got struct {
		Amount      int64  `json:"amount"`
		Currency    string `json:"currency"`
		Account     string `json:"account"`
		Category    string `json:"category"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal output %q: %v", out, err)
	}
	if got.Amount != -1250 || got.Currency != "EUR" {
		t.Errorf("amount/currency = %d %s, want -1250 EUR", got.Amount, got.Currency)
	}
	if got.Account != "Contanti" || got.Category != "Spesa" || got.Description != "Esselunga" {
		t.Errorf("account/category/description = %q/%q/%q, want Contanti/Spesa/Esselunga",
			got.Account, got.Category, got.Description)
	}
}

// TestAddDoubleDashEscapesAmount covers "fui add -- -12,50 ..." as an alternative to relying on
// splitLeadingPositional's own detection of the leading amount token.
func TestAddDoubleDashEscapesAmount(t *testing.T) {
	h := newHarness(t)
	h.useItalianLocale()
	h.account("Checking", "checking", "EUR", "0")

	out, _ := h.mustRun("add", "--", "-12,50", "Cash", "withdrawal", "-a", "Checking", "--json")
	if !strings.Contains(out, `"amount":-1250`) {
		t.Errorf("output = %q, want amount -1250", out)
	}
}

// TestAddMissingAccountFailsWithMissingField: without a TTY and without --no-input even set
// explicitly, a required field that is empty and cannot be prompted for fails missing_field.
func TestAddMissingAccountFailsWithMissingField(t *testing.T) {
	h := newHarness(t)
	_, stderr, code := h.run("add", "10", "Salary", "--json")
	if code != 1 {
		t.Fatalf("exit = %d, want 1; stderr=%s", code, stderr)
	}
	if !strings.Contains(stderr, "missing_field") {
		t.Errorf("stderr = %q, want missing_field", stderr)
	}
}
