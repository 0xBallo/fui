package cli

import "github.com/spf13/cobra"

// newAccountArchiveCmd archives an active account; it stays visible in history and reports but
// stops being offered as a transaction target.
func newAccountArchiveCmd(deps *Deps) *cobra.Command {
	return &cobra.Command{
		Use:     "archive NAME",
		Short:   "Archive an account",
		Args:    exactArgs(1),
		Example: "  fui account archive \"Old Savings\"",
		RunE: func(cmd *cobra.Command, args []string) error {
			rt, err := runtimeOf(cmd, deps)
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			plan, err := deps.Service.PlanArchiveAccount(ctx, args[0])
			if err != nil {
				return err
			}
			return writeChange(cmd, deps, rt, plan.Plan, func() error {
				acct, err := deps.Service.ApplyArchiveAccount(ctx, plan)
				if err != nil {
					return err
				}
				return renderAccountByName(cmd, deps, rt, acct.Name)
			})
		},
	}
}
