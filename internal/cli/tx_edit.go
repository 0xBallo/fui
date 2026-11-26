package cli

import (
	"strconv"

	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

// newTxEditCmd changes the given fields of one transaction; unset flags leave their field
// unchanged. --category "" and --payee "" clear the field.
func newTxEditCmd(deps *Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "edit ID",
		Short: "Edit a transaction",
		Args:  exactArgs(1),
		Example: "  fui tx edit 42 --amount -15,00 --confirm-reconciled\n" +
			"  fui tx edit 42 --payee Trattoria --tag outing --untag work\n" +
			"  fui tx edit 42 --payee \"\"",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTxEdit(cmd, deps, args[0])
		},
	}
	cmd.Flags().String("amount", "", "new amount (not on a transfer leg)")
	cmd.Flags().String("date", "", "new accounting date")
	cmd.Flags().String("description", "", "new description")
	cmd.Flags().String("notes", "", "new notes")
	cmd.Flags().String("category", "", "new category (empty clears it)")
	cmd.Flags().String("status", "", "new status")
	cmd.Flags().String("kind", "", "new kind")
	cmd.Flags().String("payee", "", "new payee, created when it does not exist (empty clears it)")
	cmd.Flags().StringArray("tag", nil, "tag to add, repeatable (created when it does not exist)")
	cmd.Flags().StringArray("untag", nil, "tag to remove, repeatable")
	cmd.Flags().Bool("confirm-reconciled", false, "allow editing a reconciled transaction")
	return cmd
}

func runTxEdit(cmd *cobra.Command, deps *Deps, idArg string) error {
	rt, err := runtimeOf(cmd, deps)
	if err != nil {
		return err
	}
	id, err := strconv.ParseInt(idArg, 10, 64)
	if err != nil {
		return missingFieldError("id")
	}
	req := editTxRequest(cmd, domain.TransactionID(id))
	ctx := cmd.Context()
	plan, err := deps.Service.PlanEditTx(ctx, req)
	if err != nil {
		return err
	}
	return writeTxChange(cmd, deps, rt, plan.Plan, func() error {
		tx, err := deps.Service.ApplyEditTx(ctx, plan)
		if err != nil {
			return err
		}
		return renderTxWithNotices(ctx, deps, rt, tx.ID, plan.Notices)
	})
}

func editTxRequest(cmd *cobra.Command, id domain.TransactionID) app.EditTxRequest {
	req := app.EditTxRequest{ID: id}
	req.Amount = changedString(cmd, "amount")
	req.Date = changedString(cmd, "date")
	req.Description = changedString(cmd, "description")
	req.Notes = changedString(cmd, "notes")
	req.Category = changedString(cmd, "category")
	req.Status = changedString(cmd, "status")
	req.Kind = changedString(cmd, "kind")
	req.Payee = changedString(cmd, "payee")
	req.AddTags, _ = cmd.Flags().GetStringArray("tag")
	req.RemoveTags, _ = cmd.Flags().GetStringArray("untag")
	req.ConfirmReconciled, _ = cmd.Flags().GetBool("confirm-reconciled")
	return req
}

// changedString returns a pointer to the value of flag when the caller set it, else nil.
func changedString(cmd *cobra.Command, flag string) *string {
	if !cmd.Flags().Changed(flag) {
		return nil
	}
	v, _ := cmd.Flags().GetString(flag)
	return &v
}
