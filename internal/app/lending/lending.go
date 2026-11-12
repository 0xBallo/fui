// Package lending holds the use cases of loans and mortgages: adding a loan with a computed or
// imported amortization schedule, showing schedules, recording events (new schedule versions),
// generating pending installment transactions and confirming them into a principal transfer
// plus interest/fee expenses. Loans belong to the loans module: every use case requires it.
//
// A loan has a name (the one users type) and a liability account of kind loan, named after it,
// whose balance is the remaining debt.
package lending

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
)

// EntityLoan is the Change.Entity of a loan.
const EntityLoan = "loan"

// Config holds the user settings the use cases depend on.
type Config struct {
	Locale   domain.Locale  // number and date input format; default en-US
	Location *time.Location // zone that defines "today"; default time.Local
}

// Service runs the loan use cases over a Store.
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

func (s *Service) write(ctx context.Context, bulk bool, fn func(app.Repos) error) error {
	if bulk {
		if _, err := s.backup.Backup(ctx, app.BackupReasonBulk); err != nil {
			return fmt.Errorf("backup before bulk change: %w", err)
		}
	}
	if err := s.store.WithTx(ctx, fn); err != nil {
		return fmt.Errorf("apply: %w", err)
	}
	return nil
}

func require(enabled module.Set) error {
	return app.WrapModuleError(enabled.Require(module.Loans))
}

// parseRate turns a percentage typed as "3,15" into basis points.
func parseRate(in string) (domain.AnnualRate, error) {
	r, err := app.ParseRate(in)
	if err != nil {
		return 0, err
	}
	const perBP = domain.RateScale / 100 // scaled units of one basis point (0.01%)
	if r.Scaled()%perBP != 0 {
		return 0, &domain.Error{
			Code: domain.CodeInvalidValue, Message: "rate has more than 2 decimals",
			Details: map[string]any{"field": "rate", "value": in},
		}
	}
	return domain.NewAnnualRate(r.Scaled() / perBP)
}

// Info is a loan with its liability account.
type Info struct {
	Loan    domain.Loan
	Account domain.Account
}

// loanInfos pairs every loan with its liability account (loans without one are skipped).
func loanInfos(ctx context.Context, r app.Repos) ([]Info, error) {
	loans, err := r.Loans().List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list loans: %w", err)
	}
	accts, err := r.Accounts().List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}
	byID := make(map[domain.AccountID]domain.Account, len(accts))
	for _, a := range accts {
		byID[a.ID] = a
	}
	out := make([]Info, 0, len(loans))
	for _, l := range loans {
		out = append(out, Info{Loan: l, Account: byID[l.LiabilityAccountID]})
	}
	return out, nil
}

func pickLoan(infos []Info, ref string) (Info, error) {
	names := make([]string, len(infos))
	for i, in := range infos {
		names[i] = in.Loan.Name
	}
	i, err := domain.Resolve(ref, names)
	if err != nil {
		return Info{}, fmt.Errorf("loan: %w", err)
	}
	return infos[i], nil
}

func pickAccount(accts []domain.Account, ref string) (domain.Account, error) {
	var active []domain.Account
	names := []string{}
	for _, a := range accts {
		if !a.Archived {
			active = append(active, a)
			names = append(names, a.Name)
		}
	}
	i, err := domain.Resolve(ref, names)
	if err != nil {
		return domain.Account{}, fmt.Errorf("account: %w", err)
	}
	return active[i], nil
}

func moneyFields(f map[string]any, prefix string, m domain.Money) {
	f[prefix] = m.Minor()
	f[prefix+"_currency"] = m.Currency().Code()
}

func invalid(field, msg string) error {
	return &domain.Error{Code: domain.CodeInvalidValue, Message: msg, Details: map[string]any{"field": field}}
}

// AddRequest carries user-typed values. Name, Kind, Principal, TermMonths, RateType, Rate and
// Debit are required; Disbursed defaults to today. Installment is the bank-stated payment that
// overrides the computed one. Imported, when set, is the schedule to store instead of computing
// one (origin imported): its principal must sum to the capital and end with a zero residual.
type AddRequest struct {
	Name        string
	Kind        string
	Principal   string
	Disbursed   string
	TermMonths  int
	RateType    string
	Rate        string // annual percentage, e.g. "3,15"
	Debit       string // account the installments are debited from
	Installment string
	Imported    []domain.Installment
}

// AddPlan is the plan to store Loan, its liability Account and its first Schedule. Loan has no
// liability account ID yet: ApplyAdd sets it once it has created Account.
type AddPlan struct {
	app.Plan
	Loan     domain.Loan
	Account  domain.Account // ID is set on apply
	Schedule domain.Schedule
}

// PlanAdd validates the request and builds the loan, its liability account (opening balance
// minus the principal, at the disbursement date) and its schedule, without writing.
func (s *Service) PlanAdd(ctx context.Context, enabled module.Set, req AddRequest) (AddPlan, error) {
	if err := require(enabled); err != nil {
		return AddPlan{}, err
	}
	var out AddPlan
	err := s.store.Read(ctx, func(r app.Repos) error {
		accts, err := r.Accounts().List(ctx)
		if err != nil {
			return fmt.Errorf("list accounts: %w", err)
		}
		debit, err := pickAccount(accts, req.Debit)
		if err != nil {
			return err
		}
		loan, rate, err := s.newLoan(req, debit)
		if err != nil {
			return err
		}
		sched, err := s.initialSchedule(loan, rate, req)
		if err != nil {
			return err
		}
		principal := loan.Principal
		opening, err := principal.Neg()
		if err != nil {
			return err
		}
		acct, err := domain.NewAccount(domain.Account{
			Name: req.Name, Kind: domain.AccountLoan, Currency: principal.Currency(),
			OpeningBalance: opening, OpenedOn: loan.DisbursedOn,
		})
		if err != nil {
			return err
		}
		for _, a := range accts {
			if strings.EqualFold(a.Name, acct.Name) {
				return &domain.Error{Code: app.CodeAlreadyExists, Message: fmt.Sprintf("account %q already exists", acct.Name)}
			}
		}
		f := map[string]any{
			"name": acct.Name, "kind": string(loan.Kind), "rate_type": string(loan.RateType), "debit_account": debit.Name,
			"disbursed_on": loan.DisbursedOn.String(), "term_months": loan.TermMonths, "rate_bp": rate.BasisPoints(),
			"origin": string(sched.Origin), "installments": len(sched.Installments),
		}
		moneyFields(f, "principal", principal)
		out = AddPlan{
			Plan: app.Plan{Changes: []app.Change{{
				Op: app.OpCreate, Entity: EntityLoan, Fields: f,
				Summary: fmt.Sprintf("create loan %q of %s over %d months", acct.Name, principal, loan.TermMonths),
			}}},
			Loan: loan, Account: acct, Schedule: sched,
		}
		return nil
	})
	return out, err
}

// pendingLiability stands in for the liability account ID, which only exists once ApplyAdd has
// created the account; it just has to be set and differ from every real account.
const pendingLiability domain.AccountID = -1

// newLoan validates the loan fields of req against its debit account and returns the loan and
// the initial annual rate.
func (s *Service) newLoan(req AddRequest, debit domain.Account) (domain.Loan, domain.AnnualRate, error) {
	kind, err := domain.ParseLoanKind(req.Kind)
	if err != nil {
		return domain.Loan{}, 0, err
	}
	rt, err := domain.ParseRateType(req.RateType)
	if err != nil {
		return domain.Loan{}, 0, err
	}
	principal, err := s.amount(req.Principal, debit.Currency)
	if err != nil {
		return domain.Loan{}, 0, err
	}
	disbursed, err := s.date(req.Disbursed)
	if err != nil {
		return domain.Loan{}, 0, err
	}
	rate, err := parseRate(req.Rate)
	if err != nil {
		return domain.Loan{}, 0, err
	}
	loan, err := domain.NewLoan(domain.Loan{
		Name: req.Name, LiabilityAccountID: pendingLiability,
		Kind: kind, Principal: principal, DisbursedOn: disbursed, TermMonths: req.TermMonths,
		RateType: rt, DebitAccount: debit.ID,
	})
	return loan, rate, err
}

func (s *Service) initialSchedule(loan domain.Loan, rate domain.AnnualRate, req AddRequest) (domain.Schedule, error) {
	if len(req.Imported) == 0 {
		var bank int64
		if req.Installment != "" {
			m, err := s.amount(req.Installment, loan.Principal.Currency())
			if err != nil {
				return domain.Schedule{}, err
			}
			bank = m.Minor()
		}
		return domain.ComputeSchedule(loan, loan.DisbursedOn, rate, loan.TermMonths, bank)
	}
	var sum int64
	zero, err := domain.NewMoney(0, loan.Principal.Currency())
	if err != nil {
		return domain.Schedule{}, err
	}
	imported := make([]domain.Installment, len(req.Imported))
	for i, inst := range req.Imported {
		// Importers may leave amounts out: that is zero, not a missing currency.
		for _, m := range []*domain.Money{&inst.Principal, &inst.Interest, &inst.Fees, &inst.Remaining} {
			if m.Currency().IsZero() {
				*m = zero
			}
		}
		imported[i] = inst
		sum += inst.Principal.Minor()
		if i > 0 && !inst.Due.After(imported[i-1].Due) {
			return domain.Schedule{}, invalid("installments", "imported installments must be in ascending due order")
		}
	}
	last := imported[len(imported)-1]
	if sum != loan.Principal.Minor() || last.Remaining.Minor() != 0 {
		return domain.Schedule{}, invalid("installments", "imported installments must repay the principal exactly")
	}
	return domain.Schedule{
		ValidFrom: loan.DisbursedOn, Origin: domain.ScheduleOriginImported, Rate: rate,
		Installments: imported,
	}, nil
}

// ApplyAdd stores the loan, its liability account and its schedule, and returns the loan.
func (s *Service) ApplyAdd(ctx context.Context, p AddPlan) (Info, error) {
	var out Info
	err := s.write(ctx, false, func(r app.Repos) error {
		acct, err := r.Accounts().Insert(ctx, p.Account)
		if err != nil {
			return fmt.Errorf("create loan account: %w", err)
		}
		loanRow := p.Loan
		loanRow.LiabilityAccountID = acct.ID
		loan, err := r.Loans().Insert(ctx, loanRow)
		if err != nil {
			return fmt.Errorf("create loan: %w", err)
		}
		sched := p.Schedule
		sched.LoanID = loan.ID
		if err := r.Loans().AddScheduleVersion(ctx, sched); err != nil {
			return fmt.Errorf("store schedule: %w", err)
		}
		out = Info{Loan: loan, Account: acct}
		return nil
	})
	return out, err
}

// View is a loan with its events and the schedule version in force at the requested date.
type View struct {
	Info
	Events   []domain.LoanEvent
	Schedule domain.Schedule
}

// List returns every loan.
func (s *Service) List(ctx context.Context, enabled module.Set) ([]Info, error) {
	if err := require(enabled); err != nil {
		return nil, err
	}
	var out []Info
	err := s.store.Read(ctx, func(r app.Repos) (err error) {
		out, err = loanInfos(ctx, r)
		return err
	})
	return out, err
}

// Show returns loan ref with its events and the schedule version in force at at (today when
// empty): this is both the show and the schedule use case.
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
		infos, err := loanInfos(ctx, r)
		if err != nil {
			return err
		}
		if out.Info, err = pickLoan(infos, ref); err != nil {
			return err
		}
		if out.Events, err = r.Loans().ListEvents(ctx, out.Loan.ID); err != nil {
			return fmt.Errorf("list events: %w", err)
		}
		// Before the first version ScheduleAt has nothing: show the first one.
		if when.Before(out.Loan.DisbursedOn) {
			when = out.Loan.DisbursedOn
		}
		if out.Schedule, err = r.Loans().ScheduleAt(ctx, out.Loan.ID, when); err != nil {
			return fmt.Errorf("schedule at %s: %w", when, err)
		}
		return nil
	})
	return out, err
}

// EventRequest records a fact that changes the schedule from Date on. Kind is rate_change,
// renegotiation or prepayment; Rate (percent) is used by the first two, TermMonths by
// renegotiation, Prepaid and Mode (reduce_installment|reduce_term) by prepayment.
type EventRequest struct {
	Loan       string
	Kind       string
	Date       string
	Rate       string
	TermMonths *int
	Prepaid    string
	Mode       string
}

// EventPlan is the plan to store Event and the new schedule version it produces.
type EventPlan struct {
	app.Plan
	Event    domain.LoanEvent
	Schedule domain.Schedule
}

// PlanEvent computes the new schedule version from the event date; earlier installments are
// untouched because the new version only covers dues from the event date on.
func (s *Service) PlanEvent(ctx context.Context, enabled module.Set, req EventRequest) (EventPlan, error) {
	if err := require(enabled); err != nil {
		return EventPlan{}, err
	}
	date, err := s.date(req.Date)
	if err != nil {
		return EventPlan{}, err
	}
	kind := domain.LoanEventKind(strings.ToLower(strings.TrimSpace(req.Kind)))
	if !kind.Valid() {
		return EventPlan{}, invalid("kind", fmt.Sprintf("unknown event kind %q", req.Kind))
	}
	var out EventPlan
	err = s.store.Read(ctx, func(r app.Repos) error {
		infos, err := loanInfos(ctx, r)
		if err != nil {
			return err
		}
		info, err := pickLoan(infos, req.Loan)
		if err != nil {
			return err
		}
		ev, err := s.newEvent(req, info.Loan, kind, date)
		if err != nil {
			return err
		}
		cur, err := r.Loans().ScheduleAt(ctx, info.Loan.ID, date)
		if err != nil {
			return fmt.Errorf("schedule at %s: %w", date, err)
		}
		if cur.ValidFrom == date {
			return &domain.Error{
				Code:    app.CodeAlreadyExists,
				Message: fmt.Sprintf("a schedule version valid from %s already exists", date),
			}
		}
		sched, err := domain.ApplyEvent(ev, cur, info.Loan)
		if err != nil {
			return err
		}
		f := map[string]any{
			"loan": info.Loan.Name, "kind": string(kind), "date": date.String(),
			"rate_bp": sched.Rate.BasisPoints(), "installments": len(sched.Installments),
		}
		out = EventPlan{
			Plan: app.Plan{Changes: []app.Change{{
				Op: app.OpCreate, Entity: "loan_event", Fields: f,
				Summary: fmt.Sprintf("%s on loan %q from %s: new schedule at %s", kind, info.Loan.Name, date, sched.Rate),
			}}},
			Event: ev, Schedule: sched,
		}
		return nil
	})
	return out, err
}

// ApplyEvent stores the event and the new schedule version.
func (s *Service) ApplyEvent(ctx context.Context, p EventPlan) error {
	return s.write(ctx, false, func(r app.Repos) error {
		if _, err := r.Loans().InsertEvent(ctx, p.Event); err != nil {
			return fmt.Errorf("store event: %w", err)
		}
		if err := r.Loans().AddScheduleVersion(ctx, p.Schedule); err != nil {
			return fmt.Errorf("store schedule: %w", err)
		}
		return nil
	})
}

// newEvent builds the loan event of req.
func (s *Service) newEvent(req EventRequest, loan domain.Loan, kind domain.LoanEventKind, date domain.Date) (domain.LoanEvent, error) {
	ev := domain.LoanEvent{LoanID: loan.ID, Kind: kind, EventDate: date, NewTermMonths: req.TermMonths}
	if req.Rate != "" {
		rate, err := parseRate(req.Rate)
		if err != nil {
			return domain.LoanEvent{}, err
		}
		ev.NewRate = &rate
	}
	if req.Prepaid != "" {
		m, err := s.amount(req.Prepaid, loan.Principal.Currency())
		if err != nil {
			return domain.LoanEvent{}, err
		}
		ev.PrepaidAmount = &m
	}
	if req.Mode != "" {
		mode := domain.PrepaymentMode(strings.ToLower(strings.TrimSpace(req.Mode)))
		if !mode.Valid() {
			return domain.LoanEvent{}, invalid("mode", fmt.Sprintf("unknown prepayment mode %q", req.Mode))
		}
		ev.PrepayMode = &mode
	}
	return ev, nil
}
