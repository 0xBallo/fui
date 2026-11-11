package cli

import (
	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/cli/render"
)

// newCategoryRenameCmd renames a category.
func newCategoryRenameCmd(deps *Deps) *cobra.Command {
	return &cobra.Command{
		Use:     "rename NAME NEW_NAME",
		Short:   "Rename a category",
		Args:    exactArgs(2),
		Example: "  fui category rename Dinning Dining",
		RunE: func(cmd *cobra.Command, args []string) error {
			rt, err := runtimeOf(cmd, deps)
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			plan, err := deps.Service.PlanRenameCategory(ctx, app.RenameCategoryRequest{
				Category: args[0], NewName: args[1],
			})
			if err != nil {
				return err
			}
			return writeChange(cmd, deps, rt, plan.Plan, func() error {
				c, err := deps.Service.ApplyRenameCategory(ctx, plan)
				if err != nil {
					return err
				}
				return render.WriteCategory(deps.Stdout, rt.format, c)
			})
		},
	}
}
