package cli

import (
	"github.com/spf13/cobra"
)

// newBudgetCmd groups the budget subcommands.
func newBudgetCmd(deps *Deps) *cobra.Command {
	cmd := &cobra.Command{Use: "budget", Short: "Set and check budgets (caps or envelopes)"}
	cmd.AddCommand(
		newBudgetSetCmd(deps), newBudgetListCmd(deps), newBudgetStatusCmd(deps),
		newBudgetAssignCmd(deps), newBudgetMoveCmd(deps), newBudgetCopyCmd(deps),
	)
	return cmd
}

// changedBool returns a pointer to the value of flag when the caller set it, else nil, mirroring
// changedString (tx_edit.go) for the tri-state budget.set --carry flag.
func changedBool(cmd *cobra.Command, flag string) *bool {
	if !cmd.Flags().Changed(flag) {
		return nil
	}
	v, _ := cmd.Flags().GetBool(flag)
	return &v
}

// changedInt returns a pointer to the value of flag when the caller set it, else nil.
func changedInt(cmd *cobra.Command, flag string) *int {
	if !cmd.Flags().Changed(flag) {
		return nil
	}
	v, _ := cmd.Flags().GetInt(flag)
	return &v
}
