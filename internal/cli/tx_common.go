package cli

import (
	"context"
	"fmt"
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
	return renderTxWithNotices(ctx, deps, rt, id, nil)
}

// renderTxWithNotices is renderTxByID for a write that created things implicitly: the notices
// go to stderr in human mode and into the transaction object with --json.
func renderTxWithNotices(ctx context.Context, deps *Deps, rt runtimeFlags, id domain.TransactionID, notices []string) error {
	view, err := deps.Service.ShowTx(ctx, id)
	if err != nil {
		return err
	}
	if rt.format == render.JSON {
		view.Notices = notices
	} else if err := writeNotices(deps, notices); err != nil {
		return err
	}
	return render.WriteTx(deps.Stdout, rt.format, deps.Locale(), rt.color, view)
}

// writeNotices prints each notice on its own stderr line.
func writeNotices(deps *Deps, notices []string) error {
	for _, n := range notices {
		if _, err := fmt.Fprintln(deps.Stderr, n); err != nil {
			return fmt.Errorf("write notice: %w", err)
		}
	}
	return nil
}

// writeTxChange is writeChange for tx add and edit: on a dry run the notices are printed too
// (stderr in human mode; the plan carries them with --json).
func writeTxChange(cmd *cobra.Command, deps *Deps, rt runtimeFlags, plan app.Plan, apply func() error) error {
	if rt.dryRun && rt.format != render.JSON {
		if err := writeNotices(deps, plan.Notices); err != nil {
			return err
		}
	}
	return writeChange(cmd, deps, rt, plan, apply)
}

// registerAddFlags declares the flags of fui add.
func registerAddFlags(cmd *cobra.Command) {
	cmd.Flags().StringP("account", "a", "", "account the transaction is on (required)")
	cmd.Flags().StringP("category", "c", "", "category")
	cmd.Flags().String("date", "", "accounting date (default: today)")
	cmd.Flags().String("status", "", "pending, booked (default) or reconciled")
	cmd.Flags().String("kind", "", "payment (default), fee, refund, ...")
	cmd.Flags().String("notes", "", "free-form notes")
	cmd.Flags().String("payee", "", "payee (created when it does not exist)")
	cmd.Flags().StringArray("tag", nil, "tag, repeatable (created when it does not exist)")
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
	payee, _ := cmd.Flags().GetString("payee")
	tags, _ := cmd.Flags().GetStringArray("tag")
	return app.AddTxRequest{
		Account: account, Amount: amount, Description: strings.Join(rest, " "),
		Category: category, Date: date, Status: status, Kind: kind, Notes: notes,
		Payee: payee, Tags: tags,
	}, nil
}
