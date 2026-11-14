package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
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
type undoPayload struct {
	Group       int64
	FeeMinor    int64
	FeeCurrency string
	Before      []txState
	After       []txState
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
		if !reflect.DeepEqual(cur[0], want) {
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
		out = UndoPlan{Entry: e}
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
		if err := revert(ctx, r, p); err != nil {
			return fmt.Errorf("undo %s: %w", e.Kind, err)
		}
		return r.OpLog().MarkUndone(ctx, e.ID, s.clock.Now())
	})
}

func revert(ctx context.Context, r Repos, p undoPayload) error {
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
	if p.Group != 0 && len(p.After) == 0 {
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
	}
	for _, st := range p.Before {
		if err := restoreTx(ctx, r, st, p.After); err != nil {
			return err
		}
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
