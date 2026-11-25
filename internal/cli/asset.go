package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app/assetting"
	"github.com/0xballo/fui/internal/module"
)

// NewAssetCmd builds the asset command group.
func NewAssetCmd(deps *Deps, svc *assetting.Service) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "asset",
		Short: "Add, list, show, edit, value, link and remove assets",
	}
	cmd.AddCommand(
		newAssetAddCmd(deps, svc),
		newAssetListCmd(deps, svc),
		newAssetValueCmd(deps, svc),
		newAssetShowCmd(deps, svc),
		newAssetEditCmd(deps, svc),
		newAssetRemoveCmd(deps, svc),
		newAssetLinkCmd(deps, svc),
		newAssetUnlinkCmd(deps, svc),
	)
	annotateModule(cmd, module.Assets)
	return cmd
}

func requireAssetting(svc *assetting.Service) error {
	if svc == nil {
		return fmt.Errorf("fui: asset service not configured")
	}
	return nil
}
