package cli

import (
	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app"
)

// newCategoryRemoveCmd removes a category; one with transactions needs --reassign-to.
func newCategoryRemoveCmd(deps *Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "rm NAME",
		Short:   "Remove a category",
		Args:    cobra.ExactArgs(1),
		Example: "  fui category rm \"Other Expenses\" --reassign-to Shopping",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCategoryRemove(cmd, deps, args[0])
		},
	}
	cmd.Flags().String("reassign-to", "", "category to move existing transactions to")
	return cmd
}

func runCategoryRemove(cmd *cobra.Command, deps *Deps, name string) error {
	rt, err := runtimeOf(cmd, deps)
	if err != nil {
		return err
	}
	reassignTo, _ := cmd.Flags().GetString("reassign-to")
	ctx := cmd.Context()
	plan, err := deps.Service.PlanRemoveCategory(ctx, app.RemoveCategoryRequest{
		Category: name, ReassignTo: reassignTo,
	})
	if err != nil {
		return err
	}
	return writeChange(cmd, deps, rt, plan.Plan, func() error {
		res, err := deps.Service.ApplyRemoveCategory(ctx, plan)
		if err != nil {
			return err
		}
		return renderCategoryRemoval(deps, rt, res)
	})
}
