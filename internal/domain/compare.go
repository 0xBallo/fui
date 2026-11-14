package domain

import (
	"maps"
	"slices"
)

// CompareRow is one line of a period comparison. Delta is B minus A. DeltaPermille
// is the relative change on A, valid only when HasPct (A is not zero).
type CompareRow struct {
	CategoryID    CategoryID
	Uncategorized bool
	A, B, Delta   Money
	DeltaPermille int64
	HasPct        bool
}

// Comparison is the result of ComparePeriods.
type Comparison struct {
	Rows  []CompareRow // empty unless compared by category
	Total CompareRow
}

// ComparePeriods compares the spending (outflows, as positive amounts) of two
// periods. Transfers and investments are excluded; split transactions are
// passed as one row per split. All transactions must share one currency.
func ComparePeriods(a, b Period, txs []Transaction, byCategory bool) (Comparison, error) {
	type pair struct{ a, b int64 }
	byCat := map[CategoryID]*pair{}
	var tot pair
	var c Currency
	for _, tx := range txs {
		if !isExpense(tx) {
			continue
		}
		inA, inB := a.Contains(tx.Date), b.Contains(tx.Date)
		if !inA && !inB {
			continue
		}
		if c.IsZero() {
			c = tx.Amount.Currency()
		} else if c != tx.Amount.Currency() {
			return Comparison{}, newError(ErrCurrencyMismatch,
				map[string]any{"left": c.Code(), "right": tx.Amount.Currency().Code()},
				"cannot compare %s with %s", c, tx.Amount.Currency())
		}
		p := byCat[tx.CategoryID]
		if p == nil {
			p = &pair{}
			byCat[tx.CategoryID] = p
		}
		v := absMinor(tx.Amount)
		if inA {
			p.a += v
			tot.a += v
		}
		if inB {
			p.b += v
			tot.b += v
		}
	}
	var out Comparison
	if c.IsZero() {
		return out, nil
	}
	mk := func(id CategoryID, p pair) (CompareRow, error) { return compareRow(c, id, p.a, p.b) }
	var err error
	if out.Total, err = mk(0, tot); err != nil {
		return Comparison{}, err
	}
	out.Total.Uncategorized = false
	if byCategory {
		ids := slices.Sorted(maps.Keys(byCat))
		for _, id := range ids {
			r, err := mk(id, *byCat[id])
			if err != nil {
				return Comparison{}, err
			}
			out.Rows = append(out.Rows, r)
		}
	}
	return out, nil
}

func compareRow(c Currency, id CategoryID, a, b int64) (CompareRow, error) {
	ma, err := NewMoney(a, c)
	if err != nil {
		return CompareRow{}, err
	}
	mb, err := NewMoney(b, c)
	if err != nil {
		return CompareRow{}, err
	}
	d, err := NewMoney(b-a, c)
	if err != nil {
		return CompareRow{}, err
	}
	r := CompareRow{CategoryID: id, Uncategorized: id == 0, A: ma, B: mb, Delta: d}
	if a != 0 {
		r.HasPct = true
		r.DeltaPermille = (b - a) * 1000 / a
	}
	return r, nil
}
