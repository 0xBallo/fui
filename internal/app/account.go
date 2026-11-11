package app

import (
	"context"
	"fmt"

	"github.com/0xballo/fui/internal/domain"
)

// AddAccountRequest carries user-typed values; empty OpeningBalance means 0 and empty OpenedOn today.
type AddAccountRequest struct {
	Name           string
	Description    string
	Kind           string
	Currency       string
	OpeningBalance string
	OpenedOn       string
	IBAN           string
}

// AddAccountPlan is the plan to create Account.
type AddAccountPlan struct {
	Plan
	Account domain.Account
}

// PlanAddAccount validates the request and plans the account creation.
func (s *Service) PlanAddAccount(ctx context.Context, req AddAccountRequest) (AddAccountPlan, error) {
	acct, err := s.newAccount(req)
	if err != nil {
		return AddAccountPlan{}, err
	}
	err = s.store.Read(ctx, func(r Repos) error {
		all, err := r.Accounts().List(ctx)
		if err != nil {
			return fmt.Errorf("list accounts: %w", err)
		}
		return checkAccountName(all, acct.Name, 0)
	})
	if err != nil {
		return AddAccountPlan{}, err
	}
	return AddAccountPlan{Plan: Plan{Changes: []Change{accountChange(OpCreate, acct)}}, Account: acct}, nil
}

func (s *Service) newAccount(req AddAccountRequest) (domain.Account, error) {
	kind, err := domain.ParseAccountKind(req.Kind)
	if err != nil {
		return domain.Account{}, fmt.Errorf("account kind: %w", err)
	}
	cur, err := domain.ParseCurrency(req.Currency)
	if err != nil {
		return domain.Account{}, fmt.Errorf("account currency: %w", err)
	}
	opening, err := s.amount(orDefault(req.OpeningBalance, "0"), cur)
	if err != nil {
		return domain.Account{}, err
	}
	opened, err := s.date(req.OpenedOn)
	if err != nil {
		return domain.Account{}, err
	}
	acct, err := domain.NewAccount(domain.Account{
		Name: req.Name, Description: req.Description, Kind: kind,
		Currency: cur, OpeningBalance: opening, OpenedOn: opened, IBAN: req.IBAN,
	})
	if err != nil {
		return domain.Account{}, fmt.Errorf("account: %w", err)
	}
	return acct, nil
}

// ApplyAddAccount creates the account and returns it with its ID.
func (s *Service) ApplyAddAccount(ctx context.Context, p AddAccountPlan) (domain.Account, error) {
	var out domain.Account
	err := s.apply(ctx, p.Bulk, func(r Repos) (err error) {
		out, err = r.Accounts().Insert(ctx, p.Account)
		return err
	})
	return out, err
}

// AccountBalance is an account with its balance at a date.
type AccountBalance struct {
	Account domain.Account
	Balance domain.Money
	At      domain.Date
}

// ListAccountsRequest selects the date (empty = today) and whether archived accounts are listed.
type ListAccountsRequest struct {
	At              string
	IncludeArchived bool
}

// ListAccounts returns the accounts with their balance at the requested date.
func (s *Service) ListAccounts(ctx context.Context, req ListAccountsRequest) ([]AccountBalance, error) {
	at, err := s.date(req.At)
	if err != nil {
		return nil, err
	}
	var out []AccountBalance
	err = s.store.Read(ctx, func(r Repos) error {
		all, err := r.Accounts().List(ctx)
		if err != nil {
			return fmt.Errorf("list accounts: %w", err)
		}
		if !req.IncludeArchived {
			all = activeAccounts(all)
		}
		for _, a := range all {
			b, err := balanceAt(ctx, r, a, at)
			if err != nil {
				return err
			}
			out = append(out, AccountBalance{Account: a, Balance: b, At: at})
		}
		return nil
	})
	return out, err
}

// ShowAccount returns one account (archived included) with its balance at at (empty = today).
func (s *Service) ShowAccount(ctx context.Context, ref, at string) (AccountBalance, error) {
	d, err := s.date(at)
	if err != nil {
		return AccountBalance{}, err
	}
	var out AccountBalance
	err = s.store.Read(ctx, func(r Repos) error {
		all, err := r.Accounts().List(ctx)
		if err != nil {
			return fmt.Errorf("list accounts: %w", err)
		}
		a, err := pickAccount(all, ref)
		if err != nil {
			return err
		}
		b, err := balanceAt(ctx, r, a, d)
		out = AccountBalance{Account: a, Balance: b, At: d}
		return err
	})
	return out, err
}

// balanceAt is the opening balance plus every movement dated on or before at.
func balanceAt(ctx context.Context, r Repos, a domain.Account, at domain.Date) (domain.Money, error) {
	sum, err := r.Transactions().SumUpTo(ctx, a.ID, at)
	if err != nil {
		return domain.Money{}, fmt.Errorf("sum movements of %q: %w", a.Name, err)
	}
	moves, err := domain.NewMoney(sum, a.Currency)
	if err != nil {
		return domain.Money{}, fmt.Errorf("balance of %q: %w", a.Name, err)
	}
	b, err := a.OpeningBalance.Add(moves)
	if err != nil {
		return domain.Money{}, fmt.Errorf("balance of %q: %w", a.Name, err)
	}
	return b, nil
}

// EditAccountRequest changes the non-nil fields of the account named by Account.
type EditAccountRequest struct {
	Account        string
	Name           *string
	Description    *string
	Kind           *string
	OpeningBalance *string
	IBAN           *string
}

// EditAccountPlan is the plan to store Account.
type EditAccountPlan struct {
	Plan
	Account domain.Account
}

// PlanEditAccount plans the edit of an active account.
func (s *Service) PlanEditAccount(ctx context.Context, req EditAccountRequest) (EditAccountPlan, error) {
	var out EditAccountPlan
	err := s.store.Read(ctx, func(r Repos) error {
		all, err := r.Accounts().List(ctx)
		if err != nil {
			return fmt.Errorf("list accounts: %w", err)
		}
		cur, err := pickAccount(activeAccounts(all), req.Account)
		if err != nil {
			return err
		}
		edited, err := s.editedAccount(cur, req)
		if err != nil {
			return err
		}
		if err := checkAccountName(all, edited.Name, cur.ID); err != nil {
			return err
		}
		out = EditAccountPlan{Plan: Plan{Changes: []Change{accountChange(OpUpdate, edited)}}, Account: edited}
		return nil
	})
	return out, err
}

func (s *Service) editedAccount(a domain.Account, req EditAccountRequest) (domain.Account, error) {
	if req.Name == nil && req.Description == nil && req.Kind == nil && req.OpeningBalance == nil && req.IBAN == nil {
		return domain.Account{}, newError(domain.ErrMissingField, map[string]any{"field": "changes"}, "nothing to change")
	}
	if req.Name != nil {
		a.Name = *req.Name
	}
	if req.Description != nil {
		a.Description = *req.Description
	}
	if req.Kind != nil {
		k, err := domain.ParseAccountKind(*req.Kind)
		if err != nil {
			return domain.Account{}, fmt.Errorf("account kind: %w", err)
		}
		a.Kind = k
	}
	if req.OpeningBalance != nil {
		m, err := s.amount(*req.OpeningBalance, a.Currency)
		if err != nil {
			return domain.Account{}, err
		}
		a.OpeningBalance = m
	}
	if req.IBAN != nil {
		a.IBAN = *req.IBAN
	}
	edited, err := domain.NewAccount(a)
	if err != nil {
		return domain.Account{}, fmt.Errorf("account: %w", err)
	}
	return edited, nil
}

// ApplyEditAccount stores the edited account.
func (s *Service) ApplyEditAccount(ctx context.Context, p EditAccountPlan) (domain.Account, error) {
	return s.updateAccount(ctx, p.Bulk, p.Account)
}

// ArchiveAccountPlan is the plan to store Account as archived.
type ArchiveAccountPlan struct {
	Plan
	Account domain.Account
}

// PlanArchiveAccount plans archiving the active account named by ref.
func (s *Service) PlanArchiveAccount(ctx context.Context, ref string) (ArchiveAccountPlan, error) {
	var out ArchiveAccountPlan
	err := s.store.Read(ctx, func(r Repos) error {
		all, err := r.Accounts().List(ctx)
		if err != nil {
			return fmt.Errorf("list accounts: %w", err)
		}
		a, err := pickAccount(activeAccounts(all), ref)
		if err != nil {
			return err
		}
		a.Archived = true
		out = ArchiveAccountPlan{Plan: Plan{Changes: []Change{accountChange(OpUpdate, a)}}, Account: a}
		return nil
	})
	return out, err
}

// ApplyArchiveAccount archives the account.
func (s *Service) ApplyArchiveAccount(ctx context.Context, p ArchiveAccountPlan) (domain.Account, error) {
	return s.updateAccount(ctx, p.Bulk, p.Account)
}

func (s *Service) updateAccount(ctx context.Context, bulk bool, a domain.Account) (domain.Account, error) {
	err := s.apply(ctx, bulk, func(r Repos) error {
		if err := r.Accounts().Update(ctx, a); err != nil {
			return fmt.Errorf("update account %q: %w", a.Name, err)
		}
		return nil
	})
	return a, err
}

// checkAccountName fails with ErrAlreadyExists when an account other than self has the same name.
func checkAccountName(all []domain.Account, name string, self domain.AccountID) error {
	for _, a := range all {
		if a.ID != self && equalNames(a.Name, name) {
			return newError(ErrAlreadyExists, map[string]any{"entity": EntityAccount, "name": a.Name},
				"an account named %q already exists", a.Name)
		}
	}
	return nil
}

func accountChange(op Op, a domain.Account) Change {
	f := map[string]any{"name": a.Name, "kind": string(a.Kind), "currency": a.Currency.Code(), "archived": a.Archived}
	moneyFields(f, "opening_balance", a.OpeningBalance)
	f["opened_on"] = a.OpenedOn.String()
	if a.Description != "" {
		f["description"] = a.Description
	}
	if a.IBAN != "" {
		f["iban"] = a.IBAN
	}
	return Change{Op: op, Entity: EntityAccount, Summary: fmt.Sprintf("%s account %q (%s, %s)", op, a.Name, a.Kind, a.Currency), Fields: f}
}

func categoryCreate(c domain.Category) Change {
	return Change{
		Op: OpCreate, Entity: EntityCategory,
		Summary: fmt.Sprintf("create %s category %q", c.Kind, c.Name),
		Fields:  map[string]any{"name": c.Name, "kind": string(c.Kind)},
	}
}
