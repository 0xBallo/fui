package domain

import "fmt"

// Split is one row of a transaction split across categories: a transaction
// with splits keeps its own date/account/payee but its amount is divided into
// rows, each assigned its own category.
type Split struct {
	Category CategoryID
	Amount   Money
	Note     string
}

// ValidateSplits checks the invariants a split must satisfy: at least two
// rows, every row in the same currency as total and non-zero, and the rows
// summing exactly to total.
func ValidateSplits(total Money, rows []Split) error {
	if len(rows) < 2 {
		return newError(ErrInvalidValue, map[string]any{"field": "splits", "rows": len(rows)},
			"a split needs at least 2 rows, got %d", len(rows))
	}
	sum, err := NewMoney(0, total.Currency())
	if err != nil {
		return err
	}
	for i, r := range rows {
		if r.Category == 0 {
			return missingField(fmt.Sprintf("splits[%d].category", i))
		}
		if r.Amount.Currency() != total.Currency() {
			return newError(ErrCurrencyMismatch,
				map[string]any{"row": i, "left": total.Currency().Code(), "right": r.Amount.Currency().Code()},
				"split row %d currency %s does not match total currency %s", i, r.Amount.Currency(), total.Currency())
		}
		if r.Amount.IsZero() {
			return newError(ErrInvalidValue, map[string]any{"field": fmt.Sprintf("splits[%d].amount", i)},
				"split row %d must not be zero", i)
		}
		sum, err = sum.Add(r.Amount)
		if err != nil {
			return err
		}
	}
	if cmp, err := sum.Cmp(total); err != nil || cmp != 0 {
		if err != nil {
			return err
		}
		return newError(ErrInvalidValue, map[string]any{"field": "splits", "sum": sum.Minor(), "total": total.Minor()},
			"split rows sum to %s, expected %s", sum, total)
	}
	return nil
}

// RebalanceLastSplit returns a copy of rows with the last row's Amount
// replaced by total minus the sum of the other rows, so the result sums
// exactly to total. It is the standard way to fix rounding drift when
// splitting an amount into equal parts. It fails with fewer than 2 rows, on a
// currency mismatch with any row but the last, or if the recomputed last
// amount would be zero.
func RebalanceLastSplit(total Money, rows []Split) ([]Split, error) {
	if len(rows) < 2 {
		return nil, newError(ErrInvalidValue, map[string]any{"field": "splits", "rows": len(rows)},
			"a split needs at least 2 rows, got %d", len(rows))
	}
	sum, err := NewMoney(0, total.Currency())
	if err != nil {
		return nil, err
	}
	for i, r := range rows[:len(rows)-1] {
		if r.Amount.Currency() != total.Currency() {
			return nil, newError(ErrCurrencyMismatch,
				map[string]any{"row": i, "left": total.Currency().Code(), "right": r.Amount.Currency().Code()},
				"split row %d currency %s does not match total currency %s", i, r.Amount.Currency(), total.Currency())
		}
		sum, err = sum.Add(r.Amount)
		if err != nil {
			return nil, err
		}
	}
	last, err := total.Sub(sum)
	if err != nil {
		return nil, err
	}
	if last.IsZero() {
		return nil, newError(ErrInvalidValue, map[string]any{"field": "splits"},
			"rebalancing the last row would make it zero")
	}
	out := make([]Split, len(rows))
	copy(out, rows)
	out[len(out)-1].Amount = last
	return out, nil
}
