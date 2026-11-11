package cli_test

import (
	"encoding/json"
	"testing"
)

// TestAmbiguousNameJSONError covers the technical spec's "ambiguous name" exit-code example:
// --json errors go to stderr as {"error":{"code","message","details"}}.
func TestAmbiguousNameJSONError(t *testing.T) {
	h := newHarness(t)
	h.account("Checking", "checking", "EUR", "0")
	h.account("Checking Savings", "savings", "EUR", "0")

	_, stderr, code := h.run("add", "10", "Pay", "-a", "Check", "--json")
	if code != 1 {
		t.Fatalf("exit = %d, want 1; stderr=%s", code, stderr)
	}

	var got struct {
		Error struct {
			Code    string         `json:"code"`
			Message string         `json:"message"`
			Details map[string]any `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(stderr), &got); err != nil {
		t.Fatalf("unmarshal stderr %q: %v", stderr, err)
	}
	if got.Error.Code != "ambiguous_name" {
		t.Errorf("code = %q, want ambiguous_name", got.Error.Code)
	}
	if got.Error.Message == "" {
		t.Error("message is empty")
	}
	if _, ok := got.Error.Details["candidates"]; !ok {
		t.Errorf("details = %v, want a \"candidates\" key", got.Error.Details)
	}
}
