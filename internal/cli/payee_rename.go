package cli

import (
	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/cli/render"
)

// newPayeeRenameCmd renames a payee.
func newPayeeRenameCmd(deps *Deps) *cobra.Command {
	return &cobra.Command{
		Use:     "rename NAME NEW_NAME",
		Short:   "Rename a payee",
		Args:    exactArgs(2),
		Example: "  fui payee rename Amazn Amazon",
		RunE: func(cmd *cobra.Command, args []string) error {
			rt, err := runtimeOf(cmd, deps)
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			plan, err := deps.Service.PlanRenamePayee(ctx, app.RenamePayeeRequest{
				Payee: args[0], NewName: args[1],
			})
			if err != nil {
				return err
			}
			return writeChange(cmd, deps, rt, plan.Plan, func() error {
				p, err := deps.Service.ApplyRenamePayee(ctx, plan)
				if err != nil {
					return err
				}
				return render.WritePayee(deps.Stdout, rt.format, p)
			})
		},
	}
}
