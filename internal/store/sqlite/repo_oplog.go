package sqlite

import (
	"context"
	"fmt"
	"time"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	gen "github.com/0xballo/fui/internal/store/sqlite/sqlc"
)

type opLogRepo struct {
	repos *repos
}

func (r opLogRepo) Append(ctx context.Context, e app.OpEntry) (app.OpEntry, error) {
	if err := r.repos.writable(); err != nil {
		return app.OpEntry{}, err
	}
	row, err := r.repos.queries.InsertOpLog(ctx, gen.InsertOpLogParams{
		Kind: e.Kind, Payload: e.Payload, CreatedAt: e.CreatedAt.UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		return app.OpEntry{}, mapSQLError(err)
	}
	return toOpEntry(row)
}

func (r opLogRepo) Latest(ctx context.Context) (app.OpEntry, error) {
	row, err := r.repos.queries.LatestOpLog(ctx)
	if err != nil {
		return app.OpEntry{}, mapNotFound(mapSQLError(err), "undoable operation")
	}
	return toOpEntry(row)
}

func (r opLogRepo) MarkUndone(ctx context.Context, id int64, at time.Time) error {
	if err := r.repos.writable(); err != nil {
		return err
	}
	n, err := r.repos.queries.MarkOpLogUndone(ctx, gen.MarkOpLogUndoneParams{
		ID: id, UndoneAt: toNullString(at.UTC().Format(time.RFC3339Nano)),
	})
	if err != nil {
		return mapSQLError(err)
	}
	if n == 0 {
		return fmt.Errorf("operation %d: %w", id, domain.ErrNotFound)
	}
	return nil
}

// RestoreTransaction inserts t and then moves it to its old ID: the row has no dependants yet.
func (r opLogRepo) RestoreTransaction(ctx context.Context, t domain.Transaction) error {
	ins, err := transactionRepo(r).Insert(ctx, t)
	if err != nil {
		return err
	}
	if _, err := r.repos.queries.SetTransactionID(ctx, gen.SetTransactionIDParams{NewID: int64(t.ID), OldID: int64(ins.ID)}); err != nil {
		return mapConstraint(err)
	}
	return nil
}

func (r opLogRepo) RestoreTransferGroup(ctx context.Context, id domain.TransferGroupID, fee domain.Money) error {
	ins, err := transferRepo(r).Insert(ctx, domain.TransferGroup{Fee: fee})
	if err != nil {
		return err
	}
	if _, err := r.repos.queries.SetTransferGroupID(ctx, gen.SetTransferGroupIDParams{NewID: int64(id), OldID: int64(ins.ID)}); err != nil {
		return mapConstraint(err)
	}
	return nil
}

func toOpEntry(row gen.OpLog) (app.OpEntry, error) {
	created, err := time.Parse(time.RFC3339Nano, row.CreatedAt)
	if err != nil {
		return app.OpEntry{}, fmt.Errorf("op_log created_at: %w", err)
	}
	e := app.OpEntry{ID: row.ID, Kind: row.Kind, Payload: row.Payload, CreatedAt: created}
	if row.UndoneAt.Valid {
		if e.UndoneAt, err = time.Parse(time.RFC3339Nano, row.UndoneAt.String); err != nil {
			return app.OpEntry{}, fmt.Errorf("op_log undone_at: %w", err)
		}
	}
	return e, nil
}
