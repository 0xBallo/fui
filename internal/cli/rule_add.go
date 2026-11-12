package cli

import (
	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app/rules"
	"github.com/0xballo/fui/internal/cli/render"
)

func newRuleAddCmd(deps *Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "add NAME",
		Short:   "Add a categorization rule",
		Args:    exactArgs(1),
		Example: "  fui rule add groceries --contains supermarket --category Food --priority 10",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRuleAdd(cmd, deps, args[0])
		},
	}
	registerRuleConditionFlags(cmd)
	registerRuleActionFlags(cmd)
	cmd.Flags().Int("priority", 100, "priority (lower runs first)")
	cmd.Flags().Bool("disabled", false, "create the rule as disabled")
	return cmd
}

func runRuleAdd(cmd *cobra.Command, deps *Deps, name string) error {
	if err := requireRules(deps); err != nil {
		return err
	}
	rt, err := runtimeOf(cmd, deps)
	if err != nil {
		return err
	}
	conds, err := ruleConditionsFromFlags(cmd.Flags(), deps)
	if err != nil {
		return err
	}
	priority, _ := cmd.Flags().GetInt("priority")
	disabled, _ := cmd.Flags().GetBool("disabled")
	ctx := cmd.Context()
	plan, err := deps.Rules.PlanAdd(ctx, rules.AddRequest{
		Name: name, Priority: priority, Disabled: disabled, Conditions: conds, Actions: ruleActionsFromFlags(cmd.Flags()),
	})
	if err != nil {
		return err
	}
	return writeChange(cmd, deps, rt, plan.Plan, func() error {
		r, err := deps.Rules.ApplyAdd(ctx, plan)
		if err != nil {
			return err
		}
		lookup, err := loadRuleLookup(ctx, deps)
		if err != nil {
			return err
		}
		return render.WriteRule(deps.Stdout, rt.format, r, lookup)
	})
}

func registerRuleConditionFlags(cmd *cobra.Command) {
	cmd.Flags().String("contains", "", "match when description contains this text")
	cmd.Flags().String("equals", "", "match when description equals this text")
	cmd.Flags().String("min", "", "minimum absolute amount")
	cmd.Flags().String("max", "", "maximum absolute amount")
	cmd.Flags().String("sign", "", "any, inflow or outflow")
	cmd.Flags().StringArray("account", nil, "account name (repeatable)")
}

func registerRuleActionFlags(cmd *cobra.Command) {
	cmd.Flags().String("category", "", "category name to set")
	cmd.Flags().String("payee", "", "payee name to set")
	cmd.Flags().StringArray("tag", nil, "tag name to add (repeatable)")
}
