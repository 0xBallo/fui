// Package recurring holds the use cases of periodic transaction templates: add/edit/remove/list,
// materialising due occurrences as pending transactions (Run), skipping the next one and
// confirming a pending transaction into a booked one. Recurrences belong to the recurring module:
// every use case here requires it enabled.
package recurring

import (
	"context"
	"fmt"
	"time"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
)

// EntityRecurrence is the Change.Entity of a recurrence.
const EntityRecurrence = "recurrence"

// Config holds the user settings the use cases depend on.
type Config struct {
	Locale   domain.Locale  // number and date input format; default en-US
	Location *time.Location // zone that defines "today"; default time.Local
}

// Service runs the recurrence use cases over a Store.
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

// today returns the current date in the configured zone.
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

// optionalDate parses a user-typed date that may be absent; an empty input means the zero Date
// (no end / no value), not today.
func (s *Service) optionalDate(in string) (domain.Date, error) {
	if in == "" {
		return domain.Date{}, nil
	}
	return s.date(in)
}

// amount parses a user-typed amount in currency c.
func (s *Service) amount(in string, c domain.Currency) (domain.Money, error) {
	m, err := domain.ParseAmount(in, s.cfg.Locale, c)
	if err != nil {
		return domain.Money{}, fmt.Errorf("amount: %w", err)
	}
	return m, nil
}

// AddRequest carries user-typed values. Account, Amount, Every and Unit are required. Start
// defaults to today; End empty means no end; Category is optional.
type AddRequest struct {
	Account     string
	Category    string
	Description string
	Amount      string
	Every       int
	Unit        string
	DayOfMonth  int
	Start       string
	End         string
}

// RecurrencePlan is the plan to store Recurrence (insert, update or delete).
type RecurrencePlan struct {
	app.Plan
	Recurrence app.Recurrence
}

// PlanAdd resolves names, parses the amount and dates and plans the recurrence creation.
func (s *Service) PlanAdd(ctx context.Context, enabled module.Set, req AddRequest) (RecurrencePlan, error) {
	if err := app.WrapModuleError(enabled.Require(module.Recurring)); err != nil {
		return RecurrencePlan{}, err
	}
	var out RecurrencePlan
	err := s.store.Read(ctx, func(r app.Repos) error {
		accts, err := r.Accounts().List(ctx)
		if err != nil {
			return fmt.Errorf("list accounts: %w", err)
		}
		acct, err := pickAccount(activeAccounts(accts), req.Account)
		if err != nil {
			return err
		}
		amount, err := s.amount(req.Amount, acct.Currency)
		if err != nil {
			return err
		}
		start, err := s.date(req.Start)
		if err != nil {
			return err
		}
		end, err := s.optionalDate(req.End)
		if err != nil {
			return err
		}
		unit, err := domain.ParseRecurrenceUnit(req.Unit)
		if err != nil {
			return fmt.Errorf("unit: %w", err)
		}
		rule, err := domain.NewRecurrenceRule(req.Every, unit, req.DayOfMonth)
		if err != nil {
			return err
		}
		catID, catName, err := resolveCategory(ctx, r, req.Category)
		if err != nil {
			return err
		}
		dom, err := domain.NewRecurrence(domain.Recurrence{
			AccountID: acct.ID, CategoryID: catID, Description: req.Description, Amount: amount,
			Rule: rule, Start: start, End: end, NextDate: start,
		})
		if err != nil {
			return fmt.Errorf("recurrence: %w", err)
		}
		rec := fromDomain(0, dom)
		out = RecurrencePlan{Plan: change(app.OpCreate, rec, acct.Name, catName), Recurrence: rec}
		return nil
	})
	return out, err
}

// ApplyAdd stores the recurrence of the plan and returns it with its ID assigned.
func (s *Service) ApplyAdd(ctx context.Context, p RecurrencePlan) (app.Recurrence, error) {
	var out app.Recurrence
	err := s.store.WithTx(ctx, func(r app.Repos) (err error) {
		out, err = r.Recurrences().Insert(ctx, p.Recurrence)
		return err
	})
	if err != nil {
		return app.Recurrence{}, fmt.Errorf("add recurrence: %w", err)
	}
	return out, nil
}

// EditRequest changes the non-nil fields of recurrence ID. Category and End accept "" to clear
// the category or remove the end date. The account, the start date and the next date cannot be
// changed: use Skip to move the next date past an occurrence.
type EditRequest struct {
	ID          app.RecurrenceID
	Category    *string
	Description *string
	Amount      *string
	Every       *int
	Unit        *string
	DayOfMonth  *int
	End         *string
}

// PlanEdit plans the edit of an existing recurrence.
func (s *Service) PlanEdit(ctx context.Context, enabled module.Set, req EditRequest) (RecurrencePlan, error) {
	if err := app.WrapModuleError(enabled.Require(module.Recurring)); err != nil {
		return RecurrencePlan{}, err
	}
	if req.Category == nil && req.Description == nil && req.Amount == nil &&
		req.Every == nil && req.Unit == nil && req.DayOfMonth == nil && req.End == nil {
		return RecurrencePlan{}, &domain.Error{
			Code: domain.CodeMissingField, Message: "nothing to change",
			Details: map[string]any{"field": "changes"},
		}
	}
	var out RecurrencePlan
	err := s.store.Read(ctx, func(r app.Repos) error {
		cur, err := r.Recurrences().Get(ctx, req.ID)
		if err != nil {
			return fmt.Errorf("get recurrence %d: %w", req.ID, err)
		}
		acct, err := r.Accounts().Get(ctx, cur.AccountID)
		if err != nil {
			return fmt.Errorf("get account: %w", err)
		}
		catName, err := s.editFields(ctx, r, &cur, req, acct)
		if err != nil {
			return err
		}
		dom, err := domain.NewRecurrence(toDomain(cur))
		if err != nil {
			return fmt.Errorf("recurrence: %w", err)
		}
		cur = fromDomain(cur.ID, dom)
		out = RecurrencePlan{Plan: change(app.OpUpdate, cur, acct.Name, catName), Recurrence: cur}
		return nil
	})
	return out, err
}

// editFields applies the non-nil fields of req to cur and returns the category name to render.
func (s *Service) editFields(ctx context.Context, r app.Repos, cur *app.Recurrence, req EditRequest, acct domain.Account) (string, error) {
	catName := ""
	if cur.CategoryID != 0 {
		c, err := r.Categories().Get(ctx, cur.CategoryID)
		if err != nil {
			return "", fmt.Errorf("get category: %w", err)
		}
		catName = c.Name
	}
	if req.Description != nil {
		cur.Description = *req.Description
	}
	if req.Amount != nil {
		amount, err := s.amount(*req.Amount, acct.Currency)
		if err != nil {
			return "", err
		}
		cur.Amount = amount
	}
	if err := applyRule(&cur.Rule, req); err != nil {
		return "", err
	}
	if req.Category != nil {
		catID, name, err := resolveCategory(ctx, r, *req.Category)
		if err != nil {
			return "", err
		}
		cur.CategoryID, catName = catID, name
	}
	if req.End != nil {
		end, err := s.optionalDate(*req.End)
		if err != nil {
			return "", err
		}
		cur.End = end
	}
	return catName, nil
}

// applyRule merges the rule-related fields of req into rule and revalidates it when any changed.
func applyRule(rule *domain.RecurrenceRule, req EditRequest) error {
	if req.Every == nil && req.Unit == nil && req.DayOfMonth == nil {
		return nil
	}
	every, unit, dayOfMonth := rule.Every, rule.Unit, rule.DayOfMonth
	if req.Every != nil {
		every = *req.Every
	}
	if req.Unit != nil {
		u, err := domain.ParseRecurrenceUnit(*req.Unit)
		if err != nil {
			return fmt.Errorf("unit: %w", err)
		}
		unit = u
	}
	if req.DayOfMonth != nil {
		dayOfMonth = *req.DayOfMonth
	}
	next, err := domain.NewRecurrenceRule(every, unit, dayOfMonth)
	if err != nil {
		return err
	}
	*rule = next
	return nil
}

// ApplyEdit stores the recurrence of the plan.
func (s *Service) ApplyEdit(ctx context.Context, p RecurrencePlan) error {
	return s.write(ctx, func(r app.Repos) error { return r.Recurrences().Update(ctx, p.Recurrence) })
}

// PlanRemove plans the removal of recurrence id.
func (s *Service) PlanRemove(ctx context.Context, enabled module.Set, id app.RecurrenceID) (RecurrencePlan, error) {
	if err := app.WrapModuleError(enabled.Require(module.Recurring)); err != nil {
		return RecurrencePlan{}, err
	}
	var out RecurrencePlan
	err := s.store.Read(ctx, func(r app.Repos) error {
		rec, err := r.Recurrences().Get(ctx, id)
		if err != nil {
			return fmt.Errorf("get recurrence %d: %w", id, err)
		}
		acct, err := r.Accounts().Get(ctx, rec.AccountID)
		if err != nil {
			return fmt.Errorf("get account: %w", err)
		}
		catName := ""
		if rec.CategoryID != 0 {
			c, err := r.Categories().Get(ctx, rec.CategoryID)
			if err != nil {
				return fmt.Errorf("get category: %w", err)
			}
			catName = c.Name
		}
		out = RecurrencePlan{Plan: change(app.OpDelete, rec, acct.Name, catName), Recurrence: rec}
		return nil
	})
	return out, err
}

// ApplyRemove deletes the recurrence of the plan. Transactions it already created keep their
// history; the store detaches their recurrence_id.
func (s *Service) ApplyRemove(ctx context.Context, p RecurrencePlan) error {
	return s.write(ctx, func(r app.Repos) error { return r.Recurrences().Delete(ctx, p.Recurrence.ID) })
}

// List returns every recurrence ordered by ID.
func (s *Service) List(ctx context.Context, enabled module.Set) ([]app.Recurrence, error) {
	if err := app.WrapModuleError(enabled.Require(module.Recurring)); err != nil {
		return nil, err
	}
	var out []app.Recurrence
	err := s.store.Read(ctx, func(r app.Repos) (err error) {
		out, err = r.Recurrences().List(ctx)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("list recurrences: %w", err)
	}
	return out, nil
}

func (s *Service) write(ctx context.Context, fn func(app.Repos) error) error {
	if err := s.store.WithTx(ctx, fn); err != nil {
		return fmt.Errorf("apply: %w", err)
	}
	return nil
}

// resolveCategory resolves an optional user-typed category reference; "" means none.
func resolveCategory(ctx context.Context, r app.Repos, ref string) (domain.CategoryID, string, error) {
	if ref == "" {
		return 0, "", nil
	}
	cats, err := r.Categories().List(ctx)
	if err != nil {
		return 0, "", fmt.Errorf("list categories: %w", err)
	}
	c, err := pickCategory(cats, ref)
	if err != nil {
		return 0, "", err
	}
	return c.ID, c.Name, nil
}

// pickAccount resolves a user-typed account reference among accts.
func pickAccount(accts []domain.Account, ref string) (domain.Account, error) {
	names := make([]string, len(accts))
	for i, a := range accts {
		names[i] = a.Name
	}
	i, err := domain.Resolve(ref, names)
	if err != nil {
		return domain.Account{}, fmt.Errorf("account: %w", err)
	}
	return accts[i], nil
}

// pickCategory resolves a user-typed category reference among cats.
func pickCategory(cats []domain.Category, ref string) (domain.Category, error) {
	names := make([]string, len(cats))
	for i, c := range cats {
		names[i] = c.Name
	}
	i, err := domain.Resolve(ref, names)
	if err != nil {
		return domain.Category{}, fmt.Errorf("category: %w", err)
	}
	return cats[i], nil
}

// activeAccounts returns the accounts that are not archived.
func activeAccounts(all []domain.Account) []domain.Account {
	out := make([]domain.Account, 0, len(all))
	for _, a := range all {
		if !a.Archived {
			out = append(out, a)
		}
	}
	return out
}

// toDomain converts an app.Recurrence to the domain type the recurrence rules operate on.
func toDomain(r app.Recurrence) domain.Recurrence {
	return domain.Recurrence{
		AccountID: r.AccountID, CategoryID: r.CategoryID, Description: r.Description, Amount: r.Amount,
		Rule: r.Rule, Start: r.Start, End: r.End, NextDate: r.NextDate,
	}
}

// fromDomain converts a domain.Recurrence back to the app.Recurrence stored by the repo, with id.
func fromDomain(id app.RecurrenceID, d domain.Recurrence) app.Recurrence {
	return app.Recurrence{
		ID: id, AccountID: d.AccountID, CategoryID: d.CategoryID, Description: d.Description, Amount: d.Amount,
		Rule: d.Rule, Start: d.Start, End: d.End, NextDate: d.NextDate,
	}
}

// change builds the plan Change for a recurrence op.
func change(op app.Op, rec app.Recurrence, acctName, catName string) app.Plan {
	f := map[string]any{
		"account": acctName, "category": catName, "description": rec.Description,
		"every": rec.Rule.Every, "unit": string(rec.Rule.Unit), "day_of_month": rec.Rule.DayOfMonth,
		"start": rec.Start.String(), "next_date": rec.NextDate.String(),
	}
	moneyFields(f, "amount", rec.Amount)
	if !rec.End.IsZero() {
		f["end"] = rec.End.String()
	}
	summary := fmt.Sprintf("%s recurrence %q", op, rec.Description)
	if rec.Description == "" {
		summary = fmt.Sprintf("%s recurrence %d", op, rec.ID)
	}
	return app.Plan{Changes: []app.Change{{Op: op, Entity: EntityRecurrence, Summary: summary, Fields: f}}}
}

// moneyFields flattens m into the keys prefix and prefix_currency of fields.
func moneyFields(fields map[string]any, prefix string, m domain.Money) {
	fields[prefix] = m.Minor()
	fields[prefix+"_currency"] = m.Currency().Code()
}
