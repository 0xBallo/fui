package cli

import "github.com/spf13/cobra"

// newBudgetMoveCmd moves an amount from one envelope to another. It needs budget.mode =
// envelope; either side may be the unassigned pool by using `budget assign` instead.
func newBudgetMoveCmd(deps *Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "move FROM TO AMOUNT",
		Short:   "Move an amount between two envelopes",
		Args:    exactArgs(3),
		Example: "  fui budget move Groceries Transport 50",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBudgetMove(cmd, deps, args[0], args[1], args[2])
		},
	}
	cmd.Flags().String("date", "", "date of the move (default: today)")
	return cmd
}
