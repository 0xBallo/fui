package cli

import (
	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/cli/render"
)

// newCategoryAddCmd creates a category (expense by default).
func newCategoryAddCmd(deps *Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "add NAME",
		Short:   "Add a category",
		Args:    cobra.ExactArgs(1),
		Example: "  fui category add Freelance --kind income",
		RunE: func(cmd *cobra.Command, args []string) error {
			rt, err := runtimeOf(cmd, deps)
			if err != nil {
				return err
			}
			kind, _ := cmd.Flags().GetString("kind")
			ctx := cmd.Context()
			plan, err := deps.Service.PlanAddCategory(ctx, app.AddCategoryRequest{Name: args[0], Kind: kind})
			if err != nil {
				return err
			}
			return writeChange(cmd, deps, rt, plan.Plan, func() error {
				c, err := deps.Service.ApplyAddCategory(ctx, plan)
				if err != nil {
					return err
				}
				return render.WriteCategory(deps.Stdout, rt.format, c)
			})
		},
	}
	cmd.Flags().String("kind", "", "income or expense (default: expense)")
	return cmd
}
