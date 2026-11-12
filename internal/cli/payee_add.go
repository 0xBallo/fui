package cli

import (
	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/cli/render"
)

// newPayeeAddCmd creates a payee, optionally with starting aliases.
func newPayeeAddCmd(deps *Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "add NAME",
		Short:   "Add a payee",
		Args:    exactArgs(1),
		Example: `  fui payee add Amazon --alias "AMZN Mktp"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			rt, err := runtimeOf(cmd, deps)
			if err != nil {
				return err
			}
			aliases, _ := cmd.Flags().GetStringArray("alias")
			ctx := cmd.Context()
			plan, err := deps.Service.PlanAddPayee(ctx, app.AddPayeeRequest{Name: args[0], Aliases: aliases})
			if err != nil {
				return err
			}
			return writeChange(cmd, deps, rt, plan.Plan, func() error {
				p, err := deps.Service.ApplyAddPayee(ctx, plan)
				if err != nil {
					return err
				}
				return render.WritePayee(deps.Stdout, rt.format, p)
			})
		},
	}
	cmd.Flags().StringArray("alias", nil, "alternate name recognized during import/rules (repeatable)")
	return cmd
}
