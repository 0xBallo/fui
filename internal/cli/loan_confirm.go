package cli

import (
	"strconv"

	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/cli/render"
	"github.com/0xballo/fui/internal/domain"
)

// newLoanConfirmCmd confirms a pending installment transaction: it splits it into principal
// (transfer to the liability account) and interest/fees (expense), warning when the debited
// amount does not match the schedule and proposing the rate that would explain the difference.
func newLoanConfirmCmd(deps *Deps) *cobra.Command {
	return &cobra.Command{
		Use:     "confirm TX_ID",
		Short:   "Confirm a pending installment transaction as booked",
		Args:    exactArgs(1),
		Example: "  fui loan confirm 231",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLoanConfirm(cmd, deps, args[0])
		},
	}
}

func runLoanConfirm(cmd *cobra.Command, deps *Deps, idArg string) error {
	if err := requireLoans(deps); err != nil {
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
	plan, err := deps.Loans.PlanConfirm(ctx, enabled, domain.TransactionID(id))
	if err != nil {
		return err
	}
	return writeChange(cmd, deps, rt, plan.Plan, func() error {
		if err := deps.Loans.ApplyConfirm(ctx, plan); err != nil {
			return err
		}
		v, err := deps.Service.ShowTx(ctx, plan.Principal.ID)
		if err != nil {
			return err
		}
		return render.WriteLoanConfirm(deps.Stdout, rt.format, deps.Locale(), rt.color, plan.Warnings, plan.ProposedRate, v)
	})
}
