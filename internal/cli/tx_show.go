package cli

import (
	"strconv"

	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/domain"
)

// newTxShowCmd shows one transaction by id, with its transfer group when it is a transfer leg.
func newTxShowCmd(deps *Deps) *cobra.Command {
	return &cobra.Command{
		Use:     "show ID",
		Short:   "Show one transaction",
		Args:    exactArgs(1),
		Example: "  fui tx show 42",
		RunE: func(cmd *cobra.Command, args []string) error {
			rt, err := runtimeOf(cmd, deps)
			if err != nil {
				return err
			}
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return missingFieldError("id")
			}
			return renderTxByID(cmd.Context(), deps, rt, domain.TransactionID(id))
		},
	}
}
