package app

import (
	"context"
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/textnorm"
)

// DefaultTxLimit is the page size of ListTx when the request sets none.
const DefaultTxLimit = 100

// TxView is a transaction with the names a UI needs to render it. Transfer is set by ShowTx only.
type TxView struct {
	Transaction domain.Transaction
	Account     string
	Category    string
	Transfer    *domain.TransferGroup
	// Tags holds the tag names ordered by tag ID; ListTx fills it, other builders leave it empty.
	Tags []string `json:"tags,omitempty"`
	// Payee is the payee name; ShowTx fills it.
	Payee string `json:"payee,omitempty"`
	// Notices is set by the caller of a write to report what it created implicitly.
	Notices []string `json:"notices,omitempty"`
}

// AddTxRequest carries user-typed values. Account and Amount are required; Date defaults to
// today, Status to booked and Kind to payment. Category is optional. Payee and Tags are matched
// by exact name, case-insensitively; one that does not exist yet is created with the transaction.
type AddTxRequest struct {
	Account     string
	Amount      string
	Description string
	Category    string
	Date        string
	Status      string
	Kind        string
	Notes       string
	Payee       string
	Tags        []string
}

// AddTxPlan is the plan to create Tx.
type AddTxPlan struct {
	Plan
	Tx    domain.Transaction
	Links txLinks
}

// PlanAddTx resolves names, parses the amount in the account currency and plans the creation.
func (s *Service) PlanAddTx(ctx context.Context, req AddTxRequest) (AddTxPlan, error) {
	var out AddTxPlan
	err := s.store.Read(ctx, func(r Repos) error {
		accts, cats, err := loadNames(ctx, r)
		if err != nil {
			return err
		}
		tx, acct, cat, err := s.newTx(req, accts, cats)
		if err != nil {
			return err
		}
		l, err := planLinks(ctx, r, linkRequest{Payee: &req.Payee, Add: req.Tags})
		if err != nil {
			return err
		}
		tx.PayeeID = l.PayeeID
		ch := txChange(OpCreate, tx, acct.Name, cat)
		l.annotate(&ch, nil)
		out = AddTxPlan{Plan: l.plan(ch), Tx: tx, Links: l}
		return nil
	})
	return out, err
}

func (s *Service) newTx(req AddTxRequest, accts []domain.Account, cats []domain.Category) (domain.Transaction, domain.Account, string, error) {
	acct, err := pickAccount(activeAccounts(accts), req.Account)
	if err != nil {
		return domain.Transaction{}, domain.Account{}, "", err
	}
	amount, err := s.amount(req.Amount, acct.Currency)
	if err != nil {
		return domain.Transaction{}, domain.Account{}, "", err
	}
	date, err := s.date(req.Date)
	if err != nil {
		return domain.Transaction{}, domain.Account{}, "", err
	}
	status, err := domain.ParseTxStatus(orDefault(req.Status, string(domain.StatusBooked)))
	if err != nil {
		return domain.Transaction{}, domain.Account{}, "", fmt.Errorf("status: %w", err)
	}
	kind, err := domain.ParseTxKind(orDefault(req.Kind, string(domain.KindPayment)))
	if err != nil {
		return domain.Transaction{}, domain.Account{}, "", fmt.Errorf("kind: %w", err)
	}
	tx := domain.Transaction{
		AccountID: acct.ID, Date: date, Amount: amount, Description: req.Description,
		Notes: req.Notes, Status: status, Kind: kind,
	}
	var catName string
	if req.Category != "" {
		c, err := pickCategory(cats, req.Category)
		if err != nil {
			return domain.Transaction{}, domain.Account{}, "", err
		}
		tx.CategoryID, catName = c.ID, c.Name
	}
	tx, err = domain.NewTransaction(tx)
	if err != nil {
		return domain.Transaction{}, domain.Account{}, "", fmt.Errorf("transaction: %w", err)
	}
	return tx, acct, catName, nil
}

// ApplyAddTx creates the transaction and returns it with its ID.
func (s *Service) ApplyAddTx(ctx context.Context, p AddTxPlan) (domain.Transaction, error) {
	var out domain.Transaction
	err := s.apply(ctx, p.Bulk, func(r Repos) (err error) {
		tx := p.Tx
		if err := p.Links.createPayee(ctx, r, &tx); err != nil {
			return err
		}
		if out, err = r.Transactions().Insert(ctx, tx); err != nil {
			return err
		}
		if err := p.Links.storeTags(ctx, r, out.ID); err != nil {
			return err
		}
		after, err := snapshot(ctx, r, out.ID)
		if err != nil {
			return err
		}
		return s.journal(ctx, r, OpKindTxAdd, undoPayload{After: after})
	})
	return out, err
}

// ListTxRequest filters a transaction listing; every field is optional. Dates accept the same
// formats as input dates. The amount range is parsed in the currency of the filtered accounts
// (every account when none is given), which must then share one currency. The period is given
// either as From/To, or as the convenience Month ("YYYY-MM") or Year, mutually exclusive (an
// invalid_filter error otherwise, same as domain.TxFilterSpec). Payees are
// resolved by name or alias. Tags OR among themselves and AND with every other field, same as domain.TxFilter.
type ListTxRequest struct {
	From, To      string
	Month         string
	Year          int
	Accounts      []string
	Categories    []string
	Uncategorized bool
	Payees        []string
	Tags          []string
	Statuses      []string
	Kinds         []string
	Batch         int64 // import batch id, 0 means unset
	Text          string
	MinAmount     string
	MaxAmount     string
	Sign          domain.TxSign
	Sort          domain.TxSort
	Desc          bool
	After         *TxCursor // Next of the previous page
	Limit         int       // page size, DefaultTxLimit when 0
}

// TxPage is one page of a listing, most recent first. Next is nil on the last page.
type TxPage struct {
	Items []TxView
	Next  *TxCursor
}

// ResolveTxFilter resolves a ListTxRequest into the TxFilter a repository understands, the same
// way ListTx does: account and category names against the archive, the amount range in the
// currency of the selected accounts. Other use cases that filter transactions the way `tx list`
// does (for example `export`) call this instead of duplicating name and amount resolution.
func (s *Service) ResolveTxFilter(ctx context.Context, req ListTxRequest) (domain.TxFilter, error) {
	var f domain.TxFilter
	err := s.store.Read(ctx, func(r Repos) error {
		spec, err := s.resolveTxSpec(ctx, r, req)
		if err != nil {
			return err
		}
		f, err = domain.NewTxFilter(spec)
		return err
	})
	return f, err
}

// ResolveTxFilterSpec resolves req into a TxFilterSpec inside an existing read, exactly like
// Service.ResolveTxFilter does, for use cases that have no Service (the analytics reports). The
// spec is returned unvalidated so the caller can adjust it before domain.NewTxFilter.
func ResolveTxFilterSpec(ctx context.Context, r Repos, clock Clock, cfg Config, req ListTxRequest) (domain.TxFilterSpec, error) {
	return New(nil, clock, nil, cfg).resolveTxSpec(ctx, r, req)
}

// resolveTxSpec loads what the names in req refer to and resolves them.
func (s *Service) resolveTxSpec(ctx context.Context, r Repos, req ListTxRequest) (domain.TxFilterSpec, error) {
	accts, cats, err := loadNames(ctx, r)
	if err != nil {
		return domain.TxFilterSpec{}, err
	}
	tags, err := s.loadTagsIfNeeded(ctx, r, req.Tags)
	if err != nil {
		return domain.TxFilterSpec{}, err
	}
	var payees []domain.Payee
	if len(req.Payees) > 0 {
		if payees, err = r.Payees().List(ctx); err != nil {
			return domain.TxFilterSpec{}, fmt.Errorf("list payees: %w", err)
		}
	}
	return s.txFilterSpec(req, accts, cats, tags, payees)
}

// ListTx returns one page of the matching transactions.
func (s *Service) ListTx(ctx context.Context, req ListTxRequest) (TxPage, error) {
	limit := req.Limit
	if limit <= 0 {
		limit = DefaultTxLimit
	}
	var page TxPage
	err := s.store.Read(ctx, func(r Repos) error {
		accts, cats, err := loadNames(ctx, r)
		if err != nil {
			return err
		}
		spec, err := s.resolveTxSpec(ctx, r, req)
		if err != nil {
			return err
		}
		if req.Sort == "" {
			spec.Desc = true // default order: newest first, unless an explicit sort key overrides it
		}
		f, err := domain.NewTxFilter(spec)
		if err != nil {
			return err
		}
		rows, err := r.Transactions().List(ctx, f, req.After, limit+1)
		if err != nil {
			return fmt.Errorf("list transactions: %w", err)
		}
		if len(rows) > limit {
			rows = rows[:limit]
			last := rows[limit-1]
			page.Next = &TxCursor{Date: last.Date, ID: last.ID}
		}
		v := newViewer(accts, cats)
		for _, t := range rows {
			tv := v.view(t)
			// ponytail: one query per row, fine for a 200-row page; batch it if pages grow
			rowTags, err := r.Tags().TagsOf(ctx, t.ID)
			if err != nil {
				return fmt.Errorf("tags of transaction %d: %w", t.ID, err)
			}
			for _, tg := range rowTags {
				tv.Tags = append(tv.Tags, tg.Name)
			}
			page.Items = append(page.Items, tv)
		}
		return nil
	})
	return page, err
}

// loadTagsIfNeeded lists the tags only when the request filters by tag name, to avoid the extra
// query otherwise.
func (s *Service) loadTagsIfNeeded(ctx context.Context, r Repos, names []string) ([]domain.Tag, error) {
	if len(names) == 0 {
		return nil, nil
	}
	tags, err := r.Tags().List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}
	return tags, nil
}

func (s *Service) txFilterSpec(req ListTxRequest, accts []domain.Account, cats []domain.Category, tags []domain.Tag, payees []domain.Payee) (domain.TxFilterSpec, error) {
	spec := domain.TxFilterSpec{
		Text: textnorm.Normalize(req.Text), Year: req.Year, Uncategorized: req.Uncategorized,
		ImportBatchID: domain.ImportBatchID(req.Batch), Sign: req.Sign, Sort: req.Sort, Desc: req.Desc,
	}
	var err error
	if spec.From, err = s.optionalDate(req.From); err != nil {
		return domain.TxFilterSpec{}, err
	}
	if spec.To, err = s.optionalDate(req.To); err != nil {
		return domain.TxFilterSpec{}, err
	}
	if spec.Month, err = s.optionalMonth(req.Month); err != nil {
		return domain.TxFilterSpec{}, err
	}
	selected := accts
	if len(req.Accounts) > 0 {
		selected = selected[:0:0]
		for _, ref := range req.Accounts {
			a, err := pickAccount(accts, ref)
			if err != nil {
				return domain.TxFilterSpec{}, err
			}
			spec.AccountIDs = append(spec.AccountIDs, a.ID)
			selected = append(selected, a)
		}
	}
	for _, ref := range req.Categories {
		c, err := pickCategory(cats, ref)
		if err != nil {
			return domain.TxFilterSpec{}, err
		}
		spec.CategoryIDs = append(spec.CategoryIDs, c.ID)
	}
	for _, ref := range req.Tags {
		tg, err := pickTag(tags, ref)
		if err != nil {
			return domain.TxFilterSpec{}, err
		}
		spec.TagIDs = append(spec.TagIDs, tg.ID)
	}
	for _, ref := range req.Payees {
		py, err := pickPayee(payees, ref)
		if err != nil {
			return domain.TxFilterSpec{}, err
		}
		spec.PayeeIDs = append(spec.PayeeIDs, py.ID)
	}
	if err := s.applyStatusAndKind(req, &spec); err != nil {
		return domain.TxFilterSpec{}, err
	}
	if spec.MinAbs, spec.MaxAbs, err = s.amountRange(req, selected); err != nil {
		return domain.TxFilterSpec{}, err
	}
	return spec, nil
}

// applyStatusAndKind parses req's status and kind filters into spec.
func (s *Service) applyStatusAndKind(req ListTxRequest, spec *domain.TxFilterSpec) error {
	for _, st := range req.Statuses {
		v, err := domain.ParseTxStatus(st)
		if err != nil {
			return fmt.Errorf("status: %w", err)
		}
		spec.Statuses = append(spec.Statuses, v)
	}
	for _, k := range req.Kinds {
		v, err := domain.ParseTxKind(k)
		if err != nil {
			return fmt.Errorf("kind: %w", err)
		}
		spec.Kinds = append(spec.Kinds, v)
	}
	return nil
}

// optionalMonth parses "YYYY-MM" into a Date on the first of that month, the convenience form
// domain.TxFilterSpec.Month takes; it returns the zero Date when in is empty.
func (s *Service) optionalMonth(in string) (domain.Date, error) {
	if in == "" {
		return domain.Date{}, nil
	}
	d, err := domain.ParseDate(in + "-01")
	if err != nil {
		return domain.Date{}, fmt.Errorf("month: %w", err)
	}
	return d, nil
}

func (s *Service) optionalDate(in string) (domain.Date, error) {
	if in == "" {
		return domain.Date{}, nil
	}
	return s.date(in)
}

func (s *Service) amountRange(req ListTxRequest, selected []domain.Account) (lo, hi *int64, err error) {
	if req.MinAmount == "" && req.MaxAmount == "" {
		return nil, nil, nil
	}
	cur, err := commonCurrency(selected)
	if err != nil {
		return nil, nil, err
	}
	parse := func(in string) (*int64, error) {
		if in == "" {
			return nil, nil
		}
		m, err := s.amount(in, cur)
		if err != nil {
			return nil, err
		}
		v := m.Minor()
		if v < 0 && v != math.MinInt64 {
			v = -v // --min/--max bound the absolute value
		}
		return &v, nil
	}
	if lo, err = parse(req.MinAmount); err != nil {
		return nil, nil, err
	}
	if hi, err = parse(req.MaxAmount); err != nil {
		return nil, nil, err
	}
	return lo, hi, nil
}

// commonCurrency returns the single currency of accts or fails when they mix currencies.
func commonCurrency(accts []domain.Account) (domain.Currency, error) {
	if len(accts) == 0 {
		return domain.Currency{}, newError(domain.ErrMissingField, map[string]any{"field": "account"}, "no account to take the currency from")
	}
	for _, a := range accts[1:] {
		if a.Currency != accts[0].Currency {
			return domain.Currency{}, newError(domain.ErrCurrencyMismatch,
				map[string]any{"left": accts[0].Currency.Code(), "right": a.Currency.Code()},
				"amount range needs accounts in one currency, got %s and %s", accts[0].Currency, a.Currency)
		}
	}
	return accts[0].Currency, nil
}

// ShowTx returns one transaction; for a transfer leg it also returns the transfer group.
func (s *Service) ShowTx(ctx context.Context, id domain.TransactionID) (TxView, error) {
	var out TxView
	err := s.store.Read(ctx, func(r Repos) error {
		accts, cats, err := loadNames(ctx, r)
		if err != nil {
			return err
		}
		t, err := r.Transactions().Get(ctx, id)
		if err != nil {
			return fmt.Errorf("get transaction %d: %w", id, err)
		}
		out = newViewer(accts, cats).view(t)
		if err := fillLinkNames(ctx, r, &out); err != nil {
			return err
		}
		if t.TransferGroupID != 0 {
			g, err := r.Transfers().Get(ctx, t.TransferGroupID)
			if err != nil {
				return fmt.Errorf("get transfer %d: %w", t.TransferGroupID, err)
			}
			out.Transfer = &g
		}
		return nil
	})
	return out, err
}

// EditTxRequest changes the non-nil fields of transaction ID. An empty Category or Payee clears
// it. Payee, AddTags and RemoveTags follow the matching rules of AddTxRequest; RemoveTags names
// must exist.
// Editing a reconciled transaction needs ConfirmReconciled; the amount of a transfer leg
// cannot be edited (edit by removing and re-creating the transfer).
type EditTxRequest struct {
	ID                domain.TransactionID
	Amount            *string
	Date              *string
	Description       *string
	Notes             *string
	Category          *string
	Status            *string
	Kind              *string
	Payee             *string
	AddTags           []string
	RemoveTags        []string
	ConfirmReconciled bool
}

func (r EditTxRequest) empty() bool {
	return r.Amount == nil && r.Date == nil && r.Description == nil && r.Notes == nil &&
		r.Category == nil && r.Status == nil && r.Kind == nil && r.Payee == nil &&
		len(r.AddTags) == 0 && len(r.RemoveTags) == 0
}

// EditTxPlan is the plan to store Tx.
type EditTxPlan struct {
	Plan
	Tx    domain.Transaction
	Links txLinks
}

// PlanEditTx plans the edit of a transaction.
func (s *Service) PlanEditTx(ctx context.Context, req EditTxRequest) (EditTxPlan, error) {
	var out EditTxPlan
	err := s.store.Read(ctx, func(r Repos) error {
		accts, cats, err := loadNames(ctx, r)
		if err != nil {
			return err
		}
		cur, err := r.Transactions().Get(ctx, req.ID)
		if err != nil {
			return fmt.Errorf("get transaction %d: %w", req.ID, err)
		}
		if err := checkReconciled(cur, req.ConfirmReconciled); err != nil {
			return err
		}
		edited, err := s.editedTx(cur, req, cats)
		if err != nil {
			return err
		}
		if edited.Amount != cur.Amount {
			rows, err := r.Splits().SplitsOf(ctx, cur.ID)
			if err != nil {
				return fmt.Errorf("get splits of %d: %w", cur.ID, err)
			}
			if len(rows) > 0 {
				return newError(domain.ErrInvalidValue, map[string]any{"field": "amount", "reason": "split"},
					"transaction %d is split: unsplit it before changing the amount", cur.ID)
			}
		}
		l, err := planLinks(ctx, r, linkRequest{ID: cur.ID, Payee: req.Payee, Add: req.AddTags, Remove: req.RemoveTags})
		if err != nil {
			return err
		}
		if req.Payee != nil {
			edited.PayeeID = l.PayeeID
		}
		v := newViewer(accts, cats).view(edited)
		ch := txChange(OpUpdate, edited, v.Account, v.Category)
		l.annotate(&ch, req.Payee)
		out = EditTxPlan{Plan: l.plan(ch), Tx: edited, Links: l}
		return nil
	})
	return out, err
}

func (s *Service) editedTx(t domain.Transaction, req EditTxRequest, cats []domain.Category) (domain.Transaction, error) {
	if req.empty() {
		return domain.Transaction{}, newError(domain.ErrMissingField, map[string]any{"field": "changes"}, "nothing to change")
	}
	if req.Amount != nil {
		if t.TransferGroupID != 0 {
			return domain.Transaction{}, newError(domain.ErrInvalidValue, map[string]any{"field": "amount"},
				"the amount of a transfer leg cannot be edited")
		}
		m, err := s.amount(*req.Amount, t.Amount.Currency())
		if err != nil {
			return domain.Transaction{}, err
		}
		t.Amount = m
	}
	if err := s.setTxFields(&t, req, cats); err != nil {
		return domain.Transaction{}, err
	}
	edited, err := domain.NewTransaction(t)
	if err != nil {
		return domain.Transaction{}, fmt.Errorf("transaction: %w", err)
	}
	return edited, nil
}

func (s *Service) setTxFields(t *domain.Transaction, req EditTxRequest, cats []domain.Category) error {
	var err error
	if req.Date != nil {
		if t.Date, err = s.date(*req.Date); err != nil {
			return err
		}
	}
	if req.Description != nil {
		t.Description = *req.Description
	}
	if req.Notes != nil {
		t.Notes = *req.Notes
	}
	if req.Category != nil {
		t.CategoryID = 0
		if *req.Category != "" {
			c, err := pickCategory(cats, *req.Category)
			if err != nil {
				return err
			}
			t.CategoryID = c.ID
		}
	}
	if req.Status != nil {
		if t.Status, err = domain.ParseTxStatus(*req.Status); err != nil {
			return fmt.Errorf("status: %w", err)
		}
	}
	if req.Kind != nil {
		if t.Kind, err = domain.ParseTxKind(*req.Kind); err != nil {
			return fmt.Errorf("kind: %w", err)
		}
	}
	return nil
}

// ApplyEditTx stores the edited transaction.
func (s *Service) ApplyEditTx(ctx context.Context, p EditTxPlan) (domain.Transaction, error) {
	err := s.apply(ctx, p.Bulk, func(r Repos) error {
		before, err := snapshot(ctx, r, p.Tx.ID)
		if err != nil {
			return err
		}
		tx := p.Tx
		if err := p.Links.createPayee(ctx, r, &tx); err != nil {
			return err
		}
		if err := r.Transactions().Update(ctx, tx); err != nil {
			return fmt.Errorf("update transaction %d: %w", p.Tx.ID, err)
		}
		if err := p.Links.storeTags(ctx, r, p.Tx.ID); err != nil {
			return err
		}
		after, err := snapshot(ctx, r, p.Tx.ID)
		if err != nil {
			return err
		}
		return s.journal(ctx, r, OpKindTxEdit, undoPayload{Before: before, After: after})
	})
	return p.Tx, err
}

// CategorizeTxRequest sets Category on every transaction in IDs, clearing it when Category is
// empty. Reconciled transactions among them need ConfirmReconciled.
type CategorizeTxRequest struct {
	IDs               []domain.TransactionID
	Category          string
	ConfirmReconciled bool
}

// CategorizeTxPlan is the plan to store every transaction of Txs with its new category. Bulk is
// always true: it is a mass change over potentially many transactions.
type CategorizeTxPlan struct {
	Plan
	Txs []domain.Transaction
}

// PlanCategorizeTx plans setting Category on every transaction in req.IDs. It fails, without
// writing to the store, if IDs is empty, an ID does not exist, Category does not resolve, or a
// reconciled transaction is included without ConfirmReconciled.
func (s *Service) PlanCategorizeTx(ctx context.Context, req CategorizeTxRequest) (CategorizeTxPlan, error) {
	var out CategorizeTxPlan
	if len(req.IDs) == 0 {
		return out, newError(domain.ErrMissingField, map[string]any{"field": "ids"}, "no transactions selected")
	}
	err := s.store.Read(ctx, func(r Repos) error {
		accts, cats, err := loadNames(ctx, r)
		if err != nil {
			return err
		}
		var catID domain.CategoryID
		var catName string
		if req.Category != "" {
			c, err := pickCategory(cats, req.Category)
			if err != nil {
				return err
			}
			catID, catName = c.ID, c.Name
		}
		v := newViewer(accts, cats)
		for _, id := range req.IDs {
			t, err := r.Transactions().Get(ctx, id)
			if err != nil {
				return fmt.Errorf("get transaction %d: %w", id, err)
			}
			if err := checkReconciled(t, req.ConfirmReconciled); err != nil {
				return err
			}
			t.CategoryID = catID
			edited, err := domain.NewTransaction(t)
			if err != nil {
				return fmt.Errorf("transaction %d: %w", id, err)
			}
			out.Txs = append(out.Txs, edited)
			out.Changes = append(out.Changes, txChange(OpUpdate, edited, v.view(edited).Account, catName))
		}
		out.Bulk = true
		return nil
	})
	return out, err
}

// ApplyCategorizeTx stores every transaction of the plan in a single transaction, taking one
// backup first since it is a bulk change.
func (s *Service) ApplyCategorizeTx(ctx context.Context, p CategorizeTxPlan) error {
	return s.apply(ctx, p.Bulk, func(r Repos) error {
		for _, t := range p.Txs {
			if err := r.Transactions().Update(ctx, t); err != nil {
				return fmt.Errorf("update transaction %d: %w", t.ID, err)
			}
		}
		return nil
	})
}

// RemoveTxRequest removes transaction ID; removing a transfer leg removes the whole transfer.
// Removing a reconciled transaction needs ConfirmReconciled.
type RemoveTxRequest struct {
	ID                domain.TransactionID
	ConfirmReconciled bool
}

// RemoveTxPlan lists the transactions (and the transfer group, if any) to delete.
type RemoveTxPlan struct {
	Plan
	TxIDs   []domain.TransactionID
	GroupID domain.TransferGroupID
}

// PlanRemoveTx plans the removal of a transaction.
func (s *Service) PlanRemoveTx(ctx context.Context, req RemoveTxRequest) (RemoveTxPlan, error) {
	var out RemoveTxPlan
	err := s.store.Read(ctx, func(r Repos) error {
		accts, cats, err := loadNames(ctx, r)
		if err != nil {
			return err
		}
		t, err := r.Transactions().Get(ctx, req.ID)
		if err != nil {
			return fmt.Errorf("get transaction %d: %w", req.ID, err)
		}
		rows := []domain.Transaction{t}
		if t.TransferGroupID != 0 {
			if rows, err = r.Transactions().ByTransferGroup(ctx, t.TransferGroupID); err != nil {
				return fmt.Errorf("get transfer legs: %w", err)
			}
			out.GroupID = t.TransferGroupID
		}
		v := newViewer(accts, cats)
		for _, row := range rows {
			if err := checkReconciled(row, req.ConfirmReconciled); err != nil {
				return err
			}
			out.TxIDs = append(out.TxIDs, row.ID)
			x := v.view(row)
			out.Changes = append(out.Changes, txChange(OpDelete, row, x.Account, x.Category))
		}
		return nil
	})
	return out, err
}

// ApplyRemoveTx deletes the planned transactions and their transfer group.
func (s *Service) ApplyRemoveTx(ctx context.Context, p RemoveTxPlan) error {
	return s.apply(ctx, p.Bulk, func(r Repos) error {
		before, err := snapshot(ctx, r, p.TxIDs...)
		if err != nil {
			return err
		}
		undo := undoPayload{Before: before}
		if p.GroupID != 0 {
			g, err := r.Transfers().Get(ctx, p.GroupID)
			if err != nil {
				return fmt.Errorf("get transfer %d: %w", p.GroupID, err)
			}
			undo.Group, undo.FeeMinor, undo.FeeCurrency = int64(p.GroupID), g.Fee.Minor(), g.Fee.Currency().Code()
		}
		for _, id := range p.TxIDs {
			if err := r.Transactions().Delete(ctx, id); err != nil {
				return fmt.Errorf("delete transaction %d: %w", id, err)
			}
		}
		if p.GroupID != 0 {
			if err := r.Transfers().Delete(ctx, p.GroupID); err != nil {
				return fmt.Errorf("delete transfer %d: %w", p.GroupID, err)
			}
		}
		return s.journal(ctx, r, OpKindTxRemove, undo)
	})
}

func checkReconciled(t domain.Transaction, confirmed bool) error {
	if t.Status == domain.StatusReconciled && !confirmed {
		return newError(domain.ErrReconciled, map[string]any{"id": int64(t.ID)},
			"transaction %d is reconciled: confirm to change it", t.ID)
	}
	return nil
}

// loadNames loads every account and category, which the use cases need to resolve and show names.
func loadNames(ctx context.Context, r Repos) ([]domain.Account, []domain.Category, error) {
	accts, err := r.Accounts().List(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("list accounts: %w", err)
	}
	cats, err := r.Categories().List(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("list categories: %w", err)
	}
	return accts, cats, nil
}

type viewer struct {
	accounts   map[domain.AccountID]string
	categories map[domain.CategoryID]string
}

func newViewer(accts []domain.Account, cats []domain.Category) viewer {
	v := viewer{accounts: map[domain.AccountID]string{}, categories: map[domain.CategoryID]string{}}
	for _, a := range accts {
		v.accounts[a.ID] = a.Name
	}
	for _, c := range cats {
		v.categories[c.ID] = c.Name
	}
	return v
}

func (v viewer) view(t domain.Transaction) TxView {
	return TxView{Transaction: t, Account: v.accounts[t.AccountID], Category: v.categories[t.CategoryID]}
}

func txChange(op Op, t domain.Transaction, account, category string) Change {
	f := map[string]any{
		"id": int64(t.ID), "account": account, "date": t.Date.String(), "description": t.Description,
		"status": string(t.Status), "kind": string(t.Kind),
	}
	moneyFields(f, "amount", t.Amount)
	if category != "" {
		f["category"] = category
	}
	if t.Notes != "" {
		f["notes"] = t.Notes
	}
	return Change{
		Op: op, Entity: EntityTransaction, Fields: f,
		Summary: fmt.Sprintf("%s transaction %s %s %q on %s", op, t.Date, t.Amount, t.Description, account),
	}
}

// linkRequest asks planLinks to resolve the payee and tag references of a transaction write.
// Payee nil leaves the payee alone, empty clears it. ID is zero for a transaction not yet stored.
type linkRequest struct {
	ID     domain.TransactionID
	Payee  *string
	Add    []string
	Remove []string
}

// txLinks is the resolved payee and tag state of a transaction write, including the payee and
// tags it creates; Apply* writes them in the same store transaction as the row itself.
type txLinks struct {
	PayeeID  domain.PayeeID
	NewPayee *domain.Payee
	// TagIDs is the final tag set made of existing tags; NewTags are added once created.
	TagIDs  []domain.TagID
	NewTags []domain.Tag
	// SetTags is false when the tag set is unchanged.
	SetTags bool
	// TagNames is the final tag set, for the plan summary.
	TagNames []string
	notices  []string
	creates  []Change
}

func planLinks(ctx context.Context, r Repos, req linkRequest) (txLinks, error) {
	var l txLinks
	if req.Payee != nil && *req.Payee != "" {
		payees, err := r.Payees().List(ctx)
		if err != nil {
			return l, fmt.Errorf("list payees: %w", err)
		}
		if i := slices.IndexFunc(payees, func(p domain.Payee) bool { return p.MatchesAlias(*req.Payee) }); i >= 0 {
			l.PayeeID = payees[i].ID
		} else {
			p, err := domain.NewPayee(*req.Payee)
			if err != nil {
				return l, fmt.Errorf("payee: %w", err)
			}
			l.NewPayee = &p
			l.notices = append(l.notices, fmt.Sprintf("created payee %s", p.Name))
			l.creates = append(l.creates, Change{
				Op: OpCreate, Entity: EntityPayee, Summary: fmt.Sprintf("add payee %q", p.Name),
				Fields: map[string]any{"name": p.Name},
			})
		}
	}
	if len(req.Add) == 0 && len(req.Remove) == 0 {
		return l, nil
	}
	return l, l.planTags(ctx, r, req)
}

func (l *txLinks) planTags(ctx context.Context, r Repos, req linkRequest) error {
	tags, err := r.Tags().List(ctx)
	if err != nil {
		return fmt.Errorf("list tags: %w", err)
	}
	var cur []domain.Tag
	if req.ID != 0 {
		if cur, err = r.Tags().TagsOf(ctx, req.ID); err != nil {
			return fmt.Errorf("tags of transaction %d: %w", req.ID, err)
		}
	}
	add, created, err := resolveTagNames(tags, req.Add)
	if err != nil {
		return err
	}
	final := slices.Clone(cur)
	for _, tg := range add {
		if !slices.ContainsFunc(final, func(x domain.Tag) bool { return x.ID == tg.ID }) {
			final = append(final, tg)
		}
	}
	for _, ref := range req.Remove {
		tg, err := pickTag(tags, ref)
		if err != nil {
			return err
		}
		final = slices.DeleteFunc(final, func(x domain.Tag) bool { return x.ID == tg.ID })
	}
	l.SetTags = true
	for _, tg := range final {
		l.TagIDs = append(l.TagIDs, tg.ID)
		l.TagNames = append(l.TagNames, tg.Name)
	}
	for _, tg := range created {
		l.NewTags = append(l.NewTags, tg)
		l.TagNames = append(l.TagNames, tg.Name)
		l.notices = append(l.notices, fmt.Sprintf("created tag %s", tg.Name))
		l.creates = append(l.creates, Change{
			Op: OpCreate, Entity: EntityTag, Summary: fmt.Sprintf("add tag %q", tg.Name),
			Fields: map[string]any{"name": tg.Name},
		})
	}
	return nil
}

// annotate adds the payee and tags to the transaction change ch; payee is the edit request's
// payee (nil on add, where an unset payee has nothing to show).
func (l txLinks) annotate(ch *Change, payee *string) {
	if payee != nil && *payee != "" {
		ch.Fields["payee"] = strings.TrimSpace(*payee)
		if l.NewPayee != nil {
			ch.Fields["payee"] = l.NewPayee.Name
		}
	}
	if l.SetTags {
		ch.Fields["tags"] = l.TagNames
	}
}

// plan builds the Plan: the creations first, then the transaction change.
func (l txLinks) plan(ch Change) Plan {
	return Plan{Changes: append(slices.Clone(l.creates), ch), Notices: l.notices}
}

func (l txLinks) createPayee(ctx context.Context, r Repos, tx *domain.Transaction) error {
	if l.NewPayee == nil {
		return nil
	}
	p, err := r.Payees().Insert(ctx, *l.NewPayee)
	if err != nil {
		return fmt.Errorf("insert payee %q: %w", l.NewPayee.Name, err)
	}
	tx.PayeeID = p.ID
	return nil
}

func (l txLinks) storeTags(ctx context.Context, r Repos, id domain.TransactionID) error {
	if !l.SetTags {
		return nil
	}
	ids := slices.Clone(l.TagIDs)
	for _, tg := range l.NewTags {
		ins, err := r.Tags().Insert(ctx, tg)
		if err != nil {
			return fmt.Errorf("insert tag %q: %w", tg.Name, err)
		}
		ids = append(ids, ins.ID)
	}
	if err := r.Tags().SetTags(ctx, id, ids); err != nil {
		return fmt.Errorf("set tags of transaction %d: %w", id, err)
	}
	return nil
}

// fillLinkNames sets the payee name and the tag names of v.
func fillLinkNames(ctx context.Context, r Repos, v *TxView) error {
	if id := v.Transaction.PayeeID; id != 0 {
		p, err := r.Payees().Get(ctx, id)
		if err != nil {
			return fmt.Errorf("get payee %d: %w", id, err)
		}
		v.Payee = p.Name
	}
	tags, err := r.Tags().TagsOf(ctx, v.Transaction.ID)
	if err != nil {
		return fmt.Errorf("tags of transaction %d: %w", v.Transaction.ID, err)
	}
	for _, tg := range tags {
		v.Tags = append(v.Tags, tg.Name)
	}
	return nil
}
