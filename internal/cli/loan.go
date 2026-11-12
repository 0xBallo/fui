package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newLoanCmd groups loan and mortgage commands.
func newLoanCmd(deps *Deps) *cobra.Command {
	cmd := &cobra.Command{Use: "loan", Short: "Add, show, schedule, run or confirm loans and mortgages"}
	cmd.AddCommand(
		newLoanAddCmd(deps),
		newLoanShowCmd(deps),
		newLoanScheduleCmd(deps),
		newLoanEventCmd(deps),
		newLoanRunCmd(deps),
		newLoanConfirmCmd(deps),
	)
	return cmd
}

// requireLoans returns an error when deps has no loans service configured, so commands fail
// cleanly instead of panicking in builds that have not wired one yet.
func requireLoans(deps *Deps) error {
	if deps.Loans == nil {
		return fmt.Errorf("fui: loans service not configured")
	}
	return nil
}
