package app

import (
	"context"
	"fmt"

	"github.com/0xballo/fui/internal/domain"
)

// AddPayeeRequest creates a payee, optionally with starting aliases.
type AddPayeeRequest struct {
	Name    string
	Aliases []string
}

// AddPayeePlan is the plan to create Payee.
type AddPayeePlan struct {
	Plan
	Payee domain.Payee
}

// PlanAddPayee validates the request and plans the creation.
func (s *Service) PlanAddPayee(ctx context.Context, req AddPayeeRequest) (AddPayeePlan, error) {
	p, err := domain.NewPayee(req.Name, req.Aliases...)
	if err != nil {
		return AddPayeePlan{}, fmt.Errorf("payee: %w", err)
	}
	err = s.store.Read(ctx, func(r Repos) error {
		payees, err := r.Payees().List(ctx)
		if err != nil {
			return fmt.Errorf("list payees: %w", err)
		}
		return checkPayeeNames(payees, p, 0)
	})
	if err != nil {
		return AddPayeePlan{}, err
	}
	ch := Change{Op: OpCreate, Entity: EntityPayee, Summary: fmt.Sprintf("add payee %q", p.Name), Fields: map[string]any{"name": p.Name}}
	return AddPayeePlan{Plan: Plan{Changes: []Change{ch}}, Payee: p}, nil
}

// ApplyAddPayee creates the payee and returns it with its ID.
func (s *Service) ApplyAddPayee(ctx context.Context, p AddPayeePlan) (domain.Payee, error) {
	var out domain.Payee
	err := s.apply(ctx, p.Bulk, func(r Repos) (err error) {
		out, err = r.Payees().Insert(ctx, p.Payee)
		return err
	})
	return out, err
}

// ListPayees returns every payee.
func (s *Service) ListPayees(ctx context.Context) ([]domain.Payee, error) {
	var out []domain.Payee
	err := s.store.Read(ctx, func(r Repos) (err error) {
		out, err = r.Payees().List(ctx)
		return err
	})
	return out, err
}

// RenamePayeeRequest renames the payee named by Payee and, when NewAliases is non-nil, replaces
// its aliases.
type RenamePayeeRequest struct {
	Payee      string
	NewName    string
	NewAliases []string
}

// RenamePayeePlan is the plan to store Payee.
type RenamePayeePlan struct {
	Plan
	Payee domain.Payee
}

// PlanRenamePayee plans the rename.
func (s *Service) PlanRenamePayee(ctx context.Context, req RenamePayeeRequest) (RenamePayeePlan, error) {
	var out RenamePayeePlan
	err := s.store.Read(ctx, func(r Repos) error {
		payees, err := r.Payees().List(ctx)
		if err != nil {
			return fmt.Errorf("list payees: %w", err)
		}
		cur, err := pickPayee(payees, req.Payee)
		if err != nil {
			return err
		}
		aliases := req.NewAliases
		if aliases == nil {
			aliases = cur.Aliases
		}
		renamed, err := domain.NewPayee(req.NewName, aliases...)
		if err != nil {
			return fmt.Errorf("payee: %w", err)
		}
		renamed.ID = cur.ID
		if err := checkPayeeNames(payees, renamed, cur.ID); err != nil {
			return err
		}
		ch := Change{
			Op: OpUpdate, Entity: EntityPayee,
			Summary: fmt.Sprintf("rename payee %q to %q", cur.Name, renamed.Name),
			Fields:  map[string]any{"id": int64(cur.ID), "name": renamed.Name, "previous_name": cur.Name},
		}
		out = RenamePayeePlan{Plan: Plan{Changes: []Change{ch}}, Payee: renamed}
		return nil
	})
	return out, err
}

// ApplyRenamePayee stores the renamed payee.
func (s *Service) ApplyRenamePayee(ctx context.Context, p RenamePayeePlan) (domain.Payee, error) {
	err := s.apply(ctx, p.Bulk, func(r Repos) error {
		if err := r.Payees().Update(ctx, p.Payee); err != nil {
			return fmt.Errorf("rename payee %d: %w", p.Payee.ID, err)
		}
		return nil
	})
	return p.Payee, err
}

// RemovePayeeRequest removes Payee; it fails with ErrInUse while any transaction references it.
type RemovePayeeRequest struct {
	Payee string
}

// RemovePayeePlan deletes Payee.
type RemovePayeePlan struct {
	Plan
	Payee domain.Payee
}

// PlanRemovePayee plans the removal.
func (s *Service) PlanRemovePayee(ctx context.Context, req RemovePayeeRequest) (RemovePayeePlan, error) {
	var out RemovePayeePlan
	err := s.store.Read(ctx, func(r Repos) error {
		payees, err := r.Payees().List(ctx)
		if err != nil {
			return fmt.Errorf("list payees: %w", err)
		}
		p, err := pickPayee(payees, req.Payee)
		if err != nil {
			return err
		}
		ch := Change{
			Op: OpDelete, Entity: EntityPayee, Summary: fmt.Sprintf("delete payee %q", p.Name),
			Fields: map[string]any{"id": int64(p.ID), "name": p.Name},
		}
		out = RemovePayeePlan{Plan: Plan{Changes: []Change{ch}}, Payee: p}
		return nil
	})
	return out, err
}

// ApplyRemovePayee deletes the payee; it fails with ErrInUse while any transaction references it.
func (s *Service) ApplyRemovePayee(ctx context.Context, p RemovePayeePlan) error {
	return s.apply(ctx, p.Bulk, func(r Repos) error {
		if err := r.Payees().Delete(ctx, p.Payee.ID); err != nil {
			return fmt.Errorf("delete payee %q: %w", p.Payee.Name, err)
		}
		return nil
	})
}

// MergePayeeRequest reassigns every transaction paid to Source to Target, keeps Source's name as
// an alias of Target, and removes Source.
type MergePayeeRequest struct {
	Source string
	Target string
}

// MergePayeePlan merges Source into Target. Moved is how many transactions were reassigned.
type MergePayeePlan struct {
	Plan
	Source domain.Payee
	Target domain.Payee
	Moved  int
}

// PlanMergePayee plans the merge.
func (s *Service) PlanMergePayee(ctx context.Context, req MergePayeeRequest) (MergePayeePlan, error) {
	var out MergePayeePlan
	err := s.store.Read(ctx, func(r Repos) error {
		payees, err := r.Payees().List(ctx)
		if err != nil {
			return fmt.Errorf("list payees: %w", err)
		}
		src, err := pickPayee(payees, req.Source)
		if err != nil {
			return err
		}
		dst, err := pickPayee(payees, req.Target)
		if err != nil {
			return err
		}
		if src.ID == dst.ID {
			return newError(domain.ErrInvalidValue, map[string]any{"field": "target", "value": req.Target},
				"cannot merge payee %q into itself", src.Name)
		}
		f, err := domain.NewTxFilter(domain.TxFilterSpec{PayeeIDs: []domain.PayeeID{src.ID}})
		if err != nil {
			return err
		}
		txs, err := r.Transactions().List(ctx, f, nil, 0)
		if err != nil {
			return fmt.Errorf("list transactions paid to %q: %w", src.Name, err)
		}
		out = MergePayeePlan{Source: src, Target: dst, Moved: len(txs), Plan: Plan{Bulk: len(txs) > 0}}
		out.Changes = append(out.Changes, Change{
			Op: OpUpdate, Entity: EntityPayee,
			Summary: fmt.Sprintf("reassign %d transactions from %q to %q", len(txs), src.Name, dst.Name),
			Fields:  map[string]any{"source": src.Name, "target": dst.Name, "count": len(txs)},
		}, Change{
			Op: OpDelete, Entity: EntityPayee, Summary: fmt.Sprintf("delete payee %q", src.Name),
			Fields: map[string]any{"id": int64(src.ID), "name": src.Name},
		})
		return nil
	})
	return out, err
}

// ApplyMergePayee reassigns every transaction paid to Source to Target, keeps Source's name as an
// alias of Target, then deletes Source.
func (s *Service) ApplyMergePayee(ctx context.Context, p MergePayeePlan) error {
	return s.apply(ctx, p.Bulk, func(r Repos) error {
		f, err := domain.NewTxFilter(domain.TxFilterSpec{PayeeIDs: []domain.PayeeID{p.Source.ID}})
		if err != nil {
			return err
		}
		txs, err := r.Transactions().List(ctx, f, nil, 0)
		if err != nil {
			return fmt.Errorf("list transactions paid to %q: %w", p.Source.Name, err)
		}
		for _, tx := range txs {
			tx.PayeeID = p.Target.ID
			if err := r.Transactions().Update(ctx, tx); err != nil {
				return fmt.Errorf("reassign transaction %d: %w", tx.ID, err)
			}
		}
		target := p.Target
		if !target.MatchesAlias(p.Source.Name) {
			target.Aliases = append(append([]string{}, target.Aliases...), p.Source.Name)
			if err := r.Payees().Update(ctx, target); err != nil {
				return fmt.Errorf("alias payee %q: %w", target.Name, err)
			}
		}
		if err := r.Payees().Delete(ctx, p.Source.ID); err != nil {
			return fmt.Errorf("delete payee %q: %w", p.Source.Name, err)
		}
		return nil
	})
}

// checkPayeeNames fails with ErrAlreadyExists when another payee's name or any alias collides
// (case-insensitively) with p's name or aliases.
func checkPayeeNames(payees []domain.Payee, p domain.Payee, self domain.PayeeID) error {
	for _, other := range payees {
		if other.ID == self {
			continue
		}
		if other.MatchesAlias(p.Name) {
			return newError(ErrAlreadyExists, map[string]any{"entity": EntityPayee, "name": other.Name},
				"a payee named %q already exists", other.Name)
		}
		for _, a := range p.Aliases {
			if other.MatchesAlias(a) {
				return newError(ErrAlreadyExists, map[string]any{"entity": EntityPayee, "name": other.Name},
					"alias %q is already used by payee %q", a, other.Name)
			}
		}
	}
	return nil
}
