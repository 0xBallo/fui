package reporting

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

// EntitySavedReport is the Change.Entity of a saved report.
const EntitySavedReport = "saved_report"

// Report kinds a saved report can be based on.
const (
	KindBalance  = "balance-over-time"
	KindBudget   = "budget-vs-actual"
	KindDebt     = "remaining-debt"
	KindInterest = "interest-paid"
	KindNetWorth = "net-worth"
	KindPayees   = "top-payees"
	KindForecast = "forecast"
	KindSpending = "spending"
	KindCashFlow = "cash-flow"
)

var reportKinds = []string{KindBalance, KindBudget, KindDebt, KindInterest, KindNetWorth, KindPayees, KindForecast, KindSpending, KindCashFlow}

// SavedSpec is the stored definition of a saved report; Period is relative (see ResolveRelative)
// so the report stays current. The forecast ignores Period.
type SavedSpec struct {
	Period  string `json:"period,omitempty"`
	Account string `json:"account,omitempty"`
	Loan    string `json:"loan,omitempty"`
	Limit   int    `json:"limit,omitempty"`
	// MinBalance is the forecast's minimum balance.
	MinBalance string `json:"min_balance,omitempty"`
}

// SavePlan is the plan to store a saved report.
type SavePlan struct {
	app.Plan
	Report app.SavedReport
}

// PlanSave validates a saved report definition: a unique name, a known kind and a valid
// relative period.
func (a *Analytics) PlanSave(ctx context.Context, name, kind string, spec SavedSpec) (SavePlan, error) {
	if err := a.gate(); err != nil {
		return SavePlan{}, err
	}
	name, kind = strings.TrimSpace(name), strings.TrimSpace(kind)
	if name == "" {
		return SavePlan{}, fmt.Errorf("saved report name: %w", domain.ErrMissingField)
	}
	if !slices.Contains(reportKinds, kind) {
		return SavePlan{}, invalid("kind", fmt.Sprintf("unknown report kind %q", kind))
	}
	if spec.Period != "" && !ValidRelativePeriod(spec.Period) {
		return SavePlan{}, invalid("period", fmt.Sprintf("unknown relative period %q", spec.Period))
	}
	raw, err := json.Marshal(spec)
	if err != nil {
		return SavePlan{}, fmt.Errorf("encode params: %w", err)
	}
	var out SavePlan
	err = a.store.Read(ctx, func(r app.Repos) error {
		existing, err := r.SavedReports().List(ctx)
		if err != nil {
			return fmt.Errorf("list saved reports: %w", err)
		}
		if slices.ContainsFunc(existing, func(s app.SavedReport) bool { return strings.EqualFold(s.Name, name) }) {
			return &domain.Error{Code: app.CodeAlreadyExists, Message: fmt.Sprintf("saved report %q already exists", name)}
		}
		out = SavePlan{
			Plan: app.Plan{Changes: []app.Change{{
				Op: app.OpCreate, Entity: EntitySavedReport,
				Fields:  map[string]any{"name": name, "kind": kind, "params": string(raw)},
				Summary: fmt.Sprintf("save report %q (%s)", name, kind),
			}}},
			Report: app.SavedReport{Name: name, Kind: kind, ParamsJSON: string(raw)},
		}
		return nil
	})
	return out, err
}

// ApplySave stores the saved report.
func (a *Analytics) ApplySave(ctx context.Context, p SavePlan) (app.SavedReport, error) {
	var out app.SavedReport
	err := a.store.WithTx(ctx, func(r app.Repos) (err error) {
		out, err = r.SavedReports().Insert(ctx, p.Report)
		return err
	})
	if err != nil {
		return app.SavedReport{}, fmt.Errorf("apply: %w", err)
	}
	return out, nil
}

// List returns the saved reports ordered by name.
func (a *Analytics) List(ctx context.Context) ([]app.SavedReport, error) {
	if err := a.gate(); err != nil {
		return nil, err
	}
	var out []app.SavedReport
	err := a.store.Read(ctx, func(r app.Repos) (err error) {
		out, err = r.SavedReports().List(ctx)
		return err
	})
	return out, err
}

// RemovePlan is the plan to delete a saved report.
type RemovePlan struct {
	app.Plan
	ID app.SavedReportID
}

// PlanRemove resolves the saved report ref (a unique name or prefix) to delete.
func (a *Analytics) PlanRemove(ctx context.Context, ref string) (RemovePlan, error) {
	sr, err := a.find(ctx, ref)
	if err != nil {
		return RemovePlan{}, err
	}
	return RemovePlan{
		Plan: app.Plan{Changes: []app.Change{{
			Op: app.OpDelete, Entity: EntitySavedReport, Fields: map[string]any{"name": sr.Name},
			Summary: fmt.Sprintf("remove saved report %q", sr.Name),
		}}},
		ID: sr.ID,
	}, nil
}

// ApplyRemove deletes the saved report.
func (a *Analytics) ApplyRemove(ctx context.Context, p RemovePlan) error {
	if err := a.store.WithTx(ctx, func(r app.Repos) error { return r.SavedReports().Delete(ctx, p.ID) }); err != nil {
		return fmt.Errorf("apply: %w", err)
	}
	return nil
}

func (a *Analytics) find(ctx context.Context, ref string) (app.SavedReport, error) {
	all, err := a.List(ctx)
	if err != nil {
		return app.SavedReport{}, err
	}
	names := make([]string, len(all))
	for i, s := range all {
		names[i] = s.Name
	}
	i, err := domain.Resolve(ref, names)
	if err != nil {
		return app.SavedReport{}, fmt.Errorf("saved report: %w", err)
	}
	return all[i], nil
}

// SavedResult is the output of running a saved report; Data is the result type of its kind.
type SavedResult struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
	Data any    `json:"data"`
}

// Run resolves the relative period of saved report ref against today and runs its base report.
func (a *Analytics) Run(ctx context.Context, ref string) (SavedResult, error) {
	sr, err := a.find(ctx, ref)
	if err != nil {
		return SavedResult{}, err
	}
	var spec SavedSpec
	if err := json.Unmarshal([]byte(sr.ParamsJSON), &spec); err != nil {
		return SavedResult{}, fmt.Errorf("decode params of %q: %w", sr.Name, err)
	}
	req := ReportRequest{Account: spec.Account, Loan: spec.Loan, Limit: spec.Limit}
	if spec.Period != "" {
		today, err := a.today()
		if err != nil {
			return SavedResult{}, err
		}
		if req.Period, err = ResolveRelative(spec.Period, today, a.cfg.MonthStartDay); err != nil {
			return SavedResult{}, err
		}
	}
	out := SavedResult{Name: sr.Name, Kind: sr.Kind}
	switch sr.Kind {
	case KindBalance:
		out.Data, err = a.BalanceOverTime(ctx, req)
	case KindBudget:
		out.Data, err = a.BudgetVsActual(ctx, req)
	case KindDebt:
		out.Data, err = a.RemainingDebt(ctx, req)
	case KindInterest:
		out.Data, err = a.InterestPaid(ctx, req)
	case KindNetWorth:
		out.Data, err = a.NetWorth(ctx, req)
	case KindPayees:
		out.Data, err = a.TopPayees(ctx, req)
	case KindForecast:
		out.Data, err = a.Forecast(ctx, ForecastRequest{MinBalance: spec.MinBalance})
	case KindSpending, KindCashFlow:
		out.Data, err = a.mvp(ctx, sr.Kind, req)
	default:
		err = invalid("kind", fmt.Sprintf("unknown report kind %q", sr.Kind))
	}
	return out, err
}

func invalid(field, msg string) error {
	return &domain.Error{Code: domain.CodeInvalidValue, Message: msg, Details: map[string]any{"field": field}}
}

// mvp runs the spending or cash-flow report over the Source, for the account named in req.
func (a *Analytics) mvp(ctx context.Context, kind string, req ReportRequest) (any, error) {
	var ids []domain.AccountID
	if req.Account != "" {
		err := a.store.Read(ctx, func(r app.Repos) error {
			accts, err := r.Accounts().List(ctx)
			if err != nil {
				return fmt.Errorf("list accounts: %w", err)
			}
			acct, err := pickAccount(accts, req.Account)
			ids = []domain.AccountID{acct.ID}
			return err
		})
		if err != nil {
			return nil, err
		}
	}
	if kind == KindSpending {
		uc, err := NewSpending(a.source, a.clock)
		if err != nil {
			return nil, err
		}
		return uc.Run(ctx, SpendingParams{Period: req.Period, MonthStartDay: a.cfg.MonthStartDay, AccountIDs: ids, BaseCurrency: a.cfg.BaseCurrency})
	}
	uc, err := NewCashFlow(a.source, a.clock)
	if err != nil {
		return nil, err
	}
	return uc.Run(ctx, CashFlowParams{Period: req.Period, MonthStartDay: a.cfg.MonthStartDay, AccountIDs: ids, BaseCurrency: a.cfg.BaseCurrency})
}
