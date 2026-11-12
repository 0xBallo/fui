// Package budgeting holds the use cases of the budget module: setting caps and envelopes,
// assigning and moving envelope money, copying a month, and evaluating a month's status by
// feeding the domain budget engine with the transactions of the store.
package budgeting

import (
	"context"
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
	EntityBudget     = "budget"
	EntityBudgetMove = "budget_move"
)

// Config holds the settings the budget use cases depend on.
type Config struct {
	Locale        domain.Locale   // amount input format; default en-US
	BaseCurrency  domain.Currency // budgets are expressed in it
	MonthStartDay int             // 1..31; default 1
	Mode          domain.BudgetMode
	Location      *time.Location // zone that defines "today"; default time.Local
	Modules       module.Set     // enabled modules; the budget module must be among them
}

// Service runs the budget use cases over a Store.
type Service struct {
	store app.Store
	clock app.Clock
	cfg   Config
}

// New builds a Service, defaulting Locale, MonthStartDay and Mode when unset.
func New(store app.Store, clock app.Clock, cfg Config) *Service {
	if cfg.Locale == "" {
		cfg.Locale = domain.LocaleENUS
	}
	if cfg.Location == nil {
		cfg.Location = time.Local
	}
	if cfg.MonthStartDay == 0 {
		cfg.MonthStartDay = 1
	}
	if cfg.Mode == "" {
		cfg.Mode = domain.BudgetCap
	}
	return &Service{store: store, clock: clock, cfg: cfg}
}

// gate fails with module_disabled unless the budget module is enabled.
func (s *Service) gate() error {
	if err := s.cfg.Modules.Require(module.Budget); err != nil {
		return app.WrapModuleError(err)
	}
	if s.cfg.BaseCurrency.IsZero() {
		return &domain.Error{
			Code: domain.CodeMissingField, Message: "no base currency configured",
			Details: map[string]any{"field": "base_currency"},
		}
	}
	return nil
}

// month returns the budget period containing the first day of in ("YYYY-MM"); empty means today.
func (s *Service) month(in string) (domain.Period, error) {
	var d domain.Date
	var err error
	if in == "" {
		d, err = domain.FromTime(s.clock.Now().In(s.cfg.Location))
	} else {
		d, err = domain.ParseDate(in + "-01")
	}
	if err != nil {
		return domain.Period{}, fmt.Errorf("month: %w", err)
	}
	return domain.MonthPeriod(d, s.cfg.MonthStartDay)
}

func (s *Service) write(ctx context.Context, fn func(app.Repos) error) error {
	if err := s.store.WithTx(ctx, fn); err != nil {
		return fmt.Errorf("apply: %w", err)
	}
	return nil
}

func (s *Service) money(in string) (domain.Money, error) {
	m, err := domain.ParseAmount(in, s.cfg.Locale, s.cfg.BaseCurrency)
	if err != nil {
		return domain.Money{}, fmt.Errorf("amount: %w", err)
	}
	return m, nil
}

// SetRequest creates the budget Name or updates it. With Month ("YYYY-MM") only the amount of
// that month is overridden; the budget must exist. Without it, a non-empty Categories/Tags
// replaces the targets and a non-empty Amount the default amount. Nil Carry keeps the flag.
type SetRequest struct {
	Name       string
	Categories []string
	Tags       []string
	Amount     string
	Month      string
	Carry      *bool
}

// SetPlan is the plan to store Budget (insert when Budget.ID is zero, else replace).
type SetPlan struct {
	app.Plan
	Budget domain.Budget
}

// PlanSet validates req against the stored budgets.
func (s *Service) PlanSet(ctx context.Context, req SetRequest) (SetPlan, error) {
	if err := s.gate(); err != nil {
		return SetPlan{}, err
	}
	var out SetPlan
	err := s.store.Read(ctx, func(r app.Repos) error {
		all, err := r.Budgets().List(ctx)
		if err != nil {
			return fmt.Errorf("list budgets: %w", err)
		}
		i := slices.IndexFunc(all, func(b domain.Budget) bool { return strings.EqualFold(b.Name, req.Name) })
		b, op := domain.Budget{Name: req.Name, Mode: s.cfg.Mode}, app.OpCreate
		if i >= 0 {
			b, op = all[i], app.OpUpdate
		}
		if req.Month != "" && i < 0 {
			return &domain.Error{Code: domain.CodeNotFound, Message: fmt.Sprintf("budget %q does not exist", req.Name)}
		}
		if err := s.fill(ctx, r, &b, req); err != nil {
			return err
		}
		if b, err = domain.NewBudget(b); err != nil {
			return err
		}
		if i >= 0 {
			all[i] = b
		} else {
			all = append(all, b)
		}
		if err := domain.ValidateBudgets(all); err != nil {
			return err
		}
		out = SetPlan{Plan: app.Plan{Changes: []app.Change{{
			Op: op, Entity: EntityBudget, Summary: fmt.Sprintf("%s budget %q", op, b.Name),
			Fields: budgetFields(b),
		}}}, Budget: b}
		return nil
	})
	return out, err
}

// fill applies req to b.
func (s *Service) fill(ctx context.Context, r app.Repos, b *domain.Budget, req SetRequest) error {
	var amount domain.Money
	if req.Amount != "" {
		var err error
		if amount, err = s.money(req.Amount); err != nil {
			return err
		}
	}
	if req.Month != "" {
		p, err := s.month(req.Month)
		if err != nil {
			return err
		}
		if req.Amount == "" {
			return &domain.Error{
				Code: domain.CodeMissingField, Message: "amount is required for a month override",
				Details: map[string]any{"field": "amount"},
			}
		}
		ov := make(map[domain.MonthKey]domain.Money, len(b.Overrides)+1)
		for k, v := range b.Overrides {
			ov[k] = v
		}
		ov[domain.MonthKeyOf(p)] = amount
		b.Overrides = ov
		return nil
	}
	if req.Amount != "" {
		b.Amount = amount
	}
	if len(req.Categories)+len(req.Tags) > 0 {
		ts, err := targets(ctx, r, req.Categories, req.Tags)
		if err != nil {
			return err
		}
		b.Targets = ts
	}
	if req.Carry != nil {
		b.Carry = *req.Carry
	}
	return nil
}

func targets(ctx context.Context, r app.Repos, cats, tags []string) ([]domain.BudgetTarget, error) {
	var out []domain.BudgetTarget
	if len(cats) > 0 {
		all, err := r.Categories().List(ctx)
		if err != nil {
			return nil, fmt.Errorf("list categories: %w", err)
		}
		names := make([]string, len(all))
		for i, c := range all {
			names[i] = c.Name
		}
		for _, ref := range cats {
			i, err := domain.Resolve(ref, names)
			if err != nil {
				return nil, fmt.Errorf("category: %w", err)
			}
			out = append(out, domain.BudgetTarget{CategoryID: all[i].ID})
		}
	}
	if len(tags) > 0 {
		all, err := r.Tags().List(ctx)
		if err != nil {
			return nil, fmt.Errorf("list tags: %w", err)
		}
		names := make([]string, len(all))
		for i, t := range all {
			names[i] = t.Name
		}
		for _, ref := range tags {
			i, err := domain.Resolve(ref, names)
			if err != nil {
				return nil, fmt.Errorf("tag: %w", err)
			}
			out = append(out, domain.BudgetTarget{TagID: all[i].ID})
		}
	}
	return out, nil
}

// ApplySet stores the budget of the plan and returns it with its ID.
func (s *Service) ApplySet(ctx context.Context, p SetPlan) (domain.Budget, error) {
	out := p.Budget
	err := s.write(ctx, func(r app.Repos) (err error) {
		if out.ID == 0 {
			out, err = r.Budgets().Insert(ctx, out)
			return err
		}
		return r.Budgets().Update(ctx, out)
	})
	return out, err
}

// MoveRequest moves Amount between envelopes on Date (empty = today). An empty From or To is the
// unassigned pool, so an assignment has an empty From.
type MoveRequest struct {
	From, To string
	Amount   string
	Date     string
}

// MovePlan is the plan to append Move to the ledger.
type MovePlan struct {
	app.Plan
	Move domain.BudgetMove
}

// PlanMove validates req; it needs the envelope mode.
func (s *Service) PlanMove(ctx context.Context, req MoveRequest) (MovePlan, error) {
	if err := s.gate(); err != nil {
		return MovePlan{}, err
	}
	if s.cfg.Mode != domain.BudgetEnvelope {
		return MovePlan{}, &domain.Error{
			Code: domain.CodeInvalidValue, Message: "moves need budget.mode = envelope",
			Details: map[string]any{"field": "budget.mode", "value": string(s.cfg.Mode)},
		}
	}
	amount, err := s.money(req.Amount)
	if err != nil {
		return MovePlan{}, err
	}
	var date domain.Date
	if req.Date == "" {
		date, err = domain.FromTime(s.clock.Now().In(s.cfg.Location))
	} else {
		date, err = domain.ParseDate(req.Date)
	}
	if err != nil {
		return MovePlan{}, fmt.Errorf("date: %w", err)
	}
	var out MovePlan
	err = s.store.Read(ctx, func(r app.Repos) error {
		all, err := r.Budgets().List(ctx)
		if err != nil {
			return fmt.Errorf("list budgets: %w", err)
		}
		from, err := pick(all, req.From)
		if err != nil {
			return err
		}
		to, err := pick(all, req.To)
		if err != nil {
			return err
		}
		mv := domain.BudgetMove{Date: date, From: from, To: to, Amount: amount}
		if err := mv.Validate(); err != nil {
			return err
		}
		out = MovePlan{Plan: app.Plan{Changes: []app.Change{moveChange(mv, req.From, req.To)}}, Move: mv}
		return nil
	})
	return out, err
}

// pick resolves a budget name; empty is the pool (ID 0).
func pick(all []domain.Budget, ref string) (domain.BudgetID, error) {
	if ref == "" {
		return 0, nil
	}
	names := make([]string, len(all))
	for i, b := range all {
		names[i] = b.Name
	}
	i, err := domain.Resolve(ref, names)
	if err != nil {
		return 0, fmt.Errorf("budget: %w", err)
	}
	return all[i].ID, nil
}

// ApplyMove appends the move of the plan.
func (s *Service) ApplyMove(ctx context.Context, p MovePlan) error {
	return s.write(ctx, func(r app.Repos) error { return r.Budgets().AddMove(ctx, p.Move) })
}

func moveChange(mv domain.BudgetMove, from, to string) app.Change {
	f := map[string]any{"date": mv.Date.String(), "from": from, "to": to}
	amountFields(f, mv.Amount)
	return app.Change{
		Op: app.OpCreate, Entity: EntityBudgetMove,
		Summary: fmt.Sprintf("move %s from %q to %q", mv.Amount, from, to), Fields: f,
	}
}

func amountFields(f map[string]any, m domain.Money) {
	f["amount"] = m.Minor()
	f["amount_currency"] = m.Currency().Code()
}

func budgetFields(b domain.Budget) map[string]any {
	f := map[string]any{"name": b.Name, "mode": string(b.Mode), "carry": b.Carry, "targets": b.Targets}
	amountFields(f, b.Amount)
	return f
}

// CopyPlan is the plan to copy the previous month into Period: cap amount overrides in cap
// mode, the net assignments from the pool in envelope mode.
type CopyPlan struct {
	app.Plan
	Period  domain.Period
	Updates []domain.Budget
	Moves   []domain.BudgetMove
}

// PlanCopyPrevious plans copying the previous month ("YYYY-MM", empty = current) into it.
func (s *Service) PlanCopyPrevious(ctx context.Context, monthIn string) (CopyPlan, error) {
	if err := s.gate(); err != nil {
		return CopyPlan{}, err
	}
	p, err := s.month(monthIn)
	if err != nil {
		return CopyPlan{}, err
	}
	prevEnd, err := p.From().AddDays(-1)
	if err != nil {
		return CopyPlan{}, err
	}
	prev, err := domain.MonthPeriod(prevEnd, s.cfg.MonthStartDay)
	if err != nil {
		return CopyPlan{}, err
	}
	out := CopyPlan{Period: p}
	err = s.store.Read(ctx, func(r app.Repos) error {
		all, err := r.Budgets().List(ctx)
		if err != nil {
			return fmt.Errorf("list budgets: %w", err)
		}
		var moves []domain.BudgetMove
		if s.cfg.Mode == domain.BudgetEnvelope {
			if moves, err = r.Budgets().Moves(ctx, prev.From(), prev.To()); err != nil {
				return fmt.Errorf("list moves: %w", err)
			}
		}
		for _, b := range all {
			if err := s.copyOne(&out, b, prev, moves); err != nil {
				return err
			}
		}
		return nil
	})
	out.Bulk = len(out.Changes) > 1
	return out, err
}

// copyOne adds to out what copying prev into out.Period means for budget b.
func (s *Service) copyOne(out *CopyPlan, b domain.Budget, prev domain.Period, moves []domain.BudgetMove) error {
	p := out.Period
	if s.cfg.Mode == domain.BudgetEnvelope {
		net, err := poolNet(b.ID, moves, s.cfg.BaseCurrency)
		if err != nil || net.Sign() <= 0 {
			return err
		}
		mv := domain.BudgetMove{Date: p.From(), To: b.ID, Amount: net}
		out.Moves = append(out.Moves, mv)
		out.Changes = append(out.Changes, moveChange(mv, "", b.Name))
		return nil
	}
	want := b.AmountFor(domain.MonthKeyOf(prev))
	if want == b.AmountFor(domain.MonthKeyOf(p)) {
		return nil
	}
	ov := map[domain.MonthKey]domain.Money{domain.MonthKeyOf(p): want}
	for k, v := range b.Overrides {
		if _, set := ov[k]; !set {
			ov[k] = v
		}
	}
	b.Overrides = ov
	out.Updates = append(out.Updates, b)
	out.Changes = append(out.Changes, app.Change{
		Op: app.OpUpdate, Entity: EntityBudget,
		Summary: fmt.Sprintf("set %q to %s for %s", b.Name, want, p.From()), Fields: budgetFields(b),
	})
	return nil
}

// poolNet is the net amount assigned from the pool to envelope id in moves.
func poolNet(id domain.BudgetID, moves []domain.BudgetMove, cur domain.Currency) (domain.Money, error) {
	net, err := domain.NewMoney(0, cur)
	if err != nil {
		return domain.Money{}, err
	}
	for _, mv := range moves {
		switch {
		case mv.From == 0 && mv.To == id:
			net, err = net.Add(mv.Amount)
		case mv.To == 0 && mv.From == id:
			net, err = net.Sub(mv.Amount)
		}
		if err != nil {
			return domain.Money{}, err
		}
	}
	return net, nil
}

// ApplyCopyPrevious stores the plan; a bulk plan is applied atomically.
func (s *Service) ApplyCopyPrevious(ctx context.Context, p CopyPlan) error {
	return s.write(ctx, func(r app.Repos) error {
		for _, b := range p.Updates {
			if err := r.Budgets().Update(ctx, b); err != nil {
				return err
			}
		}
		for _, mv := range p.Moves {
			if err := r.Budgets().AddMove(ctx, mv); err != nil {
				return err
			}
		}
		return nil
	})
}

// List returns every budget ordered by ID.
func (s *Service) List(ctx context.Context) ([]domain.Budget, error) {
	if err := s.gate(); err != nil {
		return nil, err
	}
	var out []domain.Budget
	err := s.store.Read(ctx, func(r app.Repos) (err error) {
		out, err = r.Budgets().List(ctx)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("list budgets: %w", err)
	}
	return out, nil
}
