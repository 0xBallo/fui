package importing

import (
	"context"
	"fmt"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

// UndoPlan is the plan to remove the transactions of one import batch and the batch itself.
// Accounts, categories and ref associations the import created are kept.
type UndoPlan struct {
	app.Plan
	Batch        app.ImportBatch
	Transactions []domain.TransactionID
}

// ListBatches returns every import batch, most recent first.
func (s *Service) ListBatches(ctx context.Context) ([]app.ImportBatch, error) {
	var out []app.ImportBatch
	err := s.store.Read(ctx, func(r app.Repos) error {
		var err error
		out, err = r.Imports().Batches(ctx)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("list import batches: %w", err)
	}
	return out, nil
}

// PlanUndo lists the transactions of the batch. It fails with domain.ErrReconciled when one of
// them is already reconciled.
func (s *Service) PlanUndo(ctx context.Context, batch domain.ImportBatchID) (UndoPlan, error) {
	var out UndoPlan
	err := s.store.Read(ctx, func(r app.Repos) error {
		b, err := r.Imports().Batch(ctx, batch)
		if err != nil {
			return fmt.Errorf("get import batch %d: %w", batch, err)
		}
		txs, err := batchTransactions(ctx, r, batch)
		if err != nil {
			return err
		}
		out = UndoPlan{Batch: b}
		for _, t := range txs {
			out.Transactions = append(out.Transactions, t.ID)
		}
		out.Changes = []app.Change{
			{
				Op: app.OpDelete, Entity: app.EntityTransaction, Fields: map[string]any{"count": len(txs), "import_batch": int64(batch)},
				Summary: fmt.Sprintf("delete %d transactions of import batch %d", len(txs), batch),
			},
			{
				Op: app.OpDelete, Entity: EntityImportBatch, Fields: map[string]any{"id": int64(batch), "source": b.Source},
				Summary: fmt.Sprintf("delete import batch %d (%s)", batch, b.Source),
			},
		}
		out.Bulk = true
		return nil
	})
	return out, err
}

// batchTransactions returns the transactions of the batch, refusing reconciled ones.
func batchTransactions(ctx context.Context, r app.Repos, batch domain.ImportBatchID) ([]domain.Transaction, error) {
	txs, err := r.Transactions().List(ctx, app.TxFilter{ImportBatchID: batch}, nil, 0)
	if err != nil {
		return nil, fmt.Errorf("list transactions of batch %d: %w", batch, err)
	}
	for _, t := range txs {
		if t.Status == domain.StatusReconciled {
			return nil, &domain.Error{
				Code: domain.CodeReconciled, Details: map[string]any{"transaction": int64(t.ID), "import_batch": int64(batch)},
				Message: fmt.Sprintf("import batch %d has the reconciled transaction %d", batch, t.ID),
			}
		}
	}
	return txs, nil
}

// ApplyUndo backs up the archive, then removes the transactions, their transfer groups and the
// batch in one transaction. A transfer leg outside the batch is left unlinked.
func (s *Service) ApplyUndo(ctx context.Context, p UndoPlan) error {
	if _, err := s.backup.Backup(ctx, app.BackupReasonBulk); err != nil {
		return fmt.Errorf("backup before undo: %w", err)
	}
	err := s.store.WithTx(ctx, func(r app.Repos) error {
		txs, err := batchTransactions(ctx, r, p.Batch.ID)
		if err != nil {
			return err
		}
		groups := map[domain.TransferGroupID]bool{}
		for _, t := range txs {
			if t.TransferGroupID != 0 {
				groups[t.TransferGroupID] = true
			}
			if err := r.Transactions().Delete(ctx, t.ID); err != nil {
				return fmt.Errorf("delete transaction %d: %w", t.ID, err)
			}
		}
		for g := range groups {
			if err := dropGroup(ctx, r, g); err != nil {
				return err
			}
		}
		if err := r.Imports().DeleteBatch(ctx, p.Batch.ID); err != nil {
			return fmt.Errorf("delete import batch %d: %w", p.Batch.ID, err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("apply undo: %w", err)
	}
	return nil
}

// dropGroup unlinks the legs that survive and deletes the group.
func dropGroup(ctx context.Context, r app.Repos, g domain.TransferGroupID) error {
	left, err := r.Transactions().ByTransferGroup(ctx, g)
	if err != nil {
		return fmt.Errorf("list legs of transfer %d: %w", g, err)
	}
	for _, t := range left {
		t.TransferGroupID = 0
		if err := r.Transactions().Update(ctx, t); err != nil {
			return fmt.Errorf("unlink transaction %d: %w", t.ID, err)
		}
	}
	if err := r.Transfers().Delete(ctx, g); err != nil {
		return fmt.Errorf("delete transfer %d: %w", g, err)
	}
	return nil
}
