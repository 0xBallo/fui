package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/cli/render"
)

func newRuleRemoveCmd(deps *Deps) *cobra.Command {
	return &cobra.Command{
		Use:     "rm NAME",
		Short:   "Remove a categorization rule",
		Args:    exactArgs(1),
		Example: "  fui rule rm groceries",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRuleRemove(cmd, deps, args[0])
		},
	}
}

func runRuleRemove(cmd *cobra.Command, deps *Deps, name string) error {
	if err := requireRules(deps); err != nil {
		return err
	}
	rt, err := runtimeOf(cmd, deps)
	if err != nil {
		return err
	}
	ctx := cmd.Context()
	plan, err := deps.Rules.PlanRemove(ctx, name)
	if err != nil {
		return err
	}
	return writeChange(cmd, deps, rt, plan.Plan, func() error {
		if err := deps.Rules.ApplyRemove(ctx, plan); err != nil {
			return err
		}
		if rt.format == render.JSON {
			return render.WriteJSON(deps.Stdout, struct {
				Removed string `json:"removed"`
			}{Removed: plan.Rule.Name})
		}
		_, err := fmt.Fprintf(deps.Stdout, "Removed rule %q.\n", plan.Rule.Name)
		if err != nil {
			return fmt.Errorf("write remove-rule note: %w", err)
		}
		return nil
	})
}
