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

func seedBudgetTag(t *testing.T, deps *Deps, name string) {
	t.Helper()
	ctx := context.Background()
	plan, err := deps.Service.PlanAddTag(ctx, app.AddTagRequest{Name: name})
	if err != nil {
		t.Fatalf("PlanAddTag: %v", err)
	}
	if _, err := deps.Service.ApplyAddTag(ctx, plan); err != nil {
		t.Fatalf("ApplyAddTag: %v", err)
	}
}

func TestBudgetSetTagTarget(t *testing.T) {
	deps := newBudgetTestDeps(t, domain.BudgetCap)
	seedBudgetTag(t, deps, "trip")

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

func TestBudgetCopyFrom(t *testing.T) {
	deps := newBudgetTestDeps(t, domain.BudgetCap)
	seedCategory(t, deps, "Groceries")
	mustRunCmd(t, deps, newBudgetSetCmd(deps), []string{"Groceries", "350"})
	mustRunCmd(t, deps, newBudgetSetCmd(deps), []string{"Groceries", "400", "--month", "2026-09"})

	// --from is honoured even when it is not the month right before --to.
	buf := outBuf(deps)
	mustRunCmd(t, deps, newBudgetCopyCmd(deps), []string{"--from", "2026-09", "--to", "2026-11", "--yes", "--json"})
	if !strings.Contains(buf.String(), `"updates":1`) {
		t.Fatalf("budget copy = %q, want updates:1", buf.String())
	}
	buf = outBuf(deps)
	mustRunCmd(t, deps, newBudgetListCmd(deps), []string{"--json"})
	if !strings.Contains(buf.String(), `"2026-11":40000`) {
		t.Fatalf("budget list = %q, want the 2026-11 override", buf.String())
	}

	for _, args := range [][]string{
		{"--from", "2026-11", "--to", "2026-11", "--yes"},
		{"--from", "nope", "--yes"},
	} {
		if err := runCmd(deps, newBudgetCopyCmd(deps), args); err == nil {
			t.Fatalf("copy %v: expected an error", args)
		}
	}
}

func TestBudgetSetFlags(t *testing.T) {
	deps := newBudgetTestDeps(t, domain.BudgetCap)
	for _, c := range []string{"Rent", "Utilities", "Fun", "Gifts"} {
		seedCategory(t, deps, c)
	}
	seedBudgetTag(t, deps, "home")

	tests := []struct {
		name    string
		args    []string
		want    []string
		wantErr string
	}{
		{
			name: "multi target yearly thresholds no-carry",
			args: []string{
				"Home", "1200", "--category", "Rent", "--category", "Utilities", "--tag", "home",
				"--period", "year", "--warn", "70", "--over", "110", "--no-carry", "--json",
			},
			want: []string{`"name":"Home"`, `"period":"year"`, `"warn_at":70`, `"over_at":110`, `"carry":false`},
		},
		{
			name: "new budget defaults",
			args: []string{"Fun", "500", "--json"},
			want: []string{`"period":"month"`, `"warn_at":80`, `"over_at":100`, `"carry":true`},
		},
		{name: "carry and no-carry", args: []string{"Rent", "5", "--carry", "--no-carry"}, wantErr: "mutually exclusive"},
		{name: "bad period", args: []string{"Rent", "5", "--period", "week"}, wantErr: "period"},
		{name: "unknown category", args: []string{"Nothing", "5"}, wantErr: "Nothing"},
		{name: "dry run", args: []string{"Gifts", "50", "--dry-run", "--json"}, want: []string{"changes"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			buf := outBuf(deps)
			err := runCmd(deps, newBudgetSetCmd(deps), tc.args)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %v, want containing %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("set %v: %v", tc.args, err)
			}
			for _, w := range tc.want {
				if !strings.Contains(buf.String(), w) {
					t.Fatalf("output %q missing %q", buf.String(), w)
				}
			}
		})
	}
}

func TestBudgetStatusYearlyEnvelope(t *testing.T) {
	deps := newBudgetTestDeps(t, domain.BudgetEnvelope)
	seedCategory(t, deps, "Holidays")
	mustRunCmd(t, deps, newBudgetSetCmd(deps), []string{"Holidays", "1200", "--period", "year"})
	mustRunCmd(t, deps, newBudgetAssignCmd(deps), []string{"Holidays", "1200", "--date", "2026-11-02"})

	buf := outBuf(deps)
	mustRunCmd(t, deps, newBudgetStatusCmd(deps), []string{"--month", "2026-11", "--json"})
	if !strings.Contains(buf.String(), `"period":"year"`) || !strings.Contains(buf.String(), `"suggested_monthly":60000`) {
		t.Fatalf("status = %q, want yearly with suggested_monthly 60000", buf.String())
	}
	buf = outBuf(deps)
	mustRunCmd(t, deps, newBudgetStatusCmd(deps), []string{"--month", "2026-11", "--csv"})
	if !strings.Contains(buf.String(), "SUGGESTED_MONTHLY") || !strings.Contains(buf.String(), "year") {
		t.Fatalf("status csv = %q", buf.String())
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
