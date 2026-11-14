package simulating

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/reporting"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/domain/simulate"
	"github.com/0xballo/fui/internal/module"
)

// EntitySavedSimulation is the Change.Entity of a saved simulation.
const EntitySavedSimulation = "saved_simulation"

// Simulation kinds.
const (
	KindInvestment = "investment"
	KindPAC        = "pac"
	KindCashFlow   = "cashflow"
)

// Config holds the settings simulation use cases depend on.
type Config struct {
	Locale        domain.Locale
	Location      *time.Location
	BaseCurrency  domain.Currency
	MonthStartDay int
}

// Service runs simulation use cases over a Store.
type Service struct {
	store app.Store
	clock app.Clock
	cfg   Config
}

// New builds a Service with default locale, location and month start day.
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
	return &Service{store: store, clock: clock, cfg: cfg}
}

// Rates are the three annual returns used side by side, in basis points.
type Rates struct {
	PessimisticBP int64 `json:"pessimistic_bp"`
	BaseBP        int64 `json:"base_bp"`
	OptimisticBP  int64 `json:"optimistic_bp"`
}

// InvestmentParams are the investment simulation parameters.
type InvestmentParams struct {
	InitialMinor            int64  `json:"initial_minor,omitempty"`
	InitialCurrentPositions bool   `json:"initial_current_positions,omitempty"`
	Years                   int    `json:"years"`
	Compounding             string `json:"compounding"`
	Rates                   Rates  `json:"rates"`
	CostBP                  int64  `json:"cost_bp,omitempty"`
	TaxBP                   int64  `json:"tax_bp,omitempty"`
	InflationBP             int64  `json:"inflation_bp,omitempty"`
}

// PACParams are the periodic-accumulation simulation parameters.
type PACParams struct {
	InitialMinor            int64  `json:"initial_minor,omitempty"`
	InitialCurrentPositions bool   `json:"initial_current_positions,omitempty"`
	ContributionMinor       int64  `json:"contribution_minor"`
	Frequency               string `json:"frequency"`
	ContributionGrowthBP    int64  `json:"contribution_growth_bp,omitempty"`
	Years                   int    `json:"years"`
	Compounding             string `json:"compounding"`
	Rates                   Rates  `json:"rates"`
	CostBP                  int64  `json:"cost_bp,omitempty"`
	TaxBP                   int64  `json:"tax_bp,omitempty"`
	InflationBP             int64  `json:"inflation_bp,omitempty"`
}

// CashFlowParams are the cash-flow simulation parameters.
type CashFlowParams struct {
	Months int             `json:"months"`
	Events []CashFlowEvent `json:"events,omitempty"`
}

// CashFlowEvent is one hypothetical event of a cash-flow simulation.
type CashFlowEvent struct {
	Kind        string `json:"kind"`
	Date        string `json:"date"`
	AmountMinor int64  `json:"amount_minor,omitempty"`
	RateBP      int64  `json:"rate_bp,omitempty"`
	Years       int    `json:"years,omitempty"`
}

// RunRequest selects which simulation to run.
type RunRequest struct {
	Kind       string            `json:"kind"`
	Investment *InvestmentParams `json:"investment,omitempty"`
	PAC        *PACParams        `json:"pac,omitempty"`
	CashFlow   *CashFlowParams   `json:"cashflow,omitempty"`
}

// CashFlowMonth is one month-end point of a cash-flow simulation.
type CashFlowMonth struct {
	End           string `json:"end"`
	BaseMinor     int64  `json:"base_minor"`
	ScenarioMinor int64  `json:"scenario_minor"`
}

// CashFlowResult is the output of a cash-flow simulation run.
type CashFlowResult struct {
	Start          string          `json:"start"`
	OpeningMinor   int64           `json:"opening_minor"`
	Months         []CashFlowMonth `json:"months"`
	FirstBelowZero string          `json:"first_below_zero,omitempty"`
	EndDeltaMinor  int64           `json:"end_delta_minor"`
	RunwayMonths   int             `json:"runway_months,omitempty"`
}

// RunResult is the output of one simulation run.
type RunResult struct {
	Kind       string           `json:"kind"`
	Investment *simulate.Triple `json:"investment,omitempty"`
	PAC        *simulate.Triple `json:"pac,omitempty"`
	CashFlow   *CashFlowResult  `json:"cashflow,omitempty"`
}

// SavePlan is the plan to store a saved simulation.
type SavePlan struct {
	app.Plan
	Simulation app.SavedSimulation
}

// RemovePlan is the plan to delete a saved simulation.
type RemovePlan struct {
	app.Plan
	ID app.SavedSimulationID
}

// RerunResult is the result of rerunning one saved simulation.
type RerunResult struct {
	Simulation app.SavedSimulation `json:"simulation"`
	RunResult  RunResult           `json:"result"`
}

// CompareResult compares 2-3 saved simulation reruns side by side.
type CompareResult struct {
	Items []RerunResult `json:"items"`
}

// Run validates and runs one simulation kind without saving it.
func (s *Service) Run(ctx context.Context, enabled module.Set, req RunRequest) (RunResult, error) {
	if err := require(enabled); err != nil {
		return RunResult{}, err
	}
	switch strings.ToLower(strings.TrimSpace(req.Kind)) {
	case KindInvestment:
		if req.Investment == nil {
			return RunResult{}, missingParams(req.Kind)
		}
		out, err := s.runInvestment(ctx, *req.Investment)
		if err != nil {
			return RunResult{}, err
		}
		return RunResult{Kind: KindInvestment, Investment: &out}, nil
	case KindPAC:
		if req.PAC == nil {
			return RunResult{}, missingParams(req.Kind)
		}
		out, err := s.runPAC(ctx, *req.PAC)
		if err != nil {
			return RunResult{}, err
		}
		return RunResult{Kind: KindPAC, PAC: &out}, nil
	case KindCashFlow:
		if req.CashFlow == nil {
			return RunResult{}, missingParams(req.Kind)
		}
		out, err := s.runCashFlow(ctx, enabled, *req.CashFlow)
		if err != nil {
			return RunResult{}, err
		}
		return RunResult{Kind: KindCashFlow, CashFlow: &out}, nil
	default:
		return RunResult{}, invalid("kind", fmt.Sprintf("unknown simulation kind %q", req.Kind))
	}
}

// PlanSave validates a simulation and plans storing it with a unique name.
func (s *Service) PlanSave(ctx context.Context, enabled module.Set, name string, req RunRequest) (SavePlan, error) {
	if err := require(enabled); err != nil {
		return SavePlan{}, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return SavePlan{}, fmt.Errorf("saved simulation name: %w", domain.ErrMissingField)
	}
	kind, paramsJSON, err := encodeParams(req)
	if err != nil {
		return SavePlan{}, err
	}
	if _, err := s.Run(ctx, enabled, req); err != nil {
		return SavePlan{}, err
	}
	var out SavePlan
	err = s.store.Read(ctx, func(r app.Repos) error {
		existing, err := r.SavedSimulations().List(ctx)
		if err != nil {
			return fmt.Errorf("list saved simulations: %w", err)
		}
		if slices.ContainsFunc(existing, func(it app.SavedSimulation) bool { return strings.EqualFold(it.Name, name) }) {
			return &domain.Error{Code: app.CodeAlreadyExists, Message: fmt.Sprintf("saved simulation %q already exists", name)}
		}
		out = SavePlan{
			Plan: app.Plan{Changes: []app.Change{{
				Op: app.OpCreate, Entity: EntitySavedSimulation,
				Fields:  map[string]any{"name": name, "kind": kind, "params": paramsJSON},
				Summary: fmt.Sprintf("save simulation %q (%s)", name, kind),
			}}},
			Simulation: app.SavedSimulation{Name: name, Kind: kind, ParamsJSON: paramsJSON},
		}
		return nil
	})
	return out, err
}

// ApplySave stores one saved simulation.
func (s *Service) ApplySave(ctx context.Context, p SavePlan) (app.SavedSimulation, error) {
	var out app.SavedSimulation
	err := s.store.WithTx(ctx, func(r app.Repos) (err error) {
		out, err = r.SavedSimulations().Insert(ctx, p.Simulation)
		return err
	})
	if err != nil {
		return app.SavedSimulation{}, fmt.Errorf("apply: %w", err)
	}
	return out, nil
}

// List returns saved simulations ordered by name.
func (s *Service) List(ctx context.Context, enabled module.Set) ([]app.SavedSimulation, error) {
	if err := require(enabled); err != nil {
		return nil, err
	}
	var out []app.SavedSimulation
	err := s.store.Read(ctx, func(r app.Repos) (err error) {
		out, err = r.SavedSimulations().List(ctx)
		return err
	})
	return out, err
}

// PlanRemove resolves ref to one saved simulation and plans deleting it.
func (s *Service) PlanRemove(ctx context.Context, enabled module.Set, ref string) (RemovePlan, error) {
	if err := require(enabled); err != nil {
		return RemovePlan{}, err
	}
	sim, err := s.find(ctx, ref)
	if err != nil {
		return RemovePlan{}, err
	}
	return RemovePlan{
		Plan: app.Plan{Changes: []app.Change{{
			Op: app.OpDelete, Entity: EntitySavedSimulation,
			Fields:  map[string]any{"name": sim.Name},
			Summary: fmt.Sprintf("remove saved simulation %q", sim.Name),
		}}},
		ID: sim.ID,
	}, nil
}

// ApplyRemove deletes one saved simulation.
func (s *Service) ApplyRemove(ctx context.Context, p RemovePlan) error {
	if err := s.store.WithTx(ctx, func(r app.Repos) error { return r.SavedSimulations().Delete(ctx, p.ID) }); err != nil {
		return fmt.Errorf("apply: %w", err)
	}
	return nil
}

// Rerun reruns the saved simulation selected by ref.
func (s *Service) Rerun(ctx context.Context, enabled module.Set, ref string) (RerunResult, error) {
	if err := require(enabled); err != nil {
		return RerunResult{}, err
	}
	sim, err := s.find(ctx, ref)
	if err != nil {
		return RerunResult{}, err
	}
	req, err := decodeRunRequest(sim)
	if err != nil {
		return RerunResult{}, err
	}
	out, err := s.Run(ctx, enabled, req)
	if err != nil {
		return RerunResult{}, err
	}
	return RerunResult{Simulation: sim, RunResult: out}, nil
}

// Compare reruns 2-3 saved simulations by refs and returns their results side by side.
func (s *Service) Compare(ctx context.Context, enabled module.Set, refs []string) (CompareResult, error) {
	if err := require(enabled); err != nil {
		return CompareResult{}, err
	}
	if len(refs) < 2 || len(refs) > 3 {
		return CompareResult{}, invalid("refs", "compare accepts 2 or 3 saved simulations")
	}
	out := CompareResult{Items: make([]RerunResult, 0, len(refs))}
	for _, ref := range refs {
		item, err := s.Rerun(ctx, enabled, ref)
		if err != nil {
			return CompareResult{}, err
		}
		out.Items = append(out.Items, item)
	}
	return out, nil
}

func (s *Service) find(ctx context.Context, ref string) (app.SavedSimulation, error) {
	var all []app.SavedSimulation
	err := s.store.Read(ctx, func(r app.Repos) (err error) {
		all, err = r.SavedSimulations().List(ctx)
		return err
	})
	if err != nil {
		return app.SavedSimulation{}, err
	}
	names := make([]string, len(all))
	for i, it := range all {
		names[i] = it.Name
	}
	i, err := domain.Resolve(ref, names)
	if err != nil {
		return app.SavedSimulation{}, fmt.Errorf("saved simulation: %w", err)
	}
	return all[i], nil
}

func require(enabled module.Set) error {
	return app.WrapModuleError(enabled.Require(module.Simulations))
}

func encodeParams(req RunRequest) (string, string, error) {
	kind := strings.ToLower(strings.TrimSpace(req.Kind))
	var (
		raw []byte
		err error
	)
	switch kind {
	case KindInvestment:
		if req.Investment == nil {
			return "", "", missingParams(req.Kind)
		}
		raw, err = json.Marshal(req.Investment)
	case KindPAC:
		if req.PAC == nil {
			return "", "", missingParams(req.Kind)
		}
		raw, err = json.Marshal(req.PAC)
	case KindCashFlow:
		if req.CashFlow == nil {
			return "", "", missingParams(req.Kind)
		}
		raw, err = json.Marshal(req.CashFlow)
	default:
		return "", "", invalid("kind", fmt.Sprintf("unknown simulation kind %q", req.Kind))
	}
	if err != nil {
		return "", "", fmt.Errorf("encode params: %w", err)
	}
	return kind, string(raw), nil
}

func decodeRunRequest(sim app.SavedSimulation) (RunRequest, error) {
	req := RunRequest{Kind: sim.Kind}
	switch sim.Kind {
	case KindInvestment:
		var p InvestmentParams
		if err := json.Unmarshal([]byte(sim.ParamsJSON), &p); err != nil {
			return RunRequest{}, fmt.Errorf("decode params of %q: %w", sim.Name, err)
		}
		req.Investment = &p
	case KindPAC:
		var p PACParams
		if err := json.Unmarshal([]byte(sim.ParamsJSON), &p); err != nil {
			return RunRequest{}, fmt.Errorf("decode params of %q: %w", sim.Name, err)
		}
		req.PAC = &p
	case KindCashFlow:
		var p CashFlowParams
		if err := json.Unmarshal([]byte(sim.ParamsJSON), &p); err != nil {
			return RunRequest{}, fmt.Errorf("decode params of %q: %w", sim.Name, err)
		}
		req.CashFlow = &p
	default:
		return RunRequest{}, invalid("kind", fmt.Sprintf("unknown simulation kind %q", sim.Kind))
	}
	return req, nil
}

func (s *Service) runInvestment(ctx context.Context, p InvestmentParams) (simulate.Triple, error) {
	base, err := s.investmentBase(ctx, p.InitialMinor, p.InitialCurrentPositions)
	if err != nil {
		return simulate.Triple{}, err
	}
	comp, err := parseCompounding(p.Compounding)
	if err != nil {
		return simulate.Triple{}, err
	}
	return simulate.Compare(simulate.Params{
		Initial:     base,
		Years:       p.Years,
		Compounding: comp,
		CostBP:      p.CostBP,
		TaxBP:       p.TaxBP,
		InflationBP: p.InflationBP,
	}, simulate.Rates{
		Pessimistic: p.Rates.PessimisticBP,
		Base:        p.Rates.BaseBP,
		Optimistic:  p.Rates.OptimisticBP,
	})
}

func (s *Service) runPAC(ctx context.Context, p PACParams) (simulate.Triple, error) {
	base, err := s.investmentBase(ctx, p.InitialMinor, p.InitialCurrentPositions)
	if err != nil {
		return simulate.Triple{}, err
	}
	comp, err := parseCompounding(p.Compounding)
	if err != nil {
		return simulate.Triple{}, err
	}
	freq, err := parseFrequency(p.Frequency)
	if err != nil {
		return simulate.Triple{}, err
	}
	return simulate.Compare(simulate.Params{
		Initial:          base,
		Contribution:     p.ContributionMinor,
		Frequency:        freq,
		ContributionGrow: p.ContributionGrowthBP,
		Years:            p.Years,
		Compounding:      comp,
		CostBP:           p.CostBP,
		TaxBP:            p.TaxBP,
		InflationBP:      p.InflationBP,
	}, simulate.Rates{
		Pessimistic: p.Rates.PessimisticBP,
		Base:        p.Rates.BaseBP,
		Optimistic:  p.Rates.OptimisticBP,
	})
}

func (s *Service) runCashFlow(ctx context.Context, enabled module.Set, p CashFlowParams) (CashFlowResult, error) {
	if p.Months < 1 {
		return CashFlowResult{}, invalid("months", "months must be at least 1")
	}
	start, err := s.today()
	if err != nil {
		return CashFlowResult{}, err
	}
	opening, baseFlows, err := s.forecastBase(ctx, enabled, start, p.Months)
	if err != nil {
		return CashFlowResult{}, err
	}
	events, err := parseEvents(p.Events)
	if err != nil {
		return CashFlowResult{}, err
	}
	res, err := simulate.CashFlow(start, opening, baseFlows, events, p.Months)
	if err != nil {
		return CashFlowResult{}, err
	}
	out := CashFlowResult{
		Start:         start.String(),
		OpeningMinor:  opening,
		EndDeltaMinor: res.EndDelta,
		RunwayMonths:  res.RunwayMonths,
		Months:        make([]CashFlowMonth, len(res.Months)),
	}
	if res.HasFirstBelowZero {
		out.FirstBelowZero = res.FirstBelowZero.String()
	}
	for i, row := range res.Months {
		out.Months[i] = CashFlowMonth{
			End:           row.End.String(),
			BaseMinor:     row.Base,
			ScenarioMinor: row.Scenario,
		}
	}
	return out, nil
}

func (s *Service) forecastBase(ctx context.Context, enabled module.Set, start domain.Date, months int) (int64, []simulate.Flow, error) {
	horizon, err := start.AddMonths(months)
	if err != nil {
		return 0, nil, err
	}
	days, err := daysBetween(start, horizon)
	if err != nil {
		return 0, nil, err
	}
	uc := reporting.NewAnalytics(s.store, s.clock, nil, reporting.Config{
		Locale:        s.cfg.Locale,
		BaseCurrency:  s.cfg.BaseCurrency,
		MonthStartDay: s.cfg.MonthStartDay,
		Location:      s.cfg.Location,
		Modules:       enabled,
	})
	forecast, err := uc.Forecast(ctx, reporting.ForecastRequest{HorizonDays: days})
	if err != nil {
		return 0, nil, err
	}
	if len(forecast.Total) == 0 {
		return 0, nil, errors.New("forecast returned no total points")
	}
	opening := forecast.Total[0].Amount.Minor
	flows := make([]simulate.Flow, 0, len(forecast.Total)-1)
	prev := forecast.Total[0].Amount.Minor
	for i := 1; i < len(forecast.Total); i++ {
		day, err := domain.ParseDate(forecast.Total[i].Date)
		if err != nil {
			return 0, nil, fmt.Errorf("forecast date %q: %w", forecast.Total[i].Date, err)
		}
		cur := forecast.Total[i].Amount.Minor
		flows = append(flows, simulate.Flow{Date: day, Amount: cur - prev})
		prev = cur
	}
	return opening, flows, nil
}

func (s *Service) investmentBase(ctx context.Context, fixed int64, fromCurrentPositions bool) (int64, error) {
	if !fromCurrentPositions {
		return fixed, nil
	}
	return s.currentInvestmentValue(ctx)
}

func (s *Service) currentInvestmentValue(ctx context.Context) (int64, error) {
	if s.cfg.BaseCurrency.IsZero() {
		return 0, fmt.Errorf("base currency: %w", domain.ErrMissingField)
	}
	today, err := s.today()
	if err != nil {
		return 0, err
	}
	total, _ := domain.NewMoney(0, s.cfg.BaseCurrency)
	err = s.store.Read(ctx, func(r app.Repos) error {
		accounts, err := r.Accounts().List(ctx)
		if err != nil {
			return fmt.Errorf("list accounts: %w", err)
		}
		for _, acc := range accounts {
			if acc.Archived || acc.Kind != domain.AccountInvestment {
				continue
			}
			bal, err := balanceAt(ctx, r, acc, today)
			if err != nil {
				return err
			}
			base, err := s.toBase(ctx, r, bal, today)
			if err != nil {
				return err
			}
			if total, err = total.Add(base); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return total.Minor(), nil
}

func (s *Service) toBase(ctx context.Context, r app.Repos, m domain.Money, date domain.Date) (domain.Money, error) {
	if m.Currency() == s.cfg.BaseCurrency {
		return m, nil
	}
	fx, err := r.FXRates().OnOrBefore(ctx, m.Currency(), date)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.Money{}, &domain.Error{
			Code:    app.CodeRateMissing,
			Message: fmt.Sprintf("no exchange rate on or before %s for %s", date, m.Currency()),
			Details: map[string]any{"currencies": []string{m.Currency().Code()}, "date": date.String()},
		}
	}
	if err != nil {
		return domain.Money{}, fmt.Errorf("look up %s rate: %w", m.Currency(), err)
	}
	return domain.Convert(m, fx.Rate, s.cfg.BaseCurrency)
}

func balanceAt(ctx context.Context, r app.Repos, acct domain.Account, date domain.Date) (domain.Money, error) {
	if date.Before(acct.OpenedOn) {
		return domain.NewMoney(0, acct.Currency)
	}
	sum, err := r.Transactions().SumUpTo(ctx, acct.ID, date)
	if err != nil {
		return domain.Money{}, fmt.Errorf("balance of %q: %w", acct.Name, err)
	}
	mv, err := domain.NewMoney(sum, acct.Currency)
	if err != nil {
		return domain.Money{}, err
	}
	return acct.OpeningBalance.Add(mv)
}

func parseFrequency(in string) (simulate.Frequency, error) {
	switch strings.ToLower(strings.TrimSpace(in)) {
	case "monthly":
		return simulate.FrequencyMonthly, nil
	case "quarterly":
		return simulate.FrequencyQuarterly, nil
	case "yearly":
		return simulate.FrequencyYearly, nil
	default:
		return 0, invalid("frequency", fmt.Sprintf("unknown frequency %q", in))
	}
}

func parseCompounding(in string) (simulate.Compounding, error) {
	switch strings.ToLower(strings.TrimSpace(in)) {
	case "monthly":
		return simulate.CompoundMonthly, nil
	case "annual":
		return simulate.CompoundAnnual, nil
	default:
		return 0, invalid("compounding", fmt.Sprintf("unknown compounding %q", in))
	}
}

func parseEvents(in []CashFlowEvent) ([]simulate.Event, error) {
	out := make([]simulate.Event, len(in))
	for i, item := range in {
		d, err := domain.ParseDate(strings.TrimSpace(item.Date))
		if err != nil {
			return nil, fmt.Errorf("event date: %w", err)
		}
		var kind simulate.EventKind
		switch strings.ToLower(strings.TrimSpace(item.Kind)) {
		case "one_off":
			kind = simulate.OneOff
		case "recurring_change":
			kind = simulate.RecurringChange
		case "stop_income":
			kind = simulate.StopIncome
		case "new_loan":
			kind = simulate.NewLoan
		default:
			return nil, invalid("event.kind", fmt.Sprintf("unknown event kind %q", item.Kind))
		}
		out[i] = simulate.Event{Kind: kind, Date: d, Amount: item.AmountMinor, RateBP: item.RateBP, Years: item.Years}
	}
	return out, nil
}

func daysBetween(from, to domain.Date) (int, error) {
	if to.Before(from) {
		return 0, invalid("months", "invalid horizon")
	}
	count := 0
	cur := from
	for cur.Before(to) {
		next, err := cur.AddDays(1)
		if err != nil {
			return 0, err
		}
		cur = next
		count++
	}
	return count, nil
}

func (s *Service) today() (domain.Date, error) {
	d, err := domain.FromTime(s.clock.Now().In(s.cfg.Location))
	if err != nil {
		return domain.Date{}, fmt.Errorf("today: %w", err)
	}
	return d, nil
}

func missingParams(kind string) error {
	return fmt.Errorf("params for %q: %w", kind, domain.ErrMissingField)
}

func invalid(field, msg string) error {
	return &domain.Error{Code: domain.CodeInvalidValue, Message: msg, Details: map[string]any{"field": field}}
}
