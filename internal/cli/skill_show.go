package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/skill"
)

// newSkillShowCmd prints the agent skill document, with the running version already substituted,
// without installing it anywhere. It needs neither the archive nor a terminal.
func newSkillShowCmd(deps *Deps) *cobra.Command {
	return &cobra.Command{
		Use:     "show",
		Short:   "Print the agent skill document",
		Args:    noArgs(),
		Example: "  fui skill show",
		RunE: func(_ *cobra.Command, _ []string) error {
			if _, err := fmt.Fprint(deps.Stdout, skill.Render(deps.Version)); err != nil {
				return fmt.Errorf("write skill document: %w", err)
			}
			return nil
		},
	}
}
