package cli

import (
	"strconv"

	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/recurring"
	"github.com/0xballo/fui/internal/cli/render"
)

// newRecurringEditCmd changes the template fields of an existing recurrence. The account and the
// dates of the schedule are fixed; `recurring skip` moves the next date.
func newRecurringEditCmd(deps *Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "edit ID",
		Short:   "Edit a recurring transaction",
		Args:    exactArgs(1),
		Example: "  fui recurring edit 4 --amount -11,99 --description \"Netflix 4K\"",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRecurringEdit(cmd, deps, args[0])
		},
	}
	cmd.Flags().String("category", "", "new category (empty clears it)")
	cmd.Flags().String("description", "", "new description")
	cmd.Flags().String("amount", "", "new amount of each occurrence, signed")
	cmd.Flags().Int("every", 0, "repeat every N units (N >= 1)")
	cmd.Flags().String("unit", "", "days, weeks or months")
	cmd.Flags().Int("day-of-month", 0, "fixed day of month, for the month and year units")
	cmd.Flags().String("end", "", "last occurrence date (empty removes the end)")
	return cmd
}

func runRecurringEdit(cmd *cobra.Command, deps *Deps, idArg string) error {
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
	req := recurring.EditRequest{
		ID:          app.RecurrenceID(id),
		Category:    changedString(cmd, "category"),
		Description: changedString(cmd, "description"),
		Amount:      changedString(cmd, "amount"),
		Every:       changedInt(cmd, "every"),
		Unit:        changedString(cmd, "unit"),
		DayOfMonth:  changedInt(cmd, "day-of-month"),
		End:         changedString(cmd, "end"),
	}
	ctx := cmd.Context()
	plan, err := deps.Recurring.PlanEdit(ctx, enabled, req)
	if err != nil {
		return err
	}
	return writeChange(cmd, deps, rt, plan.Plan, func() error {
		if err := deps.Recurring.ApplyEdit(ctx, plan); err != nil {
			return err
		}
		lookup, err := loadRecurrenceLookup(ctx, deps)
		if err != nil {
			return err
		}
		return render.WriteRecurrence(deps.Stdout, rt.format, deps.Locale(), plan.Recurrence, lookup)
	})
}
