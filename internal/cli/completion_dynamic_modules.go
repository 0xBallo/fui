package cli

import (
	"context"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/module"
)

// moduleCompletion builds a completion source for a module-gated command: it is silent when the
// archive cannot be read and yields nothing unless one of mods is enabled. fetch returns the
// candidate values; it is only called when the gate passes.
func moduleCompletion(
	deps *Deps, mods []module.ID, fetch func(ctx context.Context, enabled module.Set) ([]string, error),
) cobra.CompletionFunc {
	return func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if deps == nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		enabled, err := enabledModulesFromDeps(deps)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		allowed := false
		for _, id := range mods {
			allowed = allowed || enabled.Has(id)
		}
		if !allowed {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		values, err := fetch(context.Background(), enabled)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return filterByPrefix(values, toComplete), cobra.ShellCompDirectiveNoFileComp
	}
}

func completeBudgetNames(deps *Deps) cobra.CompletionFunc {
	return moduleCompletion(deps, []module.ID{module.Budget}, func(ctx context.Context, _ module.Set) ([]string, error) {
		if deps.Budgeting == nil {
			return nil, nil
		}
		rows, err := deps.Budgeting.List(ctx)
		if err != nil {
			return nil, err
		}
		names := make([]string, 0, len(rows))
		for _, row := range rows {
			names = append(names, row.Name)
		}
		return names, nil
	})
}

// completeRecurrenceIDs suggests recurrence ids with their description as hint.
func completeRecurrenceIDs(deps *Deps) cobra.CompletionFunc {
	return moduleCompletion(deps, []module.ID{module.Recurring}, func(ctx context.Context, enabled module.Set) ([]string, error) {
		if deps.Recurring == nil {
			return nil, nil
		}
		rows, err := deps.Recurring.List(ctx, enabled)
		if err != nil {
			return nil, err
		}
		out := make([]string, 0, len(rows))
		for _, row := range rows {
			out = append(out, strconv.FormatInt(int64(row.ID), 10)+"\t"+row.Description)
		}
		return out, nil
	})
}

func completeLoanNames(deps *Deps) cobra.CompletionFunc {
	return moduleCompletion(deps, []module.ID{module.Loans}, func(ctx context.Context, enabled module.Set) ([]string, error) {
		if deps.Loans == nil {
			return nil, nil
		}
		rows, err := deps.Loans.List(ctx, enabled)
		if err != nil {
			return nil, err
		}
		names := make([]string, 0, len(rows))
		for _, row := range rows {
			names = append(names, row.Loan.Name)
		}
		return names, nil
	})
}

func completeRuleNames(deps *Deps) cobra.CompletionFunc {
	return moduleCompletion(deps, []module.ID{module.Core}, func(ctx context.Context, _ module.Set) ([]string, error) {
		if deps.Rules == nil {
			return nil, nil
		}
		rows, err := deps.Rules.List(ctx)
		if err != nil {
			return nil, err
		}
		names := make([]string, 0, len(rows))
		for _, row := range rows {
			names = append(names, row.Name)
		}
		return names, nil
	})
}

func completeSecurityNames(deps *Deps) cobra.CompletionFunc {
	return moduleCompletion(deps, []module.ID{module.Investments, module.Crypto}, func(ctx context.Context, _ module.Set) ([]string, error) {
		if deps.Investing == nil {
			return nil, nil
		}
		rows, err := deps.Investing.ListSecurities(ctx)
		if err != nil {
			return nil, err
		}
		names := make([]string, 0, len(rows))
		for _, row := range rows {
			names = append(names, row.Name)
		}
		return names, nil
	})
}

func completeAssetNames(deps *Deps) cobra.CompletionFunc {
	return moduleCompletion(deps, []module.ID{module.Assets}, func(ctx context.Context, enabled module.Set) ([]string, error) {
		if deps.Assets == nil {
			return nil, nil
		}
		rows, err := deps.Assets.List(ctx, enabled, "")
		if err != nil {
			return nil, err
		}
		names := make([]string, 0, len(rows))
		for _, row := range rows {
			names = append(names, row.Asset.Name)
		}
		return names, nil
	})
}

func moduleCompletions(deps *Deps, completeAccount cobra.CompletionFunc) []completionRegistration {
	budget := completeSingleArg(0, completeBudgetNames(deps))
	recurrence := completeSingleArg(0, completeRecurrenceIDs(deps))
	loan := completeSingleArg(0, completeLoanNames(deps))
	rule := completeSingleArg(0, completeRuleNames(deps))
	security := completeSingleArg(0, completeSecurityNames(deps))
	asset := completeSingleArg(0, completeAssetNames(deps))
	return []completionRegistration{
		{path: []string{"reconcile"}, args: completeSingleArg(0, completeAccount)},
		{path: []string{"budget", "show"}, args: budget},
		{path: []string{"budget", "status"}, args: budget},
		{path: []string{"budget", "set"}, args: budget},
		{path: []string{"recurring", "skip"}, args: recurrence},
		{path: []string{"recurring", "edit"}, args: recurrence},
		{path: []string{"recurring", "rm"}, args: recurrence},
		{path: []string{"loan", "show"}, args: loan},
		{path: []string{"loan", "schedule"}, args: loan},
		{path: []string{"loan", "event"}, args: loan},
		{path: []string{"loan", "run"}, flags: map[string]cobra.CompletionFunc{"loan": completeLoanNames(deps)}},
		{path: []string{"rule", "edit"}, args: rule},
		{path: []string{"rule", "rm"}, args: rule},
		{path: []string{"rule", "test"}, args: rule},
		{path: []string{"invest", "buy"}, args: security},
		{path: []string{"invest", "sell"}, args: security},
		{path: []string{"invest", "dividend"}, args: security},
		{path: []string{"security", "price"}, args: security},
		{path: []string{"asset", "value"}, args: asset},
		{path: []string{"asset", "show"}, args: asset},
	}
}
