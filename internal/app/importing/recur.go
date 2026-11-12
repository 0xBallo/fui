package importing

import (
	"context"
	"fmt"

	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
)

// matchRecurrences is the recurring-module import hook (step 4b, after the categorization
// rules): a booked row that matches a pending transaction a recurrence created confirms it
// instead of being imported as a separate, new transaction — see domain.MatchPending. Ignored
// when the recurring module is off.
func (b *builder) matchRecurrences(ctx context.Context) error {
	if !b.req.Modules.Has(module.Recurring) {
		return nil
	}
	pending := map[domain.AccountID][]domain.Transaction{}
	for i := range b.plan.Rows {
		row := &b.plan.Rows[i]
		if row.Tx.Status != domain.StatusBooked || row.Tx.AccountID < 0 {
			continue
		}
		cands, ok := pending[row.Tx.AccountID]
		if !ok {
			var err error
			if cands, err = b.pendingRecurrenceTx(ctx, row.Tx.AccountID); err != nil {
				return err
			}
		}
		match, ok := domain.MatchPending(cands, row.Tx, 0)
		if !ok {
			pending[row.Tx.AccountID] = cands
			continue
		}
		row.ConfirmsPending, row.Class, row.Include, row.Matched = match.ID, domain.DedupNew, true, nil
		pending[row.Tx.AccountID] = removeMatched(cands, match.ID)
	}
	return nil
}

// pendingRecurrenceTx returns the pending transactions of accountID that a recurrence created.
func (b *builder) pendingRecurrenceTx(ctx context.Context, accountID domain.AccountID) ([]domain.Transaction, error) {
	f, err := domain.NewTxFilter(domain.TxFilterSpec{AccountIDs: []domain.AccountID{accountID}, Statuses: []domain.TxStatus{domain.StatusPending}})
	if err != nil {
		return nil, fmt.Errorf("pending recurrence filter: %w", err)
	}
	all, err := b.r.Transactions().List(ctx, f, nil, 0)
	if err != nil {
		return nil, fmt.Errorf("list pending transactions: %w", err)
	}
	out := make([]domain.Transaction, 0, len(all))
	for _, t := range all {
		if t.RecurrenceID != 0 {
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
