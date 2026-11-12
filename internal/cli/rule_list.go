package cli

import (
	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/cli/render"
)

func newRuleListCmd(deps *Deps) *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Short:   "List categorization rules",
		Args:    noArgs(),
		Example: "  fui rule list --json",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := requireRules(deps); err != nil {
				return err
			}
			rt, err := runtimeOf(cmd, deps)
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			items, err := deps.Rules.List(ctx)
			if err != nil {
				return err
			}
			lookup, err := loadRuleLookup(ctx, deps)
			if err != nil {
				return err
			}
			return render.WriteRuleList(deps.Stdout, rt.format, items, lookup)
		},
	}
}
