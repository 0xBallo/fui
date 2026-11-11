package cli

import "github.com/spf13/cobra"

// newFXCmd groups the manual exchange rate subcommands. Manual rates are an MVP-only feature not
// in the functional spec's command table: fui needs them to convert balances in NetWorth/KPIs
// before a live-rate connector exists.
func newFXCmd(deps *Deps) *cobra.Command {
	cmd := &cobra.Command{Use: "fx", Short: "Set or list manual exchange rates"}
	cmd.AddCommand(newFXSetCmd(deps), newFXListCmd(deps))
	return cmd
}
