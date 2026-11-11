package cli

import "github.com/spf13/cobra"

// newAccountShowCmd shows one account (archived included) with its balance.
func newAccountShowCmd(deps *Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "show NAME",
		Short:   "Show one account",
		Args:    cobra.ExactArgs(1),
		Example: "  fui account show Checking --at 2026-01-01",
		RunE: func(cmd *cobra.Command, args []string) error {
			rt, err := runtimeOf(cmd, deps)
			if err != nil {
				return err
			}
			at, _ := cmd.Flags().GetString("at")
			ab, err := deps.Service.ShowAccount(cmd.Context(), args[0], at)
			if err != nil {
				return err
			}
			return writeAccount(deps, rt, ab)
		},
	}
	cmd.Flags().String("at", "", "balance date (default: today)")
	return cmd
}
