package importing

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/lending"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
)

// hasLoanWrites reports whether any loan row is to be imported, so that a loan-only plan applies.
func (p ImportPlan) hasLoanWrites(enabled []module.ID) bool {
	return slices.ContainsFunc(p.Loans, func(r LoanPlanRow) bool {
		return r.Include && !r.Unchanged && (r.Module == "" || slices.Contains(enabled, r.Module))
	})
}

// loans writes the loan rows of the plan: it creates the loans the import introduces and
// stores their bank schedule versions. A group the lending service rejects becomes a warning and
// the batch goes on, because the service validates a version before it writes anything. The
// pending installments of a loan are not generated here.
func (w *writer) loans(ctx context.Context, res *ImportResult) error {
	created := map[string]domain.LoanID{} // loans created by this import, by liability ref
	failed := map[string]bool{}           // refs of loans that could not be created
	for _, row := range w.p.Loans {
		if !row.Include || row.Unchanged {
			continue
		}
		// The module-off rows are reported by loanWarnings.
		if row.Module != "" && !slices.Contains(w.enabled, row.Module) {
			continue
		}
		if failed[row.AccountRef] {
			w.warnLoan(res, row, errors.New("its loan was not created"))
			continue
		}
		v := row.Version
		isNew := false
		switch id, ok := created[row.AccountRef]; {
		case row.New:
			loan, err := w.insertLoan(ctx, row)
			if err != nil {
				return err
			}
			created[row.AccountRef], v.LoanID, isNew = loan.ID, loan.ID, true
		case ok:
			v.LoanID = id
		}
		outcome, err := lending.ImportVersion(ctx, w.r, v)
		if err != nil {
			if err := w.rejectVersion(ctx, res, row, v.LoanID, isNew, err); err != nil {
				return err
			}
			if isNew {
				delete(created, row.AccountRef)
				failed[row.AccountRef] = true
			}
			continue
		}
		if isNew {
			res.Loans++
		}
		if outcome == lending.ImportCreated {
			res.LoanVersions++
		}
	}
	return nil
}

// rejectVersion turns a rejection of the version into a warning, dropping the loan this import
// had just inserted (it has no version yet); any other error is returned.
func (w *writer) rejectVersion(ctx context.Context, res *ImportResult, row LoanPlanRow, id domain.LoanID, isNew bool, err error) error {
	if !isImportRejection(err) {
		return fmt.Errorf("import loan schedule of line %d: %w", row.Line, err)
	}
	if isNew {
		if derr := w.r.Loans().Delete(ctx, id); derr != nil {
			return fmt.Errorf("drop loan of line %d: %w", row.Line, derr)
		}
	}
	w.warnLoan(res, row, err)
	return nil
}

// insertLoan stores the new loan of row, replacing the placeholder account IDs of the accounts
// this import creates.
func (w *writer) insertLoan(ctx context.Context, row LoanPlanRow) (domain.Loan, error) {
	loan := row.Loan
	if loan.LiabilityAccountID < 0 {
		id, ok := w.ids[row.AccountRef]
		if !ok {
			return domain.Loan{}, fmt.Errorf("loan of line %d: liability account %q not created", row.Line, row.AccountRef)
		}
		loan.LiabilityAccountID = id
	}
	if loan.DebitAccount < 0 {
		id, ok := w.ids[row.DebitRef]
		if !ok {
			return domain.Loan{}, fmt.Errorf("loan of line %d: debit account %q not created", row.Line, row.DebitRef)
		}
		loan.DebitAccount = id
	}
	loan, err := w.r.Loans().Insert(ctx, loan)
	if err != nil {
		return domain.Loan{}, fmt.Errorf("create loan of line %d: %w", row.Line, err)
	}
	return loan, nil
}

func (w *writer) warnLoan(res *ImportResult, row LoanPlanRow, err error) {
	res.Warnings = append(res.Warnings, RowWarning{Line: row.Line, Message: fmt.Sprintf("loan schedule skipped: %v", err)})
}

// isImportRejection tells the errors that mean "this version cannot be stored" from failures
// of the store.
func isImportRejection(err error) bool {
	var de *domain.Error
	return errors.As(err, &de) && (de.Code == app.CodeAlreadyExists || de.Code == domain.CodeInvalidValue)
}
