package cli

import (
	"context"
	"strings"

	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/cli/render"
	"github.com/0xballo/fui/internal/domain"
)

// renderTxByID fetches and renders transaction id, the common last step of every tx write
// (add, edit): it re-reads the stored row so the output always carries resolved account and
// category names, and transfer details when id is a transfer leg.
func renderTxByID(ctx context.Context, deps *Deps, rt runtimeFlags, id domain.TransactionID) error {
	view, err := deps.Service.ShowTx(ctx, id)
	if err != nil {
		return err
	}
	return render.WriteTx(deps.Stdout, rt.format, deps.Locale(), rt.color, view)
}

// registerAddFlags declares the flags of fui add.
func registerAddFlags(cmd *cobra.Command) {
	cmd.Flags().StringP("account", "a", "", "account the transaction is on (required)")
	cmd.Flags().StringP("category", "c", "", "category")
	cmd.Flags().String("date", "", "accounting date (default: today)")
	cmd.Flags().String("status", "", "pending, booked (default) or reconciled")
	cmd.Flags().String("kind", "", "payment (default), fee, refund, ...")
	cmd.Flags().String("notes", "", "free-form notes")
}

// addTxRequest builds the AddTxRequest for fui add from amount, the joined rest positionals as
// the description, and the flags registered by registerAddFlags, prompting for a missing account.
func addTxRequest(cmd *cobra.Command, deps *Deps, rt runtimeFlags, amount string, rest []string) (app.AddTxRequest, error) {
	account, _ := cmd.Flags().GetString("account")
	account, err := promptString(deps, rt, "account", "Account", account)
	if err != nil {
		return app.AddTxRequest{}, err
	}
	category, _ := cmd.Flags().GetString("category")
	date, _ := cmd.Flags().GetString("date")
	status, _ := cmd.Flags().GetString("status")
	kind, _ := cmd.Flags().GetString("kind")
	notes, _ := cmd.Flags().GetString("notes")
	return app.AddTxRequest{
		Account: account, Amount: amount, Description: strings.Join(rest, " "),
		Category: category, Date: date, Status: status, Kind: kind, Notes: notes,
	}, nil
}
