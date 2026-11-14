// Package investing holds the use cases of the investments module: securities, manual prices,
// buy/sell/dividend operations with an optional linked cash movement, and positions valued in
// the base currency.
package investing

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
)

// Entity names used in Change.Entity.
const (
	EntitySecurity      = "security"
	EntitySecurityPrice = "security_price"
	EntitySecurityOp    = "security_op"
)

// Config holds the settings the investing use cases depend on.
type Config struct {
	Locale       domain.Locale   // amount input format; default en-US
	BaseCurrency domain.Currency // positions are valued in it
	Location     *time.Location  // zone that defines "today"; default time.Local
	Modules      module.Set      // enabled modules; investments (and crypto for crypto kinds) must be among them
}

// Service runs the investing use cases over a Store.
type Service struct {
	store app.Store
	clock app.Clock
	cfg   Config
}

// New builds a Service, defaulting Locale and Location when unset.
func New(store app.Store, clock app.Clock, cfg Config) *Service {
	if cfg.Locale == "" {
		cfg.Locale = domain.LocaleENUS
	}
	if cfg.Location == nil {
		cfg.Location = time.Local
	}
	return &Service{store: store, clock: clock, cfg: cfg}
}

func (s *Service) gate(kind domain.SecurityKind) error {
	if err := s.cfg.Modules.Require(module.Investments); err != nil {
		return app.WrapModuleError(err)
	}
	if kind == domain.SecurityCrypto {
		if err := s.cfg.Modules.Require(module.Crypto); err != nil {
			return app.WrapModuleError(err)
		}
	}
	return nil
}

func (s *Service) today() (domain.Date, error) {
	d, err := domain.FromTime(s.clock.Now().In(s.cfg.Location))
	if err != nil {
		return domain.Date{}, fmt.Errorf("today: %w", err)
	}
	return d, nil
}

// date parses in (empty means today).
func (s *Service) date(in string) (domain.Date, error) {
	today, err := s.today()
	if err != nil || strings.TrimSpace(in) == "" {
		return today, err
	}
	d, err := domain.ParseDateInput(in, s.cfg.Locale, today)
	if err != nil {
		return domain.Date{}, fmt.Errorf("date: %w", err)
	}
	return d, nil
}

func (s *Service) write(ctx context.Context, fn func(app.Repos) error) error {
	if err := s.store.WithTx(ctx, fn); err != nil {
		return fmt.Errorf("apply: %w", err)
	}
	return nil
}

func notFound(what, ref string) error {
	return &domain.Error{Code: domain.CodeNotFound, Message: fmt.Sprintf("%s %q not found", what, ref)}
}

// findSecurity resolves ref against ticker, ISIN and name, case-insensitively.
func findSecurity(ctx context.Context, r app.Repos, ref string) (domain.Security, error) {
	all, err := r.Securities().List(ctx)
	if err != nil {
		return domain.Security{}, fmt.Errorf("list securities: %w", err)
	}
	i := slices.IndexFunc(all, func(x domain.Security) bool {
		return strings.EqualFold(x.Ticker, ref) || strings.EqualFold(x.ISIN, ref) || strings.EqualFold(x.Name, ref)
	})
	if i < 0 {
		return domain.Security{}, notFound("security", ref)
	}
	return all[i], nil
}

func findAccount(ctx context.Context, r app.Repos, name string) (domain.Account, error) {
	all, err := r.Accounts().List(ctx)
	if err != nil {
		return domain.Account{}, fmt.Errorf("list accounts: %w", err)
	}
	i := slices.IndexFunc(all, func(a domain.Account) bool { return strings.EqualFold(a.Name, name) })
	if i < 0 {
		return domain.Account{}, notFound("account", name)
	}
	return all[i], nil
}

// ListSecurities returns every security ordered by ID.
func (s *Service) ListSecurities(ctx context.Context) ([]domain.Security, error) {
	if err := s.gate(""); err != nil {
		return nil, err
	}
	var out []domain.Security
	err := s.store.Read(ctx, func(r app.Repos) (err error) {
		out, err = r.Securities().List(ctx)
		return err
	})
	return out, err
}

// AddSecurityRequest carries user-typed values. Name, Kind and Currency are required, plus a
// Ticker or an ISIN. QuantityScale defaults to 8 for crypto and 0 otherwise.
type AddSecurityRequest struct {
	Name          string
	Ticker        string
	ISIN          string
	Kind          string
	Currency      string
	QuantityScale *int
}

// AddSecurityPlan is the plan to create Security.
type AddSecurityPlan struct {
	app.Plan
	Security domain.Security
}

// PlanAddSecurity validates req and rejects a name, ticker or ISIN already in use.
func (s *Service) PlanAddSecurity(ctx context.Context, req AddSecurityRequest) (AddSecurityPlan, error) {
	kind, err := domain.ParseSecurityKind(req.Kind)
	if err != nil {
		return AddSecurityPlan{}, err
	}
	if err := s.gate(kind); err != nil {
		return AddSecurityPlan{}, err
	}
	cur, err := domain.ParseCurrency(req.Currency)
	if err != nil {
		return AddSecurityPlan{}, err
	}
	scale := 0
	if kind == domain.SecurityCrypto {
		scale = 8
	}
	if req.QuantityScale != nil {
		scale = *req.QuantityScale
	}
	sec, err := domain.NewSecurity(domain.Security{
		Name: req.Name, Ticker: req.Ticker, ISIN: req.ISIN, Kind: kind, Currency: cur, QuantityScale: scale,
	})
	if err != nil {
		return AddSecurityPlan{}, err
	}
	err = s.store.Read(ctx, func(r app.Repos) error {
		all, err := r.Securities().List(ctx)
		if err != nil {
			return fmt.Errorf("list securities: %w", err)
		}
		for _, o := range all {
			if strings.EqualFold(o.Name, sec.Name) || (sec.Ticker != "" && o.Ticker == sec.Ticker) ||
				(sec.ISIN != "" && o.ISIN == sec.ISIN) {
				return &domain.Error{
					Code: app.CodeAlreadyExists, Message: fmt.Sprintf("security %q already exists", o.Name),
					Details: map[string]any{"name": o.Name},
				}
			}
		}
		return nil
	})
	if err != nil {
		return AddSecurityPlan{}, err
	}
	return AddSecurityPlan{Plan: app.Plan{Changes: []app.Change{{
		Op: app.OpCreate, Entity: EntitySecurity, Summary: fmt.Sprintf("create security %q", sec.Name),
		Fields: map[string]any{
			"name": sec.Name, "ticker": sec.Ticker, "isin": sec.ISIN, "kind": string(sec.Kind),
			"currency": sec.Currency.Code(), "quantity_scale": sec.QuantityScale,
		},
	}}}, Security: sec}, nil
}

// ApplyAddSecurity stores the planned security and returns it with its ID.
func (s *Service) ApplyAddSecurity(ctx context.Context, p AddSecurityPlan) (domain.Security, error) {
	if err := s.gate(p.Security.Kind); err != nil {
		return domain.Security{}, err
	}
	var out domain.Security
	err := s.write(ctx, func(r app.Repos) (err error) {
		out, err = r.Securities().Insert(ctx, p.Security)
		return err
	})
	return out, err
}

// SetPriceRequest sets the manual price of Security (ticker, ISIN or name) on Date (default today).
type SetPriceRequest struct {
	Security string
	Date     string
	Price    string
}

// SetPricePlan is the plan to store Price, replacing any price of the same day.
type SetPricePlan struct {
	app.Plan
	Price domain.SecurityPrice
}

// PlanSetPrice parses the price in the currency of the security.
func (s *Service) PlanSetPrice(ctx context.Context, req SetPriceRequest) (SetPricePlan, error) {
	if err := s.gate(""); err != nil {
		return SetPricePlan{}, err
	}
	d, err := s.date(req.Date)
	if err != nil {
		return SetPricePlan{}, err
	}
	var out SetPricePlan
	err = s.store.Read(ctx, func(r app.Repos) error {
		sec, err := findSecurity(ctx, r, req.Security)
		if err != nil {
			return err
		}
		if err := s.gate(sec.Kind); err != nil {
			return err
		}
		price, err := domain.ParseAmount(req.Price, s.cfg.Locale, sec.Currency)
		if err != nil {
			return fmt.Errorf("price: %w", err)
		}
		if price.Sign() < 0 {
			return &domain.Error{
				Code: domain.CodeInvalidValue, Message: "price must not be negative",
				Details: map[string]any{"field": "price"},
			}
		}
		prices, err := r.Securities().ListPrices(ctx, sec.ID)
		if err != nil {
			return fmt.Errorf("list prices: %w", err)
		}
		op := app.OpCreate
		if slices.ContainsFunc(prices, func(p domain.SecurityPrice) bool { return p.Date == d }) {
			op = app.OpUpdate
		}
		out = SetPricePlan{Plan: app.Plan{Changes: []app.Change{{
			Op: op, Entity: EntitySecurityPrice, Summary: fmt.Sprintf("%s price of %s on %s", op, sec.Name, d),
			Fields: moneyFields(map[string]any{"security": sec.Name, "date": d.String()}, "price", price),
		}}}, Price: domain.SecurityPrice{Security: sec.ID, Date: d, Price: price}}
		return nil
	})
	return out, err
}

// ApplySetPrice stores the planned price.
func (s *Service) ApplySetPrice(ctx context.Context, p SetPricePlan) error {
	if err := s.gate(""); err != nil {
		return err
	}
	return s.write(ctx, func(r app.Repos) error { return r.Securities().UpsertPrice(ctx, p.Price) })
}

func moneyFields(f map[string]any, prefix string, m domain.Money) map[string]any {
	f[prefix] = m.Minor()
	f[prefix+"_currency"] = m.Currency().Code()
	return f
}

// OpRequest carries user-typed values of a security operation. Account holds the position;
// CashAccount, when set, receives the linked liquidity movement (kind investment, in the
// currency of the security). Date defaults to today, Fees to zero.
type OpRequest struct {
	Kind        domain.SecurityOpKind
	Account     string
	CashAccount string
	Security    string
	Date        string
	Quantity    string
	Price       string
	Fees        string
}

// OpPlan is the plan to store Op and, when CashTx is set, its linked cash transaction.
type OpPlan struct {
	app.Plan
	Op     domain.SecurityOp
	CashTx *domain.Transaction
}

// PlanBuy plans a purchase.
func (s *Service) PlanBuy(ctx context.Context, req OpRequest) (OpPlan, error) {
	req.Kind = domain.OpBuy
	return s.PlanOp(ctx, req)
}

// PlanSell plans a sale; selling more than the account holds fails with insufficient_quantity.
func (s *Service) PlanSell(ctx context.Context, req OpRequest) (OpPlan, error) {
	req.Kind = domain.OpSell
	return s.PlanOp(ctx, req)
}

// PlanDividend plans a dividend (Quantity is the quantity held, Price the amount per unit).
func (s *Service) PlanDividend(ctx context.Context, req OpRequest) (OpPlan, error) {
	req.Kind = domain.OpDividend
	return s.PlanOp(ctx, req)
}

// PlanOp validates the operation against the stored history of the position.
func (s *Service) PlanOp(ctx context.Context, req OpRequest) (OpPlan, error) {
	if err := s.gate(""); err != nil {
		return OpPlan{}, err
	}
	d, err := s.date(req.Date)
	if err != nil {
		return OpPlan{}, err
	}
	var out OpPlan
	err = s.store.Read(ctx, func(r app.Repos) error {
		out, err = s.buildOp(ctx, r, req, d)
		return err
	})
	return out, err
}

func (s *Service) buildOp(ctx context.Context, r app.Repos, req OpRequest, d domain.Date) (OpPlan, error) {
	sec, err := findSecurity(ctx, r, req.Security)
	if err != nil {
		return OpPlan{}, err
	}
	if err := s.gate(sec.Kind); err != nil {
		return OpPlan{}, err
	}
	acct, err := findAccount(ctx, r, req.Account)
	if err != nil {
		return OpPlan{}, err
	}
	qty, err := domain.ParseQuantity(req.Quantity, sec.QuantityScale)
	if err != nil {
		return OpPlan{}, fmt.Errorf("quantity: %w", err)
	}
	price, err := domain.ParseAmount(req.Price, s.cfg.Locale, sec.Currency)
	if err != nil {
		return OpPlan{}, fmt.Errorf("price: %w", err)
	}
	op := domain.SecurityOp{Date: d, Account: acct.ID, Security: sec.ID, Kind: req.Kind, Quantity: qty, UnitPrice: price}
	if strings.TrimSpace(req.Fees) != "" {
		if op.Fees, err = domain.ParseAmount(req.Fees, s.cfg.Locale, sec.Currency); err != nil {
			return OpPlan{}, fmt.Errorf("fees: %w", err)
		}
	}
	if op, err = domain.NewSecurityOp(op); err != nil {
		return OpPlan{}, err
	}
	if err := checkHeld(ctx, r, sec, acct.ID, &op, 0); err != nil {
		return OpPlan{}, err
	}
	plan := OpPlan{Op: op}
	change := app.Change{
		Op: app.OpCreate, Entity: EntitySecurityOp,
		Summary: fmt.Sprintf("%s %s %s of %s", op.Kind, qty, sec.Name, d),
		Fields: moneyFields(map[string]any{
			"kind": string(op.Kind), "date": d.String(), "account": acct.Name, "security": sec.Name,
			"quantity": qty.String(),
		}, "unit_price", price),
	}
	plan.Changes = append(plan.Changes, change)
	if strings.TrimSpace(req.CashAccount) == "" {
		return plan, nil
	}
	cash, err := findAccount(ctx, r, req.CashAccount)
	if err != nil {
		return OpPlan{}, err
	}
	tx, err := cashTx(sec, cash, op)
	if err != nil {
		return OpPlan{}, err
	}
	plan.CashTx = &tx
	plan.Changes = append(plan.Changes, app.Change{
		Op: app.OpCreate, Entity: app.EntityTransaction,
		Summary: fmt.Sprintf("create %s %s on %s", tx.Kind, tx.Amount, cash.Name),
		Fields:  moneyFields(map[string]any{"account": cash.Name, "date": d.String(), "kind": string(tx.Kind)}, "amount", tx.Amount),
	})
	return plan, nil
}

// cashTx builds the liquidity movement of op: money out for a buy, in for the other kinds,
// fees always reducing the cash. It needs the cash account in the currency of the security.
func cashTx(sec domain.Security, cash domain.Account, op domain.SecurityOp) (domain.Transaction, error) {
	if cash.Currency != sec.Currency {
		return domain.Transaction{}, &domain.Error{
			Code: domain.CodeCurrencyMismatch, Details: map[string]any{"left": cash.Currency.Code(), "right": sec.Currency.Code()},
			Message: fmt.Sprintf("cash account %q is in %s, security in %s", cash.Name, cash.Currency, sec.Currency),
		}
	}
	gross, err := op.Quantity.Value(op.UnitPrice)
	if err != nil {
		return domain.Transaction{}, fmt.Errorf("gross amount: %w", err)
	}
	var amt domain.Money
	if op.Kind == domain.OpBuy {
		if amt, err = gross.Add(op.Fees); err == nil {
			amt, err = amt.Neg()
		}
	} else {
		amt, err = gross.Sub(op.Fees)
	}
	if err != nil {
		return domain.Transaction{}, fmt.Errorf("cash amount: %w", err)
	}
	return domain.NewTransaction(domain.Transaction{
		AccountID: cash.ID, Date: op.Date, Amount: amt, Status: domain.StatusBooked, Kind: domain.KindInvestment,
		Description: fmt.Sprintf("%s %s %s", op.Kind, op.Quantity, sec.Name),
	})
}

// checkHeld replays the stored operations of the position (account, security) with add
// included and skip (an op ID) excluded; a sell exceeding the held quantity fails.
func checkHeld(ctx context.Context, r app.Repos, sec domain.Security, acct domain.AccountID, add *domain.SecurityOp, skip domain.SecurityOpID) error {
	stored, err := r.Securities().ListOpsBySecurity(ctx, sec.ID, domain.Date{}, domain.Date{})
	if err != nil {
		return fmt.Errorf("list operations: %w", err)
	}
	ops := stored[:0]
	last := domain.Date{}
	for _, o := range stored {
		if o.Account == acct && o.ID != skip {
			ops = append(ops, o)
		}
	}
	if add != nil {
		ops = append(ops, *add)
	}
	for _, o := range ops {
		if o.Date.After(last) {
			last = o.Date
		}
	}
	if _, err := domain.ComputePosition(sec, ops, nil, last); err != nil {
		return fmt.Errorf("position check: %w", err)
	}
	return nil
}

// ApplyOp stores the cash transaction (if any) and the operation linked to it, atomically,
// re-checking the held quantity against the current data.
func (s *Service) ApplyOp(ctx context.Context, p OpPlan) (domain.SecurityOp, error) {
	if err := s.gate(""); err != nil {
		return domain.SecurityOp{}, err
	}
	var out domain.SecurityOp
	err := s.write(ctx, func(r app.Repos) error {
		sec, err := r.Securities().Get(ctx, p.Op.Security)
		if err != nil {
			return fmt.Errorf("security: %w", err)
		}
		if err := s.gate(sec.Kind); err != nil {
			return err
		}
		op := p.Op
		if err := checkHeld(ctx, r, sec, op.Account, &op, 0); err != nil {
			return err
		}
		if p.CashTx != nil {
			tx, err := r.Transactions().Insert(ctx, *p.CashTx)
			if err != nil {
				return fmt.Errorf("insert cash transaction: %w", err)
			}
			op.CashTx = tx.ID
		}
		out, err = r.Securities().InsertOp(ctx, op)
		return err
	})
	return out, err
}

// RemoveOpPlan is the plan to delete Op and its linked cash transaction (zero when none).
type RemoveOpPlan struct {
	app.Plan
	Op domain.SecurityOp
}

// PlanRemoveOp rejects a removal that would leave a later sell without enough held quantity,
// and one whose cash transaction is reconciled.
func (s *Service) PlanRemoveOp(ctx context.Context, id domain.SecurityOpID) (RemoveOpPlan, error) {
	if err := s.gate(""); err != nil {
		return RemoveOpPlan{}, err
	}
	var out RemoveOpPlan
	err := s.store.Read(ctx, func(r app.Repos) error {
		op, err := r.Securities().GetOp(ctx, id)
		if err != nil {
			return fmt.Errorf("security operation %d: %w", id, err)
		}
		sec, err := r.Securities().Get(ctx, op.Security)
		if err != nil {
			return fmt.Errorf("security: %w", err)
		}
		if err := s.gate(sec.Kind); err != nil {
			return err
		}
		if err := checkHeld(ctx, r, sec, op.Account, nil, op.ID); err != nil {
			return err
		}
		out = RemoveOpPlan{Op: op, Plan: app.Plan{Changes: []app.Change{{
			Op: app.OpDelete, Entity: EntitySecurityOp, Summary: fmt.Sprintf("delete %s %s of %s", op.Kind, op.Quantity, sec.Name),
			Fields: map[string]any{"id": int64(op.ID), "kind": string(op.Kind), "date": op.Date.String()},
		}}}}
		if op.CashTx == 0 {
			return nil
		}
		tx, err := r.Transactions().Get(ctx, op.CashTx)
		if err != nil {
			return fmt.Errorf("cash transaction: %w", err)
		}
		if tx.Status == domain.StatusReconciled {
			return domain.ErrReconciled
		}
		out.Changes = append(out.Changes, app.Change{
			Op: app.OpDelete, Entity: app.EntityTransaction, Summary: fmt.Sprintf("delete %s %s", tx.Kind, tx.Amount),
			Fields: map[string]any{"id": int64(tx.ID)},
		})
		return nil
	})
	return out, err
}

// ApplyRemoveOp deletes the operation and its linked cash transaction atomically.
func (s *Service) ApplyRemoveOp(ctx context.Context, p RemoveOpPlan) error {
	if err := s.gate(""); err != nil {
		return err
	}
	return s.write(ctx, func(r app.Repos) error {
		cur, err := r.Securities().GetOp(ctx, p.Op.ID)
		if err != nil {
			return fmt.Errorf("security operation %d: %w", p.Op.ID, err)
		}
		sec, err := r.Securities().Get(ctx, cur.Security)
		if err != nil {
			return fmt.Errorf("security: %w", err)
		}
		if err := checkHeld(ctx, r, sec, cur.Account, nil, cur.ID); err != nil {
			return err
		}
		if err := r.Securities().DeleteOp(ctx, cur.ID); err != nil {
			return fmt.Errorf("delete operation: %w", err)
		}
		if cur.CashTx != 0 {
			if err := r.Transactions().Delete(ctx, cur.CashTx); err != nil && !errors.Is(err, domain.ErrNotFound) {
				return fmt.Errorf("delete cash transaction: %w", err)
			}
		}
		return nil
	})
}

// PositionsRequest filters ListPositions: Account by name, Date as of (default today).
type PositionsRequest struct {
	Account string
	Date    string
}

// PositionLine is the position of one security in one account. Value is the market value in
// the base currency, valid only when Position.HasPrice.
type PositionLine struct {
	Account  domain.Account
	Security domain.Security
	Position domain.Position
	Value    domain.Money
}

// ListPositions computes one line per (account, security) with operations up to the date,
// ordered by account then security, converting market values at the latest rate on or before it.
func (s *Service) ListPositions(ctx context.Context, req PositionsRequest) ([]PositionLine, error) {
	if err := s.gate(""); err != nil {
		return nil, err
	}
	d, err := s.date(req.Date)
	if err != nil {
		return nil, err
	}
	var out []PositionLine
	err = s.store.Read(ctx, func(r app.Repos) error {
		accts, err := r.Accounts().List(ctx)
		if err != nil {
			return fmt.Errorf("list accounts: %w", err)
		}
		var only domain.AccountID
		if req.Account != "" {
			a, err := findAccount(ctx, r, req.Account)
			if err != nil {
				return err
			}
			only = a.ID
		}
		secs, err := r.Securities().List(ctx)
		if err != nil {
			return fmt.Errorf("list securities: %w", err)
		}
		for _, sec := range secs {
			if err := s.gate(sec.Kind); err != nil {
				continue // crypto positions stay hidden while the crypto module is off
			}
			lines, err := s.securityLines(ctx, r, sec, accts, only, d)
			if err != nil {
				return err
			}
			out = append(out, lines...)
		}
		slices.SortStableFunc(out, func(a, b PositionLine) int {
			return cmp.Or(cmp.Compare(a.Account.ID, b.Account.ID), cmp.Compare(a.Security.ID, b.Security.ID))
		})
		return nil
	})
	return out, err
}

func (s *Service) securityLines(ctx context.Context, r app.Repos, sec domain.Security, accts []domain.Account, only domain.AccountID, d domain.Date) ([]PositionLine, error) {
	ops, err := r.Securities().ListOpsBySecurity(ctx, sec.ID, domain.Date{}, d)
	if err != nil {
		return nil, fmt.Errorf("list operations: %w", err)
	}
	prices, err := r.Securities().ListPrices(ctx, sec.ID)
	if err != nil {
		return nil, fmt.Errorf("list prices: %w", err)
	}
	byAcct := map[domain.AccountID][]domain.SecurityOp{}
	for _, op := range ops {
		if only == 0 || op.Account == only {
			byAcct[op.Account] = append(byAcct[op.Account], op)
		}
	}
	var lines []PositionLine
	for _, a := range accts {
		aops, ok := byAcct[a.ID]
		if !ok {
			continue
		}
		pos, err := domain.ComputePosition(sec, aops, prices, d)
		if err != nil {
			return nil, fmt.Errorf("position of %s in %s: %w", sec.Name, a.Name, err)
		}
		line := PositionLine{Account: a, Security: sec, Position: pos}
		if pos.HasPrice {
			if line.Value, err = s.toBase(ctx, r, pos.MarketValue, d); err != nil {
				return nil, err
			}
		}
		lines = append(lines, line)
	}
	return lines, nil
}

// toBase converts m at the latest rate on or before d; the base currency is worth exactly 1.
func (s *Service) toBase(ctx context.Context, r app.Repos, m domain.Money, d domain.Date) (domain.Money, error) {
	base := s.cfg.BaseCurrency
	if base.IsZero() {
		return domain.Money{}, &domain.Error{
			Code: domain.CodeMissingField, Message: "no base currency configured",
			Details: map[string]any{"field": "base_currency"},
		}
	}
	if m.Currency() == base {
		return m, nil
	}
	fx, err := r.FXRates().OnOrBefore(ctx, m.Currency(), d)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.Money{}, &domain.Error{
			Code: app.CodeRateMissing, Message: fmt.Sprintf("no exchange rate on or before %s for %s", d, m.Currency()),
			Details: map[string]any{"currencies": []string{m.Currency().Code()}, "date": d.String()},
		}
	}
	if err != nil {
		return domain.Money{}, fmt.Errorf("look up %s rate: %w", m.Currency(), err)
	}
	return domain.Convert(m, fx.Rate, base)
}
