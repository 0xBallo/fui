package rules

import (
	"context"
	"fmt"
	"slices"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

// TestRequest asks which rules would act on a transaction. Give TxID to test an archived one, or
// Text, Account and AmountMinor (signed) to test a hypothetical one. Nothing is written.
type TestRequest struct {
	TxID        domain.TransactionID
	Text        string
	Account     string
	AmountMinor int64
}

// Test runs the enabled rules over the described transaction.
func (s *Service) Test(ctx context.Context, req TestRequest) (domain.RuleApplyResult, error) {
	var out domain.RuleApplyResult
	err := s.store.Read(ctx, func(r app.Repos) error {
		tx, err := testTx(ctx, r, req)
		if err != nil {
			return err
		}
		rules, err := r.Rules().ListEnabledByPriority(ctx)
		if err != nil {
			return fmt.Errorf("list rules: %w", err)
		}
		out, err = domain.Apply(rules, tx, domain.NormalizeDescription(tx.Description))
		return err
	})
	return out, err
}

func testTx(ctx context.Context, r app.Repos, req TestRequest) (domain.Transaction, error) {
	if req.TxID != 0 {
		tx, err := r.Transactions().Get(ctx, req.TxID)
		if err != nil {
			return tx, fmt.Errorf("get transaction %d: %w", req.TxID, err)
		}
		return tx, nil
	}
	accts, err := r.Accounts().List(ctx)
	if err != nil {
		return domain.Transaction{}, fmt.Errorf("list accounts: %w", err)
	}
	names := make([]string, len(accts))
	for i, a := range accts {
		names[i] = a.Name
	}
	i, err := domain.Resolve(req.Account, names)
	if err != nil {
		return domain.Transaction{}, fmt.Errorf("account: %w", err)
	}
	m, err := domain.NewMoney(req.AmountMinor, accts[i].Currency)
	if err != nil {
		return domain.Transaction{}, fmt.Errorf("amount: %w", err)
	}
	return domain.Transaction{AccountID: accts[i].ID, Amount: m, Description: req.Text}, nil
}

// ApplyRequest applies the enabled rules to the transactions matching Filter. Rules only fill
// what is unset: a category or payee already present is kept, and tags are added to the existing
// ones, so re-applying changes nothing. Transfer legs are skipped, and so are reconciled
// transactions unless ConfirmReconciled is set.
type ApplyRequest struct {
	Filter            domain.TxFilter
	ConfirmReconciled bool
}

// TxEffect is the outcome of the rules on one transaction.
type TxEffect struct {
	Tx     domain.Transaction // with category and payee filled
	TagIDs []domain.TagID     // full tag set to store, when Result.TagsRule is set
	Result domain.RuleApplyResult
}

// ApplyPlan lists the transactions the rules would change; Skipped counts the reconciled ones left alone.
type ApplyPlan struct {
	app.Plan
	Effects []TxEffect
	Skipped int
}

// PlanApply computes the effects of the rules on the matching transactions, without writing.
func (s *Service) PlanApply(ctx context.Context, req ApplyRequest) (ApplyPlan, error) {
	var out ApplyPlan
	err := s.store.Read(ctx, func(r app.Repos) error {
		rules, err := r.Rules().ListEnabledByPriority(ctx)
		if err != nil {
			return fmt.Errorf("list rules: %w", err)
		}
		txs, err := r.Transactions().List(ctx, req.Filter, nil, 0)
		if err != nil {
			return fmt.Errorf("list transactions: %w", err)
		}
		for _, tx := range txs {
			if tx.TransferGroupID != 0 {
				continue
			}
			e, changed, err := effect(ctx, r, rules, tx)
			if err != nil {
				return err
			}
			if !changed {
				continue
			}
			if tx.Status == domain.StatusReconciled && !req.ConfirmReconciled {
				out.Skipped++
				continue
			}
			out.Effects = append(out.Effects, e)
			out.Changes = append(out.Changes, app.Change{
				Op: app.OpUpdate, Entity: app.EntityTransaction,
				Summary: fmt.Sprintf("apply rules to transaction %d", tx.ID),
				Fields: map[string]any{
					"id": int64(tx.ID), "category_id": int64(e.Tx.CategoryID), "category_rule": e.Result.CategoryRule,
					"payee_id": int64(e.Tx.PayeeID), "payee_rule": e.Result.PayeeRule,
					"tag_ids": e.TagIDs, "tags_rule": e.Result.TagsRule,
				},
			})
		}
		out.Bulk = len(out.Effects) > 0
		return nil
	})
	return out, err
}

// effect fills the unset fields of tx from the rules and reports whether anything changed.
func effect(ctx context.Context, r app.Repos, rules []domain.Rule, tx domain.Transaction) (TxEffect, bool, error) {
	res, err := domain.Apply(rules, tx, domain.NormalizeDescription(tx.Description))
	if err != nil {
		return TxEffect{}, false, fmt.Errorf("transaction %d: %w", tx.ID, err)
	}
	e := TxEffect{Tx: tx, Result: res}
	changed := false
	if tx.CategoryID == 0 && res.CategoryID != 0 {
		e.Tx.CategoryID, changed = res.CategoryID, true
	} else {
		e.Result.CategoryRule = ""
	}
	if tx.PayeeID == 0 && res.PayeeID != 0 {
		e.Tx.PayeeID, changed = res.PayeeID, true
	} else {
		e.Result.PayeeRule = ""
	}
	if res.TagsRule != "" {
		have, err := r.Tags().TagsOf(ctx, tx.ID)
		if err != nil {
			return TxEffect{}, false, fmt.Errorf("tags of transaction %d: %w", tx.ID, err)
		}
		ids := make([]domain.TagID, 0, len(have)+len(res.TagIDs))
		for _, t := range have {
			ids = append(ids, t.ID)
		}
		n := len(ids)
		for _, id := range res.TagIDs {
			if !slices.Contains(ids, id) {
				ids = append(ids, id)
			}
		}
		if len(ids) > n {
			e.TagIDs, changed = ids, true
		} else {
			e.Result.TagsRule = ""
		}
	}
	return e, changed, nil
}

// ApplyApply stores the effects of the plan in one transaction, after a backup.
func (s *Service) ApplyApply(ctx context.Context, p ApplyPlan) error {
	if len(p.Effects) == 0 {
		return nil
	}
	if _, err := s.backup.Backup(ctx, app.BackupReasonBulk); err != nil {
		return fmt.Errorf("backup before bulk change: %w", err)
	}
	return s.write(ctx, func(r app.Repos) error {
		for _, e := range p.Effects {
			if err := r.Transactions().Update(ctx, e.Tx); err != nil {
				return fmt.Errorf("update transaction %d: %w", e.Tx.ID, err)
			}
			if e.Result.TagsRule != "" {
				if err := r.Tags().SetTags(ctx, e.Tx.ID, e.TagIDs); err != nil {
					return fmt.Errorf("set tags of transaction %d: %w", e.Tx.ID, err)
				}
			}
		}
		return nil
	})
}
