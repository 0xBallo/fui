package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/cli/render"
)

// newConfigGetCmd reads one dotted configuration key, e.g. "backup.keep".
func newConfigGetCmd(deps *Deps) *cobra.Command {
	return &cobra.Command{
		Use:     "get KEY",
		Short:   "Read a configuration value",
		Args:    cobra.ExactArgs(1),
		Example: "  fui config get currency",
		RunE: func(cmd *cobra.Command, args []string) error {
			rt, err := runtimeOf(cmd, deps)
			if err != nil {
				return err
			}
			value, err := deps.Config.Get(args[0])
			if err != nil {
				return err
			}
			if rt.format == render.JSON {
				return render.WriteJSON(deps.Stdout, struct {
					Key   string `json:"key"`
					Value string `json:"value"`
				}{args[0], value})
			}
			if _, err := fmt.Fprintln(deps.Stdout, value); err != nil {
				return fmt.Errorf("write config value: %w", err)
			}
			return nil
		},
	}
}
