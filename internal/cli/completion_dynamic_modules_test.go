package cli

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app/rules"
	"github.com/0xballo/fui/internal/config"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
)

func TestModuleCompletionGatesAndSilencesErrors(t *testing.T) {
	h := newStandalone(t, domain.LocaleENUS, config.LocaleUS)
	h.seedAccount(t, "Savings")
	h.seedCategory(t, "Food")
	ruleCmd := newRuleCmd(h.deps)
	if err := h.run(ruleCmd, "rule", "add", "groceries", "--contains", "shop", "--category", "Food", "--priority", "1"); err != nil {
		t.Fatalf("rule add: %v", err)
	}

	root := NewRoot(h.deps)
	for _, tc := range []struct {
		path []string
		want []string
	}{
		{[]string{"rule", "rm"}, []string{"groceries"}},
		{[]string{"rule", "edit"}, []string{"groceries"}},
		{[]string{"reconcile"}, []string{"Savings"}},
	} {
		cmd, _, err := root.Find(tc.path)
		if err != nil {
			t.Fatalf("find %v: %v", tc.path, err)
		}
		got, directive := cmd.ValidArgsFunction(cmd, nil, "")
		if directive != cobra.ShellCompDirectiveNoFileComp || !slices.Equal(got, tc.want) {
			t.Errorf("%v completion = %v (%d), want %v", tc.path, got, directive, tc.want)
		}
		if got, _ := cmd.ValidArgsFunction(cmd, []string{"x"}, ""); len(got) != 0 {
			t.Errorf("%v completes a second argument: %v", tc.path, got)
		}
	}

	// A disabled module yields nothing, even though the data exists.
	h.deps.Config.Modules.Enabled = []module.ID{module.Budget}
	if got, _ := completeRuleNames(h.deps)(nil, nil, ""); len(got) != 0 {
		t.Errorf("rule names with core disabled = %v, want none", got)
	}
}

func TestModuleCompletionNilServicesAreSilent(t *testing.T) {
	deps := &Deps{Version: "test"}
	for name, fn := range map[string]cobra.CompletionFunc{
		"budget":     completeBudgetNames(deps),
		"recurrence": completeRecurrenceIDs(deps),
		"loan":       completeLoanNames(deps),
		"rule":       completeRuleNames(deps),
		"security":   completeSecurityNames(deps),
		"asset":      completeAssetNames(deps),
	} {
		got, directive := fn(nil, nil, "")
		if len(got) != 0 || directive != cobra.ShellCompDirectiveNoFileComp {
			t.Errorf("%s completion = %v (%d), want none", name, got, directive)
		}
	}
	if got, _ := completeBudgetNames(nil)(nil, nil, ""); len(got) != 0 {
		t.Errorf("nil deps completion = %v", got)
	}
}

func TestModuleCompletionFetchErrorIsSilent(t *testing.T) {
	deps := &Deps{Version: "test", Rules: rules.New(nil, nil)}
	fn := moduleCompletion(deps, []module.ID{module.Core}, func(_ context.Context, _ module.Set) ([]string, error) {
		return nil, errors.New("archive locked")
	})
	if got, directive := fn(nil, nil, ""); len(got) != 0 || directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("completion on fetch error = %v (%d)", got, directive)
	}
}
