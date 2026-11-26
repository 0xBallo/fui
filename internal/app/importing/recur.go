package importing

import (
	"context"
	"errors"
	"fmt"

	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
)

// matchRecurrences is the import hook (step 4b, after the categorization rules): a booked row
// that matches a pending transaction a recurrence or a loan installment created confirms it
// instead of being imported as a separate, new transaction — see domain.MatchPending. Each source
// is gated on its own module: recurring for recurrences, loans for installments.
func (b *builder) matchRecurrences(ctx context.Context) error {
	recurring, loans := b.req.Modules.Has(module.Recurring), b.req.Modules.Has(module.Loans)
	if !recurring && !loans {
		return nil
	}
	pending := map[domain.AccountID][]domain.Transaction{}
	installments := map[domain.TransactionID]bool{}
	for i := range b.plan.Rows {
		row := &b.plan.Rows[i]
		if row.Tx.Status != domain.StatusBooked || row.Tx.AccountID < 0 {
			continue
		}
		cands, ok := pending[row.Tx.AccountID]
		if !ok {
			var err error
			if cands, err = b.pendingCandidates(ctx, row.Tx.AccountID, recurring, loans, installments); err != nil {
				return err
			}
		}
		match, ok := domain.MatchPending(cands, row.Tx, 0)
		if !ok {
			pending[row.Tx.AccountID] = cands
			continue
		}
		row.ConfirmsPending, row.Class, row.Include, row.Matched = match.ID, domain.DedupNew, true, nil
		row.ConfirmsInstallment = installments[match.ID]
		pending[row.Tx.AccountID] = removeMatched(cands, match.ID)
	}
	return nil
}

// pendingCandidates returns the pending transactions of accountID that a recurrence (when
// recurring is set) or a loan installment (when loans is set) created, and records the
// installment ones in installments.
func (b *builder) pendingCandidates(ctx context.Context, accountID domain.AccountID, recurring, loans bool, installments map[domain.TransactionID]bool) ([]domain.Transaction, error) {
	f, err := domain.NewTxFilter(domain.TxFilterSpec{AccountIDs: []domain.AccountID{accountID}, Statuses: []domain.TxStatus{domain.StatusPending}})
	if err != nil {
		return nil, fmt.Errorf("pending candidates filter: %w", err)
	}
	all, err := b.r.Transactions().List(ctx, f, nil, 0)
	if err != nil {
		return nil, fmt.Errorf("list pending transactions: %w", err)
	}
	out := make([]domain.Transaction, 0, len(all))
	for _, t := range all {
		if loans {
			_, err := b.r.Loans().InstallmentOfTransaction(ctx, t.ID)
			switch {
			case err == nil:
				installments[t.ID] = true
				out = append(out, t)
				continue
			case !errors.Is(err, domain.ErrNotFound):
				return nil, fmt.Errorf("installment of transaction %d: %w", t.ID, err)
			}
		}
		if recurring && t.RecurrenceID != 0 {
			out = append(out, t)
		}
	}
	return out, nil
}

// removeMatched drops id from cands so the same pending transaction cannot confirm two rows.
func removeMatched(cands []domain.Transaction, id domain.TransactionID) []domain.Transaction {
	out := make([]domain.Transaction, 0, len(cands))
	for _, c := range cands {
		if c.ID != id {
			out = append(out, c)
		}
	}
	return out
}
