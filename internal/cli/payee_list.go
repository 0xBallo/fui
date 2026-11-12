package cli

import (
	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/cli/render"
)

// newPayeeListCmd lists every payee.
func newPayeeListCmd(deps *Deps) *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Short:   "List payees",
		Args:    noArgs(),
		Example: "  fui payee list --json",
		RunE: func(cmd *cobra.Command, _ []string) error {
			rt, err := runtimeOf(cmd, deps)
			if err != nil {
				return err
			}
			payees, err := deps.Service.ListPayees(cmd.Context())
			if err != nil {
				return err
			}
			return render.WritePayeeList(deps.Stdout, rt.format, payees)
		},
	}
}
