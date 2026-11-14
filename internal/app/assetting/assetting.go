// Package assetting holds the use cases of manually valued assets (property, vehicles, other
// goods): adding an asset with an optional linked loan, recording dated valuations, listing assets
// with their latest value and showing a valuation history. Every use case requires the assets module.
package assetting

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
)

// Change entities of this package.
const (
	EntityAsset     = "asset"
	EntityValuation = "valuation"
)

// Config holds the user settings the use cases depend on.
type Config struct {
	Locale       domain.Locale  // number and date input format; default en-US
	Location     *time.Location // zone that defines "today"; default time.Local
	BaseCurrency domain.Currency
}

// Service runs the asset use cases over a Store.
type Service struct {
	store  app.Store
	clock  app.Clock
	backup app.Backuper
	cfg    Config
}

// New builds a Service, defaulting Locale and Location when unset.
func New(store app.Store, clock app.Clock, backup app.Backuper, cfg Config) *Service {
	if cfg.Locale == "" {
		cfg.Locale = domain.LocaleENUS
	}
	if cfg.Location == nil {
		cfg.Location = time.Local
	}
	return &Service{store: store, clock: clock, backup: backup, cfg: cfg}
}

func require(enabled module.Set) error {
	return app.WrapModuleError(enabled.Require(module.Assets))
}

func (s *Service) today() (domain.Date, error) {
	d, err := domain.FromTime(s.clock.Now().In(s.cfg.Location))
	if err != nil {
		return domain.Date{}, fmt.Errorf("today: %w", err)
	}
	return d, nil
}

// date parses a user-typed date; an empty input means today.
func (s *Service) date(in string) (domain.Date, error) {
	today, err := s.today()
	if err != nil || in == "" {
		return today, err
	}
	d, err := domain.ParseDateInput(in, s.cfg.Locale, today)
	if err != nil {
		return domain.Date{}, fmt.Errorf("date: %w", err)
	}
	return d, nil
}

func (s *Service) amount(in string, c domain.Currency) (domain.Money, error) {
	m, err := domain.ParseAmount(in, s.cfg.Locale, c)
	if err != nil {
		return domain.Money{}, fmt.Errorf("amount: %w", err)
	}
	return m, nil
}

func moneyFields(f map[string]any, prefix string, m domain.Money) {
	f[prefix] = m.Minor()
	f[prefix+"_currency"] = m.Currency().Code()
}

func pickAsset(assets []domain.Asset, ref string) (domain.Asset, error) {
	names := make([]string, len(assets))
	for i, a := range assets {
		names[i] = a.Name
	}
	i, err := domain.Resolve(ref, names)
	if err != nil {
		return domain.Asset{}, fmt.Errorf("asset: %w", err)
	}
	return assets[i], nil
}

func pickLoan(loans []domain.Loan, ref string) (domain.Loan, error) {
	names := make([]string, len(loans))
	for i, l := range loans {
		names[i] = l.Name
	}
	i, err := domain.Resolve(ref, names)
	if err != nil {
		return domain.Loan{}, fmt.Errorf("loan: %w", err)
	}
	return loans[i], nil
}

// AddRequest carries user-typed values. Name, Kind and Value (the purchase value) are required;
// PurchaseDate defaults to today. Currency defaults to the linked loan's currency, then to the
// configured base currency. Loan optionally names the loan that financed the asset.
type AddRequest struct {
	Name         string
	Kind         string
	Value        string
	PurchaseDate string
	Currency     string
	Loan         string
}

// AddPlan is the plan to store Asset (ID set on apply).
type AddPlan struct {
	app.Plan
	Asset domain.Asset
}

// PlanAdd validates the request, resolving the optional loan link, without writing.
func (s *Service) PlanAdd(ctx context.Context, enabled module.Set, req AddRequest) (AddPlan, error) {
	if err := require(enabled); err != nil {
		return AddPlan{}, err
	}
	var out AddPlan
	err := s.store.Read(ctx, func(r app.Repos) error {
		asset, err := s.newAsset(ctx, r, req)
		if err != nil {
			return err
		}
		f := map[string]any{
			"name": asset.Name, "kind": string(asset.Kind), "purchase_date": asset.PurchaseDate.String(),
		}
		moneyFields(f, "purchase_value", asset.PurchaseValue)
		if asset.LoanID != 0 {
			f["loan_id"] = int64(asset.LoanID)
		}
		out = AddPlan{
			Plan: app.Plan{Changes: []app.Change{{
				Op: app.OpCreate, Entity: EntityAsset, Fields: f,
				Summary: fmt.Sprintf("create %s asset %q valued %s", asset.Kind, asset.Name, asset.PurchaseValue),
			}}},
			Asset: asset,
		}
		return nil
	})
	return out, err
}

func (s *Service) newAsset(ctx context.Context, r app.Repos, req AddRequest) (domain.Asset, error) {
	kind, err := domain.ParseAssetKind(req.Kind)
	if err != nil {
		return domain.Asset{}, err
	}
	var loan domain.Loan
	if strings.TrimSpace(req.Loan) != "" {
		loans, err := r.Loans().List(ctx)
		if err != nil {
			return domain.Asset{}, fmt.Errorf("list loans: %w", err)
		}
		if loan, err = pickLoan(loans, req.Loan); err != nil {
			return domain.Asset{}, err
		}
	}
	cur, err := s.currency(req.Currency, loan)
	if err != nil {
		return domain.Asset{}, err
	}
	if loan.ID != 0 && loan.Principal.Currency() != cur {
		return domain.Asset{}, fmt.Errorf("loan %q is in %s, asset in %s: %w",
			loan.Name, loan.Principal.Currency(), cur, domain.ErrCurrencyMismatch)
	}
	value, err := s.amount(req.Value, cur)
	if err != nil {
		return domain.Asset{}, err
	}
	purchased, err := s.date(req.PurchaseDate)
	if err != nil {
		return domain.Asset{}, err
	}
	asset, err := domain.NewAsset(domain.Asset{
		Name: req.Name, Kind: kind, PurchaseDate: purchased, PurchaseValue: value, LoanID: loan.ID,
	})
	if err != nil {
		return domain.Asset{}, err
	}
	existing, err := r.Assets().List(ctx)
	if err != nil {
		return domain.Asset{}, fmt.Errorf("list assets: %w", err)
	}
	for _, a := range existing {
		if strings.EqualFold(a.Name, asset.Name) {
			return domain.Asset{}, &domain.Error{Code: app.CodeAlreadyExists, Message: fmt.Sprintf("asset %q already exists", asset.Name)}
		}
	}
	return asset, nil
}

func (s *Service) currency(code string, loan domain.Loan) (domain.Currency, error) {
	switch {
	case strings.TrimSpace(code) != "":
		return domain.ParseCurrency(code)
	case loan.ID != 0:
		return loan.Principal.Currency(), nil
	case !s.cfg.BaseCurrency.IsZero():
		return s.cfg.BaseCurrency, nil
	}
	return domain.Currency{}, &domain.Error{
		Code: domain.CodeInvalidValue, Message: "currency is required",
		Details: map[string]any{"field": "currency"},
	}
}

// ApplyAdd stores the planned asset and returns it with its ID.
func (s *Service) ApplyAdd(ctx context.Context, p AddPlan) (domain.Asset, error) {
	var out domain.Asset
	err := s.store.WithTx(ctx, func(r app.Repos) (err error) {
		out, err = r.Assets().Insert(ctx, p.Asset)
		if err != nil {
			return fmt.Errorf("create asset: %w", err)
		}
		return nil
	})
	return out, err
}

// ValueRequest records the value of Asset on Date (default today). Value is in the asset currency.
type ValueRequest struct {
	Asset string
	Date  string
	Value string
}

// ValuePlan is the plan to store Valuation. Overwrite is set when the asset already has a
// valuation on that date (Previous): the plan is then Bulk, so the UI asks for confirmation and
// Apply takes a backup before replacing it.
type ValuePlan struct {
	app.Plan
	Asset     domain.Asset
	Valuation domain.Valuation
	Overwrite bool
	Previous  domain.Money
}

// PlanValue validates the request without writing.
func (s *Service) PlanValue(ctx context.Context, enabled module.Set, req ValueRequest) (ValuePlan, error) {
	if err := require(enabled); err != nil {
		return ValuePlan{}, err
	}
	when, err := s.date(req.Date)
	if err != nil {
		return ValuePlan{}, err
	}
	var out ValuePlan
	err = s.store.Read(ctx, func(r app.Repos) error {
		assets, err := r.Assets().List(ctx)
		if err != nil {
			return fmt.Errorf("list assets: %w", err)
		}
		asset, err := pickAsset(assets, req.Asset)
		if err != nil {
			return err
		}
		value, err := s.amount(req.Value, asset.PurchaseValue.Currency())
		if err != nil {
			return err
		}
		val, err := domain.NewValuation(domain.Valuation{AssetID: asset.ID, Date: when, Value: value})
		if err != nil {
			return err
		}
		history, err := r.Assets().ListValuations(ctx, asset.ID)
		if err != nil {
			return fmt.Errorf("list valuations: %w", err)
		}
		out = ValuePlan{Asset: asset, Valuation: val}
		for _, h := range history {
			if h.Date == when {
				out.Overwrite, out.Previous = true, h.Value
			}
		}
		out.Plan = valuePlan(out)
		return nil
	})
	return out, err
}

func valuePlan(p ValuePlan) app.Plan {
	f := map[string]any{"asset": p.Asset.Name, "date": p.Valuation.Date.String(), "overwrite": p.Overwrite}
	moneyFields(f, "value", p.Valuation.Value)
	c := app.Change{Op: app.OpCreate, Entity: EntityValuation, Fields: f}
	c.Summary = fmt.Sprintf("value %q at %s on %s", p.Asset.Name, p.Valuation.Value, p.Valuation.Date)
	if p.Overwrite {
		moneyFields(f, "previous", p.Previous)
		c.Op = app.OpUpdate
		c.Summary = fmt.Sprintf("replace valuation of %q on %s: %s -> %s",
			p.Asset.Name, p.Valuation.Date, p.Previous, p.Valuation.Value)
	}
	return app.Plan{Changes: []app.Change{c}, Bulk: p.Overwrite}
}

// ApplyValue stores the valuation, replacing any on the same date (after a backup when it does).
func (s *Service) ApplyValue(ctx context.Context, p ValuePlan) error {
	if p.Overwrite {
		if _, err := s.backup.Backup(ctx, app.BackupReasonBulk); err != nil {
			return fmt.Errorf("backup before overwriting valuation: %w", err)
		}
	}
	err := s.store.WithTx(ctx, func(r app.Repos) error {
		if err := r.Assets().SetValuation(ctx, p.Valuation); err != nil {
			return fmt.Errorf("store valuation: %w", err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("apply: %w", err)
	}
	return nil
}

// Item is an asset with its latest valuation at the requested date. Debt is the residual debt of
// the linked loan (zero without a link) and Net is Latest minus Debt.
type Item struct {
	Asset  domain.Asset
	Latest domain.Valuation // falls back to the purchase date and value
	Debt   domain.Money
	Net    domain.Money
	At     domain.Date
}

// View is an Item with the full valuation history, oldest first.
type View struct {
	Item
	History []domain.Valuation
}

// List returns every asset with its latest value at at (empty = today).
func (s *Service) List(ctx context.Context, enabled module.Set, at string) ([]Item, error) {
	if err := require(enabled); err != nil {
		return nil, err
	}
	when, err := s.date(at)
	if err != nil {
		return nil, err
	}
	out := []Item{}
	err = s.store.Read(ctx, func(r app.Repos) error {
		assets, err := r.Assets().List(ctx)
		if err != nil {
			return fmt.Errorf("list assets: %w", err)
		}
		for _, a := range assets {
			it, _, err := item(ctx, r, a, when)
			if err != nil {
				return err
			}
			out = append(out, it)
		}
		return nil
	})
	return out, err
}

// Show returns asset ref with its valuation history and its value at at (empty = today).
func (s *Service) Show(ctx context.Context, enabled module.Set, ref, at string) (View, error) {
	if err := require(enabled); err != nil {
		return View{}, err
	}
	when, err := s.date(at)
	if err != nil {
		return View{}, err
	}
	var out View
	err = s.store.Read(ctx, func(r app.Repos) error {
		assets, err := r.Assets().List(ctx)
		if err != nil {
			return fmt.Errorf("list assets: %w", err)
		}
		a, err := pickAsset(assets, ref)
		if err != nil {
			return err
		}
		out.Item, out.History, err = item(ctx, r, a, when)
		return err
	})
	return out, err
}

func item(ctx context.Context, r app.Repos, a domain.Asset, at domain.Date) (Item, []domain.Valuation, error) {
	history, err := r.Assets().ListValuations(ctx, a.ID)
	if err != nil {
		return Item{}, nil, fmt.Errorf("list valuations of %q: %w", a.Name, err)
	}
	latest, err := domain.LatestValuation(a, history, at)
	if err != nil {
		return Item{}, nil, fmt.Errorf("latest valuation of %q: %w", a.Name, err)
	}
	debt, err := domain.NewMoney(0, a.PurchaseValue.Currency())
	if err != nil {
		return Item{}, nil, err
	}
	if a.LoanID != 0 {
		if debt, err = loanDebt(ctx, r, a, at); err != nil {
			return Item{}, nil, err
		}
	}
	net, err := latest.Value.Sub(debt)
	if err != nil {
		return Item{}, nil, fmt.Errorf("net value of %q: %w", a.Name, err)
	}
	return Item{Asset: a, Latest: latest, Debt: debt, Net: net, At: at}, history, nil
}

// loanDebt is the residual debt (never negative) of the loan linked to a: the opposite of the
// balance of its liability account.
func loanDebt(ctx context.Context, r app.Repos, a domain.Asset, at domain.Date) (domain.Money, error) {
	loan, err := r.Loans().Get(ctx, a.LoanID)
	if err != nil {
		return domain.Money{}, fmt.Errorf("loan of asset %q: %w", a.Name, err)
	}
	acct, err := r.Accounts().Get(ctx, loan.LiabilityAccountID)
	if err != nil {
		return domain.Money{}, fmt.Errorf("liability account of loan %q: %w", loan.Name, err)
	}
	sum, err := r.Transactions().SumUpTo(ctx, acct.ID, at)
	if err != nil {
		return domain.Money{}, fmt.Errorf("sum movements of %q: %w", acct.Name, err)
	}
	moves, err := domain.NewMoney(sum, acct.Currency)
	if err != nil {
		return domain.Money{}, err
	}
	bal, err := acct.OpeningBalance.Add(moves)
	if err != nil {
		return domain.Money{}, fmt.Errorf("balance of %q: %w", acct.Name, err)
	}
	debt, err := bal.Neg()
	if err != nil {
		return domain.Money{}, err
	}
	if debt.Sign() < 0 {
		debt, err = domain.NewMoney(0, acct.Currency)
		if err != nil {
			return domain.Money{}, err
		}
	}
	if debt.Currency() != a.PurchaseValue.Currency() {
		return domain.Money{}, fmt.Errorf("loan %q is in %s, asset %q in %s: %w",
			loan.Name, debt.Currency(), a.Name, a.PurchaseValue.Currency(), domain.ErrCurrencyMismatch)
	}
	return debt, nil
}
