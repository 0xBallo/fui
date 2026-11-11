package cli

import (
	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/cli/render"
)

// newAccountCmd groups the account subcommands.
func newAccountCmd(deps *Deps) *cobra.Command {
	cmd := &cobra.Command{Use: "account", Short: "Add, list, show, edit or archive accounts"}
	cmd.AddCommand(
		newAccountAddCmd(deps), newAccountListCmd(deps), newAccountShowCmd(deps),
		newAccountEditCmd(deps), newAccountArchiveCmd(deps),
	)
	return cmd
}

// renderAccountByName fetches and renders one account by name, re-read so the output carries
// its balance.
func renderAccountByName(cmd *cobra.Command, deps *Deps, rt runtimeFlags, name string) error {
	ab, err := deps.Service.ShowAccount(cmd.Context(), name, "")
	if err != nil {
		return err
	}
	return writeAccount(deps, rt, ab)
}

func writeAccount(deps *Deps, rt runtimeFlags, ab app.AccountBalance) error {
	return render.WriteAccount(deps.Stdout, rt.format, deps.Locale(), rt.color, ab)
}
