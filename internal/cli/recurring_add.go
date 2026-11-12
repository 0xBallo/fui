package cli

import (
	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app/recurring"
	"github.com/0xballo/fui/internal/cli/render"
)

// newRecurringAddCmd creates a recurrence: a template that materialises pending transactions on
// a schedule (see `fui recurring run`).
func newRecurringAddCmd(deps *Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "add",
		Short:   "Add a recurring transaction",
		Args:    noArgs(),
		Example: "  fui recurring add --account Checking --amount -9,99 --description Netflix --every 1 --unit month",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runRecurringAdd(cmd, deps)
		},
	}
	registerRecurrenceFlags(cmd)
	cmd.Flags().String("start", "", "first occurrence date (default: today)")
	return cmd
}

func registerRecurrenceFlags(cmd *cobra.Command) {
	cmd.Flags().String("account", "", "account the occurrences post to (required)")
	cmd.Flags().String("category", "", "category to set on each occurrence")
	cmd.Flags().String("description", "", "description to set on each occurrence")
	cmd.Flags().String("amount", "", "amount of each occurrence, signed (required)")
	cmd.Flags().Int("every", 1, "repeat every N units (required, N >= 1)")
	cmd.Flags().String("unit", "", "days, weeks or months (required)")
	cmd.Flags().Int("day-of-month", 0, "fixed day of month, for the month and year units")
	cmd.Flags().String("end", "", "last occurrence date (default: no end)")
}

func runRecurringAdd(cmd *cobra.Command, deps *Deps) error {
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
	account, _ := cmd.Flags().GetString("account")
	category, _ := cmd.Flags().GetString("category")
	description, _ := cmd.Flags().GetString("description")
	amount, _ := cmd.Flags().GetString("amount")
	every, _ := cmd.Flags().GetInt("every")
	unit, _ := cmd.Flags().GetString("unit")
	dayOfMonth, _ := cmd.Flags().GetInt("day-of-month")
	start, _ := cmd.Flags().GetString("start")
	end, _ := cmd.Flags().GetString("end")
	ctx := cmd.Context()
	plan, err := deps.Recurring.PlanAdd(ctx, enabled, recurring.AddRequest{
		Account: account, Category: category, Description: description, Amount: amount,
		Every: every, Unit: unit, DayOfMonth: dayOfMonth, Start: start, End: end,
	})
	if err != nil {
		return err
	}
	return writeChange(cmd, deps, rt, plan.Plan, func() error {
		rec, err := deps.Recurring.ApplyAdd(ctx, plan)
		if err != nil {
			return err
		}
		lookup, err := loadRecurrenceLookup(ctx, deps)
		if err != nil {
			return err
		}
		return render.WriteRecurrence(deps.Stdout, rt.format, deps.Locale(), rec, lookup)
	})
}
