package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app/reporting"
	"github.com/0xballo/fui/internal/cli/render"
	"github.com/0xballo/fui/internal/domain"
)

// newReportRunCmd runs one of the predefined reports: categories (spending by category) or
// cashflow (monthly cash-flow). Both default to the current accounting month and exclude
// transfers.
func newReportRunCmd(deps *Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "run NAME",
		Short:   "Run a predefined report (categories or cashflow)",
		Args:    cobra.ExactArgs(1),
		Example: "  fui report run categories --month 2026-10",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runReport(cmd, deps, args[0])
		},
	}
	reportPeriodFlags(cmd)
	reportAccountFlag(cmd)
	return cmd
}

func runReport(cmd *cobra.Command, deps *Deps, name string) error {
	rt, err := runtimeOf(cmd, deps)
	if err != nil {
		return err
	}
	ctx := cmd.Context()
	period, err := reportPeriod(cmd, deps.Config.MonthStartDay)
	if err != nil {
		return err
	}
	names, _ := cmd.Flags().GetStringArray("account")
	accountIDs, err := resolveAccountIDs(ctx, deps, names)
	if err != nil {
		return err
	}
	base, err := deps.BaseCurrency()
	if err != nil {
		return err
	}
	switch name {
	case "categories":
		return runSpendingReport(ctx, deps, rt, period, accountIDs, base)
	case "cashflow":
		return runCashFlowReport(ctx, deps, rt, period, accountIDs, base)
	default:
		return &domain.Error{
			Code: domain.ErrInvalidValue.Code, Message: fmt.Sprintf("unknown report %q", name),
			Details: map[string]any{"field": "report", "value": name},
		}
	}
}

func runSpendingReport(
	ctx context.Context, deps *Deps, rt runtimeFlags, period domain.Period,
	accountIDs []domain.AccountID, base domain.Currency,
) error {
	result, err := deps.Reporting.Spending.Run(ctx, reporting.SpendingParams{
		Period: period, MonthStartDay: deps.Config.MonthStartDay, AccountIDs: accountIDs, BaseCurrency: base,
	})
	if err != nil {
		return err
	}
	return render.WriteSpending(deps.Stdout, rt.format, deps.Locale(), rt.color, result)
}

func runCashFlowReport(
	ctx context.Context, deps *Deps, rt runtimeFlags, period domain.Period,
	accountIDs []domain.AccountID, base domain.Currency,
) error {
	result, err := deps.Reporting.CashFlow.Run(ctx, reporting.CashFlowParams{
		Period: period, MonthStartDay: deps.Config.MonthStartDay, AccountIDs: accountIDs, BaseCurrency: base,
	})
	if err != nil {
		return err
	}
	return render.WriteCashFlow(deps.Stdout, rt.format, deps.Locale(), rt.color, result)
}
