package assetting

import (
	"context"
	"fmt"
	"strings"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
)

// EditRequest changes the non-nil fields of the asset named Ref. Currency can only change while
// the asset has no valuations; PurchaseValue is parsed in the (possibly new) asset currency.
type EditRequest struct {
	Ref           string
	Name          *string
	Kind          *string
	PurchaseDate  *string
	PurchaseValue *string
	Currency      *string
}

// EditPlan is the plan to replace Before with After.
type EditPlan struct {
	app.Plan
	Before domain.Asset
	After  domain.Asset
}

func (s *Service) pickAssetIn(ctx context.Context, r app.Repos, ref string) (domain.Asset, error) {
	assets, err := r.Assets().List(ctx)
	if err != nil {
		return domain.Asset{}, fmt.Errorf("list assets: %w", err)
	}
	return pickAsset(assets, ref)
}

// PlanEdit validates the edit without writing.
func (s *Service) PlanEdit(ctx context.Context, enabled module.Set, req EditRequest) (EditPlan, error) {
	if err := require(enabled); err != nil {
		return EditPlan{}, err
	}
	var out EditPlan
	err := s.store.Read(ctx, func(r app.Repos) error {
		before, err := s.pickAssetIn(ctx, r, req.Ref)
		if err != nil {
			return err
		}
		after, err := s.edited(ctx, r, before, req)
		if err != nil {
			return err
		}
		f := map[string]any{"name": after.Name, "kind": string(after.Kind), "purchase_date": after.PurchaseDate.String()}
		moneyFields(f, "purchase_value", after.PurchaseValue)
		out = EditPlan{
			Plan: app.Plan{Changes: []app.Change{{
				Op: app.OpUpdate, Entity: EntityAsset, Fields: f,
				Summary: fmt.Sprintf("edit asset %q", before.Name),
			}}},
			Before: before, After: after,
		}
		return nil
	})
	return out, err
}

func (s *Service) edited(ctx context.Context, r app.Repos, a domain.Asset, req EditRequest) (domain.Asset, error) {
	if req.Name == nil && req.Kind == nil && req.PurchaseDate == nil && req.PurchaseValue == nil && req.Currency == nil {
		return domain.Asset{}, &domain.Error{Code: domain.CodeInvalidValue, Message: "nothing to change"}
	}
	a, err := s.editedFields(a, req)
	if err != nil {
		return domain.Asset{}, err
	}
	if a, err = s.editedValue(ctx, r, a, req); err != nil {
		return domain.Asset{}, err
	}
	if a, err = domain.NewAsset(a); err != nil {
		return domain.Asset{}, err
	}
	existing, err := r.Assets().List(ctx)
	if err != nil {
		return domain.Asset{}, fmt.Errorf("list assets: %w", err)
	}
	for _, o := range existing {
		if o.ID != a.ID && strings.EqualFold(o.Name, a.Name) {
			return domain.Asset{}, &domain.Error{Code: app.CodeAlreadyExists, Message: fmt.Sprintf("asset %q already exists", a.Name)}
		}
	}
	return a, nil
}

func (s *Service) editedFields(a domain.Asset, req EditRequest) (domain.Asset, error) {
	if req.Name != nil {
		a.Name = *req.Name
	}
	if req.Kind != nil {
		k, err := domain.ParseAssetKind(*req.Kind)
		if err != nil {
			return domain.Asset{}, err
		}
		a.Kind = k
	}
	if req.PurchaseDate != nil {
		d, err := s.date(*req.PurchaseDate)
		if err != nil {
			return domain.Asset{}, err
		}
		a.PurchaseDate = d
	}
	return a, nil
}

// editedValue applies the currency and purchase value edits, which depend on each other.
func (s *Service) editedValue(ctx context.Context, r app.Repos, a domain.Asset, req EditRequest) (domain.Asset, error) {
	cur := a.PurchaseValue.Currency()
	if req.Currency != nil {
		c, err := domain.ParseCurrency(*req.Currency)
		if err != nil {
			return domain.Asset{}, err
		}
		if c != cur {
			if err := s.currencyChangeAllowed(ctx, r, a); err != nil {
				return domain.Asset{}, err
			}
			cur = c
		}
	}
	var (
		v   domain.Money
		err error
	)
	switch {
	case req.PurchaseValue != nil:
		v, err = s.amount(*req.PurchaseValue, cur)
	case cur != a.PurchaseValue.Currency():
		v, err = domain.NewMoney(a.PurchaseValue.Minor(), cur)
	default:
		return a, nil
	}
	if err != nil {
		return domain.Asset{}, err
	}
	a.PurchaseValue = v
	return a, nil
}

func (s *Service) currencyChangeAllowed(ctx context.Context, r app.Repos, a domain.Asset) error {
	vals, err := r.Assets().ListValuations(ctx, a.ID)
	if err != nil {
		return fmt.Errorf("list valuations: %w", err)
	}
	if len(vals) > 0 {
		return &domain.Error{Code: app.CodeInUse, Message: fmt.Sprintf("asset %q has valuations: its currency cannot change", a.Name)}
	}
	if a.LoanID != 0 {
		return fmt.Errorf("asset %q is linked to a loan: %w", a.Name, domain.ErrCurrencyMismatch)
	}
	return nil
}

// ApplyEdit stores the edited asset.
func (s *Service) ApplyEdit(ctx context.Context, p EditPlan) (domain.Asset, error) {
	err := s.store.WithTx(ctx, func(r app.Repos) error {
		if err := r.Assets().Update(ctx, p.After); err != nil {
			return fmt.Errorf("update asset: %w", err)
		}
		return nil
	})
	if err != nil {
		return domain.Asset{}, err
	}
	return p.After, nil
}

// RemovePlan is the plan to delete Asset and its Valuations (a Bulk plan).
type RemovePlan struct {
	app.Plan
	Asset      domain.Asset
	Valuations int
}

// PlanRemove resolves ref and counts the valuations that would be dropped.
func (s *Service) PlanRemove(ctx context.Context, enabled module.Set, ref string) (RemovePlan, error) {
	if err := require(enabled); err != nil {
		return RemovePlan{}, err
	}
	var out RemovePlan
	err := s.store.Read(ctx, func(r app.Repos) error {
		a, err := s.pickAssetIn(ctx, r, ref)
		if err != nil {
			return err
		}
		vals, err := r.Assets().ListValuations(ctx, a.ID)
		if err != nil {
			return fmt.Errorf("list valuations: %w", err)
		}
		out = RemovePlan{Asset: a, Valuations: len(vals), Plan: app.Plan{Bulk: true, Changes: []app.Change{{
			Op: app.OpDelete, Entity: EntityAsset,
			Fields:  map[string]any{"name": a.Name, "valuations": len(vals)},
			Summary: fmt.Sprintf("delete asset %q and its %d valuations", a.Name, len(vals)),
		}}}}
		return nil
	})
	return out, err
}

// ApplyRemove deletes the asset and its valuations after a backup.
func (s *Service) ApplyRemove(ctx context.Context, p RemovePlan) error {
	if _, err := s.backup.Backup(ctx, app.BackupReasonBulk); err != nil {
		return fmt.Errorf("backup before removing asset: %w", err)
	}
	return s.store.WithTx(ctx, func(r app.Repos) error {
		if err := r.Assets().Delete(ctx, p.Asset.ID); err != nil {
			return fmt.Errorf("delete asset: %w", err)
		}
		return nil
	})
}

// RemoveValuationPlan is the plan to delete the valuation of Asset on Date.
type RemoveValuationPlan struct {
	app.Plan
	Asset     domain.Asset
	Valuation domain.Valuation
}

// PlanRemoveValuation resolves the asset and checks a valuation exists on date (empty = today).
func (s *Service) PlanRemoveValuation(ctx context.Context, enabled module.Set, ref, date string) (RemoveValuationPlan, error) {
	if err := require(enabled); err != nil {
		return RemoveValuationPlan{}, err
	}
	when, err := s.date(date)
	if err != nil {
		return RemoveValuationPlan{}, err
	}
	var out RemoveValuationPlan
	err = s.store.Read(ctx, func(r app.Repos) error {
		a, err := s.pickAssetIn(ctx, r, ref)
		if err != nil {
			return err
		}
		vals, err := r.Assets().ListValuations(ctx, a.ID)
		if err != nil {
			return fmt.Errorf("list valuations: %w", err)
		}
		for _, v := range vals {
			if v.Date != when {
				continue
			}
			f := map[string]any{"asset": a.Name, "date": when.String()}
			moneyFields(f, "value", v.Value)
			out = RemoveValuationPlan{Asset: a, Valuation: v, Plan: app.Plan{Changes: []app.Change{{
				Op: app.OpDelete, Entity: EntityValuation, Fields: f,
				Summary: fmt.Sprintf("delete valuation of %q on %s (%s)", a.Name, when, v.Value),
			}}}}
			return nil
		}
		return &domain.Error{Code: domain.CodeNotFound, Message: fmt.Sprintf("asset %q has no valuation on %s", a.Name, when)}
	})
	return out, err
}

// ApplyRemoveValuation deletes the planned valuation.
func (s *Service) ApplyRemoveValuation(ctx context.Context, p RemoveValuationPlan) error {
	return s.store.WithTx(ctx, func(r app.Repos) error {
		if err := r.Assets().DeleteValuation(ctx, p.Asset.ID, p.Valuation.Date); err != nil {
			return fmt.Errorf("delete valuation: %w", err)
		}
		return nil
	})
}

// LinkPlan is the plan to link Asset to Loan.
type LinkPlan struct {
	app.Plan
	Asset domain.Asset
	Loan  domain.Loan
}

// PlanLink links an asset to the loan that financed it. A loan secures one asset only, so it is
// refused (CodeInUse) when another asset already uses it; the loan currency must match.
func (s *Service) PlanLink(ctx context.Context, enabled module.Set, assetRef, loanRef string) (LinkPlan, error) {
	if err := require(enabled); err != nil {
		return LinkPlan{}, err
	}
	var out LinkPlan
	err := s.store.Read(ctx, func(r app.Repos) error {
		assets, err := r.Assets().List(ctx)
		if err != nil {
			return fmt.Errorf("list assets: %w", err)
		}
		a, err := pickAsset(assets, assetRef)
		if err != nil {
			return err
		}
		loans, err := r.Loans().List(ctx)
		if err != nil {
			return fmt.Errorf("list loans: %w", err)
		}
		l, err := pickLoan(loans, loanRef)
		if err != nil {
			return err
		}
		for _, o := range assets {
			if o.ID != a.ID && o.LoanID == l.ID {
				return &domain.Error{Code: app.CodeInUse, Message: fmt.Sprintf("loan %q is already linked to asset %q", l.Name, o.Name)}
			}
		}
		if l.Principal.Currency() != a.PurchaseValue.Currency() {
			return fmt.Errorf("loan %q is in %s, asset %q in %s: %w",
				l.Name, l.Principal.Currency(), a.Name, a.PurchaseValue.Currency(), domain.ErrCurrencyMismatch)
		}
		out = LinkPlan{Asset: a, Loan: l, Plan: app.Plan{Changes: []app.Change{{
			Op: app.OpUpdate, Entity: EntityAsset,
			Fields:  map[string]any{"name": a.Name, "loan_id": int64(l.ID), "loan": l.Name},
			Summary: fmt.Sprintf("link asset %q to loan %q", a.Name, l.Name),
		}}}}
		return nil
	})
	return out, err
}

// ApplyLink stores the link.
func (s *Service) ApplyLink(ctx context.Context, p LinkPlan) error {
	a := p.Asset
	a.LoanID = p.Loan.ID
	return s.update(ctx, a)
}

// UnlinkPlan is the plan to remove the loan link of Asset.
type UnlinkPlan struct {
	app.Plan
	Asset domain.Asset
}

// PlanUnlink fails with a not-found error when the asset has no linked loan.
func (s *Service) PlanUnlink(ctx context.Context, enabled module.Set, assetRef string) (UnlinkPlan, error) {
	if err := require(enabled); err != nil {
		return UnlinkPlan{}, err
	}
	var out UnlinkPlan
	err := s.store.Read(ctx, func(r app.Repos) error {
		a, err := s.pickAssetIn(ctx, r, assetRef)
		if err != nil {
			return err
		}
		if a.LoanID == 0 {
			return &domain.Error{Code: domain.CodeNotFound, Message: fmt.Sprintf("asset %q has no linked loan", a.Name)}
		}
		out = UnlinkPlan{Asset: a, Plan: app.Plan{Changes: []app.Change{{
			Op: app.OpUpdate, Entity: EntityAsset,
			Fields:  map[string]any{"name": a.Name, "loan_id": int64(0)},
			Summary: fmt.Sprintf("unlink asset %q from its loan", a.Name),
		}}}}
		return nil
	})
	return out, err
}

// ApplyUnlink clears the link.
func (s *Service) ApplyUnlink(ctx context.Context, p UnlinkPlan) error {
	a := p.Asset
	a.LoanID = 0
	return s.update(ctx, a)
}

func (s *Service) update(ctx context.Context, a domain.Asset) error {
	return s.store.WithTx(ctx, func(r app.Repos) error {
		if err := r.Assets().Update(ctx, a); err != nil {
			return fmt.Errorf("update asset: %w", err)
		}
		return nil
	})
}
