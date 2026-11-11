package cli

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/domain"
)

// reportPeriodFlags registers --month, --from and --to, shared by report run and kpi runway.
func reportPeriodFlags(cmd *cobra.Command) {
	cmd.Flags().String("month", "", "report this single accounting month, e.g. 2026-10")
	cmd.Flags().String("from", "", "earliest date, inclusive (with --to, overrides --month)")
	cmd.Flags().String("to", "", "latest date, inclusive (with --from, overrides --month)")
}

// reportPeriod resolves --month/--from/--to into a Period; the zero Period means "let the report
// use case pick its own default" (the current accounting month).
func reportPeriod(cmd *cobra.Command, monthStartDay int) (domain.Period, error) {
	month, _ := cmd.Flags().GetString("month")
	from, _ := cmd.Flags().GetString("from")
	to, _ := cmd.Flags().GetString("to")
	if from != "" || to != "" {
		fromDate, err := domain.ParseDate(from)
		if err != nil {
			return domain.Period{}, err
		}
		toDate, err := domain.ParseDate(to)
		if err != nil {
			return domain.Period{}, err
		}
		return domain.NewPeriod(fromDate, toDate)
	}
	if month == "" {
		return domain.Period{}, nil
	}
	day, err := domain.ParseDate(month + "-01")
	if err != nil {
		return domain.Period{}, err
	}
	return domain.MonthPeriod(day, monthStartDay)
}

// reportAccountFlag registers the repeatable --account filter, shared by report run and kpi
// runway.
func reportAccountFlag(cmd *cobra.Command) {
	cmd.Flags().StringArray("account", nil, "filter by account (repeatable)")
}

// resolveAccountIDs turns account names into IDs via the same resolution ShowAccount uses, so an
// ambiguous or unknown name fails exactly like every other command.
func resolveAccountIDs(ctx context.Context, deps *Deps, names []string) ([]domain.AccountID, error) {
	ids := make([]domain.AccountID, 0, len(names))
	for _, name := range names {
		ab, err := deps.Service.ShowAccount(ctx, name, "")
		if err != nil {
			return nil, err
		}
		ids = append(ids, ab.Account.ID)
	}
	return ids, nil
}
