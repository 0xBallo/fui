package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/cli/render"
)

// newBackupCmd writes a manual backup of the archive and prints its path. It needs no
// confirmation: unlike a bulk write, it never loses data.
func newBackupCmd(deps *Deps) *cobra.Command {
	return &cobra.Command{
		Use:     "backup",
		Short:   "Write a backup of the archive",
		Example: "  fui backup",
		RunE: func(cmd *cobra.Command, _ []string) error {
			rt, err := runtimeOf(cmd, deps)
			if err != nil {
				return err
			}
			path, err := deps.Service.Backup(cmd.Context())
			if err != nil {
				return err
			}
			if rt.format == render.JSON {
				return render.WriteJSON(deps.Stdout, struct {
					Path string `json:"path"`
				}{path})
			}
			if _, err := fmt.Fprintln(deps.Stdout, path); err != nil {
				return fmt.Errorf("write backup path: %w", err)
			}
			return nil
		},
	}
}
