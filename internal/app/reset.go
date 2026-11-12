package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/0xballo/fui/internal/domain"
)

// OpKeep marks a Change that reports data a reset leaves in place.
const OpKeep Op = "keep"

// Entity names of the settings a reset lists next to the categories.
const (
	entityTag   = "tag"
	entityPayee = "payee"
)

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

type resetData struct {
	accounts []domain.Account
	txs      []domain.Transaction
	cats     []domain.Category
	tags     []domain.Tag
	payees   []domain.Payee
	fx       []domain.FXRate
	batches  []ImportBatch
}

// readResetData lists everything a reset counts.
func (s *Service) readResetData(ctx context.Context) (resetData, error) {
	var d resetData
	err := s.store.Read(ctx, func(r Repos) (err error) {
		if d.accounts, err = r.Accounts().List(ctx); err != nil {
			return fmt.Errorf("list accounts: %w", err)
		}
		all, err := domain.NewTxFilter(domain.TxFilterSpec{Desc: true})
		if err != nil {
			return fmt.Errorf("transaction filter: %w", err)
		}
		if d.txs, err = r.Transactions().List(ctx, all, nil, 0); err != nil {
			return fmt.Errorf("list transactions: %w", err)
		}
		if d.cats, err = r.Categories().List(ctx); err != nil {
			return fmt.Errorf("list categories: %w", err)
		}
		if d.tags, err = r.Tags().List(ctx); err != nil {
			return fmt.Errorf("list tags: %w", err)
		}
		if d.payees, err = r.Payees().List(ctx); err != nil {
			return fmt.Errorf("list payees: %w", err)
		}
		if d.fx, err = r.FXRates().List(ctx, domain.Currency{}); err != nil {
			return fmt.Errorf("list fx rates: %w", err)
		}
		if d.batches, err = r.Imports().Batches(ctx); err != nil {
			return fmt.Errorf("list import batches: %w", err)
		}
		return nil
	})
	return d, err
}

// PlanReset counts what a reset deletes and keeps; it touches nothing. An archive with no
// accounts, transactions or categories counts as nonexistent (the store creates an empty file on
// open), and the plan is then init's.
func (s *Service) PlanReset(ctx context.Context, req ResetRequest) (ResetPlan, error) {
	d, err := s.readResetData(ctx)
	if err != nil {
		return ResetPlan{}, err
	}
	accounts, txs, cats, tags, payees, fx, batches := d.accounts, d.txs, d.cats, d.tags, d.payees, d.fx, d.batches

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
		plan.add(OpKeep, entityTag, "tags", len(tags))
		plan.add(OpKeep, entityPayee, "payees", len(payees))
	} else {
		plan.add(OpDelete, EntityCategory, "categories", len(cats))
		plan.add(OpDelete, entityTag, "tags", len(tags))
		plan.add(OpDelete, entityPayee, "payees", len(payees))
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
