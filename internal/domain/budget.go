package domain

import (
	"fmt"
	"slices"
)

// BudgetID identifies a budget. The zero ID denotes the unassigned pool ("Da assegnare") in a BudgetMove.
type BudgetID int64

// BudgetMode selects how a budget is evaluated.
type BudgetMode string

// Budget modes.
const (
	BudgetCap      BudgetMode = "cap"
	BudgetEnvelope BudgetMode = "envelope"
)

// Valid reports whether m is a known mode.
func (m BudgetMode) Valid() bool { return m == BudgetCap || m == BudgetEnvelope }

// ParseBudgetMode parses a budget mode name.
func ParseBudgetMode(s string) (BudgetMode, error) {
	return parseEnum("budget mode", s, BudgetMode.Valid)
}

// BudgetTarget is what a budget covers: exactly one of a category or a tag.
type BudgetTarget struct {
	CategoryID CategoryID
	TagID      TagID
}

// BudgetThresholds are the percentages of the available amount at which a cap warns and is over.
type BudgetThresholds struct {
	Warn int
	Over int
}

// DefaultBudgetThresholds are the spec defaults (80% and 100%).
var DefaultBudgetThresholds = BudgetThresholds{Warn: 80, Over: 100}

// MonthKey identifies a budget month by the calendar month in which its period starts,
// so it stays stable under month_start_day.
type MonthKey struct {
	Year  int
	Month int
}

// MonthKeyOf returns the key of a budget period.
func MonthKeyOf(p Period) MonthKey { return MonthKey{p.From().Year(), p.From().Month()} }

// Budget is a monthly cap or envelope over categories or tags.
type Budget struct {
	ID         BudgetID
	Name       string
	Mode       BudgetMode
	Targets    []BudgetTarget
	Amount     Money // default monthly amount, base currency
	Overrides  map[MonthKey]Money
	Carry      bool // caps only: envelopes always carry their balance
	Thresholds BudgetThresholds
}

// NewBudget validates b and fills the default thresholds.
func NewBudget(b Budget) (Budget, error) {
	name, err := requireName("name", b.Name)
	if err != nil {
		return Budget{}, err
	}
	b.Name = name
	if !b.Mode.Valid() {
		return Budget{}, enumError("mode", string(b.Mode))
	}
	if len(b.Targets) == 0 {
		return Budget{}, missingField("targets")
	}
	for _, t := range b.Targets {
		if (t.CategoryID == 0) == (t.TagID == 0) {
			return Budget{}, invalidValue("target", "exactly one of category or tag")
		}
	}
	if b.Amount.Currency().IsZero() {
		return Budget{}, missingField("amount")
	}
	if b.Amount.Sign() < 0 {
		return Budget{}, invalidValue("amount", b.Amount.String())
	}
	if err := b.checkOverrides(); err != nil {
		return Budget{}, err
	}
	if b.Thresholds == (BudgetThresholds{}) {
		b.Thresholds = DefaultBudgetThresholds
	}
	if t := b.Thresholds; t.Warn < 1 || t.Warn > t.Over {
		return Budget{}, invalidValue("thresholds", fmt.Sprintf("%d/%d", t.Warn, t.Over))
	}
	return b, nil
}

// AmountFor returns the amount for a month: the override if present, else the default.
func (b Budget) AmountFor(k MonthKey) Money {
	if m, ok := b.Overrides[k]; ok {
		return m
	}
	return b.Amount
}

// TagOnly reports whether the budget covers tags only; such budgets are reported separately
// and never deducted from "Da assegnare".
func (b Budget) TagOnly() bool {
	return !slices.ContainsFunc(b.Targets, func(t BudgetTarget) bool { return t.CategoryID != 0 })
}

// ValidateBudgets enforces one budget per category (periods are monthly, so per period too).
// Tags may overlap.
func ValidateBudgets(budgets []Budget) error {
	seen := map[CategoryID]string{}
	for _, b := range budgets {
		for _, t := range b.Targets {
			if t.CategoryID == 0 {
				continue
			}
			if other, dup := seen[t.CategoryID]; dup {
				return newError(ErrInvalidValue, map[string]any{"field": "targets", "category_id": int64(t.CategoryID)},
					"category %d is covered by both %q and %q", t.CategoryID, other, b.Name)
			}
			seen[t.CategoryID] = b.Name
		}
	}
	return nil
}

// BudgetPeriods returns n consecutive budget months, the first one containing from.
func BudgetPeriods(from Date, n, startDay int) ([]Period, error) {
	out := make([]Period, 0, n)
	d := from
	for range n {
		p, err := MonthPeriod(d, startDay)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
		if d, err = p.To().AddDays(1); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// BudgetMove moves money between envelopes; a zero From or To is the unassigned pool, so
// an assignment is a move out of the pool. Moves are budget movements, not transactions.
type BudgetMove struct {
	Date   Date
	From   BudgetID
	To     BudgetID
	Amount Money // positive
}

// Validate checks the move is well formed.
func (m BudgetMove) Validate() error {
	switch {
	case m.Date.IsZero():
		return missingField("date")
	case m.Amount.Currency().IsZero():
		return missingField("amount")
	case m.Amount.Sign() <= 0:
		return invalidValue("amount", m.Amount.String())
	case m.From == m.To:
		return invalidValue("move", "source and destination are the same")
	}
	return nil
}

func (b Budget) checkOverrides() error {
	for k, m := range b.Overrides {
		if k.Month < 1 || k.Month > 12 {
			return invalidValue("override month", fmt.Sprintf("%d-%d", k.Year, k.Month))
		}
		if m.Currency() != b.Amount.Currency() {
			return newError(ErrCurrencyMismatch, nil, "override is in %s, budget in %s", m.Currency(), b.Amount.Currency())
		}
		if m.Sign() < 0 {
			return invalidValue("override amount", m.String())
		}
	}
	return nil
}
