package cli

import (
	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app/budgeting"
	"github.com/0xballo/fui/internal/cli/render"
)

// newBudgetCopyCmd copies the budgets of --from (default: the month before --to) into --to
// (default: current month): cap amount overrides in cap mode, the net pool assignments in
// envelope mode.
func newBudgetCopyCmd(deps *Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "copy",
		Short:   "Copy a month's budgets into another month",
		Args:    noArgs(),
		Example: "  fui budget copy --from 2026-10 --to 2026-11",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runBudgetCopy(cmd, deps)
		},
	}
	cmd.Flags().String("from", "", "source month (YYYY-MM, default: the month before --to)")
	cmd.Flags().String("to", "", "target month (YYYY-MM, default: current)")
	return cmd
}

func runBudgetCopy(cmd *cobra.Command, deps *Deps) error {
	rt, err := runtimeOf(cmd, deps)
	if err != nil {
		return err
	}
	svc, err := requireBudgeting(deps)
	if err != nil {
		return err
	}
	from, _ := cmd.Flags().GetString("from")
	to, _ := cmd.Flags().GetString("to")
	ctx := cmd.Context()
	plan, err := svc.PlanCopy(ctx, budgeting.CopyRequest{Month: to, From: from})
	if err != nil {
		return err
	}
	return writeChange(cmd, deps, rt, plan.Plan, func() error {
		if err := svc.ApplyCopyPrevious(ctx, plan); err != nil {
			return err
		}
		return renderBudgetCopyResult(deps, rt, len(plan.Updates), len(plan.Moves))
	})
}

func renderBudgetCopyResult(deps *Deps, rt runtimeFlags, updates, moves int) error {
	return render.WriteBudgetCopyResult(deps.Stdout, rt.format, updates, moves)
}
