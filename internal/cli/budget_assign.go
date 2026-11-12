package cli

import (
	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app/budgeting"
	"github.com/0xballo/fui/internal/cli/render"
)

// newBudgetAssignCmd assigns an amount from the unassigned pool ("Da assegnare") to an envelope.
// It needs budget.mode = envelope.
func newBudgetAssignCmd(deps *Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "assign ENVELOPE AMOUNT",
		Short:   "Assign an amount from the unassigned pool to an envelope",
		Args:    exactArgs(2),
		Example: "  fui budget assign Groceries 400",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBudgetMove(cmd, deps, "", args[0], args[1])
		},
	}
	cmd.Flags().String("date", "", "date of the assignment (default: today)")
	return cmd
}

// runBudgetMove plans and applies the envelope move shared by `assign` (from the pool) and
// `move` (between two envelopes); from or to empty means the unassigned pool.
func runBudgetMove(cmd *cobra.Command, deps *Deps, from, to, amount string) error {
	rt, err := runtimeOf(cmd, deps)
	if err != nil {
		return err
	}
	svc, err := requireBudgeting(deps)
	if err != nil {
		return err
	}
	date, _ := cmd.Flags().GetString("date")
	ctx := cmd.Context()
	plan, err := svc.PlanMove(ctx, budgeting.MoveRequest{From: from, To: to, Amount: amount, Date: date})
	if err != nil {
		return err
	}
	return writeChange(cmd, deps, rt, plan.Plan, func() error {
		if err := svc.ApplyMove(ctx, plan); err != nil {
			return err
		}
		return render.WriteBudgetMove(deps.Stdout, rt.format, deps.Locale(), plan.Move.Date, from, to, plan.Move.Amount)
	})
}
