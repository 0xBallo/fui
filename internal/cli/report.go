package cli

import "github.com/spf13/cobra"

// newReportCmd is the parent of the predefined reports.
func newReportCmd(deps *Deps) *cobra.Command {
	cmd := &cobra.Command{Use: "report", Short: "Run a predefined report"}
	cmd.AddCommand(newReportRunCmd(deps))
	return cmd
}
