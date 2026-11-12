package domain

import "slices"

// Reconcile computes the difference between the bank statement balance and the
// internal balance at asOf, plus the booked transaction IDs up to asOf that are
// not yet reconciled.
//
// difference is statementBalance - internalBalance.
func Reconcile(statementBalance, opening Money, txs []Transaction, asOf Date) (difference Money, unreconciled []TransactionID, err error) {
	if statementBalance.Currency().IsZero() {
		return Money{}, nil, missingField("statement_balance")
	}
	if asOf.IsZero() {
		return Money{}, nil, missingField("date")
	}
	internalBalance, err := reconciledBalanceAt(opening, txs, asOf)
	if err != nil {
		return Money{}, nil, err
	}
	difference, err = statementBalance.Sub(internalBalance)
	if err != nil {
		return Money{}, nil, err
	}
	unreconciled = collectUnreconciledBooked(txs, asOf)
	return difference, unreconciled, nil
}

func reconciledBalanceAt(opening Money, txs []Transaction, asOf Date) (Money, error) {
	if opening.Currency().IsZero() {
		return Money{}, missingField("currency")
	}
	sum := opening
	for _, tx := range txs {
		if tx.Date.IsZero() {
			return Money{}, missingField("date")
		}
		if tx.Date.After(asOf) || tx.Status == StatusPending {
			continue
		}
		next, err := sum.Add(tx.Amount)
		if err != nil {
			return Money{}, err
		}
		sum = next
	}
	return sum, nil
}

// CloseReconciliation returns the booked transaction IDs up to asOf that
// should transition to reconciled. It fails unless reconciliation difference is
// exactly zero.
func CloseReconciliation(statementBalance, opening Money, txs []Transaction, asOf Date) ([]TransactionID, error) {
	difference, unreconciled, err := Reconcile(statementBalance, opening, txs, asOf)
	if err != nil {
		return nil, err
	}
	if !difference.IsZero() {
		return nil, newError(ErrInvalidValue, map[string]any{
			"field":      "difference",
			"difference": difference.Minor(),
		}, "cannot close reconciliation when difference is %s", difference)
	}
	return unreconciled, nil
}

func collectUnreconciledBooked(txs []Transaction, asOf Date) []TransactionID {
	ids := make([]TransactionID, 0, len(txs))
	for _, tx := range txs {
		if tx.ID == 0 || tx.Date.IsZero() || tx.Date.After(asOf) {
			continue
		}
		if tx.Status == StatusBooked {
			ids = append(ids, tx.ID)
		}
	}
	slices.Sort(ids)
	return ids
}
