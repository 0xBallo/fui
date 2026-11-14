package cli

import (
	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app/investing"
	"github.com/0xballo/fui/internal/cli/render"
	"github.com/0xballo/fui/internal/module"
)

// NewPositionCmd builds the position command group.
func NewPositionCmd(deps *Deps, svc *investing.Service) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "position",
		Short: "List computed investment positions",
	}
	cmd.AddCommand(newPositionListCmd(deps, svc))
	annotateModule(cmd, module.Investments)
	return cmd
}

func newPositionListCmd(deps *Deps, svc *investing.Service) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List positions by account and security",
		Args:    noArgs(),
		Example: "  fui position list --account Broker --date 2026-03-31 --json",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := requireInvesting(svc); err != nil {
				return err
			}
			rt, err := runtimeOf(cmd, deps)
			if err != nil {
				return err
			}
			account, _ := cmd.Flags().GetString("account")
			date, _ := cmd.Flags().GetString("date")
			lines, err := svc.ListPositions(cmd.Context(), investing.PositionsRequest{Account: account, Date: date})
			if err != nil {
				return err
			}
			return render.WritePositionList(deps.Stdout, rt.format, deps.Locale(), rt.color, lines)
		},
	}
	cmd.Flags().String("account", "", "account name filter")
	cmd.Flags().String("date", "", "compute positions as of this date (default: today)")
	return cmd
}
