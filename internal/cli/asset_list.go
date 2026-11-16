package cli

import (
	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app/assetting"
)

func newAssetListCmd(deps *Deps, svc *assetting.Service) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List assets and their latest valuation",
		Args:    noArgs(),
		Example: "  fui asset list --at 2026-12-31 --json",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runAssetList(cmd, deps, svc)
		},
	}
	cmd.Flags().String("at", "", "show values as of this date (default: today)")
	return cmd
}

func runAssetList(cmd *cobra.Command, deps *Deps, svc *assetting.Service) error {
	if err := requireAssetting(svc); err != nil {
		return err
	}
	rt, err := runtimeOf(cmd, deps)
	if err != nil {
		return err
	}
	enabled, err := enabledModulesFromDeps(deps)
	if err != nil {
		return err
	}
	at, _ := cmd.Flags().GetString("at")
	items, err := svc.List(cmd.Context(), enabled, at)
	if err != nil {
		return err
	}
	return writeAssetList(deps.Stdout, rt.format, deps.Locale(), rt.color, items)
}
