package cli

import (
	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/cli/render"
)

// newKPINetWorthCmd computes UC-04's net worth: every account balance converted to the base
// currency, plus the total, as of --at (default today).
func newKPINetWorthCmd(deps *Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "networth",
		Short:   "Compute the net worth KPI (every account balance converted to the base currency)",
		Example: "  fui kpi networth --json",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runKPINetWorth(cmd, deps)
		},
	}
	cmd.Flags().String("at", "", "evaluate as of this date (default: today)")
	cmd.Flags().Bool("include-archived", false, "include archived accounts")
	return cmd
}

func runKPINetWorth(cmd *cobra.Command, deps *Deps) error {
	rt, err := runtimeOf(cmd, deps)
	if err != nil {
		return err
	}
	at, _ := cmd.Flags().GetString("at")
	includeArchived, _ := cmd.Flags().GetBool("include-archived")
	nw, err := deps.Service.NetWorth(cmd.Context(), app.NetWorthRequest{At: at, IncludeArchived: includeArchived})
	if err != nil {
		return err
	}
	return render.WriteNetWorth(deps.Stdout, rt.format, deps.Locale(), rt.color, nw)
}
