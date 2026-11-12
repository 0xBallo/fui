package cli

import "github.com/spf13/cobra"

// newKPICmd is the parent of key-performance-indicator commands.
func newKPICmd(deps *Deps) *cobra.Command {
	cmd := &cobra.Command{Use: "kpi", Short: "Compute a key performance indicator"}
	cmd.AddCommand(newKPIRunwayCmd(deps), newKPINetWorthCmd(deps))
	return cmd
}
