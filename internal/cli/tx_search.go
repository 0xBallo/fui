package cli

import (
	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/cli/render"
)

// newTxSearchCmd finds transactions by a case-insensitive, accent-insensitive substring of their
// description, payee or notes, with the same filter flags as tx list (minus paging: a search is
// a single page).
func newTxSearchCmd(deps *Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "search TEXT",
		Short:   "Search transactions by text",
		Args:    exactArgs(1),
		Example: `  fui tx search "amazon prime" --expense`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTxSearch(cmd, deps, args[0])
		},
	}
	registerFilterFlags(cmd)
	return cmd
}

func runTxSearch(cmd *cobra.Command, deps *Deps, text string) error {
	rt, err := runtimeOf(cmd, deps)
	if err != nil {
		return err
	}
	req, err := readFilterFlags(cmd)
	if err != nil {
		return err
	}
	req.Text = text
	page, err := deps.Service.ListTx(cmd.Context(), req)
	if err != nil {
		return err
	}
	return render.WriteTxPage(deps.Stdout, rt.format, deps.Locale(), rt.color, page)
}
