package importing

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

// ImportResult reports what ApplyImport wrote. Batch is the zero value when nothing was imported.
type ImportResult struct {
	Batch        app.ImportBatch
	Transactions int
	Accounts     int
	Categories   int
	Payees       int
	Tags         int
	Transfers    int
	BackupPath   string
}

// ApplyImport is step 8: a backup (reason "import"), then one transaction that creates the new
// accounts and categories, the import batch and the included rows. A plan with unmapped
// accounts cannot be applied, and one with no included row writes nothing.
func (s *Service) ApplyImport(ctx context.Context, p ImportPlan) (ImportResult, error) {
	if len(p.Unmapped) > 0 {
		refs := make([]string, len(p.Unmapped))
		for i, u := range p.Unmapped {
			refs[i] = u.Ref
		}
		return ImportResult{}, newError(ErrUnmappedAccounts, map[string]any{"refs": refs},
			"accounts not mapped: %s", strings.Join(refs, ", "))
	}
	if p.Counts().Included == 0 {
		return ImportResult{}, nil
	}
	path, err := s.backup.Backup(ctx, app.BackupReasonImport)
	if err != nil {
		return ImportResult{}, fmt.Errorf("backup before import: %w", err)
	}
	res := ImportResult{BackupPath: path}
	err = s.store.WithTx(ctx, func(r app.Repos) error {
		w := &writer{
			r: r, p: p, now: s.clock.Now().UTC(),
			ids: map[string]domain.AccountID{}, cats: map[string]domain.CategoryID{},
			payees: map[string]domain.PayeeID{}, tags: map[string]domain.TagID{},
		}
		return w.run(ctx, &res)
	})
	if err != nil {
		return ImportResult{}, fmt.Errorf("apply import: %w", err)
	}
	return res, nil
}

// writer performs the writes of one ApplyImport inside its transaction.
type writer struct {
	r       app.Repos
	p       ImportPlan
	now     time.Time
	ids     map[string]domain.AccountID // created accounts by ref
	cats    map[string]domain.CategoryID
	payees  map[string]domain.PayeeID // by NormalizePayee of the name or an alias
	tags    map[string]domain.TagID   // by Tag.Key
	txs     []domain.Transaction      // prepared transaction of every included row, by row index
	rowTags [][]domain.TagID          // tag IDs to set on each included row, by row index
}

func (w *writer) run(ctx context.Context, res *ImportResult) error {
	if err := w.accounts(ctx, res); err != nil {
		return err
	}
	batch, err := w.r.Imports().InsertBatch(ctx, app.ImportBatch{
		Connector: w.p.Connector, Source: w.p.Source, SourceSHA256: w.p.SourceSHA256, ImportedAt: w.now,
	})
	if err != nil {
		return fmt.Errorf("create import batch: %w", err)
	}
	res.Batch = batch
	if err := w.prepare(ctx, batch.ID, res); err != nil {
		return err
	}
	if err := w.link(ctx, res); err != nil {
		return err
	}
	for i, row := range w.p.Rows {
		if !row.Include {
			continue
		}
		t, err := w.r.Transactions().Insert(ctx, w.txs[i])
		if err != nil {
			return fmt.Errorf("create transaction of line %d: %w", row.Line, err)
		}
		if ids := w.rowTags[i]; len(ids) > 0 {
			if err := w.r.Tags().SetTags(ctx, t.ID, ids); err != nil {
				return fmt.Errorf("set tags of line %d: %w", row.Line, err)
			}
		}
		res.Transactions++
	}
	return nil
}

// accounts creates the new accounts and saves the ref associations.
func (w *writer) accounts(ctx context.Context, res *ImportResult) error {
	for _, na := range w.p.NewAccounts {
		a, err := w.r.Accounts().Insert(ctx, na.Account)
		if err != nil {
			return fmt.Errorf("create account %q: %w", na.Account.Name, err)
		}
		w.ids[na.Ref] = a.ID
		if err := w.r.Imports().SetAccountRef(ctx, w.p.Connector, na.Ref, a.ID); err != nil {
			return fmt.Errorf("bind %q: %w", na.Ref, err)
		}
		res.Accounts++
	}
	for _, b := range w.p.Bindings {
		if err := w.r.Imports().SetAccountRef(ctx, w.p.Connector, b.Ref, b.AccountID); err != nil {
			return fmt.Errorf("bind %q: %w", b.Ref, err)
		}
	}
	return nil
}

// loadLookups indexes the existing categories, payees and tags by name, so that prepare can
// tell a row's category/payee/tags apart from one still to be created.
func (w *writer) loadLookups(ctx context.Context) error {
	cats, err := w.r.Categories().List(ctx)
	if err != nil {
		return fmt.Errorf("list categories: %w", err)
	}
	for _, c := range cats {
		w.cats[strings.ToLower(c.Name)] = c.ID
	}
	payees, err := w.r.Payees().List(ctx)
	if err != nil {
		return fmt.Errorf("list payees: %w", err)
	}
	for _, p := range payees {
		w.payees[domain.NormalizePayee(p.Name)] = p.ID
		for _, a := range p.Aliases {
			w.payees[domain.NormalizePayee(a)] = p.ID
		}
	}
	tags, err := w.r.Tags().List(ctx)
	if err != nil {
		return fmt.Errorf("list tags: %w", err)
	}
	for _, t := range tags {
		w.tags[t.Key()] = t.ID
	}
	return nil
}

// prepare builds the transaction of each included row: real account, category, payee, tags,
// batch, fingerprint.
func (w *writer) prepare(ctx context.Context, batch domain.ImportBatchID, res *ImportResult) error {
	if err := w.loadLookups(ctx); err != nil {
		return err
	}
	w.txs = make([]domain.Transaction, len(w.p.Rows))
	w.rowTags = make([][]domain.TagID, len(w.p.Rows))
	for i, row := range w.p.Rows {
		if !row.Include {
			continue
		}
		tx := row.Tx
		tx.ID, tx.ImportBatchID = 0, batch
		if tx.AccountID < 0 {
			tx.AccountID = w.ids[row.AccountRef]
		}
		var err error
		if tx.CategoryID, err = w.category(ctx, row, res); err != nil {
			return err
		}
		if tx.PayeeID == 0 {
			if tx.PayeeID, err = w.payee(ctx, row, res); err != nil {
				return err
			}
		}
		if w.rowTags[i], err = w.tagsFor(ctx, row, res); err != nil {
			return err
		}
		if tx.Fingerprint, err = domain.Fingerprint(tx.AccountID, tx.Date, tx.Amount, tx.Description); err != nil {
			return fmt.Errorf("fingerprint of line %d: %w", row.Line, err)
		}
		w.txs[i] = tx
	}
	return nil
}

// category returns the ID of the row's category, creating it on first use.
func (w *writer) category(ctx context.Context, row Row, res *ImportResult) (domain.CategoryID, error) {
	if row.Category == "" {
		return 0, nil
	}
	key := strings.ToLower(row.Category)
	if id, ok := w.cats[key]; ok {
		return id, nil
	}
	c, err := domain.NewCategory(row.Category, kindFor(row.Tx))
	if err != nil {
		return 0, fmt.Errorf("category of line %d: %w", row.Line, err)
	}
	for _, planned := range w.p.NewCategories {
		if strings.EqualFold(planned.Name, row.Category) {
			c = planned
		}
	}
	c, err = w.r.Categories().Insert(ctx, c)
	if err != nil {
		return 0, fmt.Errorf("create category %q: %w", c.Name, err)
	}
	w.cats[key] = c.ID
	res.Categories++
	return c.ID, nil
}

// payee returns the ID of the row's payee, creating it on first use. It returns 0 unchanged
// when the row names no payee, leaving a payee the rules may already have set.
func (w *writer) payee(ctx context.Context, row Row, res *ImportResult) (domain.PayeeID, error) {
	if row.Payee == "" {
		return 0, nil
	}
	key := domain.NormalizePayee(row.Payee)
	if id, ok := w.payees[key]; ok {
		return id, nil
	}
	p, err := domain.NewPayee(row.Payee)
	if err != nil {
		return 0, fmt.Errorf("payee of line %d: %w", row.Line, err)
	}
	p, err = w.r.Payees().Insert(ctx, p)
	if err != nil {
		return 0, fmt.Errorf("create payee %q: %w", p.Name, err)
	}
	w.payees[key] = p.ID
	res.Payees++
	return p.ID, nil
}

// tagsFor returns the IDs to set on the row: what the rules matched plus the row's named tags,
// creating the ones not in the archive yet.
func (w *writer) tagsFor(ctx context.Context, row Row, res *ImportResult) ([]domain.TagID, error) {
	ids := append([]domain.TagID(nil), row.Rules.TagIDs...)
	seen := make(map[domain.TagID]bool, len(ids))
	for _, id := range ids {
		seen[id] = true
	}
	for _, name := range row.Tags {
		key := strings.ToLower(name)
		id, ok := w.tags[key]
		if !ok {
			t, err := domain.NewTag(name)
			if err != nil {
				return nil, fmt.Errorf("tag of line %d: %w", row.Line, err)
			}
			if t, err = w.r.Tags().Insert(ctx, t); err != nil {
				return nil, fmt.Errorf("create tag %q: %w", t.Name, err)
			}
			w.tags[key], id = t.ID, t.ID
			res.Tags++
		}
		if !seen[id] {
			seen[id], ids = true, append(ids, id)
		}
	}
	return ids, nil
}

// link creates the transfer groups: the legs that share a label, then the accepted proposals.
func (w *writer) link(ctx context.Context, res *ImportResult) error {
	keep := func(i int) bool { return w.p.Rows[i].Include }
	for _, p := range labelledPairs(w.p.Rows, keep) {
		g, err := newGroup(w.txs[p.out], w.txs[p.in])
		if err != nil {
			continue // reported in the plan; both legs are imported unlinked
		}
		id, err := w.insertGroup(ctx, g, res)
		if err != nil {
			return err
		}
		w.txs[p.out].TransferGroupID, w.txs[p.in].TransferGroupID = id, id
	}
	for _, pr := range w.p.Transfers {
		if !pr.Accept {
			continue
		}
		if err := w.accept(ctx, pr, res); err != nil {
			return fmt.Errorf("transfer proposal for row %d: %w", pr.Row, err)
		}
	}
	return nil
}

func (w *writer) insertGroup(ctx context.Context, g domain.TransferGroup, res *ImportResult) (domain.TransferGroupID, error) {
	g, err := w.r.Transfers().Insert(ctx, g)
	if err != nil {
		return 0, fmt.Errorf("create transfer: %w", err)
	}
	res.Transfers++
	return g.ID, nil
}

// free reports whether row i is included and not linked to a transfer yet.
func (w *writer) free(i int) bool {
	return i >= 0 && i < len(w.p.Rows) && w.p.Rows[i].Include && w.txs[i].TransferGroupID == 0
}

// partnerOf returns the partner of a proposal, a prepared row or an archived movement, or nil
// when it cannot be linked any more (excluded, or linked meanwhile).
func (w *writer) partnerOf(ctx context.Context, pr TransferProposal) (*domain.Transaction, error) {
	if pr.Other >= 0 {
		if !w.free(pr.Other) {
			return nil, nil
		}
		return &w.txs[pr.Other], nil
	}
	archived, err := w.r.Transactions().Get(ctx, pr.Existing)
	if err != nil {
		return nil, fmt.Errorf("get transaction %d: %w", pr.Existing, err)
	}
	if archived.TransferGroupID != 0 {
		return nil, nil
	}
	return &archived, nil
}

// accept links the row of an accepted proposal to its partner and saves an archived partner.
func (w *writer) accept(ctx context.Context, pr TransferProposal, res *ImportResult) error {
	if !w.free(pr.Row) {
		return nil // excluded, or already linked by its label
	}
	partner, err := w.partnerOf(ctx, pr)
	if err != nil || partner == nil {
		return err
	}
	mine := &w.txs[pr.Row]
	out, in := mine, partner
	if mine.Amount.Sign() > 0 {
		out, in = partner, mine
	}
	g, err := newGroup(*out, *in)
	if err != nil {
		return err
	}
	id, err := w.insertGroup(ctx, g, res)
	if err != nil {
		return err
	}
	for _, t := range []*domain.Transaction{mine, partner} {
		t.TransferGroupID, t.Kind = id, domain.KindTransfer
	}
	if pr.Other < 0 {
		if err := w.r.Transactions().Update(ctx, *partner); err != nil {
			return fmt.Errorf("link transaction %d: %w", partner.ID, err)
		}
	}
	return nil
}
