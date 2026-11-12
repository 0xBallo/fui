package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app/rules"
	"github.com/0xballo/fui/internal/cli/render"
)

func newRuleEditCmd(deps *Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "edit NAME",
		Short:   "Edit a categorization rule",
		Args:    exactArgs(1),
		Example: "  fui rule edit groceries --priority 5 --disable",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRuleEdit(cmd, deps, args[0])
		},
	}
	registerRuleConditionFlags(cmd)
	registerRuleActionFlags(cmd)
	cmd.Flags().Int("priority", 0, "priority (lower runs first)")
	cmd.Flags().Bool("set-priority", false, "set --priority")
	cmd.Flags().Bool("enable", false, "enable the rule")
	cmd.Flags().Bool("disable", false, "disable the rule")
	cmd.Flags().Bool("set-conditions", false, "replace the full conditions block from condition flags")
	cmd.Flags().Bool("set-actions", false, "replace the full actions block from action flags")
	return cmd
}

func runRuleEdit(cmd *cobra.Command, deps *Deps, name string) error {
	if err := requireRules(deps); err != nil {
		return err
	}
	rt, err := runtimeOf(cmd, deps)
	if err != nil {
		return err
	}
	req, err := ruleEditRequest(cmd, deps, name)
	if err != nil {
		return err
	}
	ctx := cmd.Context()
	plan, err := deps.Rules.PlanEdit(ctx, req)
	if err != nil {
		return err
	}
	return writeChange(cmd, deps, rt, plan.Plan, func() error {
		if err := deps.Rules.ApplyEdit(ctx, plan); err != nil {
			return err
		}
		lookup, err := loadRuleLookup(ctx, deps)
		if err != nil {
			return err
		}
		return render.WriteRule(deps.Stdout, rt.format, plan.Rule, lookup)
	})
}

func ruleEditRequest(cmd *cobra.Command, deps *Deps, name string) (rules.EditRequest, error) {
	var req rules.EditRequest
	req.Name = name

	setPriority, _ := cmd.Flags().GetBool("set-priority")
	if setPriority {
		priority, _ := cmd.Flags().GetInt("priority")
		req.Priority = &priority
	}
	enable, _ := cmd.Flags().GetBool("enable")
	disable, _ := cmd.Flags().GetBool("disable")
	if enable && disable {
		return rules.EditRequest{}, newUsageError(fmt.Errorf("--enable and --disable are mutually exclusive"))
	}
	if enable {
		v := true
		req.Enabled = &v
	}
	if disable {
		v := false
		req.Enabled = &v
	}
	setConditions, _ := cmd.Flags().GetBool("set-conditions")
	if setConditions {
		conds, err := ruleConditionsFromFlags(cmd.Flags(), deps)
		if err != nil {
			return rules.EditRequest{}, err
		}
		req.Conditions = &conds
	}
	setActions, _ := cmd.Flags().GetBool("set-actions")
	if setActions {
		actions := ruleActionsFromFlags(cmd.Flags())
		req.Actions = &actions
	}
	return req, nil
}
