package cli

import (
	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app/recurring"
	"github.com/0xballo/fui/internal/cli/render"
)

// newRecurringRunCmd materialises every due occurrence through --until (today when empty) as a
// pending transaction, advancing each recurrence's next date past what it creates.
func newRecurringRunCmd(deps *Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "run",
		Short:   "Materialise due recurring transactions as pending",
		Args:    noArgs(),
		Example: "  fui recurring run --until today",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runRecurringRun(cmd, deps)
		},
	}
	cmd.Flags().String("until", "", "materialise occurrences due on or before this date (default: today)")
	return cmd
}

func runRecurringRun(cmd *cobra.Command, deps *Deps) error {
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
	until, _ := cmd.Flags().GetString("until")
	ctx := cmd.Context()
	plan, err := deps.Recurring.PlanRun(ctx, enabled, recurring.RunRequest{Until: until})
	if err != nil {
		return err
	}
	return writeChange(cmd, deps, rt, plan.Plan, func() error {
		created, err := deps.Recurring.ApplyRun(ctx, plan)
		if err != nil {
			return err
		}
		return render.WriteRecurringRunResult(deps.Stdout, rt.format, created)
	})
}
