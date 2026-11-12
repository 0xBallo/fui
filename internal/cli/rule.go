package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newRuleCmd groups categorization-rule commands.
func newRuleCmd(deps *Deps) *cobra.Command {
	cmd := &cobra.Command{Use: "rule", Short: "Add, list, edit, remove, test or apply categorization rules"}
	cmd.AddCommand(
		newRuleAddCmd(deps),
		newRuleListCmd(deps),
		newRuleEditCmd(deps),
		newRuleRemoveCmd(deps),
		newRuleTestCmd(deps),
		newRuleApplyCmd(deps),
	)
	return cmd
}

func requireRules(deps *Deps) error {
	if deps.Rules == nil {
		return fmt.Errorf("fui: rules service not configured")
	}
	return nil
}
