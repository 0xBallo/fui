package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	gen "github.com/0xballo/fui/internal/store/sqlite/sqlc"
)

type importRepo struct {
	repos *repos
}

func (r importRepo) InsertBatch(ctx context.Context, b app.ImportBatch) (app.ImportBatch, error) {
	if err := r.repos.writable(); err != nil {
		return app.ImportBatch{}, err
	}
	row, err := r.repos.queries.InsertImportBatch(ctx, gen.InsertImportBatchParams{
		Connector:    b.Connector,
		Source:       b.Source,
		SourceSha256: b.SourceSHA256,
		ImportedAt:   b.ImportedAt.UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		return app.ImportBatch{}, mapConstraint(err)
	}
	return toImportBatch(row)
}

func (r importRepo) Batch(ctx context.Context, id domain.ImportBatchID) (app.ImportBatch, error) {
	row, err := r.repos.queries.GetImportBatch(ctx, int64(id))
	if err != nil {
		return app.ImportBatch{}, mapNotFound(mapSQLError(err), "get import batch")
	}
	return toImportBatch(row)
}

func (r importRepo) BatchBySHA256(ctx context.Context, sha string) (app.ImportBatch, error) {
	row, err := r.repos.queries.ImportBatchBySHA256(ctx, sha)
	if err != nil {
		return app.ImportBatch{}, mapNotFound(mapSQLError(err), "get import batch by sha256")
	}
	return toImportBatch(row)
}

func (r importRepo) Batches(ctx context.Context) ([]app.ImportBatch, error) {
	rows, err := r.repos.queries.ListImportBatches(ctx)
	if err != nil {
		return nil, mapSQLError(err)
	}
	out := make([]app.ImportBatch, 0, len(rows))
	for _, row := range rows {
		batch, convErr := toImportBatch(row)
		if convErr != nil {
			return nil, convErr
		}
		out = append(out, batch)
	}
	return out, nil
}

func (r importRepo) DeleteBatch(ctx context.Context, id domain.ImportBatchID) error {
	if err := r.repos.writable(); err != nil {
		return err
	}
	changed, err := r.repos.queries.DeleteImportBatch(ctx, int64(id))
	if err != nil {
		return mapSQLError(err)
	}
	if changed == 0 {
		return fmt.Errorf("import batch %d: %w", id, domain.ErrNotFound)
	}
	return nil
}

func (r importRepo) AccountRef(ctx context.Context, connector, ref string) (domain.AccountID, error) {
	id, err := r.repos.queries.GetConnectorAccountRef(ctx, gen.GetConnectorAccountRefParams{
		Connector: connector,
		Ref:       ref,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return r.pendingAccountRef(ctx, connector, ref)
	}
	if err != nil {
		return 0, mapNotFound(mapSQLError(err), "get connector account ref")
	}
	return domain.AccountID(id), nil
}

// pendingAccountRef resolves an association kept by a reset: when an account named as the kept
// one exists it is bound for good (in a writable transaction) and the pending row is consumed.
func (r importRepo) pendingAccountRef(ctx context.Context, connector, ref string) (domain.AccountID, error) {
	id, err := r.repos.queries.GetPendingConnectorRefAccount(ctx, gen.GetPendingConnectorRefAccountParams{
		Connector: connector,
		Ref:       ref,
	})
	if err != nil {
		return 0, mapNotFound(mapSQLError(err), "get connector account ref")
	}
	if r.repos.writable() == nil {
		if err := r.SetAccountRef(ctx, connector, ref, domain.AccountID(id)); err != nil {
			return 0, err
		}
	}
	return domain.AccountID(id), nil
}

func (r importRepo) SetAccountRef(ctx context.Context, connector, ref string, id domain.AccountID) error {
	if err := r.repos.writable(); err != nil {
		return err
	}
	if _, err := r.repos.queries.GetAccount(ctx, int64(id)); err != nil {
		return mapNotFound(mapSQLError(err), "account")
	}
	if err := r.repos.queries.UpsertConnectorAccountRef(ctx, gen.UpsertConnectorAccountRefParams{
		Connector: connector,
		Ref:       ref,
		AccountID: int64(id),
	}); err != nil {
		return mapConstraint(err)
	}
	if err := r.repos.queries.DeletePendingConnectorRef(ctx, gen.DeletePendingConnectorRefParams{
		Connector: connector,
		Ref:       ref,
	}); err != nil {
		return mapSQLError(err)
	}
	return nil
}
