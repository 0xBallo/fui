package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

func TestBudgetSetListStatusCap(t *testing.T) {
	deps := newBudgetTestDeps(t, domain.BudgetCap)
	seedAccount(t, deps, "Main")
	seedCategory(t, deps, "Groceries")

	mustRunCmd(t, deps, newBudgetSetCmd(deps), []string{"Groceries", "400"})

	buf := outBuf(deps)
	mustRunCmd(t, deps, newBudgetListCmd(deps), []string{"--json"})
	if !strings.Contains(buf.String(), `"name":"Groceries"`) {
		t.Fatalf("budget list = %q, want Groceries", buf.String())
	}

	// A month override needs the amount and the budget to already exist.
	buf = outBuf(deps)
	mustRunCmd(t, deps, newBudgetSetCmd(deps), []string{"Groceries", "500", "--month", "2026-12", "--json"})
	if !strings.Contains(buf.String(), `"name":"Groceries"`) {
		t.Fatalf("budget set month override = %q, want Groceries", buf.String())
	}

	seedTx(t, deps, "Main", "Groceries", "-120")

	buf = outBuf(deps)
	mustRunCmd(t, deps, newBudgetStatusCmd(deps), []string{"--month", "2026-01", "--json"})
	out := buf.String()
	if !strings.Contains(out, `"name":"Groceries"`) || !strings.Contains(out, `"spent":12000`) {
		t.Fatalf("budget status = %q, want Groceries spent 12000", out)
	}

	buf = outBuf(deps)
	mustRunCmd(t, deps, newBudgetStatusCmd(deps), []string{"--month", "2026-01"})
	out = buf.String()
	if !strings.Contains(out, "Groceries") || !strings.Contains(out, "OK") {
		t.Fatalf("budget status table = %q, want Groceries OK", out)
	}
}

func TestBudgetSetTagTarget(t *testing.T) {
	deps := newBudgetTestDeps(t, domain.BudgetCap)
	ctx := context.Background()
	plan, err := deps.Service.PlanAddTag(ctx, app.AddTagRequest{Name: "trip"})
	if err != nil {
		t.Fatalf("PlanAddTag: %v", err)
	}
	if _, err := deps.Service.ApplyAddTag(ctx, plan); err != nil {
		t.Fatalf("ApplyAddTag: %v", err)
	}

	buf := outBuf(deps)
	mustRunCmd(t, deps, newBudgetSetCmd(deps), []string{"#trip", "300", "--json"})
	if !strings.Contains(buf.String(), `"name":"trip"`) {
		t.Fatalf("budget set #trip = %q, want name trip", buf.String())
	}
}

func TestBudgetAssignMoveEnvelope(t *testing.T) {
	deps := newBudgetTestDeps(t, domain.BudgetEnvelope)
	seedCategory(t, deps, "Groceries")
	seedCategory(t, deps, "Transport")
	mustRunCmd(t, deps, newBudgetSetCmd(deps), []string{"Groceries", "400"})
	mustRunCmd(t, deps, newBudgetSetCmd(deps), []string{"Transport", "100"})

	buf := outBuf(deps)
	mustRunCmd(t, deps, newBudgetAssignCmd(deps), []string{"Groceries", "200", "--date", "2026-01-05", "--json"})
	if !strings.Contains(buf.String(), `"to":"Groceries"`) || !strings.Contains(buf.String(), `"amount":20000`) {
		t.Fatalf("budget assign = %q, want Groceries 20000", buf.String())
	}

	buf = outBuf(deps)
	mustRunCmd(t, deps, newBudgetMoveCmd(deps), []string{"Groceries", "Transport", "50", "--date", "2026-01-06", "--json"})
	if !strings.Contains(buf.String(), `"from":"Groceries"`) || !strings.Contains(buf.String(), `"to":"Transport"`) {
		t.Fatalf("budget move = %q, want Groceries -> Transport", buf.String())
	}

	buf = outBuf(deps)
	mustRunCmd(t, deps, newBudgetStatusCmd(deps), []string{"--month", "2026-01", "--json"})
	out := buf.String()
	if !strings.Contains(out, `"to_assign"`) {
		t.Fatalf("budget status envelope = %q, want to_assign", out)
	}

	buf = outBuf(deps)
	mustRunCmd(t, deps, newBudgetStatusCmd(deps), []string{"--month", "2026-01"})
	out = buf.String()
	if !strings.Contains(out, "Groceries") || !strings.Contains(out, "To assign") {
		t.Fatalf("budget status envelope table = %q, want Groceries and To assign", out)
	}

	buf = outBuf(deps)
	mustRunCmd(t, deps, newBudgetAssignCmd(deps), []string{"Transport", "20", "--date", "2026-01-07"})
	if !strings.Contains(buf.String(), "Transport") {
		t.Fatalf("budget assign table = %q, want Transport", buf.String())
	}

	buf = outBuf(deps)
	mustRunCmd(t, deps, newBudgetMoveCmd(deps), []string{"Transport", "Groceries", "10", "--date", "2026-01-08"})
	if !strings.Contains(buf.String(), "Transport") || !strings.Contains(buf.String(), "Groceries") {
		t.Fatalf("budget move table = %q, want Transport and Groceries", buf.String())
	}
}

func TestBudgetMoveNeedsEnvelopeMode(t *testing.T) {
	deps := newBudgetTestDeps(t, domain.BudgetCap)
	seedCategory(t, deps, "Groceries")
	mustRunCmd(t, deps, newBudgetSetCmd(deps), []string{"Groceries", "400"})

	if err := runCmd(deps, newBudgetAssignCmd(deps), []string{"Groceries", "100"}); err == nil {
		t.Fatal("expected an error assigning in cap mode")
	}
}

func TestBudgetCopy(t *testing.T) {
	deps := newBudgetTestDeps(t, domain.BudgetCap)
	seedCategory(t, deps, "Groceries")
	mustRunCmd(t, deps, newBudgetSetCmd(deps), []string{"Groceries", "350"})
	mustRunCmd(t, deps, newBudgetSetCmd(deps), []string{"Groceries", "400", "--month", "2026-10"})

	buf := outBuf(deps)
	mustRunCmd(t, deps, newBudgetCopyCmd(deps), []string{"--from", "2026-10", "--to", "2026-11", "--yes", "--json"})
	if !strings.Contains(buf.String(), `"updates":1`) {
		t.Fatalf("budget copy = %q, want updates:1", buf.String())
	}

	buf = outBuf(deps)
	mustRunCmd(t, deps, newBudgetListCmd(deps), []string{"--json"})
	if !strings.Contains(buf.String(), `"2026-11":40000`) {
		t.Fatalf("budget list after copy = %q, want the 2026-11 override", buf.String())
	}

	mustRunCmd(t, deps, newBudgetSetCmd(deps), []string{"Groceries", "999", "--month", "2026-11"})
	buf = outBuf(deps)
	mustRunCmd(t, deps, newBudgetCopyCmd(deps), []string{"--from", "2026-11", "--to", "2026-12", "--yes"})
	if !strings.Contains(buf.String(), "Updated 1 budget(s)") {
		t.Fatalf("budget copy table = %q, want Updated 1 budget(s)", buf.String())
	}
}

func TestBudgetCopyFromMismatch(t *testing.T) {
	deps := newBudgetTestDeps(t, domain.BudgetCap)
	seedCategory(t, deps, "Groceries")
	mustRunCmd(t, deps, newBudgetSetCmd(deps), []string{"Groceries", "350"})
	mustRunCmd(t, deps, newBudgetSetCmd(deps), []string{"Groceries", "400", "--month", "2026-10"})

	if err := runCmd(deps, newBudgetCopyCmd(deps), []string{"--from", "2026-09", "--to", "2026-11", "--yes"}); err == nil {
		t.Fatal("expected an error for a --from that is not the month right before --to")
	}
}

// TestBudgetGroupDispatch covers newBudgetCmd routing its subcommands, the group command that
// V32 will register under the root tree.
func TestBudgetGroupDispatch(t *testing.T) {
	deps := newBudgetTestDeps(t, domain.BudgetCap)
	seedCategory(t, deps, "Groceries")
	mustRunCmd(t, deps, newBudgetCmd(deps), []string{"set", "Groceries", "400"})

	buf := outBuf(deps)
	mustRunCmd(t, deps, newBudgetCmd(deps), []string{"list", "--json"})
	if !strings.Contains(buf.String(), `"name":"Groceries"`) {
		t.Fatalf("budget list via group = %q, want Groceries", buf.String())
	}
}
