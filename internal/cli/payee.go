package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/domain"
)

// newPayeeCmd groups the payee subcommands. Not yet registered in the command tree (see V32).
func newPayeeCmd(deps *Deps) *cobra.Command {
	cmd := &cobra.Command{Use: "payee", Short: "Add, list, alias, rename or merge payees"}
	cmd.AddCommand(
		newPayeeAddCmd(deps), newPayeeListCmd(deps), newPayeeAliasCmd(deps),
		newPayeeRenameCmd(deps), newPayeeMergeCmd(deps),
	)
	return cmd
}

// resolvePayee finds the payee named name among payees, by the same partial-match rule as other
// name references (accounts, categories, tags): ambiguous or missing fails with the matching
// domain error.
func resolvePayee(payees []domain.Payee, name string) (domain.Payee, error) {
	names := make([]string, len(payees))
	for i, p := range payees {
		names[i] = p.Name
	}
	i, err := domain.Resolve(name, names)
	if err != nil {
		return domain.Payee{}, fmt.Errorf("payee: %w", err)
	}
	return payees[i], nil
}
