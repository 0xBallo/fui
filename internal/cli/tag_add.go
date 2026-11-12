package cli

import (
	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/cli/render"
)

// newTagAddCmd creates a tag.
func newTagAddCmd(deps *Deps) *cobra.Command {
	return &cobra.Command{
		Use:     "add NAME",
		Short:   "Add a tag",
		Args:    exactArgs(1),
		Example: "  fui tag add vacanze-2026",
		RunE: func(cmd *cobra.Command, args []string) error {
			rt, err := runtimeOf(cmd, deps)
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			plan, err := deps.Service.PlanAddTag(ctx, app.AddTagRequest{Name: args[0]})
			if err != nil {
				return err
			}
			return writeChange(cmd, deps, rt, plan.Plan, func() error {
				tg, err := deps.Service.ApplyAddTag(ctx, plan)
				if err != nil {
					return err
				}
				return render.WriteTag(deps.Stdout, rt.format, tg)
			})
		},
	}
}
