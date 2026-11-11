package cli

import (
	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app"
)

// newCategoryMergeCmd moves every transaction of SOURCE to TARGET and removes SOURCE.
func newCategoryMergeCmd(deps *Deps) *cobra.Command {
	return &cobra.Command{
		Use:     "merge SOURCE TARGET",
		Short:   "Merge a category into another",
		Args:    exactArgs(2),
		Example: "  fui category merge Dinning Dining --yes",
		RunE: func(cmd *cobra.Command, args []string) error {
			rt, err := runtimeOf(cmd, deps)
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			plan, err := deps.Service.PlanMergeCategory(ctx, app.MergeCategoryRequest{
				Source: args[0], Target: args[1],
			})
			if err != nil {
				return err
			}
			return writeChange(cmd, deps, rt, plan.Plan, func() error {
				res, err := deps.Service.ApplyMergeCategory(ctx, plan)
				if err != nil {
					return err
				}
				return renderCategoryRemoval(deps, rt, res)
			})
		},
	}
}
