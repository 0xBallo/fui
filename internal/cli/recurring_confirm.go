package cli

import (
	"strconv"

	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/cli/render"
	"github.com/0xballo/fui/internal/domain"
)

// newRecurringConfirmCmd books a pending transaction a recurrence created. Importing a matching
// booked movement does the same thing automatically.
func newRecurringConfirmCmd(deps *Deps) *cobra.Command {
	return &cobra.Command{
		Use:     "confirm TX_ID",
		Short:   "Confirm a pending recurring transaction as booked",
		Args:    exactArgs(1),
		Example: "  fui recurring confirm 128",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRecurringConfirm(cmd, deps, args[0])
		},
	}
}

func runRecurringConfirm(cmd *cobra.Command, deps *Deps, idArg string) error {
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
	plan, err := deps.Recurring.PlanConfirm(ctx, enabled, domain.TransactionID(id))
	if err != nil {
		return err
	}
	return writeChange(cmd, deps, rt, plan.Plan, func() error {
		if err := deps.Recurring.ApplyConfirm(ctx, plan); err != nil {
			return err
		}
		v, err := deps.Service.ShowTx(ctx, plan.Tx.ID)
		if err != nil {
			return err
		}
		return render.WriteTx(deps.Stdout, rt.format, deps.Locale(), rt.color, v)
	})
}
