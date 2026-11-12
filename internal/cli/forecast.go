package cli

import (
	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app/reporting"
	"github.com/0xballo/fui/internal/cli/render"
)

func newForecastCmd(deps *Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "forecast",
		Short:   "Forecast cash balances over the configured horizon",
		Example: "  fui forecast --horizon 60 --min-balance 500",
		Args:    noArgs(),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runForecast(cmd, deps)
		},
	}
	cmd.Flags().Int("horizon", 0, "forecast horizon in days (default: forecast.horizon from config)")
	cmd.Flags().String("min-balance", "", "minimum balance threshold in account currency")
	reportChartFlag(cmd)
	return cmd
}

func runForecast(cmd *cobra.Command, deps *Deps) error {
	rt, err := runtimeOf(cmd, deps)
	if err != nil {
		return err
	}
	analytics, err := requireAnalytics(deps)
	if err != nil {
		return err
	}
	horizon, _ := cmd.Flags().GetInt("horizon")
	minBalance, _ := cmd.Flags().GetString("min-balance")
	chart, _ := cmd.Flags().GetBool("chart")
	out, err := analytics.Forecast(cmd.Context(), reporting.ForecastRequest{
		HorizonDays: horizon,
		MinBalance:  minBalance,
	})
	if err != nil {
		return err
	}
	return render.WriteForecast(deps.Stdout, rt.format, deps.Locale(), rt.color, out, chart)
}
