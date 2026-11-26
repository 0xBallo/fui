package reporting

import (
	"cmp"
	"context"
	"fmt"
	"slices"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

// Labels of the group rows that collect what has no category, payee or tag.
const (
	NoPayeeLabel  = "(no payee)"
	UntaggedLabel = "untagged"
)

// GroupedRow is one row of a grouped report. For period groupings Key is the first day of the
// bucket; ID is the database id of the category, payee, account or tag (0 when none or for
// periods). Spending rows carry only Total (what was spent, positive); cash-flow rows carry the
// net in Total plus Inflow and Outflow (positive).
type GroupedRow struct {
	Key     string  `json:"key"`
	ID      int64   `json:"id"`
	Total   Amount  `json:"total"`
	Inflow  *Amount `json:"inflow,omitempty"`
	Outflow *Amount `json:"outflow,omitempty"`
}

// GroupedResult is a spending or cash-flow report grouped by one dimension, in the base
// currency. Total is the report total over the filtered rows, counted once: it equals the sum
// of the rows for the category, payee, account and period groupings. Tag rows count a
// transaction fully in each of its tags (and in "untagged" when it has none), so they may sum
// above Total.
type GroupedResult struct {
	Period  string       `json:"period"`
	GroupBy string       `json:"group_by"`
	Rows    []GroupedRow `json:"rows"`
	Total   Amount       `json:"total"`
}

// groupAcc accumulates one group row in the base currency.
type groupAcc struct {
	key      string
	id       int64
	in, out  domain.Money
	sortDate domain.Date // period groups only
}

type bucket struct{ start, end domain.Date }

// grouper assigns report rows to the groups of one dimension.
type grouper struct {
	by      domain.ReportGroup
	base    domain.Currency
	cats    map[domain.CategoryID]string
	payees  map[domain.PayeeID]string
	accts   map[domain.AccountID]string
	buckets []bucket
	rows    map[string]*groupAcc
}

// newGrouper prepares the grouping by `by` of the rows of period p. Period groupings start with
// one empty row per bucket so a time series has no holes.
func (a *Analytics) newGrouper(ctx context.Context, r app.Repos, by domain.ReportGroup, p domain.Period) (*grouper, error) {
	g := &grouper{by: by, base: a.cfg.BaseCurrency, rows: map[string]*groupAcc{}}
	switch by {
	case domain.GroupCategory:
		cats, err := r.Categories().List(ctx)
		if err != nil {
			return nil, fmt.Errorf("list categories: %w", err)
		}
		g.cats = make(map[domain.CategoryID]string, len(cats))
		for _, c := range cats {
			g.cats[c.ID] = c.Name
		}
	case domain.GroupPayee:
		payees, err := r.Payees().List(ctx)
		if err != nil {
			return nil, fmt.Errorf("list payees: %w", err)
		}
		g.payees = make(map[domain.PayeeID]string, len(payees))
		for _, py := range payees {
			g.payees[py.ID] = py.Name
		}
	case domain.GroupAccount:
		accts, err := r.Accounts().List(ctx)
		if err != nil {
			return nil, fmt.Errorf("list accounts: %w", err)
		}
		g.accts = make(map[domain.AccountID]string, len(accts))
		for _, ac := range accts {
			g.accts[ac.ID] = ac.Name
		}
	case domain.GroupTag:
	case domain.GroupWeek, domain.GroupMonth, domain.GroupYear:
		gran, _ := by.Granularity()
		ends, err := gran.BucketEnds(p, a.cfg.MonthStartDay)
		if err != nil {
			return nil, err
		}
		start := p.From()
		for _, end := range ends {
			g.buckets = append(g.buckets, bucket{start: start, end: end})
			g.slot(start.String(), 0, start)
			if start, err = end.AddDays(1); err != nil {
				return nil, err
			}
		}
	}
	return g, nil
}

func (g *grouper) slot(key string, id int64, sortDate domain.Date) *groupAcc {
	e := g.rows[key]
	if e == nil {
		z, _ := domain.NewMoney(0, g.base) // a valid base currency never fails
		e = &groupAcc{key: key, id: id, in: z, out: z, sortDate: sortDate}
		g.rows[key] = e
	}
	return e
}

// add counts base (the row amount in the base currency, signed) in the groups of row.
func (g *grouper) add(row reportRow, base domain.Money) error {
	in, out := base, zeroMoney(g.base)
	if base.Sign() < 0 {
		neg, err := base.Neg()
		if err != nil {
			return err
		}
		in, out = zeroMoney(g.base), neg
	}
	for _, e := range g.targets(row) {
		var err error
		if e.in, err = e.in.Add(in); err != nil {
			return err
		}
		if e.out, err = e.out.Add(out); err != nil {
			return err
		}
	}
	return nil
}

// targets returns the group rows row falls in: one, except for tags.
func (g *grouper) targets(row reportRow) []*groupAcc {
	switch g.by {
	case domain.GroupCategory:
		if row.CategoryID == 0 {
			return []*groupAcc{g.slot(UncategorizedCategoryName, 0, domain.Date{})}
		}
		return []*groupAcc{g.slot(g.cats[row.CategoryID], int64(row.CategoryID), domain.Date{})}
	case domain.GroupPayee:
		if row.PayeeID == 0 {
			return []*groupAcc{g.slot(NoPayeeLabel, 0, domain.Date{})}
		}
		return []*groupAcc{g.slot(g.payees[row.PayeeID], int64(row.PayeeID), domain.Date{})}
	case domain.GroupAccount:
		return []*groupAcc{g.slot(g.accts[row.AccountID], int64(row.AccountID), domain.Date{})}
	case domain.GroupTag:
		if len(row.Tags) == 0 {
			return []*groupAcc{g.slot(UntaggedLabel, 0, domain.Date{})}
		}
		out := make([]*groupAcc, 0, len(row.Tags))
		for _, tg := range row.Tags {
			out = append(out, g.slot(tg.Name, int64(tg.ID), domain.Date{}))
		}
		return out
	case domain.GroupWeek, domain.GroupMonth, domain.GroupYear:
	}
	i, _ := slices.BinarySearchFunc(g.buckets, row.Date, func(b bucket, d domain.Date) int {
		return b.end.Cmp(d)
	})
	return []*groupAcc{g.rows[g.buckets[i].start.String()]}
}

// result renders the groups: period groups chronologically, the others by descending total
// (spending) or descending absolute net (cash flow), then by key. spending rows carry only the
// outflow as Total.
func (g *grouper) result(spending bool) ([]GroupedRow, error) {
	accs := make([]*groupAcc, 0, len(g.rows))
	for _, e := range g.rows {
		accs = append(accs, e)
	}
	value := func(e *groupAcc) (domain.Money, error) {
		if spending {
			return e.out, nil
		}
		return e.in.Sub(e.out)
	}
	vals := make(map[string]domain.Money, len(accs))
	for _, e := range accs {
		v, err := value(e)
		if err != nil {
			return nil, err
		}
		vals[e.key] = v
	}
	_, byPeriod := g.by.Granularity()
	slices.SortFunc(accs, func(x, y *groupAcc) int {
		if byPeriod {
			return x.sortDate.Cmp(y.sortDate)
		}
		return cmp.Or(cmp.Compare(abs64(vals[y.key].Minor()), abs64(vals[x.key].Minor())), cmp.Compare(x.key, y.key))
	})
	rows := make([]GroupedRow, 0, len(accs))
	for _, e := range accs {
		row := GroupedRow{Key: e.key, ID: e.id, Total: amountOf(vals[e.key])}
		if !spending {
			in, out := amountOf(e.in), amountOf(e.out)
			row.Inflow, row.Outflow = &in, &out
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

// reportGroup validates by, defaulting to def when empty.
func reportGroup(by, def domain.ReportGroup) (domain.ReportGroup, error) {
	if by == "" {
		return def, nil
	}
	return domain.ParseReportGroup(string(by))
}
