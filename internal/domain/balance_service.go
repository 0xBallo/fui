package domain

// BalanceAt returns the balance at upTo (inclusive): opening + every movement
// whose accounting date is not after upTo.
func BalanceAt(opening Money, movements []Transaction, upTo Date) (Money, error) {
	if opening.Currency().IsZero() {
		return Money{}, missingField("currency")
	}
	if upTo.IsZero() {
		return Money{}, missingField("date")
	}
	sum := opening
	for _, movement := range movements {
		if movement.Date.IsZero() {
			return Money{}, missingField("date")
		}
		if movement.Date.After(upTo) {
			continue
		}
		next, err := sum.Add(movement.Amount)
		if err != nil {
			return Money{}, err
		}
		sum = next
	}
	return sum, nil
}
