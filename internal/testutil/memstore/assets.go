package memstore

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/0xballo/fui/internal/domain"
)

type valuationKey struct {
	assetID domain.AssetID
	date    domain.Date
}

type assets struct{ *repos }

func (a assets) check(asset domain.Asset) error {
	for _, o := range a.st.Assets {
		if o.ID != asset.ID && strings.EqualFold(o.Name, asset.Name) {
			return exists("asset", asset.Name)
		}
	}
	if asset.LoanID != 0 {
		if _, ok := a.st.Loans[asset.LoanID]; !ok {
			return notFound("asset loan", asset.LoanID)
		}
	}
	return nil
}

func (a assets) Insert(_ context.Context, asset domain.Asset) (domain.Asset, error) {
	if err := a.writable(); err != nil {
		return domain.Asset{}, err
	}
	norm, err := domain.NewAsset(asset)
	if err != nil {
		return domain.Asset{}, err
	}
	if err := a.check(norm); err != nil {
		return domain.Asset{}, err
	}
	norm.ID = domain.AssetID(a.st.nextID("assets"))
	a.st.Assets[norm.ID] = norm
	return norm, nil
}

func (a assets) Update(_ context.Context, asset domain.Asset) error {
	if err := a.writable(); err != nil {
		return err
	}
	norm, err := domain.NewAsset(asset)
	if err != nil {
		return err
	}
	if _, ok := a.st.Assets[norm.ID]; !ok {
		return notFound("asset", norm.ID)
	}
	if err := a.check(norm); err != nil {
		return err
	}
	a.st.Assets[norm.ID] = norm
	return nil
}

func (a assets) Delete(_ context.Context, id domain.AssetID) error {
	if err := a.writable(); err != nil {
		return err
	}
	if _, ok := a.st.Assets[id]; !ok {
		return notFound("asset", id)
	}
	delete(a.st.Assets, id)
	for k := range a.st.Valuations {
		if k.assetID == id {
			delete(a.st.Valuations, k)
		}
	}
	return nil
}

func (a assets) Get(_ context.Context, id domain.AssetID) (domain.Asset, error) {
	asset, ok := a.st.Assets[id]
	if !ok {
		return domain.Asset{}, notFound("asset", id)
	}
	return asset, nil
}

func (a assets) List(context.Context) ([]domain.Asset, error) {
	return sortedValues(a.st.Assets), nil
}

func (a assets) SetValuation(_ context.Context, v domain.Valuation) error {
	if err := a.writable(); err != nil {
		return err
	}
	norm, err := domain.NewValuation(v)
	if err != nil {
		return err
	}
	asset, ok := a.st.Assets[norm.AssetID]
	if !ok {
		return notFound("asset", norm.AssetID)
	}
	if asset.PurchaseValue.Currency() != norm.Value.Currency() {
		return fmt.Errorf("asset %d valuation in %s, asset in %s: %w", asset.ID,
			norm.Value.Currency().Code(), asset.PurchaseValue.Currency().Code(), domain.ErrCurrencyMismatch)
	}
	a.st.Valuations[valuationKey{norm.AssetID, norm.Date}] = norm
	return nil
}

func (a assets) ListValuations(_ context.Context, assetID domain.AssetID) ([]domain.Valuation, error) {
	out := []domain.Valuation{}
	for k, v := range a.st.Valuations {
		if k.assetID == assetID {
			out = append(out, v)
		}
	}
	slices.SortFunc(out, func(x, y domain.Valuation) int { return x.Date.Cmp(y.Date) })
	return out, nil
}

func (a assets) DeleteValuation(_ context.Context, assetID domain.AssetID, date domain.Date) error {
	if err := a.writable(); err != nil {
		return err
	}
	k := valuationKey{assetID, date}
	if _, ok := a.st.Valuations[k]; !ok {
		return notFound("asset valuation", fmt.Sprintf("%d on %s", assetID, date))
	}
	delete(a.st.Valuations, k)
	return nil
}
