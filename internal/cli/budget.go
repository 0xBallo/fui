package cli

import (
	"strings"

	"github.com/spf13/cobra"
)

// newBudgetCmd groups the budget subcommands. Not yet registered in the command tree (see V32).
func newBudgetCmd(deps *Deps) *cobra.Command {
	cmd := &cobra.Command{Use: "budget", Short: "Set and check budgets (caps or envelopes)"}
	cmd.AddCommand(
		newBudgetSetCmd(deps), newBudgetListCmd(deps), newBudgetStatusCmd(deps),
		newBudgetAssignCmd(deps), newBudgetMoveCmd(deps), newBudgetCopyCmd(deps),
	)
	return cmd
}

// parseBudgetTarget turns a "<category|#tag>" target into the budget name and the single
// category or tag it covers: a bare name targets a category, a "#name" targets a tag. The
// budget is named after its target, so repeating the same target updates that budget.
func parseBudgetTarget(target string) (name string, categories, tags []string) {
	if after, ok := strings.CutPrefix(target, "#"); ok {
		return after, nil, []string{after}
	}
	return target, []string{target}, nil
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
