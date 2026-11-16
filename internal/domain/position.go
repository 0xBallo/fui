package domain

import (
	"math/big"
	"slices"
)

// Position is the state of a holding derived from its operations up to a date.
type Position struct {
	Security  SecurityID
	AsOf      Date
	Quantity  Quantity
	CostBasis Money // total cost of the held quantity, fees included
	AvgCost   Money // PMC: cost basis per unit; zero when nothing is held
	Realized  Money // P&L realized by sells, net of fees
	Income    Money // dividends and coupons, net of fees

	HasPrice    bool  // a price on or before AsOf is known
	LastPrice   Money // valid only if HasPrice
	PriceDate   Date  // valid only if HasPrice
	MarketValue Money // Quantity × LastPrice; valid only if HasPrice
	Unrealized  Money // MarketValue − CostBasis; valid only if HasPrice
}

// opRank orders operations within a day so that the result does not depend on
// input order: buys first, so a same-day sell can draw on them.
func opRank(k SecurityOpKind) int {
	switch k {
	case OpBuy:
		return 0
	case OpSell:
		return 2
	case OpDividend, OpCoupon:
		return 1
	}
	return 1
}

func compareOps(a, b SecurityOp) int {
	if c := a.Date.Cmp(b.Date); c != 0 {
		return c
	}
	if c := opRank(a.Kind) - opRank(b.Kind); c != 0 {
		return c
	}
	return int(a.ID - b.ID)
}

// ComputePosition derives the position of sec as of asOf from its operations
// and manual prices. Operations after asOf and prices of other securities are
// ignored. Cost is tracked with the average-cost method (PMC) in exact
// rational arithmetic; each reported amount is rounded once at the end.
// Selling more than held fails with ErrInsufficientQuantity.
func ComputePosition(sec Security, ops []SecurityOp, prices []SecurityPrice, asOf Date) (Position, error) {
	sorted := make([]SecurityOp, 0, len(ops))
	for _, op := range ops {
		if op.Security != sec.ID {
			return Position{}, newError(ErrInvalidValue, map[string]any{"field": "security", "op": int64(op.ID)},
				"operation %d belongs to another security", op.ID)
		}
		if !op.Date.After(asOf) {
			sorted = append(sorted, op)
		}
	}
	slices.SortStableFunc(sorted, compareOps)

	held, err := NewQuantity(0, sec.QuantityScale)
	if err != nil {
		return Position{}, err
	}
	l := &ledger{sec: sec, held: held, cost: new(big.Rat), realized: new(big.Rat), income: new(big.Rat)}
	for _, op := range sorted {
		if err := l.apply(op); err != nil {
			return Position{}, err
		}
	}
	p, err := l.position(asOf)
	if err != nil {
		return Position{}, err
	}

	latest, ok := LastPriceOnOrBefore(prices, sec.ID, asOf)
	if !ok {
		return p, nil
	}
	if latest.Price.Currency() != sec.Currency {
		return Position{}, newError(ErrCurrencyMismatch,
			map[string]any{"left": sec.Currency.Code(), "right": latest.Price.Currency().Code()},
			"price currency %s does not match security currency %s", latest.Price.Currency(), sec.Currency)
	}
	p.HasPrice, p.LastPrice, p.PriceDate = true, latest.Price, latest.Date
	if p.MarketValue, err = p.Quantity.Value(latest.Price); err != nil {
		return Position{}, err
	}
	if p.Unrealized, err = p.MarketValue.Sub(p.CostBasis); err != nil {
		return Position{}, err
	}
	return p, nil
}

// LastPriceOnOrBefore returns the most recent price of sec dated on or before
// d. If several share the latest date, the one appearing last in prices wins.
func LastPriceOnOrBefore(prices []SecurityPrice, sec SecurityID, d Date) (SecurityPrice, bool) {
	var best SecurityPrice
	found := false
	for _, p := range prices {
		if p.Security != sec || p.Date.After(d) {
			continue
		}
		if !found || p.Date.Cmp(best.Date) >= 0 {
			best, found = p, true
		}
	}
	return best, found
}

func ratMoney(r *big.Rat, c Currency) (Money, error) {
	v, err := roundRat(r)
	if err != nil {
		return Money{}, err
	}
	return NewMoney(v, c)
}

// ledger accumulates exact (unrounded) average-cost state, in minor units.
type ledger struct {
	sec                    Security
	held                   Quantity
	cost, realized, income *big.Rat
}

func (l *ledger) apply(op SecurityOp) error {
	if op.UnitPrice.Currency() != l.sec.Currency {
		return newError(ErrCurrencyMismatch,
			map[string]any{"left": l.sec.Currency.Code(), "right": op.UnitPrice.Currency().Code()},
			"operation currency %s does not match security currency %s", op.UnitPrice.Currency(), l.sec.Currency)
	}
	gross := new(big.Rat).Mul(op.Quantity.rat(), new(big.Rat).SetInt64(op.UnitPrice.minor))
	fees := new(big.Rat).SetInt64(op.Fees.minor)
	if op.Kind != OpBuy && op.Kind != OpSell {
		l.income.Add(l.income, gross.Sub(gross, fees))
		return nil
	}
	if op.Kind == OpBuy {
		held, err := l.held.Add(op.Quantity)
		if err != nil {
			return err
		}
		l.held = held
		l.cost.Add(l.cost, gross).Add(l.cost, fees)
		return nil
	}
	left, err := l.held.Sub(op.Quantity)
	if err != nil {
		return err
	}
	// A zero-quantity sell (possible from stored data) has nothing to draw on and no meaning.
	if left.Sign() < 0 || l.held.IsZero() {
		return newError(ErrInsufficientQuantity,
			map[string]any{"date": op.Date.String(), "held": l.held.String(), "sell": op.Quantity.String()},
			"cannot sell %s on %s: only %s held", op.Quantity, op.Date, l.held)
	}
	removed := new(big.Rat).Mul(l.cost, new(big.Rat).SetFrac(big.NewInt(op.Quantity.units), big.NewInt(l.held.units)))
	l.cost.Sub(l.cost, removed)
	l.realized.Add(l.realized, gross.Sub(gross, fees).Sub(gross, removed))
	l.held = left
	return nil
}

// position rounds the exact state into a Position, once per amount.
func (l *ledger) position(asOf Date) (Position, error) {
	c := l.sec.Currency
	p := Position{Security: l.sec.ID, AsOf: asOf, Quantity: l.held}
	var err error
	if p.CostBasis, err = ratMoney(l.cost, c); err != nil {
		return Position{}, err
	}
	if p.Realized, err = ratMoney(l.realized, c); err != nil {
		return Position{}, err
	}
	if p.Income, err = ratMoney(l.income, c); err != nil {
		return Position{}, err
	}
	avg := new(big.Rat)
	if l.held.Sign() > 0 {
		avg.Quo(l.cost, l.held.rat())
	}
	if p.AvgCost, err = ratMoney(avg, c); err != nil {
		return Position{}, err
	}
	return p, nil
}
