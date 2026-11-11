package cli

import "github.com/spf13/cobra"

// newTxCmd groups the transaction read/write subcommands other than the fui add shorthand.
func newTxCmd(deps *Deps) *cobra.Command {
	cmd := &cobra.Command{Use: "tx", Short: "List, show, edit or remove transactions"}
	cmd.AddCommand(newTxListCmd(deps), newTxShowCmd(deps), newTxEditCmd(deps), newTxRemoveCmd(deps))
	return cmd
}
