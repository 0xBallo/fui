package reporting

import (
	"context"
	"fmt"
	"slices"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

// reportRow is one row a report counts: a transaction, or one split row of a split transaction
// (carrying the split category and amount). Tags are those of the whole transaction.
type reportRow struct {
	domain.Transaction
	Tags []domain.Tag
}

// rowOptions tunes reportRows for one report.
type rowOptions struct {
	excludeKinds    []domain.TxKind   // never counted, whatever the filter says
	defaultStatuses []domain.TxStatus // used when the filter names no status
	withTags        bool              // load the tags of every row
}

// transferKinds are left out of every report: moving money between own accounts is neither
// spending nor income.
var transferKinds = []domain.TxKind{domain.KindTransfer, domain.KindInvestment}

// reportRows lists the rows of period p matching req.Filter. Period fields of the filter are
// ignored. Account, payee, tag, amount, status, kind, sign and text filters apply to the
// transaction; the category filter applies to each row after splitting, so a split transaction
// matches by its split categories.
func (a *Analytics) reportRows(ctx context.Context, r app.Repos, req ReportRequest, p domain.Period, opt rowOptions) ([]reportRow, error) {
	spec, err := a.resolveFilter(ctx, r, req.Filter)
	if err != nil {
		return nil, err
	}
	catIDs, uncategorized := spec.CategoryIDs, spec.Uncategorized
	spec.CategoryIDs, spec.Uncategorized = nil, false
	spec.From, spec.To = p.From(), p.To()
	spec.Sort, spec.Desc, spec.Limit = "", false, 0
	if len(spec.Statuses) == 0 {
		spec.Statuses = opt.defaultStatuses
	}
	f, err := domain.NewTxFilter(spec)
	if err != nil {
		return nil, err
	}
	txs, err := r.Transactions().List(ctx, f, nil, 0)
	if err != nil {
		return nil, fmt.Errorf("list transactions: %w", err)
	}
	matches := func(id domain.CategoryID) bool {
		switch {
		case len(catIDs) == 0 && !uncategorized:
			return true
		case id == 0:
			return uncategorized
		}
		return slices.Contains(catIDs, id)
	}
	var out []reportRow
	for _, t := range txs {
		if slices.Contains(opt.excludeKinds, t.Kind) {
			continue
		}
		var tags []domain.Tag
		if opt.withTags {
			if tags, err = r.Tags().TagsOf(ctx, t.ID); err != nil {
				return nil, fmt.Errorf("tags of transaction %d: %w", t.ID, err)
			}
		}
		rows, err := splitRows(ctx, r, t, tags, matches)
		if err != nil {
			return nil, err
		}
		out = append(out, rows...)
	}
	return out, nil
}

// resolveFilter resolves the names of filter into a spec, ignoring its period fields.
func (a *Analytics) resolveFilter(ctx context.Context, r app.Repos, filter app.ListTxRequest) (domain.TxFilterSpec, error) {
	filter.From, filter.To, filter.Month, filter.Year = "", "", "", 0
	return app.ResolveTxFilterSpec(ctx, r, a.clock, app.Config{
		Locale: a.cfg.Locale, Location: a.cfg.Location, BaseCurrency: a.cfg.BaseCurrency,
	}, filter)
}

// splitRows expands t into one row per split (the transaction itself when it is not split),
// keeping those whose category matches.
func splitRows(ctx context.Context, r app.Repos, t domain.Transaction, tags []domain.Tag, matches func(domain.CategoryID) bool) ([]reportRow, error) {
	splits, err := r.Splits().SplitsOf(ctx, t.ID)
	if err != nil {
		return nil, fmt.Errorf("splits of transaction %d: %w", t.ID, err)
	}
	if len(splits) == 0 {
		if !matches(t.CategoryID) {
			return nil, nil
		}
		return []reportRow{{Transaction: t, Tags: tags}}, nil
	}
	var out []reportRow
	for _, sp := range splits {
		if !matches(sp.Category) {
			continue
		}
		c := t
		c.CategoryID, c.Amount = sp.Category, sp.Amount
		out = append(out, reportRow{Transaction: c, Tags: tags})
	}
	return out, nil
}
