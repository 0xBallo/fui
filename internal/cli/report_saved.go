package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app/reporting"
	"github.com/0xballo/fui/internal/cli/render"
	"github.com/0xballo/fui/internal/domain"
)

func newReportSaveCmd(deps *Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "save NAME KIND",
		Short: "Save an analytics report definition",
		Args:  exactArgs(2),
		Example: `  fui report save monthly-networth networth --period last-3-months
  fui report save food-by-payee spending --period last-month --category Food --group-by payee
  fui report save q4-vs-sep compare --a 2026-09 --b 2026-Q4 --expense
  fui report save outlook forecast --horizon 12m --granularity week`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runReportSave(cmd, deps, args[0], args[1])
		},
	}
	cmd.Flags().String("period", "", "period to save: relative (this-month, last-3-months), YYYY-MM, YYYY-Qn or YYYY")
	reportFilterFlags(cmd)
	reportGroupFlags(cmd)
	reportComparePeriodFlags(cmd)
	cmd.Flags().String("loan", "", "loan filter for remaining-debt/interest-paid reports")
	cmd.Flags().Int("limit", reporting.DefaultTopPayees, "row limit for top-payees")
	cmd.Flags().String("min-balance", "", "forecast minimum balance for forecast reports")
	cmd.Flags().String("horizon", "", "forecast horizon: Nd, Nw, Nm or a number of days")
	return cmd
}

// savedSpecFromFlags builds the definition to save from the flags of `report save`. A lone
// --account keeps the original (v1) field so such reports stay v1; any other filter goes into the
// v2 filter.
func savedSpecFromFlags(cmd *cobra.Command, kind string) (reporting.SavedSpec, error) {
	var spec reporting.SavedSpec
	spec.Period, _ = cmd.Flags().GetString("period")
	spec.Loan, _ = cmd.Flags().GetString("loan")
	spec.Limit, _ = cmd.Flags().GetInt("limit")
	spec.MinBalance, _ = cmd.Flags().GetString("min-balance")
	spec.Horizon, _ = cmd.Flags().GetString("horizon")
	spec.GroupBy, _ = cmd.Flags().GetString("group-by")
	spec.Granularity, _ = cmd.Flags().GetString("granularity")
	if reportFilterChanged(cmd) {
		filter, err := readReportFilter(cmd)
		if err != nil {
			return reporting.SavedSpec{}, err
		}
		if len(filter.Accounts) == 1 && onlyAccountFilterChanged(cmd) {
			spec.Account = filter.Accounts[0]
		} else {
			spec.Filter = &filter
		}
	}
	a, _ := cmd.Flags().GetString("a")
	b, _ := cmd.Flags().GetString("b")
	if a != "" || b != "" || kind == reporting.KindCompare {
		spec.Compare = &reporting.CompareSpec{A: a, B: b}
	}
	return spec, nil
}

// onlyAccountFilterChanged reports whether --account is the only report filter flag given.
func onlyAccountFilterChanged(cmd *cobra.Command) bool {
	for _, name := range reportFilterFlagNames {
		if f := cmd.Flags().Lookup(name); f != nil && f.Changed && name != "account" {
			return false
		}
	}
	return true
}

func runReportSave(cmd *cobra.Command, deps *Deps, name, kind string) error {
	rt, err := runtimeOf(cmd, deps)
	if err != nil {
		return err
	}
	analytics, err := requireAnalytics(deps)
	if err != nil {
		return err
	}
	kind = normalizeSavedKind(kind)
	spec, err := savedSpecFromFlags(cmd, kind)
	if err != nil {
		return err
	}
	plan, err := analytics.PlanSave(cmd.Context(), name, kind, spec)
	if err != nil {
		return err
	}
	return writeChange(cmd, deps, rt, plan.Plan, func() error {
		saved, err := analytics.ApplySave(cmd.Context(), plan)
		if err != nil {
			return err
		}
		return render.WriteSavedReport(deps.Stdout, rt.format, deps.Locale(), rt.color, saved)
	})
}

func newReportListCmd(deps *Deps) *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Short:   "List saved reports",
		Args:    noArgs(),
		Example: "  fui report list --json",
		RunE: func(cmd *cobra.Command, _ []string) error {
			rt, err := runtimeOf(cmd, deps)
			if err != nil {
				return err
			}
			analytics, err := requireAnalytics(deps)
			if err != nil {
				return err
			}
			rows, err := analytics.List(cmd.Context())
			if err != nil {
				return err
			}
			return render.WriteSavedReportList(deps.Stdout, rt.format, deps.Locale(), rt.color, rows)
		},
	}
}

func newReportRunSavedCmd(deps *Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run NAME",
		Short: "Run a saved report (or the categories/cashflow/compare built-ins)",
		Args:  exactArgs(1),
		Example: `  fui report run monthly-networth
  fui report run food-by-payee --json
  fui report run compare --a 2026-09 --b 2026-Q4 --category Food`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if args[0] == "categories" || args[0] == "cashflow" {
				return runLegacyReport(cmd, deps, args[0])
			}
			if args[0] == "compare" {
				return RunReportCompare(cmd, deps)
			}
			if reportFilterChanged(cmd) {
				return newUsageError(fmt.Errorf("filter flags do not apply to a saved report: save it with them instead"))
			}
			rt, err := runtimeOf(cmd, deps)
			if err != nil {
				return err
			}
			analytics, err := requireAnalytics(deps)
			if err != nil {
				return err
			}
			out, err := analytics.Run(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			chart, _ := cmd.Flags().GetBool("chart")
			return render.WriteSavedReportResult(deps.Stdout, rt.format, deps.Locale(), rt.color, out, chart)
		},
	}
	reportChartFlag(cmd)
	reportPeriodFlags(cmd)
	ReportCompareFlags(cmd)
	return cmd
}

func runLegacyReport(cmd *cobra.Command, deps *Deps, name string) error {
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
		result, err := deps.Reporting.Spending.Run(ctx, reporting.SpendingParams{
			Period: period, MonthStartDay: deps.Config.MonthStartDay, AccountIDs: accountIDs, BaseCurrency: base,
		})
		if err != nil {
			return err
		}
		return render.WriteSpending(deps.Stdout, rt.format, deps.Locale(), rt.color, result)
	case "cashflow":
		result, err := deps.Reporting.CashFlow.Run(ctx, reporting.CashFlowParams{
			Period: period, MonthStartDay: deps.Config.MonthStartDay, AccountIDs: accountIDs, BaseCurrency: base,
		})
		if err != nil {
			return err
		}
		return render.WriteCashFlow(deps.Stdout, rt.format, deps.Locale(), rt.color, result)
	default:
		return &domain.Error{
			Code:    domain.CodeInvalidValue,
			Message: fmt.Sprintf("unknown report %q", name),
			Details: map[string]any{"field": "report", "value": name},
		}
	}
}

func newReportRmCmd(deps *Deps) *cobra.Command {
	return &cobra.Command{
		Use:     "rm NAME",
		Short:   "Remove a saved report",
		Args:    exactArgs(1),
		Example: "  fui report rm monthly-networth --yes",
		RunE: func(cmd *cobra.Command, args []string) error {
			rt, err := runtimeOf(cmd, deps)
			if err != nil {
				return err
			}
			analytics, err := requireAnalytics(deps)
			if err != nil {
				return err
			}
			plan, err := analytics.PlanRemove(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return writeChange(cmd, deps, rt, plan.Plan, func() error {
				if err := analytics.ApplyRemove(cmd.Context(), plan); err != nil {
					return err
				}
				if rt.format == render.JSON {
					return render.WriteJSON(deps.Stdout, map[string]any{"removed": args[0]})
				}
				if _, err := fmt.Fprintf(deps.Stdout, "Removed %q.\n", args[0]); err != nil {
					return fmt.Errorf("write remove confirmation: %w", err)
				}
				return nil
			})
		},
	}
}

func normalizeSavedKind(kind string) string {
	switch kind {
	case "balance":
		return reporting.KindBalance
	case "budget-vs-actual":
		return reporting.KindBudget
	case "remaining-debt":
		return reporting.KindDebt
	case "interest-paid":
		return reporting.KindInterest
	case "networth":
		return reporting.KindNetWorth
	case "top-payees":
		return reporting.KindPayees
	case "forecast":
		return reporting.KindForecast
	case "spending":
		return reporting.KindSpending
	case "cash-flow":
		return reporting.KindCashFlow
	default:
		return kind
	}
}

func reportKindLabels() []string {
	return append(append([]string{}, analyticsReportKinds...), "forecast")
}
