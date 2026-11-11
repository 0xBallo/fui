package cli

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/cli/render"
)

// newTransferCmd moves money between two of the user's accounts, in one or two currencies.
func newTransferCmd(deps *Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "transfer",
		Short:   "Transfer money between two accounts",
		Example: "  fui transfer --from Checking --to Savings --amount 200",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runTransfer(cmd, deps)
		},
	}
	cmd.Flags().String("from", "", "source account (required)")
	cmd.Flags().String("to", "", "destination account (required)")
	cmd.Flags().String("amount", "", "amount leaving the source account (required)")
	cmd.Flags().String("dest-amount", "", "amount arriving, when the accounts differ in currency")
	cmd.Flags().String("fee", "", "fee charged on the source account")
	cmd.Flags().String("date", "", "accounting date (default: today)")
	cmd.Flags().String("description", "", "description (default: a generated one)")
	cmd.Flags().String("status", "", "pending, booked (default) or reconciled")
	return cmd
}

func runTransfer(cmd *cobra.Command, deps *Deps) error {
	rt, err := runtimeOf(cmd, deps)
	if err != nil {
		return err
	}
	req, err := transferRequest(cmd, deps, rt)
	if err != nil {
		return err
	}
	ctx := cmd.Context()
	plan, err := deps.Service.PlanTransfer(ctx, req)
	if err != nil {
		return err
	}
	return writeChange(cmd, deps, rt, plan.Plan, func() error {
		res, err := deps.Service.ApplyTransfer(ctx, plan)
		if err != nil {
			return err
		}
		return renderTransfer(ctx, deps, rt, res)
	})
}

func transferRequest(cmd *cobra.Command, deps *Deps, rt runtimeFlags) (app.TransferRequest, error) {
	from, _ := cmd.Flags().GetString("from")
	from, err := promptString(deps, rt, "from", "Source account", from)
	if err != nil {
		return app.TransferRequest{}, err
	}
	to, _ := cmd.Flags().GetString("to")
	if to, err = promptString(deps, rt, "to", "Destination account", to); err != nil {
		return app.TransferRequest{}, err
	}
	amount, _ := cmd.Flags().GetString("amount")
	if amount, err = promptString(deps, rt, "amount", "Amount", amount); err != nil {
		return app.TransferRequest{}, err
	}
	destAmount, _ := cmd.Flags().GetString("dest-amount")
	fee, _ := cmd.Flags().GetString("fee")
	date, _ := cmd.Flags().GetString("date")
	description, _ := cmd.Flags().GetString("description")
	status, _ := cmd.Flags().GetString("status")
	return app.TransferRequest{
		From: from, To: to, Amount: amount, DestAmount: destAmount, Fee: fee,
		Date: date, Description: description, Status: status,
	}, nil
}

// renderTransfer shows both legs, re-read so they carry resolved account names and the transfer
// group they now belong to.
func renderTransfer(ctx context.Context, deps *Deps, rt runtimeFlags, res app.TransferResult) error {
	out, err := deps.Service.ShowTx(ctx, res.Out.ID)
	if err != nil {
		return err
	}
	in, err := deps.Service.ShowTx(ctx, res.In.ID)
	if err != nil {
		return err
	}
	return render.WriteTxList(deps.Stdout, rt.format, deps.Locale(), rt.color, []app.TxView{out, in})
}
