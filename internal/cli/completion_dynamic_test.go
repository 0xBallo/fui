package cli_test

import (
	"bytes"
	"context"
	"slices"
	"testing"

	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/reporting"
	"github.com/0xballo/fui/internal/cli"
)

func TestDynamicCompletionAccountArgAndFlag(t *testing.T) {
	h := newHarness(t)
	h.account("Checking", "checking", "EUR", "0")
	h.account("Savings", "savings", "EUR", "0")

	var out, errw bytes.Buffer
	root := cli.NewRoot(h.deps(&out, &errw))

	show := mustFindCommand(t, root, "account", "show")
	gotArgs, directive := show.ValidArgsFunction(show, nil, "Ch")
	if directive != cobra.ShellCompDirectiveNoFileComp || !slices.Equal(gotArgs, []string{"Checking"}) {
		t.Fatalf("account show completion = %v (directive=%d), want [Checking] (%d)", gotArgs, directive, cobra.ShellCompDirectiveNoFileComp)
	}

	txList := mustFindCommand(t, root, "tx", "list")
	completeAccount, ok := txList.GetFlagCompletionFunc("account")
	if !ok {
		t.Fatal("tx list --account completion not registered")
	}
	gotFlags, directive := completeAccount(txList, nil, "Sa")
	if directive != cobra.ShellCompDirectiveNoFileComp || !slices.Equal(gotFlags, []string{"Savings"}) {
		t.Fatalf("tx list --account completion = %v (directive=%d), want [Savings] (%d)", gotFlags, directive, cobra.ShellCompDirectiveNoFileComp)
	}
}

func TestDynamicCompletionTagPayeeAndReport(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	tagPlan, err := h.svc.PlanAddTag(ctx, app.AddTagRequest{Name: "weekly"})
	if err != nil {
		t.Fatalf("PlanAddTag: %v", err)
	}
	if _, err := h.svc.ApplyAddTag(ctx, tagPlan); err != nil {
		t.Fatalf("ApplyAddTag: %v", err)
	}
	payeePlan, err := h.svc.PlanAddPayee(ctx, app.AddPayeeRequest{Name: "Amazon"})
	if err != nil {
		t.Fatalf("PlanAddPayee: %v", err)
	}
	if _, err := h.svc.ApplyAddPayee(ctx, payeePlan); err != nil {
		t.Fatalf("ApplyAddPayee: %v", err)
	}
	reportPlan, err := h.reporting.Analytics.PlanSave(ctx, "monthly-overview", "net-worth", reporting.SavedSpec{})
	if err != nil {
		t.Fatalf("PlanSave: %v", err)
	}
	if _, err := h.reporting.Analytics.ApplySave(ctx, reportPlan); err != nil {
		t.Fatalf("ApplySave: %v", err)
	}

	var out, errw bytes.Buffer
	root := cli.NewRoot(h.deps(&out, &errw))

	txList := mustFindCommand(t, root, "tx", "list")
	completeTag, ok := txList.GetFlagCompletionFunc("tag")
	if !ok {
		t.Fatal("tx list --tag completion not registered")
	}
	gotTags, directive := completeTag(txList, nil, "we")
	if directive != cobra.ShellCompDirectiveNoFileComp || !slices.Equal(gotTags, []string{"weekly"}) {
		t.Fatalf("tx list --tag completion = %v (directive=%d), want [weekly] (%d)", gotTags, directive, cobra.ShellCompDirectiveNoFileComp)
	}
	completePayee, ok := txList.GetFlagCompletionFunc("payee")
	if !ok {
		t.Fatal("tx list --payee completion not registered")
	}
	gotPayees, directive := completePayee(txList, nil, "Ama")
	if directive != cobra.ShellCompDirectiveNoFileComp || !slices.Equal(gotPayees, []string{"Amazon"}) {
		t.Fatalf("tx list --payee completion = %v (directive=%d), want [Amazon] (%d)", gotPayees, directive, cobra.ShellCompDirectiveNoFileComp)
	}

	reportRun := mustFindCommand(t, root, "report", "run")
	gotSavedRun, directive := reportRun.ValidArgsFunction(reportRun, nil, "mo")
	if directive != cobra.ShellCompDirectiveNoFileComp || !slices.Equal(gotSavedRun, []string{"monthly-overview"}) {
		t.Fatalf("report run completion = %v (directive=%d), want [monthly-overview] (%d)", gotSavedRun, directive, cobra.ShellCompDirectiveNoFileComp)
	}

	reportKind := mustFindCommand(t, root, "report")
	gotReports, directive := reportKind.ValidArgsFunction(reportKind, nil, "ne")
	if directive != cobra.ShellCompDirectiveNoFileComp || !slices.Equal(gotReports, []string{"networth"}) {
		t.Fatalf("report kind completion = %v (directive=%d), want [networth] (%d)", gotReports, directive, cobra.ShellCompDirectiveNoFileComp)
	}
	reportSave := mustFindCommand(t, root, "report", "save")
	gotKinds, directive := reportSave.ValidArgsFunction(reportSave, []string{"name"}, "to")
	if directive != cobra.ShellCompDirectiveNoFileComp || !slices.Equal(gotKinds, []string{"top-payees"}) {
		t.Fatalf("report save kind completion = %v (directive=%d), want [top-payees] (%d)", gotKinds, directive, cobra.ShellCompDirectiveNoFileComp)
	}
	reportRm := mustFindCommand(t, root, "report", "rm")
	gotSavedRm, directive := reportRm.ValidArgsFunction(reportRm, nil, "mo")
	if directive != cobra.ShellCompDirectiveNoFileComp || !slices.Equal(gotSavedRm, []string{"monthly-overview"}) {
		t.Fatalf("report rm completion = %v (directive=%d), want [monthly-overview] (%d)", gotSavedRm, directive, cobra.ShellCompDirectiveNoFileComp)
	}

	gotReportRoot, directive := reportKind.ValidArgsFunction(reportKind, nil, "ba")
	if directive != cobra.ShellCompDirectiveNoFileComp || !slices.Equal(gotReportRoot, []string{"balance"}) {
		t.Fatalf("report root completion = %v (directive=%d), want [balance] (%d)", gotReportRoot, directive, cobra.ShellCompDirectiveNoFileComp)
	}
}

func TestDynamicCompletionWithoutServiceReturnsNoSuggestions(t *testing.T) {
	deps := &cli.Deps{Version: "test"}
	root := cli.NewRoot(deps)

	txList := mustFindCommand(t, root, "tx", "list")
	completeAccount, ok := txList.GetFlagCompletionFunc("account")
	if !ok {
		t.Fatal("tx list --account completion not registered")
	}
	got, directive := completeAccount(txList, nil, "C")
	if directive != cobra.ShellCompDirectiveNoFileComp || len(got) != 0 {
		t.Fatalf("tx list --account completion = %v (directive=%d), want [] (%d)", got, directive, cobra.ShellCompDirectiveNoFileComp)
	}
}

func mustFindCommand(t *testing.T, root *cobra.Command, parts ...string) *cobra.Command {
	t.Helper()
	cmd, _, err := root.Find(parts)
	if err != nil {
		t.Fatalf("find %v: %v", parts, err)
	}
	return cmd
}
