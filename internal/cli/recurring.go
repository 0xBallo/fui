package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newRecurringCmd groups recurring-transaction template commands.
func newRecurringCmd(deps *Deps) *cobra.Command {
	cmd := &cobra.Command{Use: "recurring", Short: "Add, list, run, skip or confirm recurring transactions"}
	cmd.AddCommand(
		newRecurringAddCmd(deps),
		newRecurringListCmd(deps),
		newRecurringRunCmd(deps),
		newRecurringSkipCmd(deps),
		newRecurringConfirmCmd(deps),
	)
	return cmd
}

// requireRecurring returns an error when deps has no recurring service configured, so commands
// fail cleanly instead of panicking in builds that have not wired one yet.
func requireRecurring(deps *Deps) error {
	if deps.Recurring == nil {
		return fmt.Errorf("fui: recurring service not configured")
	}
	return nil
}
