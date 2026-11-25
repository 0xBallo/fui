package domain

import "slices"

// ReconcileTicked is the incremental variant of Reconcile: only the booked transactions in ticked
// count towards the internal balance, on top of the opening balance and the already reconciled
// ones. Ticks for unknown, pending or later transactions are ignored and duplicates count once.
//
// difference is statement - (opening + reconciled <= asOf + ticked booked <= asOf); unticked lists
// the booked, non-reconciled transactions up to asOf that are not ticked, in ID order.
func ReconcileTicked(
	statement, opening Money,
	txs []Transaction,
	ticked []TransactionID,
	asOf Date,
) (difference Money, unticked []TransactionID, err error) {
	if statement.Currency().IsZero() {
		return Money{}, nil, missingField("statement_balance")
	}
	if asOf.IsZero() {
		return Money{}, nil, missingField("date")
	}
	if opening.Currency().IsZero() {
		return Money{}, nil, missingField("currency")
	}
	tickSet := make(map[TransactionID]struct{}, len(ticked))
	for _, id := range ticked {
		tickSet[id] = struct{}{}
	}
	sum := opening
	unticked = make([]TransactionID, 0, len(txs))
	for _, tx := range txs {
		if tx.Date.IsZero() {
			return Money{}, nil, missingField("date")
		}
		if tx.Date.After(asOf) || tx.Status == StatusPending {
			continue
		}
		if tx.Status == StatusBooked {
			if _, ok := tickSet[tx.ID]; !ok {
				if tx.ID != 0 {
					unticked = append(unticked, tx.ID)
				}
				continue
			}
		}
		if sum, err = sum.Add(tx.Amount); err != nil {
			return Money{}, nil, err
		}
	}
	slices.Sort(unticked)
	if difference, err = statement.Sub(sum); err != nil {
		return Money{}, nil, err
	}
	return difference, unticked, nil
}
