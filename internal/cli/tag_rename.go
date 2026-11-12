package cli

import (
	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/cli/render"
)

// newTagRenameCmd renames a tag.
func newTagRenameCmd(deps *Deps) *cobra.Command {
	return &cobra.Command{
		Use:     "rename NAME NEW_NAME",
		Short:   "Rename a tag",
		Args:    exactArgs(2),
		Example: "  fui tag rename vacanze vacanza",
		RunE: func(cmd *cobra.Command, args []string) error {
			rt, err := runtimeOf(cmd, deps)
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			plan, err := deps.Service.PlanRenameTag(ctx, app.RenameTagRequest{
				Tag: args[0], NewName: args[1],
			})
			if err != nil {
				return err
			}
			return writeChange(cmd, deps, rt, plan.Plan, func() error {
				tg, err := deps.Service.ApplyRenameTag(ctx, plan)
				if err != nil {
					return err
				}
				return render.WriteTag(deps.Stdout, rt.format, tg)
			})
		},
	}
}
