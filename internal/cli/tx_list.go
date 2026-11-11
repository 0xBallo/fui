package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/cli/render"
	"github.com/0xballo/fui/internal/domain"
)

// newTxListCmd lists transactions, most recent first, with every ListTxRequest filter as a flag.
func newTxListCmd(deps *Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List transactions",
		Example: "  fui tx list --account Checking --from 2026-01-01 --json",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runTxList(cmd, deps)
		},
	}
	registerTxListFlags(cmd)
	return cmd
}

func registerTxListFlags(cmd *cobra.Command) {
	cmd.Flags().String("from", "", "earliest date, inclusive")
	cmd.Flags().String("to", "", "latest date, inclusive")
	cmd.Flags().StringArray("account", nil, "filter by account (repeatable)")
	cmd.Flags().StringArray("category", nil, "filter by category (repeatable)")
	cmd.Flags().StringArray("status", nil, "pending, booked or reconciled (repeatable)")
	cmd.Flags().String("text", "", "case-insensitive substring of description or notes")
	cmd.Flags().String("min", "", "minimum amount, inclusive")
	cmd.Flags().String("max", "", "maximum amount, inclusive")
	cmd.Flags().Int("limit", 0, "page size (default: 100)")
	cmd.Flags().String("after-date", "", "resume after this ISO date (from the previous page)")
	cmd.Flags().Int64("after-id", 0, "resume after this transaction id (from the previous page)")
}

func runTxList(cmd *cobra.Command, deps *Deps) error {
	rt, err := runtimeOf(cmd, deps)
	if err != nil {
		return err
	}
	req, err := txListRequest(cmd)
	if err != nil {
		return err
	}
	page, err := deps.Service.ListTx(cmd.Context(), req)
	if err != nil {
		return err
	}
	if err := render.WriteTxList(deps.Stdout, rt.format, deps.Locale(), rt.color, page.Items); err != nil {
		return err
	}
	if page.Next != nil && rt.format == render.Table {
		if _, err := fmt.Fprintf(deps.Stdout, "More rows: --after-date %s --after-id %d\n", page.Next.Date, page.Next.ID); err != nil {
			return fmt.Errorf("write pagination hint: %w", err)
		}
	}
	return nil
}

func txListRequest(cmd *cobra.Command) (app.ListTxRequest, error) {
	req := app.ListTxRequest{}
	req.From, _ = cmd.Flags().GetString("from")
	req.To, _ = cmd.Flags().GetString("to")
	req.Accounts, _ = cmd.Flags().GetStringArray("account")
	req.Categories, _ = cmd.Flags().GetStringArray("category")
	req.Statuses, _ = cmd.Flags().GetStringArray("status")
	req.Text, _ = cmd.Flags().GetString("text")
	req.MinAmount, _ = cmd.Flags().GetString("min")
	req.MaxAmount, _ = cmd.Flags().GetString("max")
	req.Limit, _ = cmd.Flags().GetInt("limit")
	afterDate, _ := cmd.Flags().GetString("after-date")
	if afterDate != "" {
		d, err := domain.ParseDate(afterDate)
		if err != nil {
			return app.ListTxRequest{}, err
		}
		id, _ := cmd.Flags().GetInt64("after-id")
		req.After = &app.TxCursor{Date: d, ID: domain.TransactionID(id)}
	}
	return req, nil
}
