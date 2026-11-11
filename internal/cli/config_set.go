package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newConfigSetCmd validates and persists one dotted configuration key, e.g. "backup.keep".
// --dry-run shows the change without writing; in agent mode it needs --yes like any other write.
func newConfigSetCmd(deps *Deps) *cobra.Command {
	return &cobra.Command{
		Use:     "set KEY VALUE",
		Short:   "Change a configuration value",
		Args:    exactArgs(2),
		Example: "  fui config set currency EUR",
		RunE: func(cmd *cobra.Command, args []string) error {
			rt, err := runtimeOf(cmd, deps)
			if err != nil {
				return err
			}
			key, value := args[0], args[1]
			if rt.dryRun {
				if _, err := fmt.Fprintf(deps.Stdout, "Would set %s to %q.\n", key, value); err != nil {
					return fmt.Errorf("write dry-run note: %w", err)
				}
				return nil
			}
			if rt.agent && !rt.yes {
				return confirmationRequired("FUI_AGENT=1 requires --yes to write")
			}
			if err := deps.Config.Set(key, value); err != nil {
				return err
			}
			if _, err := fmt.Fprintf(deps.Stdout, "Set %s to %q.\n", key, value); err != nil {
				return fmt.Errorf("write confirmation: %w", err)
			}
			return nil
		},
	}
}
