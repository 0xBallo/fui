package cli

import (
	"strconv"

	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/cli/render"
)

// newRecurringSkipCmd advances a recurrence's next date past its next occurrence without
// creating a pending transaction for it.
func newRecurringSkipCmd(deps *Deps) *cobra.Command {
	return &cobra.Command{
		Use:     "skip ID",
		Short:   "Skip the next occurrence of a recurring transaction",
		Args:    exactArgs(1),
		Example: "  fui recurring skip 4",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRecurringSkip(cmd, deps, args[0])
		},
	}
}

func runRecurringSkip(cmd *cobra.Command, deps *Deps, idArg string) error {
	if err := requireRecurring(deps); err != nil {
		return err
	}
	id, err := strconv.ParseInt(idArg, 10, 64)
	if err != nil {
		return missingFieldError("id")
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
	plan, err := deps.Recurring.PlanSkip(ctx, enabled, app.RecurrenceID(id))
	if err != nil {
		return err
	}
	return writeChange(cmd, deps, rt, plan.Plan, func() error {
		if err := deps.Recurring.ApplySkip(ctx, plan); err != nil {
			return err
		}
		lookup, err := loadRecurrenceLookup(ctx, deps)
		if err != nil {
			return err
		}
		return render.WriteRecurrence(deps.Stdout, rt.format, deps.Locale(), plan.Recurrence, lookup)
	})
}
