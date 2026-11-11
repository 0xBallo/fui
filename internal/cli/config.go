package cli

import "github.com/spf13/cobra"

// newConfigCmd groups the configuration subcommands.
func newConfigCmd(deps *Deps) *cobra.Command {
	cmd := &cobra.Command{Use: "config", Short: "Read or change the configuration"}
	cmd.AddCommand(newConfigGetCmd(deps), newConfigSetCmd(deps))
	return cmd
}
