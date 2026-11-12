package domain

import (
	"math/big"
	"slices"
)

// BudgetRow is one spending row as seen by budgets. The caller has already filtered by account
// and period scope and expands splits into one row per split, so each row counts on its own.
type BudgetRow struct {
	Date          Date
	Amount        Money
	Kind          TxKind
	Status        TxStatus
	CategoryID    CategoryID
	TagIDs        []TagID
	LoanPrincipal bool // loan principal repayments are never spending
}

// BudgetState is the threshold state of a cap.
type BudgetState string

// Cap states. The UI must pair them with a label, not only a colour.
const (
	BudgetOK      BudgetState = "ok"
	BudgetWarning BudgetState = "warning"
	BudgetOver    BudgetState = "over"
)

func (r BudgetRow) counts() bool {
	return r.Kind != KindTransfer && r.Kind != KindInvestment && r.Kind != KindExchange && !r.LoanPrincipal
}

func (b Budget) covers(r BudgetRow) bool {
	return slices.ContainsFunc(b.Targets, func(t BudgetTarget) bool {
		if t.CategoryID != 0 {
			return t.CategoryID == r.CategoryID
		}
		return slices.Contains(r.TagIDs, t.TagID)
	})
}

// sums returns spent (booked/reconciled, net of refunds) and planned (pending only) for b in p, as
// positive amounts for outflows. Pending rows never reduce what is available until confirmed.
func (b Budget) sums(p Period, rows []BudgetRow) (spent, planned Money, err error) {
	spent, planned = Money{cur: b.Amount.Currency()}, Money{cur: b.Amount.Currency()}
	for _, r := range rows {
		if !r.counts() || !p.Contains(r.Date) || !b.covers(r) {
			continue
		}
		neg, err := r.Amount.Neg()
		if err != nil {
			return Money{}, Money{}, err
		}
		dst := &spent
		if r.Status == StatusPending {
			dst = &planned
		}
		if *dst, err = dst.Add(neg); err != nil {
			return Money{}, Money{}, err
		}
	}
	return spent, planned, nil
}

// CapStatus is a cap evaluated over one period.
type CapStatus struct {
	Period    Period
	Amount    Money
	CarryIn   Money // surplus (+) or overspend (-) from the previous period
	Available Money // Amount + CarryIn
	Spent     Money
	Planned   Money
	Remaining Money // Available - Spent; planned is not deducted until confirmed
	Percent   int   // spent / available, floored, 0 when nothing was spent
	State     BudgetState
}

// CapStatuses evaluates a cap over consecutive periods. With b.Carry the remaining of each period
// (surplus or overspend) becomes the carry-in of the next, starting from opening (zero value = none).
func CapStatuses(b Budget, periods []Period, rows []BudgetRow, opening Money) ([]CapStatus, error) {
	cur := b.Amount.Currency()
	carry := Money{cur: cur}
	if !opening.Currency().IsZero() {
		carry = opening
	}
	out := make([]CapStatus, 0, len(periods))
	for _, p := range periods {
		s := CapStatus{Period: p, Amount: b.AmountFor(MonthKeyOf(p)), CarryIn: carry}
		var err error
		if s.Available, err = s.Amount.Add(carry); err != nil {
			return nil, err
		}
		if s.Spent, s.Planned, err = b.sums(p, rows); err != nil {
			return nil, err
		}
		if s.Remaining, err = s.Available.Sub(s.Spent); err != nil {
			return nil, err
		}
		s.Percent = percent(s)
		s.State = capState(s, b.Thresholds)
		out = append(out, s)
		carry = Money{cur: cur}
		if b.Carry {
			carry = s.Remaining
		}
	}
	return out, nil
}

func percent(s CapStatus) int {
	if s.Spent.Sign() <= 0 && s.Available.Sign() >= 0 {
		return 0
	}
	if s.Available.Sign() <= 0 {
		return 100
	}
	n := new(big.Int).Mul(big.NewInt(s.Spent.Minor()), big.NewInt(100))
	n.Quo(n, big.NewInt(s.Available.Minor()))
	if !n.IsInt64() || n.Int64() > 1<<31 {
		return 1 << 31
	}
	return int(n.Int64())
}

func capState(s CapStatus, t BudgetThresholds) BudgetState {
	if t == (BudgetThresholds{}) {
		t = DefaultBudgetThresholds
	}
	switch {
	case s.Remaining.Sign() < 0 || s.Percent >= t.Over:
		return BudgetOver
	case s.Percent >= t.Warn:
		return BudgetWarning
	}
	return BudgetOK
}

// EnvelopeMonth is one envelope's ledger for a period.
type EnvelopeMonth struct {
	Period    Period
	CarryIn   Money
	Assigned  Money // net of moves with the unassigned pool
	MovesIn   Money // from other envelopes
	MovesOut  Money // to other envelopes
	Spent     Money
	Planned   Money
	Balance   Money // CarryIn + Assigned + MovesIn - MovesOut - Spent
	Overspend Money // -Balance when negative, else zero; carried as a negative balance
}

// EnvelopeLedger computes an envelope over consecutive periods. Balances always carry forward
// (an overspend lowers the next month); opening is the balance before the first period.
func EnvelopeLedger(b Budget, periods []Period, rows []BudgetRow, moves []BudgetMove, opening Money) ([]EnvelopeMonth, error) {
	cur := b.Amount.Currency()
	zero := Money{cur: cur}
	carry := zero
	if !opening.Currency().IsZero() {
		carry = opening
	}
	out := make([]EnvelopeMonth, 0, len(periods))
	for _, p := range periods {
		m := EnvelopeMonth{Period: p, CarryIn: carry, Assigned: zero, MovesIn: zero, MovesOut: zero}
		var err error
		if m.Assigned, m.MovesIn, m.MovesOut, err = applyMoves(b.ID, p, moves, zero); err != nil {
			return nil, err
		}
		if m.Spent, m.Planned, err = b.sums(p, rows); err != nil {
			return nil, err
		}
		if m.Balance, err = envelopeBalance(m); err != nil {
			return nil, err
		}
		m.Overspend = zero
		if m.Balance.Sign() < 0 {
			if m.Overspend, err = m.Balance.Neg(); err != nil {
				return nil, err
			}
		}
		out = append(out, m)
		carry = m.Balance
	}
	return out, nil
}

func envelopeBalance(m EnvelopeMonth) (Money, error) {
	bal, err := m.CarryIn.Add(m.Assigned)
	for _, step := range []struct {
		v   Money
		neg bool
	}{{m.MovesIn, false}, {m.MovesOut, true}, {m.Spent, true}} {
		if err != nil {
			return Money{}, err
		}
		if step.neg {
			bal, err = bal.Sub(step.v)
		} else {
			bal, err = bal.Add(step.v)
		}
	}
	return bal, err
}

// AccountBalance is an account's balance in the base currency, converted by the caller.
type AccountBalance struct {
	Kind    AccountKind
	Balance Money
}

// IsLiquidity reports whether accounts of this kind are included in envelope budgeting:
// checking, savings and cash. Cards, loans and investments are excluded.
func (k AccountKind) IsLiquidity() bool {
	return k == AccountCash || k == AccountChecking || k == AccountSavings
}

// EnvelopeBalance is an envelope's closing balance.
type EnvelopeBalance struct {
	TagOnly bool // tag-only envelopes are reported separately, not deducted from "Da assegnare"
	Balance Money
}

// Unassigned is the envelope-mode summary: Envelopes + ToAssign == Liquidity.
type Unassigned struct {
	Liquidity Money
	Envelopes Money
	ToAssign  Money // "Da assegnare"; negative means more was assigned than available
}

// ToAssign computes "Da assegnare" as the liquidity balances minus the category envelope balances.
func ToAssign(base Currency, accounts []AccountBalance, envelopes []EnvelopeBalance) (Unassigned, error) {
	u := Unassigned{Liquidity: Money{cur: base}, Envelopes: Money{cur: base}}
	var err error
	for _, a := range accounts {
		if !a.Kind.IsLiquidity() {
			continue
		}
		if u.Liquidity, err = u.Liquidity.Add(a.Balance); err != nil {
			return Unassigned{}, err
		}
	}
	for _, e := range envelopes {
		if e.TagOnly {
			continue
		}
		if u.Envelopes, err = u.Envelopes.Add(e.Balance); err != nil {
			return Unassigned{}, err
		}
	}
	u.ToAssign, err = u.Liquidity.Sub(u.Envelopes)
	return u, err
}

// UncoveredSpending sums the spending in p not covered by any of the budgets ("non coperte").
func UncoveredSpending(budgets []Budget, p Period, rows []BudgetRow, base Currency) (Money, error) {
	total := Money{cur: base}
	for _, r := range rows {
		if !r.counts() || r.Status == StatusPending || !p.Contains(r.Date) || r.Amount.Sign() >= 0 ||
			slices.ContainsFunc(budgets, func(b Budget) bool { return b.covers(r) }) {
			continue
		}
		neg, err := r.Amount.Neg()
		if err != nil {
			return Money{}, err
		}
		if total, err = total.Add(neg); err != nil {
			return Money{}, err
		}
	}
	return total, nil
}

// applyMoves totals the moves touching envelope id in p: net assignment from the pool, moves in and moves out.
func applyMoves(id BudgetID, p Period, moves []BudgetMove, zero Money) (assigned, in, out Money, err error) {
	assigned, in, out = zero, zero, zero
	for _, mv := range moves {
		if !p.Contains(mv.Date) {
			continue
		}
		if err = mv.Validate(); err != nil {
			return zero, zero, zero, err
		}
		switch {
		case mv.From == 0 && mv.To == id:
			assigned, err = assigned.Add(mv.Amount)
		case mv.To == 0 && mv.From == id:
			assigned, err = assigned.Sub(mv.Amount)
		case mv.To == id:
			in, err = in.Add(mv.Amount)
		case mv.From == id:
			out, err = out.Add(mv.Amount)
		}
		if err != nil {
			return zero, zero, zero, err
		}
	}
	return assigned, in, out, nil
}
