package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/reporting"
	"github.com/0xballo/fui/internal/cli/render"
	"github.com/0xballo/fui/internal/config"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
)

// NewInsightsCmd builds `fui insights`; the composition of the root command registers it under the
// analytics module.
func NewInsightsCmd(deps *Deps) *cobra.Command { return newInsightsCmd(deps) }

// InsightsModule is the module that owns the insights command.
const InsightsModule = module.Analytics

// InsightConfigFrom maps the insights.* configuration keys to the domain thresholds. A zero key
// keeps the corresponding default, so a partially filled Config (tests, old files) stays valid.
func InsightConfigFrom(cfg *config.Config) domain.InsightConfig {
	out := domain.DefaultInsightConfig()
	if cfg == nil {
		return out
	}
	in := cfg.Insights
	if in.AnomalyFactorPermille > 0 {
		out.AnomalyFactorPermille = in.AnomalyFactorPermille
	}
	if in.SubscriptionTolPermil > 0 {
		out.SubscriptionTolPermil = in.SubscriptionTolPermil
	}
	if in.SubscriptionMinCount > 0 {
		out.SubscriptionMinCount = in.SubscriptionMinCount
	}
	if in.TrendPermille > 0 {
		out.TrendPermille = in.TrendPermille
	}
	if in.TrendMinMinor > 0 {
		out.TrendMinMinor = in.TrendMinMinor
	}
	return out
}

// NewInsightsService builds the insights use cases over the analytics and recurring services of
// deps, with the thresholds from the configuration.
func NewInsightsService(deps *Deps) (*reporting.Insights, error) {
	analytics, err := requireAnalytics(deps)
	if err != nil {
		return nil, err
	}
	return reporting.NewInsights(analytics, deps.Recurring, InsightConfigFrom(deps.Config)), nil
}

func newInsightsCmd(deps *Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "insights",
		Short: "Show automatic findings: anomalies, subscriptions and trends",
		Example: "  fui insights --json\n" +
			"  fui insights --dismiss 1a2b3c4d5e6f7a8b\n" +
			"  fui insights --accept 1a2b3c4d5e6f7a8b --yes",
		Args: noArgs(),
		RunE: func(cmd *cobra.Command, _ []string) error { return runInsights(cmd, deps) },
	}
	cmd.Flags().String("dismiss", "", "hide the insight with this key for good")
	cmd.Flags().String("accept", "", "turn the subscription insight with this key into a recurrence")
	return cmd
}

func runInsights(cmd *cobra.Command, deps *Deps) error {
	rt, err := runtimeOf(cmd, deps)
	if err != nil {
		return err
	}
	dismiss, _ := cmd.Flags().GetString("dismiss")
	accept, _ := cmd.Flags().GetString("accept")
	if dismiss != "" && accept != "" {
		return newUsageError(errors.New("--dismiss and --accept are mutually exclusive"))
	}
	svc, err := NewInsightsService(deps)
	if err != nil {
		return err
	}
	switch {
	case dismiss != "":
		return insightWrite(cmd, deps, rt, dismiss, "dismiss", func() error {
			if err := svc.Dismiss(cmd.Context(), dismiss); err != nil {
				return err
			}
			return writeInsightResult(deps, rt, map[string]any{"dismissed": dismiss}, "Dismissed insight %s.\n", dismiss)
		})
	case accept != "":
		if deps.Recurring == nil {
			return fmt.Errorf("fui: no recurring service configured")
		}
		return insightWrite(cmd, deps, rt, accept, "accept", func() error {
			res, err := svc.AcceptSubscription(cmd.Context(), accept)
			if err != nil {
				return err
			}
			return writeInsightResult(deps, rt, map[string]any{
				"accepted": accept, "created": res.Created, "recurrence_id": int64(res.Recurrence.ID),
			}, "Accepted insight %s as recurrence #%d.\n", accept, int64(res.Recurrence.ID))
		})
	}
	views, err := svc.Run(cmd.Context())
	if err != nil {
		return err
	}
	return writeInsights(deps, rt, views)
}

func insightWrite(cmd *cobra.Command, deps *Deps, rt runtimeFlags, key, verb string, apply func() error) error {
	plan := app.Plan{Changes: []app.Change{{
		Op: app.OpUpdate, Entity: "insight", Summary: verb + " insight " + key, Fields: map[string]any{"key": key, "action": verb},
	}}}
	return writeChange(cmd, deps, rt, plan, apply)
}

func writeInsightResult(deps *Deps, rt runtimeFlags, payload map[string]any, format string, args ...any) error {
	if rt.format == render.JSON {
		return render.WriteJSON(deps.Stdout, payload)
	}
	if _, err := fmt.Fprintf(deps.Stdout, format, args...); err != nil {
		return fmt.Errorf("write insight result: %w", err)
	}
	return nil
}

func writeInsights(deps *Deps, rt runtimeFlags, views []reporting.InsightView) error {
	if rt.format == render.JSON {
		return render.WriteJSON(deps.Stdout, views)
	}
	rows := render.Rows{Header: []string{"KIND", "KEY", "SUBJECT", "DETAIL", "AMOUNT"}, MoneyCols: []int{4}}
	for _, v := range views {
		amount, err := formatAmount(v.Amount, rt.format, deps.Locale())
		if err != nil {
			return err
		}
		rows.Body = append(rows.Body, []string{v.Kind, v.Key, v.Label, v.Detail, amount})
	}
	if rt.format == render.CSV {
		return render.WriteCSV(deps.Stdout, rows)
	}
	return render.WriteTable(deps.Stdout, rows, rt.color)
}

// formatAmount renders a reporting amount for a table (localized) or CSV (machine) cell.
func formatAmount(a reporting.Amount, f render.Format, loc domain.Locale) (string, error) {
	cur, err := domain.ParseCurrency(a.Currency)
	if err != nil {
		return "", fmt.Errorf("parse currency %q: %w", a.Currency, err)
	}
	m, err := domain.NewMoney(a.Minor, cur)
	if err != nil {
		return "", fmt.Errorf("amount %d %s: %w", a.Minor, a.Currency, err)
	}
	if f == render.CSV {
		return render.FormatMoneyMachine(m), nil
	}
	return render.FormatMoney(m, loc), nil
}
