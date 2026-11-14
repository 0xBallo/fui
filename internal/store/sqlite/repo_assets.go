package sqlite

import (
	"context"
	"fmt"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	gen "github.com/0xballo/fui/internal/store/sqlite/sqlc"
)

var _ app.AssetRepo = assetRepo{}

type assetRepo struct {
	repos *repos
}

// Assets returns the asset repository. It is not part of app.Repos until the asset use cases land.
func (r *repos) Assets() app.AssetRepo { return assetRepo{repos: r} }

func (r assetRepo) Insert(ctx context.Context, a domain.Asset) (domain.Asset, error) {
	if err := r.repos.writable(); err != nil {
		return domain.Asset{}, err
	}
	norm, err := domain.NewAsset(a)
	if err != nil {
		return domain.Asset{}, err
	}
	if err := r.checkLoan(ctx, norm); err != nil {
		return domain.Asset{}, err
	}
	row, err := r.repos.queries.InsertAsset(ctx, gen.InsertAssetParams{
		Name:          norm.Name,
		Kind:          string(norm.Kind),
		PurchaseDate:  norm.PurchaseDate.String(),
		PurchaseMinor: norm.PurchaseValue.Minor(),
		Currency:      norm.PurchaseValue.Currency().Code(),
		LoanID:        toNullInt64(norm.LoanID),
	})
	if err != nil {
		return domain.Asset{}, mapConstraint(err)
	}
	return toAsset(row)
}

func (r assetRepo) Update(ctx context.Context, a domain.Asset) error {
	if err := r.repos.writable(); err != nil {
		return err
	}
	norm, err := domain.NewAsset(a)
	if err != nil {
		return err
	}
	if err := r.checkLoan(ctx, norm); err != nil {
		return err
	}
	changed, err := r.repos.queries.UpdateAsset(ctx, gen.UpdateAssetParams{
		ID:            int64(norm.ID),
		Name:          norm.Name,
		Kind:          string(norm.Kind),
		PurchaseDate:  norm.PurchaseDate.String(),
		PurchaseMinor: norm.PurchaseValue.Minor(),
		Currency:      norm.PurchaseValue.Currency().Code(),
		LoanID:        toNullInt64(norm.LoanID),
	})
	if err != nil {
		return mapConstraint(err)
	}
	if changed == 0 {
		return fmt.Errorf("asset %d: %w", norm.ID, domain.ErrNotFound)
	}
	return nil
}

// checkLoan reports a missing linked loan as ErrNotFound rather than a generic FK failure.
func (r assetRepo) checkLoan(ctx context.Context, a domain.Asset) error {
	if a.LoanID == 0 {
		return nil
	}
	if _, err := r.repos.queries.GetLoan(ctx, int64(a.LoanID)); err != nil {
		return mapNotFound(mapSQLError(err), "asset loan")
	}
	return nil
}

func (r assetRepo) Delete(ctx context.Context, id domain.AssetID) error {
	if err := r.repos.writable(); err != nil {
		return err
	}
	changed, err := r.repos.queries.DeleteAsset(ctx, int64(id))
	if err != nil {
		return mapSQLError(err)
	}
	if changed == 0 {
		return fmt.Errorf("asset %d: %w", id, domain.ErrNotFound)
	}
	return nil
}

func (r assetRepo) Get(ctx context.Context, id domain.AssetID) (domain.Asset, error) {
	row, err := r.repos.queries.GetAsset(ctx, int64(id))
	if err != nil {
		return domain.Asset{}, mapNotFound(mapSQLError(err), "get asset")
	}
	return toAsset(row)
}

func (r assetRepo) List(ctx context.Context) ([]domain.Asset, error) {
	rows, err := r.repos.queries.ListAssets(ctx)
	if err != nil {
		return nil, mapSQLError(err)
	}
	out := make([]domain.Asset, 0, len(rows))
	for _, row := range rows {
		a, convErr := toAsset(row)
		if convErr != nil {
			return nil, convErr
		}
		out = append(out, a)
	}
	return out, nil
}

func (r assetRepo) SetValuation(ctx context.Context, v domain.Valuation) error {
	if err := r.repos.writable(); err != nil {
		return err
	}
	norm, err := domain.NewValuation(v)
	if err != nil {
		return err
	}
	asset, err := r.Get(ctx, norm.AssetID)
	if err != nil {
		return err
	}
	if asset.PurchaseValue.Currency() != norm.Value.Currency() {
		return fmt.Errorf("asset %d valuation in %s, asset in %s: %w", asset.ID,
			norm.Value.Currency().Code(), asset.PurchaseValue.Currency().Code(), domain.ErrCurrencyMismatch)
	}
	err = r.repos.queries.UpsertAssetValuation(ctx, gen.UpsertAssetValuationParams{
		AssetID:    int64(norm.AssetID),
		Date:       norm.Date.String(),
		ValueMinor: norm.Value.Minor(),
		Currency:   norm.Value.Currency().Code(),
	})
	return mapConstraint(err)
}

func (r assetRepo) ListValuations(ctx context.Context, assetID domain.AssetID) ([]domain.Valuation, error) {
	rows, err := r.repos.queries.ListAssetValuations(ctx, int64(assetID))
	if err != nil {
		return nil, mapSQLError(err)
	}
	out := make([]domain.Valuation, 0, len(rows))
	for _, row := range rows {
		v, convErr := toValuation(row)
		if convErr != nil {
			return nil, convErr
		}
		out = append(out, v)
	}
	return out, nil
}

func (r assetRepo) DeleteValuation(ctx context.Context, assetID domain.AssetID, date domain.Date) error {
	if err := r.repos.writable(); err != nil {
		return err
	}
	changed, err := r.repos.queries.DeleteAssetValuation(ctx, gen.DeleteAssetValuationParams{
		AssetID: int64(assetID), Date: date.String(),
	})
	if err != nil {
		return mapSQLError(err)
	}
	if changed == 0 {
		return fmt.Errorf("asset %d valuation on %s: %w", assetID, date, domain.ErrNotFound)
	}
	return nil
}

func toAsset(row gen.Asset) (domain.Asset, error) {
	cur, err := domain.ParseCurrency(row.Currency)
	if err != nil {
		return domain.Asset{}, fmt.Errorf("parse asset currency %q: %w", row.Currency, err)
	}
	value, err := domain.NewMoney(row.PurchaseMinor, cur)
	if err != nil {
		return domain.Asset{}, fmt.Errorf("parse asset purchase value: %w", err)
	}
	date, err := domain.ParseDate(row.PurchaseDate)
	if err != nil {
		return domain.Asset{}, fmt.Errorf("parse asset purchase_date %q: %w", row.PurchaseDate, err)
	}
	return domain.NewAsset(domain.Asset{
		ID:            domain.AssetID(row.ID),
		Name:          row.Name,
		Kind:          domain.AssetKind(row.Kind),
		PurchaseDate:  date,
		PurchaseValue: value,
		LoanID:        domain.LoanID(nullInt64OrZero(row.LoanID)),
	})
}

func toValuation(row gen.AssetValuation) (domain.Valuation, error) {
	cur, err := domain.ParseCurrency(row.Currency)
	if err != nil {
		return domain.Valuation{}, fmt.Errorf("parse valuation currency %q: %w", row.Currency, err)
	}
	value, err := domain.NewMoney(row.ValueMinor, cur)
	if err != nil {
		return domain.Valuation{}, fmt.Errorf("parse valuation value: %w", err)
	}
	date, err := domain.ParseDate(row.Date)
	if err != nil {
		return domain.Valuation{}, fmt.Errorf("parse valuation date %q: %w", row.Date, err)
	}
	return domain.NewValuation(domain.Valuation{AssetID: domain.AssetID(row.AssetID), Date: date, Value: value})
}
