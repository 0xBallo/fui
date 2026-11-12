package cli

import (
	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/cli/render"
)

// newImportListCmd lists every import batch, most recent first.
func newImportListCmd(deps *Deps) *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Short:   "List import batches",
		Example: "  fui import list --json",
		RunE: func(cmd *cobra.Command, _ []string) error {
			rt, err := runtimeOf(cmd, deps)
			if err != nil {
				return err
			}
			svc, err := requireImporting(deps)
			if err != nil {
				return err
			}
			batches, err := svc.ListBatches(cmd.Context())
			if err != nil {
				return err
			}
			return render.WriteImportBatchList(deps.Stdout, rt.format, batches)
		},
	}
}
