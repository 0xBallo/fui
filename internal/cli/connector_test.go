package cli_test

import (
	"strings"
	"testing"
)

// TestConnectorList covers `connector list`: both builtins are discovered.
func TestConnectorList(t *testing.T) {
	h := newHarness(t)
	out, _ := h.mustRun("connector", "list", "--json")
	if !strings.Contains(out, `"name":"csv"`) || !strings.Contains(out, `"name":"fui"`) {
		t.Errorf("connector list = %q, want csv and fui", out)
	}
}

// TestConnectorShow covers `connector show NAME`: the csv connector reports its config params.
func TestConnectorShow(t *testing.T) {
	h := newHarness(t)
	out, _ := h.mustRun("connector", "show", "csv", "--json")
	if !strings.Contains(out, `"key":"profile"`) {
		t.Errorf("connector show csv = %q, want the profile config param", out)
	}
}

// TestConnectorShowUnknownFails covers an unknown connector name failing with not_found.
func TestConnectorShowUnknownFails(t *testing.T) {
	h := newHarness(t)
	_, stderr, code := h.run("connector", "show", "nope", "--json")
	if code != 1 {
		t.Fatalf("exit = %d, want 1; stderr=%s", code, stderr)
	}
	if !strings.Contains(stderr, `"code":"not_found"`) {
		t.Errorf("stderr = %q, want code not_found", stderr)
	}
}

// TestConnectorTest covers `connector test NAME FILE`: it fetches and validates without
// importing, leaving the store unchanged.
func TestConnectorTest(t *testing.T) {
	h := newHarness(t)
	file := writeFUIFile(t, t.TempDir(), "statement.fui.jsonl",
		fuiStream("fui", "statement.fui.jsonl", "sha-1",
			fuiAccount("acc-1", "Checking", "EUR"),
			fuiTx("acc-1", "tx-1", "2026-11-01", -1000, "Pizza"),
		)...)
	before := h.store.Snapshot()

	out, _ := h.mustRun("connector", "test", "fui", file, "--json")
	if !strings.Contains(out, `"transactions":1`) {
		t.Errorf("connector test = %q, want transactions:1", out)
	}
	if after := h.store.Snapshot(); after != before {
		t.Errorf("connector test wrote to the store:\nbefore=%s\nafter=%s", before, after)
	}
}
