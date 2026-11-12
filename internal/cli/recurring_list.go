package cli

import (
	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/cli/render"
)

// newRecurringListCmd lists every recurrence.
func newRecurringListCmd(deps *Deps) *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Short:   "List recurring transactions",
		Args:    noArgs(),
		Example: "  fui recurring list --json",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := requireRecurring(deps); err != nil {
				return err
			}
			rt, err := runtimeOf(cmd, deps)
			if err != nil {
				return err
			}
			enabled, err := enabledModulesFromDeps(deps)
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			recs, err := deps.Recurring.List(ctx, enabled)
			if err != nil {
				return err
			}
			lookup, err := loadRecurrenceLookup(ctx, deps)
			if err != nil {
				return err
			}
			return render.WriteRecurrenceList(deps.Stdout, rt.format, deps.Locale(), recs, lookup)
		},
	}
}
