package cli

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/cli/render"
)

// newRecurringRemoveCmd deletes a recurrence; the transactions it already created are kept.
func newRecurringRemoveCmd(deps *Deps) *cobra.Command {
	return &cobra.Command{
		Use:     "rm ID",
		Short:   "Remove a recurring transaction",
		Args:    exactArgs(1),
		Example: "  fui recurring rm 4 --yes",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRecurringRemove(cmd, deps, args[0])
		},
	}
}

func runRecurringRemove(cmd *cobra.Command, deps *Deps, idArg string) error {
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
	plan, err := deps.Recurring.PlanRemove(ctx, enabled, app.RecurrenceID(id))
	if err != nil {
		return err
	}
	return writeChange(cmd, deps, rt, plan.Plan, func() error {
		if err := deps.Recurring.ApplyRemove(ctx, plan); err != nil {
			return err
		}
		if rt.format == render.JSON {
			return render.WriteJSON(deps.Stdout, struct {
				Removed int64 `json:"removed"`
			}{Removed: int64(plan.Recurrence.ID)})
		}
		if _, err := fmt.Fprintf(deps.Stdout, "Removed recurrence %d.\n", plan.Recurrence.ID); err != nil {
			return fmt.Errorf("write remove-recurrence note: %w", err)
		}
		return nil
	})
}
