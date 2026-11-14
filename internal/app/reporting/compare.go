package reporting

import (
	"context"
	"fmt"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

// CompareRequest names two relative periods (see ValidRelativePeriod); A defaults to last-month and
// B to this-month. ByCategory adds one row per category.
type CompareRequest struct {
	A, B       string
	ByCategory bool
}

// CompareRowView is one comparison line; DeltaPermille is null when A is zero.
type CompareRowView struct {
	Category      string `json:"category"`
	A             Amount `json:"a"`
	B             Amount `json:"b"`
	Delta         Amount `json:"delta"`
	DeltaPermille *int64 `json:"delta_permille"`
}

// CompareReport is the period comparison report.
type CompareReport struct {
	A     string           `json:"a"`
	B     string           `json:"b"`
	Rows  []CompareRowView `json:"rows"`
	Total CompareRowView   `json:"total"`
}

// ComparePeriods compares the spending of two periods in the base currency.
func (a *Analytics) ComparePeriods(ctx context.Context, req CompareRequest) (CompareReport, error) {
	if err := a.gate(); err != nil {
		return CompareReport{}, err
	}
	if req.A == "" {
		req.A = "last-month"
	}
	if req.B == "" {
		req.B = "this-month"
	}
	today, err := a.today()
	if err != nil {
		return CompareReport{}, err
	}
	pa, err := ResolveRelative(req.A, today, a.cfg.MonthStartDay)
	if err != nil {
		return CompareReport{}, err
	}
	pb, err := ResolveRelative(req.B, today, a.cfg.MonthStartDay)
	if err != nil {
		return CompareReport{}, err
	}
	out := CompareReport{A: periodText(pa), B: periodText(pb), Rows: []CompareRowView{}}
	from, to := span(pa, pb)
	err = a.store.Read(ctx, func(r app.Repos) error {
		txs, err := bookedSplit(ctx, r, domain.TxFilterSpec{From: from, To: to})
		if err != nil {
			return err
		}
		for j, t := range txs {
			if txs[j].Amount, err = a.toBase(ctx, r, t.Amount, t.Date); err != nil {
				return err
			}
		}
		cmp, err := domain.ComparePeriods(pa, pb, txs, req.ByCategory)
		if err != nil {
			return err
		}
		names, err := categoryNames(ctx, r)
		if err != nil {
			return err
		}
		for _, row := range cmp.Rows {
			v := rowView(row)
			v.Category = names[row.CategoryID]
			if row.Uncategorized {
				v.Category = "(uncategorized)"
			}
			out.Rows = append(out.Rows, v)
		}
		out.Total = rowView(cmp.Total)
		if cmp.Total.A.Currency().IsZero() { // no spending in either period
			z := Amount{Currency: a.cfg.BaseCurrency.Code()}
			out.Total = CompareRowView{A: z, B: z, Delta: z}
		}
		return nil
	})
	return out, err
}

func rowView(r domain.CompareRow) CompareRowView {
	v := CompareRowView{A: amountOf(r.A), B: amountOf(r.B), Delta: amountOf(r.Delta)}
	if r.HasPct {
		v.DeltaPermille = &r.DeltaPermille
	}
	return v
}

func periodText(p domain.Period) string { return fmt.Sprintf("%s..%s", p.From(), p.To()) }

func categoryNames(ctx context.Context, r app.Repos) (map[domain.CategoryID]string, error) {
	cats, err := r.Categories().List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}
	names := map[domain.CategoryID]string{}
	for _, c := range cats {
		names[c.ID] = c.Name
	}
	return names, nil
}

// span is the smallest [from, to] covering both periods.
func span(a, b domain.Period) (from, to domain.Date) {
	from, to = a.From(), a.To()
	if b.From().Before(from) {
		from = b.From()
	}
	if b.To().After(to) {
		to = b.To()
	}
	return from, to
}
