package app

import (
	"context"
	"fmt"

	"github.com/0xballo/fui/internal/domain"
)

// SplitRowRequest is one user-typed split row.
type SplitRowRequest struct {
	Category string
	Amount   string
	Note     string
}

// SplitTxRequest divides transaction ID into Rows, each assigned its own category; the sum of
// Rows must equal the transaction's amount.
type SplitTxRequest struct {
	ID   domain.TransactionID
	Rows []SplitRowRequest
}

// SplitTxPlan is the plan to replace the splits of ID with Rows.
type SplitTxPlan struct {
	Plan
	ID   domain.TransactionID
	Rows []domain.Split
}

// PlanSplitTx resolves and validates the split, but does not store it.
func (s *Service) PlanSplitTx(ctx context.Context, req SplitTxRequest) (SplitTxPlan, error) {
	var out SplitTxPlan
	err := s.store.Read(ctx, func(r Repos) error {
		tx, err := r.Transactions().Get(ctx, req.ID)
		if err != nil {
			return fmt.Errorf("transaction %d: %w", req.ID, err)
		}
		_, cats, err := loadNames(ctx, r)
		if err != nil {
			return err
		}
		rows := make([]domain.Split, len(req.Rows))
		for i, rr := range req.Rows {
			c, err := pickCategory(cats, rr.Category)
			if err != nil {
				return err
			}
			amt, err := s.amount(rr.Amount, tx.Amount.Currency())
			if err != nil {
				return fmt.Errorf("splits[%d].amount: %w", i, err)
			}
			rows[i] = domain.Split{Category: c.ID, Amount: amt, Note: rr.Note}
		}
		if err := domain.ValidateSplits(tx.Amount, rows); err != nil {
			return err
		}
		out = SplitTxPlan{ID: req.ID, Rows: rows}
		out.Changes = append(out.Changes, Change{
			Op: OpUpdate, Entity: EntitySplit,
			Summary: fmt.Sprintf("split transaction %d into %d rows", req.ID, len(rows)),
			Fields:  map[string]any{"id": int64(req.ID), "rows": len(rows)},
		})
		return nil
	})
	return out, err
}

// ApplySplitTx stores the split rows.
func (s *Service) ApplySplitTx(ctx context.Context, p SplitTxPlan) error {
	return s.apply(ctx, p.Bulk, func(r Repos) error {
		before, err := snapshot(ctx, r, p.ID)
		if err != nil {
			return err
		}
		if err := r.Splits().ReplaceSplits(ctx, p.ID, p.Rows); err != nil {
			return fmt.Errorf("split transaction %d: %w", p.ID, err)
		}
		after, err := snapshot(ctx, r, p.ID)
		if err != nil {
			return err
		}
		return s.journal(ctx, r, OpKindSplit, undoPayload{Before: before, After: after})
	})
}

// UnsplitTxRequest removes the split rows of ID, leaving it with a single category again.
type UnsplitTxRequest struct {
	ID domain.TransactionID
}

// PlanUnsplitTx plans clearing the split rows of ID.
func (s *Service) PlanUnsplitTx(ctx context.Context, req UnsplitTxRequest) (SplitTxPlan, error) {
	var out SplitTxPlan
	err := s.store.Read(ctx, func(r Repos) error {
		if _, err := r.Transactions().Get(ctx, req.ID); err != nil {
			return fmt.Errorf("transaction %d: %w", req.ID, err)
		}
		out = SplitTxPlan{ID: req.ID}
		out.Changes = append(out.Changes, Change{
			Op: OpUpdate, Entity: EntitySplit,
			Summary: fmt.Sprintf("unsplit transaction %d", req.ID),
			Fields:  map[string]any{"id": int64(req.ID)},
		})
		return nil
	})
	return out, err
}

// ApplyUnsplitTx applies a plan made by PlanUnsplitTx.
func (s *Service) ApplyUnsplitTx(ctx context.Context, p SplitTxPlan) error {
	return s.ApplySplitTx(ctx, p)
}

// ListSplits returns the split rows of id, in insertion order; none when the transaction is not split.
func (s *Service) ListSplits(ctx context.Context, id domain.TransactionID) ([]domain.Split, error) {
	var out []domain.Split
	err := s.store.Read(ctx, func(r Repos) (err error) {
		out, err = r.Splits().SplitsOf(ctx, id)
		return err
	})
	return out, err
}
