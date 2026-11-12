package render

import (
	"fmt"
	"io"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

// ReconcileDTO is the --json shape of `reconcile`.
type ReconcileDTO struct {
	Account          string           `json:"account"`
	Date             string           `json:"date"`
	StatementBalance int64            `json:"statement_balance"`
	Currency         string           `json:"currency"`
	Difference       int64            `json:"difference"`
	CanClose         bool             `json:"can_close"`
	Unreconciled     []TransactionDTO `json:"unreconciled"`
	Plan             PlanDTO          `json:"plan"`
}

func reconcileDTO(plan app.ReconcilePlan, rows []app.TxView) ReconcileDTO {
	out := ReconcileDTO{
		Account:          plan.Account.Name,
		Date:             plan.Date.String(),
		StatementBalance: plan.StatementBalance.Minor(),
		Currency:         plan.StatementBalance.Currency().Code(),
		Difference:       plan.Difference.Minor(),
		CanClose:         plan.Difference.IsZero(),
		Unreconciled:     make([]TransactionDTO, len(rows)),
		Plan:             planDTO(plan.Plan),
	}
	for i, r := range rows {
		out.Unreconciled[i] = transactionDTO(r)
	}
	return out
}

func reconcileSummaryRows(plan app.ReconcilePlan, loc domain.Locale) Rows {
	return Rows{
		Header:    []string{"ACCOUNT", "DATE", "STATEMENT BALANCE", "DIFFERENCE"},
		MoneyCols: []int{2, 3},
		Body: [][]string{{
			plan.Account.Name,
			FormatDate(plan.Date, loc),
			FormatMoney(plan.StatementBalance, loc),
			FormatMoney(plan.Difference, loc),
		}},
	}
}

// WriteReconcile renders `reconcile`.
func WriteReconcile(w io.Writer, f Format, loc domain.Locale, color bool, plan app.ReconcilePlan, rows []app.TxView) error {
	if f == JSON {
		return WriteJSON(w, reconcileDTO(plan, rows))
	}
	if f == CSV {
		if err := WriteCSV(w, transactionCSVRows(rows)); err != nil {
			return err
		}
		return nil
	}
	if err := WriteTable(w, reconcileSummaryRows(plan, loc), color); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Unreconciled transactions: %d\n", len(rows)); err != nil {
		return fmt.Errorf("write unreconciled count: %w", err)
	}
	if err := WriteTable(w, transactionTableRows(rows, loc), color); err != nil {
		return err
	}
	if plan.Difference.IsZero() {
		if _, err := fmt.Fprintln(w, "Difference is zero: pass --yes to close reconciliation."); err != nil {
			return fmt.Errorf("write close hint: %w", err)
		}
	}
	return nil
}
