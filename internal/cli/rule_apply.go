package cli

import (
	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app/rules"
	"github.com/0xballo/fui/internal/cli/render"
)

func newRuleApplyCmd(deps *Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "apply",
		Short:   "Apply enabled rules to matching transactions",
		Example: "  fui rule apply --account Checking --month 2026-10 --dry-run",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runRuleApply(cmd, deps)
		},
	}
	registerFilterFlags(cmd)
	cmd.Flags().Bool("confirm-reconciled", false, "allow applying rules to reconciled transactions")
	return cmd
}

func runRuleApply(cmd *cobra.Command, deps *Deps) error {
	if err := requireRules(deps); err != nil {
		return err
	}
	rt, err := runtimeOf(cmd, deps)
	if err != nil {
		return err
	}
	filterReq, err := readFilterFlags(cmd)
	if err != nil {
		return err
	}
	filter, err := deps.Service.ResolveTxFilter(cmd.Context(), filterReq)
	if err != nil {
		return err
	}
	confirmReconciled, _ := cmd.Flags().GetBool("confirm-reconciled")
	ctx := cmd.Context()
	plan, err := deps.Rules.PlanApply(ctx, rules.ApplyRequest{
		Filter: filter, ConfirmReconciled: confirmReconciled,
	})
	if err != nil {
		return err
	}
	preview := func() error { return render.WriteRuleApplyPlan(deps.Stdout, rt.format, plan) }
	return writeChangePreview(cmd, deps, rt, plan.Plan, preview, func() error {
		if err := deps.Rules.ApplyApply(ctx, plan); err != nil {
			return err
		}
		return render.WriteRuleApplyPlan(deps.Stdout, rt.format, plan)
	})
}
