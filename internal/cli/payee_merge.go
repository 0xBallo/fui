package cli

import (
	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app"
)

// newPayeeMergeCmd reassigns every transaction paid to SOURCE to TARGET, keeps SOURCE's name as
// an alias of TARGET, and removes SOURCE.
func newPayeeMergeCmd(deps *Deps) *cobra.Command {
	return &cobra.Command{
		Use:     "merge SOURCE TARGET",
		Short:   "Merge a payee into another",
		Args:    exactArgs(2),
		Example: "  fui payee merge Amazn Amazon --yes",
		RunE: func(cmd *cobra.Command, args []string) error {
			rt, err := runtimeOf(cmd, deps)
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			plan, err := deps.Service.PlanMergePayee(ctx, app.MergePayeeRequest{
				Source: args[0], Target: args[1],
			})
			if err != nil {
				return err
			}
			return writeChange(cmd, deps, rt, plan.Plan, func() error {
				if err := deps.Service.ApplyMergePayee(ctx, plan); err != nil {
					return err
				}
				return renderMovedCount(deps, rt, plan.Moved)
			})
		},
	}
}
