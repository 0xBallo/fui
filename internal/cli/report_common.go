package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app/reporting"
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

func reportKindPeriodFlag(cmd *cobra.Command) {
	cmd.Flags().String("period", "", "period as YYYY-MM, YYYY-MM-DD..YYYY-MM-DD, or empty for current month")
}

func reportChartFlag(cmd *cobra.Command) {
	cmd.Flags().Bool("chart", false, "render a text bar chart in table output")
}

func reportPeriodValue(cmd *cobra.Command, monthStartDay int) (domain.Period, error) {
	in, _ := cmd.Flags().GetString("period")
	in = strings.TrimSpace(in)
	if in == "" {
		return domain.Period{}, nil
	}
	if reporting.ValidRelativePeriod(in) {
		return domain.Period{}, &domain.Error{
			Code:    domain.CodeInvalidValue,
			Message: fmt.Sprintf("relative period %q is supported only by saved reports", in),
			Details: map[string]any{"field": "period", "value": in},
		}
	}
	if strings.Contains(in, "..") {
		fromText, toText, ok := strings.Cut(in, "..")
		if !ok {
			return domain.Period{}, invalidPeriod(in)
		}
		fromDate, err := domain.ParseDate(strings.TrimSpace(fromText))
		if err != nil {
			return domain.Period{}, invalidPeriod(in)
		}
		toDate, err := domain.ParseDate(strings.TrimSpace(toText))
		if err != nil {
			return domain.Period{}, invalidPeriod(in)
		}
		return domain.NewPeriod(fromDate, toDate)
	}
	day, err := domain.ParseDate(in + "-01")
	if err != nil {
		return domain.Period{}, invalidPeriod(in)
	}
	return domain.MonthPeriod(day, monthStartDay)
}

func invalidPeriod(value string) error {
	return &domain.Error{
		Code:    domain.CodeInvalidValue,
		Message: fmt.Sprintf("invalid period %q", value),
		Details: map[string]any{"field": "period", "value": value},
	}
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
