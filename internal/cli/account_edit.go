package cli

import (
	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app"
)

// newAccountEditCmd changes the given fields of an active account; unset flags leave their
// field unchanged.
func newAccountEditCmd(deps *Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "edit NAME",
		Short:   "Edit an account",
		Args:    exactArgs(1),
		Example: "  fui account edit Checking --name \"Main Checking\"",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAccountEdit(cmd, deps, args[0])
		},
	}
	cmd.Flags().String("name", "", "new name")
	cmd.Flags().String("description", "", "new description")
	cmd.Flags().String("kind", "", "new kind")
	cmd.Flags().String("opening-balance", "", "new opening balance")
	return cmd
}

func runAccountEdit(cmd *cobra.Command, deps *Deps, name string) error {
	rt, err := runtimeOf(cmd, deps)
	if err != nil {
		return err
	}
	ctx := cmd.Context()
	plan, err := deps.Service.PlanEditAccount(ctx, app.EditAccountRequest{
		Account: name, Name: changedString(cmd, "name"), Description: changedString(cmd, "description"),
		Kind: changedString(cmd, "kind"), OpeningBalance: changedString(cmd, "opening-balance"),
	})
	if err != nil {
		return err
	}
	return writeChange(cmd, deps, rt, plan.Plan, func() error {
		acct, err := deps.Service.ApplyEditAccount(ctx, plan)
		if err != nil {
			return err
		}
		return renderAccountByName(cmd, deps, rt, acct.Name)
	})
}
