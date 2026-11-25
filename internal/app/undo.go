package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"time"

	"github.com/0xballo/fui/internal/domain"
)

// Kinds of journal entries; they only label the entry, every kind is reverted the same way.
const (
	OpKindTxAdd    = "tx.add"
	OpKindTxEdit   = "tx.edit"
	OpKindTxRemove = "tx.rm"
	OpKindTransfer = "transfer"
	OpKindSplit    = "split"

	OpKindCategorize   = "tx.categorize"
	OpKindTag          = "tx.tag"
	OpKindUntag        = "tx.untag"
	OpKindRulesApply   = "rules.apply"
	OpKindRecurConfirm = "recurring.confirm"
	OpKindLoanConfirm  = "loan.confirm"
)

// UndoField names a transaction field a journal entry may own: an entry with a mask only compares
// and restores those fields, so later edits of the other fields survive an undo.
type UndoField string

// Fields that can appear in an undo mask.
const (
	UndoFieldCategory UndoField = "category"
	UndoFieldTags     UndoField = "tags"
	UndoFieldStatus   UndoField = "status"
	UndoFieldPayee    UndoField = "payee"
)

// OpEntry is one row of the undo journal. Payload is the JSON snapshot needed to revert it.
type OpEntry struct {
	ID        int64
	Kind      string
	Payload   string
	CreatedAt time.Time
	UndoneAt  time.Time // zero while the entry can still be undone
}

// OpLogRepo stores the undo journal and restores deleted rows under their old IDs.
type OpLogRepo interface {
	Append(ctx context.Context, e OpEntry) (OpEntry, error)
	// Latest returns the newest entry that is not undone, or ErrNotFound.
	Latest(ctx context.Context) (OpEntry, error)
	MarkUndone(ctx context.Context, id int64, at time.Time) error
	// RestoreTransaction inserts t keeping t.ID.
	RestoreTransaction(ctx context.Context, t domain.Transaction) error
	// RestoreTransferGroup inserts a transfer group keeping id; fee may be zero.
	RestoreTransferGroup(ctx context.Context, id domain.TransferGroupID, fee domain.Money) error
}

type splitState struct {
	Category int64
	Minor    int64
	Note     string
}

// txState is a transaction with its tags and splits, in plain JSON-friendly values.
type txState struct {
	ID            int64
	Account       int64
	Date          string
	OperationDate string
	ValueDate     string
	DueDate       string
	Minor         int64
	Currency      string
	Payee         int64
	Description   string
	Notes         string
	Category      int64
	Status        string
	Kind          string
	ExternalID    string
	Fingerprint   string
	Batch         int64
	Group         int64
	Recurrence    int64
	Tags          []int64
	Splits        []splitState
}

// undoPayload holds the affected transactions before and after the operation: Before is empty
// for an add, After is empty for a removal. Group is the transfer group involved, if any.
// Mask lists the fields the entry owns (empty: all), NewGroups the transfer groups the write
// created and CreatedPayee the payee it created implicitly (0: none).
type undoPayload struct {
	Group        int64
	NewGroups    []int64
	Mask         []string
	CreatedPayee int64
	FeeMinor     int64
	FeeCurrency  string
	Before       []txState
	After        []txState
}

func dateStr(d domain.Date) string {
	if d.IsZero() {
		return ""
	}
	return d.String()
}

func parseOptDate(s string) (domain.Date, error) {
	if s == "" {
		return domain.Date{}, nil
	}
	return domain.ParseDate(s)
}

// snapshot reads the transactions ids with their tags and splits.
func snapshot(ctx context.Context, r Repos, ids ...domain.TransactionID) ([]txState, error) {
	out := make([]txState, 0, len(ids))
	for _, id := range ids {
		t, err := r.Transactions().Get(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("snapshot transaction %d: %w", id, err)
		}
		tags, err := r.Tags().TagsOf(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("snapshot tags of %d: %w", id, err)
		}
		rows, err := r.Splits().SplitsOf(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("snapshot splits of %d: %w", id, err)
		}
		st := txState{
			ID: int64(t.ID), Account: int64(t.AccountID), Date: dateStr(t.Date),
			OperationDate: dateStr(t.OperationDate), ValueDate: dateStr(t.ValueDate), DueDate: dateStr(t.DueDate),
			Minor: t.Amount.Minor(), Currency: t.Amount.Currency().Code(), Payee: int64(t.PayeeID),
			Description: t.Description, Notes: t.Notes, Category: int64(t.CategoryID),
			Status: string(t.Status), Kind: string(t.Kind), ExternalID: t.ExternalID, Fingerprint: t.Fingerprint,
			Batch: int64(t.ImportBatchID), Group: int64(t.TransferGroupID), Recurrence: int64(t.RecurrenceID),
			Tags: make([]int64, 0, len(tags)), Splits: make([]splitState, 0, len(rows)),
		}
		for _, tg := range tags {
			st.Tags = append(st.Tags, int64(tg.ID))
		}
		for _, sp := range rows {
			st.Splits = append(st.Splits, splitState{Category: int64(sp.Category), Minor: sp.Amount.Minor(), Note: sp.Note})
		}
		out = append(out, st)
	}
	return out, nil
}

func (st txState) transaction() (domain.Transaction, error) {
	cur, err := domain.ParseCurrency(st.Currency)
	if err != nil {
		return domain.Transaction{}, fmt.Errorf("currency: %w", err)
	}
	amount, err := domain.NewMoney(st.Minor, cur)
	if err != nil {
		return domain.Transaction{}, fmt.Errorf("amount: %w", err)
	}
	t := domain.Transaction{
		ID: domain.TransactionID(st.ID), AccountID: domain.AccountID(st.Account), Amount: amount,
		PayeeID: domain.PayeeID(st.Payee), Description: st.Description, CategoryID: domain.CategoryID(st.Category),
		Notes: st.Notes, Status: domain.TxStatus(st.Status), Kind: domain.TxKind(st.Kind),
		ExternalID: st.ExternalID, Fingerprint: st.Fingerprint, ImportBatchID: domain.ImportBatchID(st.Batch),
		TransferGroupID: domain.TransferGroupID(st.Group), RecurrenceID: domain.RecurrenceID(st.Recurrence),
	}
	for _, f := range []struct {
		dst *domain.Date
		src string
	}{{&t.Date, st.Date}, {&t.OperationDate, st.OperationDate}, {&t.ValueDate, st.ValueDate}, {&t.DueDate, st.DueDate}} {
		if *f.dst, err = parseOptDate(f.src); err != nil {
			return domain.Transaction{}, err
		}
	}
	return t, nil
}

func (st txState) splits() ([]domain.Split, error) {
	cur, err := domain.ParseCurrency(st.Currency)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Split, 0, len(st.Splits))
	for _, sp := range st.Splits {
		m, err := domain.NewMoney(sp.Minor, cur)
		if err != nil {
			return nil, err
		}
		out = append(out, domain.Split{Category: domain.CategoryID(sp.Category), Amount: m, Note: sp.Note})
	}
	return out, nil
}

func (st txState) tagIDs() []domain.TagID {
	out := make([]domain.TagID, len(st.Tags))
	for i, id := range st.Tags {
		out[i] = domain.TagID(id)
	}
	return out
}

// journal appends the entry for a write that just happened inside the same transaction.
func (s *Service) journal(ctx context.Context, r Repos, kind string, p undoPayload) error {
	b, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("journal %s: %w", kind, err)
	}
	if _, err := r.OpLog().Append(ctx, OpEntry{Kind: kind, Payload: string(b), CreatedAt: s.clock.Now()}); err != nil {
		return fmt.Errorf("journal %s: %w", kind, err)
	}
	return nil
}

func findState(states []txState, id int64) (txState, bool) {
	for _, st := range states {
		if st.ID == id {
			return st, true
		}
	}
	return txState{}, false
}

// UndoPlan is the plan to revert Entry, the most recent operation not yet undone.
type UndoPlan struct {
	Plan
	Entry OpEntry
	// Warnings lists references cleared because the row they pointed to no longer exists.
	Warnings []string
}

func undoStale(id int64, why string) error {
	return newError(domain.ErrInvalidValue, map[string]any{"id": id, "reason": "stale"},
		"cannot undo: transaction %d %s since the operation", id, why)
}

// checkUndo refuses a revert when the rows it touches are reconciled or no longer look as the
// operation left them.
func checkUndo(ctx context.Context, r Repos, p undoPayload) error {
	for _, want := range p.After {
		cur, err := snapshot(ctx, r, domain.TransactionID(want.ID))
		if errors.Is(err, domain.ErrNotFound) {
			return undoStale(want.ID, "was removed")
		}
		if err != nil {
			return err
		}
		if cur[0].Status == string(domain.StatusReconciled) {
			return newError(domain.ErrReconciled, map[string]any{"id": want.ID},
				"transaction %d is reconciled: cannot undo", want.ID)
		}
		if !sameState(cur[0], want, p.Mask) {
			return undoStale(want.ID, "changed")
		}
	}
	for _, old := range p.Before {
		if _, ok := findState(p.After, old.ID); ok {
			continue
		}
		if _, err := r.Transactions().Get(ctx, domain.TransactionID(old.ID)); err == nil {
			return undoStale(old.ID, "reappeared")
		} else if !errors.Is(err, domain.ErrNotFound) {
			return fmt.Errorf("get transaction %d: %w", old.ID, err)
		}
	}
	return nil
}

func hasField(mask []string, f UndoField) bool {
	return len(mask) == 0 || slices.Contains(mask, string(f))
}

// sameState compares a and b on the masked fields only; an empty mask compares everything.
func sameState(a, b txState, mask []string) bool {
	if len(mask) == 0 {
		return reflect.DeepEqual(a, b)
	}
	return (!hasField(mask, UndoFieldCategory) || a.Category == b.Category) &&
		(!hasField(mask, UndoFieldTags) || slices.Equal(a.Tags, b.Tags)) &&
		(!hasField(mask, UndoFieldStatus) || a.Status == b.Status) &&
		(!hasField(mask, UndoFieldPayee) || a.Payee == b.Payee)
}

// overlay copies the masked fields of src onto dst.
func overlay(dst *txState, src txState, mask []string) {
	if hasField(mask, UndoFieldCategory) {
		dst.Category = src.Category
	}
	if hasField(mask, UndoFieldTags) {
		dst.Tags = src.Tags
	}
	if hasField(mask, UndoFieldStatus) {
		dst.Status = src.Status
	}
	if hasField(mask, UndoFieldPayee) {
		dst.Payee = src.Payee
	}
}

// restoreStates returns the states revert writes back, with references to rows that no longer
// exist cleared, and a warning for each cleared reference.
func restoreStates(ctx context.Context, r Repos, p undoPayload) ([]txState, []string, error) {
	var out []txState
	var warns []string
	for _, old := range p.Before {
		st := old
		if _, ok := findState(p.After, old.ID); ok && len(p.Mask) > 0 {
			cur, err := snapshot(ctx, r, domain.TransactionID(old.ID))
			if err != nil {
				return nil, nil, err
			}
			st = cur[0]
			overlay(&st, old, p.Mask)
		}
		checks := []struct {
			what string
			ref  *int64
			get  func() error
		}{
			{"payee", &st.Payee, func() error { _, err := r.Payees().Get(ctx, domain.PayeeID(st.Payee)); return err }},
			{"import batch", &st.Batch, func() error { _, err := r.Imports().Batch(ctx, domain.ImportBatchID(st.Batch)); return err }},
			{"recurrence", &st.Recurrence, func() error {
				_, err := r.Recurrences().Get(ctx, RecurrenceID(st.Recurrence))
				return err
			}},
		}
		for _, c := range checks {
			if *c.ref == 0 {
				continue
			}
			err := c.get()
			if errors.Is(err, domain.ErrNotFound) {
				warns = append(warns, fmt.Sprintf("transaction %d: %s %d no longer exists, reference cleared", st.ID, c.what, *c.ref))
				*c.ref = 0
			} else if err != nil {
				return nil, nil, fmt.Errorf("check %s %d: %w", c.what, *c.ref, err)
			}
		}
		out = append(out, st)
	}
	return out, warns, nil
}

func decodeUndo(e OpEntry) (undoPayload, error) {
	var p undoPayload
	if err := json.Unmarshal([]byte(e.Payload), &p); err != nil {
		return p, fmt.Errorf("decode operation %d: %w", e.ID, err)
	}
	return p, nil
}

// PlanUndo describes what undoing the latest journal entry changes, without writing. It fails with
// ErrNotFound when there is nothing to undo, ErrReconciled when a touched transaction is
// reconciled, and invalid_value (reason "stale") when a touched transaction changed afterwards.
func (s *Service) PlanUndo(ctx context.Context) (UndoPlan, error) {
	var out UndoPlan
	err := s.store.Read(ctx, func(r Repos) error {
		e, err := r.OpLog().Latest(ctx)
		if err != nil {
			return fmt.Errorf("nothing to undo: %w", err)
		}
		p, err := decodeUndo(e)
		if err != nil {
			return err
		}
		if err := checkUndo(ctx, r, p); err != nil {
			return err
		}
		_, warns, err := restoreStates(ctx, r, p)
		if err != nil {
			return err
		}
		out = UndoPlan{Entry: e, Warnings: warns}
		for _, st := range p.After {
			op := OpDelete
			if _, ok := findState(p.Before, st.ID); ok {
				op = OpUpdate
			}
			out.Changes = append(out.Changes, undoChange(op, e, st))
		}
		for _, st := range p.Before {
			if _, ok := findState(p.After, st.ID); !ok {
				out.Changes = append(out.Changes, undoChange(OpCreate, e, st))
			}
		}
		return nil
	})
	return out, err
}

func undoChange(op Op, e OpEntry, st txState) Change {
	return Change{
		Op: op, Entity: EntityTransaction,
		Summary: fmt.Sprintf("undo %s: %s transaction %d", e.Kind, op, st.ID),
		Fields:  map[string]any{"op_id": e.ID, "kind": e.Kind, "id": st.ID},
	}
}

// ApplyUndo reverts the entry of the plan and marks it undone, in one transaction. It fails if the
// entry is no longer the latest one or the checks of PlanUndo no longer hold.
func (s *Service) ApplyUndo(ctx context.Context, plan UndoPlan) error {
	return s.apply(ctx, plan.Bulk, func(r Repos) error {
		e, err := r.OpLog().Latest(ctx)
		if err != nil {
			return fmt.Errorf("nothing to undo: %w", err)
		}
		if e.ID != plan.Entry.ID {
			return newError(domain.ErrInvalidValue, map[string]any{"reason": "stale"}, "cannot undo: the journal changed since the plan")
		}
		p, err := decodeUndo(e)
		if err != nil {
			return err
		}
		if err := checkUndo(ctx, r, p); err != nil {
			return err
		}
		states, _, err := restoreStates(ctx, r, p)
		if err != nil {
			return err
		}
		if err := revert(ctx, r, p, states); err != nil {
			return fmt.Errorf("undo %s: %w", e.Kind, err)
		}
		return r.OpLog().MarkUndone(ctx, e.ID, s.clock.Now())
	})
}

func revert(ctx context.Context, r Repos, p undoPayload, states []txState) error {
	for _, st := range p.After {
		if _, ok := findState(p.Before, st.ID); ok {
			continue
		}
		if err := r.Transactions().Delete(ctx, domain.TransactionID(st.ID)); err != nil {
			return fmt.Errorf("delete transaction %d: %w", st.ID, err)
		}
	}
	if p.Group != 0 && len(p.Before) == 0 {
		if err := r.Transfers().Delete(ctx, domain.TransferGroupID(p.Group)); err != nil {
			return fmt.Errorf("delete transfer %d: %w", p.Group, err)
		}
	}
	for _, g := range p.NewGroups {
		if err := r.Transfers().Delete(ctx, domain.TransferGroupID(g)); err != nil {
			return fmt.Errorf("delete transfer %d: %w", g, err)
		}
	}
	if p.Group != 0 && len(p.After) == 0 {
		if err := restoreGroup(ctx, r, p); err != nil {
			return err
		}
	}
	for _, st := range states {
		if err := restoreTx(ctx, r, st, p.After); err != nil {
			return err
		}
	}
	return dropCreatedPayee(ctx, r, p.CreatedPayee)
}

// dropCreatedPayee deletes the payee a reverted write created; one still used by other rows stays.
func dropCreatedPayee(ctx context.Context, r Repos, id int64) error {
	if id == 0 {
		return nil
	}
	err := r.Payees().Delete(ctx, domain.PayeeID(id))
	if err != nil && !errors.Is(err, ErrInUse) && !errors.Is(err, domain.ErrNotFound) {
		return fmt.Errorf("delete payee %d: %w", id, err)
	}
	return nil
}

func restoreTx(ctx context.Context, r Repos, st txState, after []txState) error {
	t, err := st.transaction()
	if err != nil {
		return fmt.Errorf("transaction %d: %w", st.ID, err)
	}
	rows, err := st.splits()
	if err != nil {
		return fmt.Errorf("transaction %d splits: %w", st.ID, err)
	}
	if _, ok := findState(after, st.ID); ok {
		// Clear the splits first: they are validated against the amount the update restores.
		if err := r.Splits().ReplaceSplits(ctx, t.ID, nil); err != nil {
			return fmt.Errorf("clear splits of %d: %w", st.ID, err)
		}
		if err := r.Transactions().Update(ctx, t); err != nil {
			return fmt.Errorf("restore transaction %d: %w", st.ID, err)
		}
	} else if err := r.OpLog().RestoreTransaction(ctx, t); err != nil {
		return fmt.Errorf("restore transaction %d: %w", st.ID, err)
	}
	if err := r.Tags().SetTags(ctx, t.ID, st.tagIDs()); err != nil {
		return fmt.Errorf("restore tags of %d: %w", st.ID, err)
	}
	if err := r.Splits().ReplaceSplits(ctx, t.ID, rows); err != nil {
		return fmt.Errorf("restore splits of %d: %w", st.ID, err)
	}
	return nil
}

// Journal records a write made by code outside the transaction add/edit/rm/transfer/split use
// cases, such as categorize, tag or rules: BeginJournal before the write, Commit after it.
type Journal struct {
	ids    []domain.TransactionID
	before []txState
	groups []int64
}

// BeginJournal snapshots the state of those ids that exist, before the write.
func BeginJournal(ctx context.Context, r Repos, ids ...domain.TransactionID) (*Journal, error) {
	j := &Journal{}
	for _, id := range ids {
		if slices.Contains(j.ids, id) {
			continue
		}
		st, err := snapshot(ctx, r, id)
		if errors.Is(err, domain.ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		j.ids = append(j.ids, id)
		j.before = append(j.before, st...)
	}
	return j, nil
}

// Track adds rows created by the write: they appear only in the After state and are deleted on undo.
func (j *Journal) Track(ids ...domain.TransactionID) {
	for _, id := range ids {
		if !slices.Contains(j.ids, id) {
			j.ids = append(j.ids, id)
		}
	}
}

// TrackGroup registers a transfer group created by the write; undo deletes it.
func (j *Journal) TrackGroup(g domain.TransferGroupID) {
	j.groups = append(j.groups, int64(g))
}

// Commit snapshots the After state of the begun and tracked rows and appends the journal entry. A
// non-empty mask restricts the entry to those fields.
func (j *Journal) Commit(ctx context.Context, r Repos, clock Clock, kind string, mask ...UndoField) error {
	p := undoPayload{Before: j.before, NewGroups: j.groups}
	for _, f := range mask {
		p.Mask = append(p.Mask, string(f))
	}
	for _, id := range j.ids {
		st, err := snapshot(ctx, r, id)
		if errors.Is(err, domain.ErrNotFound) {
			continue
		}
		if err != nil {
			return err
		}
		p.After = append(p.After, st...)
	}
	b, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("journal %s: %w", kind, err)
	}
	if _, err := r.OpLog().Append(ctx, OpEntry{Kind: kind, Payload: string(b), CreatedAt: clock.Now()}); err != nil {
		return fmt.Errorf("journal %s: %w", kind, err)
	}
	return nil
}

func restoreGroup(ctx context.Context, r Repos, p undoPayload) error {
	var fee domain.Money
	if p.FeeCurrency != "" {
		cur, err := domain.ParseCurrency(p.FeeCurrency)
		if err != nil {
			return err
		}
		if fee, err = domain.NewMoney(p.FeeMinor, cur); err != nil {
			return err
		}
	}
	if err := r.OpLog().RestoreTransferGroup(ctx, domain.TransferGroupID(p.Group), fee); err != nil {
		return fmt.Errorf("restore transfer %d: %w", p.Group, err)
	}
	return nil
}
