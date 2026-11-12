package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/0xballo/fui/internal/domain"
)

// OpKeep marks a Change that reports data a reset leaves in place.
const OpKeep Op = "keep"

// ResetRequest says what the reset keeps.
type ResetRequest struct {
	KeepSettings bool
}

// ResetPlan is the plan to bring the archive back to its initial state. Changes lists what is
// deleted (OpDelete) and what is kept (OpKeep). Init is set instead when the archive does not
// exist yet: a reset then behaves like init.
type ResetPlan struct {
	Plan
	KeepSettings bool
	Init         *InitPlan
}

// ResetResult reports what ApplyReset did.
type ResetResult struct {
	// Initialized is true when the reset was delegated to init because there was no archive.
	Initialized bool
	// Created counts the categories init created (only when Initialized).
	Created int
}

// PlanReset counts what a reset deletes and keeps; it touches nothing. An archive with no
// accounts, transactions or categories counts as nonexistent (the store creates an empty file on
// open), and the plan is then init's.
func (s *Service) PlanReset(ctx context.Context, req ResetRequest) (ResetPlan, error) {
	var (
		accounts []domain.Account
		txs      []domain.Transaction
		cats     []domain.Category
		fx       []domain.FXRate
		batches  []ImportBatch
	)
	err := s.store.Read(ctx, func(r Repos) (err error) {
		if accounts, err = r.Accounts().List(ctx); err != nil {
			return fmt.Errorf("list accounts: %w", err)
		}
		if txs, err = r.Transactions().List(ctx, TxFilter{}, nil, 0); err != nil {
			return fmt.Errorf("list transactions: %w", err)
		}
		if cats, err = r.Categories().List(ctx); err != nil {
			return fmt.Errorf("list categories: %w", err)
		}
		if fx, err = r.FXRates().List(ctx, domain.Currency{}); err != nil {
			return fmt.Errorf("list fx rates: %w", err)
		}
		if batches, err = r.Imports().Batches(ctx); err != nil {
			return fmt.Errorf("list import batches: %w", err)
		}
		return nil
	})
	if err != nil {
		return ResetPlan{}, err
	}

	if len(accounts)+len(txs)+len(cats) == 0 {
		initPlan, err := s.PlanInit(ctx)
		if err != nil {
			return ResetPlan{}, err
		}
		return ResetPlan{Plan: initPlan.Plan, KeepSettings: req.KeepSettings, Init: &initPlan}, nil
	}

	plan := ResetPlan{Plan: Plan{Bulk: true}, KeepSettings: req.KeepSettings}
	plan.add(OpDelete, EntityAccount, "accounts", len(accounts))
	plan.add(OpDelete, EntityTransaction, "transactions", len(txs))
	plan.add(OpDelete, EntityFXRate, "exchange rates", len(fx))
	plan.add(OpDelete, "import_batch", "import batches", len(batches))
	if req.KeepSettings {
		plan.add(OpKeep, EntityCategory, "categories", len(cats))
	} else {
		plan.add(OpDelete, EntityCategory, "categories", len(cats))
		defaults := len(domain.DefaultCategories())
		plan.Changes = append(plan.Changes, Change{
			Op: OpCreate, Entity: EntityCategory,
			Summary: fmt.Sprintf("create %d default categories", defaults),
			Fields:  map[string]any{"count": defaults},
		})
	}
	return plan, nil
}

func (p *ResetPlan) add(op Op, entity, label string, n int) {
	if n == 0 {
		return
	}
	verb := "delete"
	if op == OpKeep {
		verb = "keep"
	}
	p.Changes = append(p.Changes, Change{
		Op: op, Entity: entity, Summary: fmt.Sprintf("%s %d %s", verb, n, label),
		Fields: map[string]any{"count": n},
	})
}

// ApplyReset runs the plan: a backup with reason "reset" that rotation never deletes, then the
// archive is replaced by a fresh one. A plan delegated to init applies init instead.
func (s *Service) ApplyReset(ctx context.Context, p ResetPlan) (ResetResult, error) {
	if p.Init != nil {
		res, err := s.ApplyInit(ctx, *p.Init)
		return ResetResult{Initialized: true, Created: res.Created}, err
	}
	rs, ok := s.backup.(Resetter)
	if !ok {
		return ResetResult{}, errors.New("reset: the archive adapter does not support reset")
	}
	if err := rs.Reset(ctx, ResetOptions{KeepSettings: p.KeepSettings}); err != nil {
		return ResetResult{}, fmt.Errorf("reset: %w", err)
	}
	return ResetResult{}, nil
}
