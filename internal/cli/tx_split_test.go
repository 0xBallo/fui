package cli

import (
	"context"
	"strconv"
	"strings"
	"testing"
)

func TestTxSplitAndUnsplit(t *testing.T) {
	deps := newTestDeps(t)
	seedAccount(t, deps, "Checking")
	seedCategory(t, deps, "Groceries")
	seedCategory(t, deps, "Household")
	tx := seedTx(t, deps, "Checking", "Groceries", "-40")
	id := strconv.FormatInt(int64(tx.ID), 10)

	buf := outBuf(deps)
	mustRunCmd(t, deps, newTxSplitCmd(deps), []string{
		id, "--row", "Groceries=-30", "--row", "Household=-10:cleaning", "--json",
	})
	out := buf.String()
	if !strings.Contains(out, `"category":"Groceries"`) || !strings.Contains(out, `"amount":-3000`) {
		t.Fatalf("tx split = %q, want the Groceries row", out)
	}
	if !strings.Contains(out, `"category":"Household"`) || !strings.Contains(out, `"note":"cleaning"`) {
		t.Fatalf("tx split = %q, want the Household row with its note", out)
	}

	mustRunCmd(t, deps, newTxUnsplitCmd(deps), []string{id, "--json"})
	splits, err := deps.Service.ListSplits(context.Background(), tx.ID)
	if err != nil {
		t.Fatalf("ListSplits: %v", err)
	}
	if len(splits) != 0 {
		t.Fatalf("splits after unsplit = %+v, want none", splits)
	}
}

func TestTxSplitRowsMustSumToAmount(t *testing.T) {
	deps := newTestDeps(t)
	seedAccount(t, deps, "Checking")
	seedCategory(t, deps, "Groceries")
	seedCategory(t, deps, "Household")
	tx := seedTx(t, deps, "Checking", "Groceries", "-40")
	id := strconv.FormatInt(int64(tx.ID), 10)

	err := runCmd(deps, newTxSplitCmd(deps), []string{
		id, "--row", "Groceries=-30", "--row", "Household=-5",
	})
	if err == nil {
		t.Fatal("expected an error when split rows do not sum to the transaction amount")
	}
}

func TestParseSplitRowInvalid(t *testing.T) {
	for _, raw := range []string{"noequals", "=10", "Cat="} {
		if _, err := parseSplitRow(raw); err == nil {
			t.Errorf("parseSplitRow(%q): want an error", raw)
		}
	}
	row, err := parseSplitRow("Groceries=-12,50:weekly")
	if err != nil {
		t.Fatalf("parseSplitRow: %v", err)
	}
	if row.Category != "Groceries" || row.Amount != "-12,50" || row.Note != "weekly" {
		t.Fatalf("parseSplitRow = %+v", row)
	}
}
