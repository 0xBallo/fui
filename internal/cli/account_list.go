package cli

import (
	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/cli/render"
)

// newAccountListCmd lists accounts with their balance.
func newAccountListCmd(deps *Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List accounts",
		Example: "  fui account list --include-archived",
		RunE: func(cmd *cobra.Command, _ []string) error {
			rt, err := runtimeOf(cmd, deps)
			if err != nil {
				return err
			}
			at, _ := cmd.Flags().GetString("at")
			includeArchived, _ := cmd.Flags().GetBool("include-archived")
			accts, err := deps.Service.ListAccounts(cmd.Context(), app.ListAccountsRequest{
				At: at, IncludeArchived: includeArchived,
			})
			if err != nil {
				return err
			}
			return render.WriteAccountList(deps.Stdout, rt.format, deps.Locale(), rt.color, accts)
		},
	}
	cmd.Flags().String("at", "", "balance date (default: today)")
	cmd.Flags().Bool("include-archived", false, "include archived accounts")
	return cmd
}
