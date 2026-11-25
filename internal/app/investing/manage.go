package investing

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

// EditSecurityRequest changes the descriptive fields of the security Ref (ticker, ISIN or name);
// nil leaves a field as is. Kind, currency and price scale are fixed at creation.
type EditSecurityRequest struct {
	Ref    string
	Name   *string
	Ticker *string
	ISIN   *string
}

// EditSecurityPlan is the plan to store Security.
type EditSecurityPlan struct {
	app.Plan
	Security domain.Security
}

// PlanEditSecurity validates the new values and rejects a name, ticker or ISIN of another security.
func (s *Service) PlanEditSecurity(ctx context.Context, req EditSecurityRequest) (EditSecurityPlan, error) {
	if err := s.gate(""); err != nil {
		return EditSecurityPlan{}, err
	}
	var out EditSecurityPlan
	err := s.store.Read(ctx, func(r app.Repos) error {
		cur, err := s.visibleSecurity(ctx, r, req.Ref)
		if err != nil {
			return err
		}
		next := cur
		fields := map[string]any{"security": cur.Name}
		if req.Name != nil {
			next.Name = *req.Name
		}
		if req.Ticker != nil {
			next.Ticker = *req.Ticker
		}
		if req.ISIN != nil {
			next.ISIN = *req.ISIN
		}
		if next, err = domain.NewSecurity(next); err != nil {
			return err
		}
		all, err := r.Securities().List(ctx)
		if err != nil {
			return fmt.Errorf("list securities: %w", err)
		}
		for _, o := range all {
			if o.ID != next.ID && (strings.EqualFold(o.Name, next.Name) || (next.Ticker != "" && o.Ticker == next.Ticker) ||
				(next.ISIN != "" && o.ISIN == next.ISIN)) {
				return &domain.Error{
					Code: app.CodeAlreadyExists, Message: fmt.Sprintf("security %q already exists", o.Name),
					Details: map[string]any{"name": o.Name},
				}
			}
		}
		fields["name"], fields["ticker"], fields["isin"] = next.Name, next.Ticker, next.ISIN
		out = EditSecurityPlan{Security: next, Plan: app.Plan{Changes: []app.Change{{
			Op: app.OpUpdate, Entity: EntitySecurity, Summary: fmt.Sprintf("update security %q", cur.Name), Fields: fields,
		}}}}
		return nil
	})
	return out, err
}

// ApplyEditSecurity stores the planned security and returns it.
func (s *Service) ApplyEditSecurity(ctx context.Context, p EditSecurityPlan) (domain.Security, error) {
	if err := s.gate(p.Security.Kind); err != nil {
		return domain.Security{}, err
	}
	var out domain.Security
	err := s.write(ctx, func(r app.Repos) error {
		if err := r.Securities().Update(ctx, p.Security); err != nil {
			return err
		}
		var err error
		out, err = r.Securities().Get(ctx, p.Security.ID)
		return err
	})
	return out, err
}

// visibleSecurity resolves ref, treating a crypto security as missing while crypto is off.
func (s *Service) visibleSecurity(ctx context.Context, r app.Repos, ref string) (domain.Security, error) {
	sec, err := findSecurity(ctx, r, ref)
	if err != nil {
		return domain.Security{}, err
	}
	if !s.visible(sec.Kind) {
		return domain.Security{}, notFound("security", ref)
	}
	return sec, nil
}

// RemoveSecurityPlan is the plan to delete Security together with its manual prices.
type RemoveSecurityPlan struct {
	app.Plan
	Security domain.Security
	Prices   int
}

func inUseSecurity(sec domain.Security, ops int) error {
	return &domain.Error{
		Code: app.CodeInUse, Message: fmt.Sprintf("security %q has %d operations", sec.Name, ops),
		Details: map[string]any{"security": sec.Name, "operations": ops},
	}
}

// PlanRemoveSecurity refuses while any operation references the security.
func (s *Service) PlanRemoveSecurity(ctx context.Context, ref string) (RemoveSecurityPlan, error) {
	if err := s.gate(""); err != nil {
		return RemoveSecurityPlan{}, err
	}
	var out RemoveSecurityPlan
	err := s.store.Read(ctx, func(r app.Repos) error {
		sec, err := s.visibleSecurity(ctx, r, ref)
		if err != nil {
			return err
		}
		ops, err := r.Securities().ListOpsBySecurity(ctx, sec.ID, domain.Date{}, domain.Date{})
		if err != nil {
			return fmt.Errorf("list operations: %w", err)
		}
		if len(ops) > 0 {
			return inUseSecurity(sec, len(ops))
		}
		prices, err := r.Securities().ListPrices(ctx, sec.ID)
		if err != nil {
			return fmt.Errorf("list prices: %w", err)
		}
		out = RemoveSecurityPlan{Security: sec, Prices: len(prices), Plan: app.Plan{Changes: []app.Change{{
			Op: app.OpDelete, Entity: EntitySecurity, Summary: fmt.Sprintf("delete security %q", sec.Name),
			Fields: map[string]any{"id": int64(sec.ID), "prices": len(prices)},
		}}}}
		return nil
	})
	return out, err
}

// ApplyRemoveSecurity deletes the security and its prices atomically, re-checking for operations.
func (s *Service) ApplyRemoveSecurity(ctx context.Context, p RemoveSecurityPlan) error {
	if err := s.gate(p.Security.Kind); err != nil {
		return err
	}
	return s.write(ctx, func(r app.Repos) error {
		repo := r.Securities()
		ops, err := repo.ListOpsBySecurity(ctx, p.Security.ID, domain.Date{}, domain.Date{})
		if err != nil {
			return fmt.Errorf("list operations: %w", err)
		}
		if len(ops) > 0 {
			return inUseSecurity(p.Security, len(ops))
		}
		prices, err := repo.ListPrices(ctx, p.Security.ID)
		if err != nil {
			return fmt.Errorf("list prices: %w", err)
		}
		for _, pr := range prices {
			if err := repo.DeletePrice(ctx, p.Security.ID, pr.Date); err != nil {
				return fmt.Errorf("delete price: %w", err)
			}
		}
		return repo.Delete(ctx, p.Security.ID)
	})
}

// ListOpsRequest filters ListOps by account name, security (ticker, ISIN or name) and op kind;
// empty fields match everything.
type ListOpsRequest struct {
	Account  string
	Security string
	Kind     string
}

// OpLine is one operation with the security and account it belongs to.
type OpLine struct {
	Op       domain.SecurityOp
	Security domain.Security
	Account  domain.Account
}

// ListOps returns the matching operations ordered by date then ID; crypto ones are hidden while
// the crypto module is off.
func (s *Service) ListOps(ctx context.Context, req ListOpsRequest) ([]OpLine, error) {
	if err := s.gate(""); err != nil {
		return nil, err
	}
	var kind domain.SecurityOpKind
	if strings.TrimSpace(req.Kind) != "" {
		k, err := domain.ParseSecurityOpKind(req.Kind)
		if err != nil {
			return nil, err
		}
		kind = k
	}
	var out []OpLine
	err := s.store.Read(ctx, func(r app.Repos) error {
		accts, err := r.Accounts().List(ctx)
		if err != nil {
			return fmt.Errorf("list accounts: %w", err)
		}
		var onlyAcct domain.AccountID
		if req.Account != "" {
			a, err := findAccount(ctx, r, req.Account)
			if err != nil {
				return err
			}
			onlyAcct = a.ID
		}
		secs, err := r.Securities().List(ctx)
		if err != nil {
			return fmt.Errorf("list securities: %w", err)
		}
		if req.Security != "" {
			sec, err := s.visibleSecurity(ctx, r, req.Security)
			if err != nil {
				return err
			}
			secs = []domain.Security{sec}
		}
		for _, sec := range secs {
			if !s.visible(sec.Kind) {
				continue
			}
			lines, err := opLines(ctx, r, sec, accts, onlyAcct, kind)
			if err != nil {
				return err
			}
			out = append(out, lines...)
		}
		slices.SortStableFunc(out, func(a, b OpLine) int {
			if c := a.Op.Date.Cmp(b.Op.Date); c != 0 {
				return c
			}
			return cmp.Compare(a.Op.ID, b.Op.ID)
		})
		return nil
	})
	return out, err
}

// opLines returns the operations of sec that match the account and kind filters (zero = any).
func opLines(ctx context.Context, r app.Repos, sec domain.Security, accts []domain.Account, onlyAcct domain.AccountID, kind domain.SecurityOpKind) ([]OpLine, error) {
	ops, err := r.Securities().ListOpsBySecurity(ctx, sec.ID, domain.Date{}, domain.Date{})
	if err != nil {
		return nil, fmt.Errorf("list operations: %w", err)
	}
	var out []OpLine
	for _, op := range ops {
		if (onlyAcct != 0 && op.Account != onlyAcct) || (kind != "" && op.Kind != kind) {
			continue
		}
		i := slices.IndexFunc(accts, func(a domain.Account) bool { return a.ID == op.Account })
		if i < 0 {
			return nil, fmt.Errorf("operation %d: %w", op.ID, notFound("account", fmt.Sprint(op.Account)))
		}
		out = append(out, OpLine{Op: op, Security: sec, Account: accts[i]})
	}
	return out, nil
}

// EditOpRequest changes an operation; nil leaves a field as is. Fees "" means zero.
type EditOpRequest struct {
	ID       domain.SecurityOpID
	Date     *string
	Quantity *string
	Price    *string
	Fees     *string
}

// EditOpPlan is the plan to store Op and, when the operation has one, its updated cash transaction.
type EditOpPlan struct {
	app.Plan
	Op     domain.SecurityOp
	CashTx *domain.Transaction
}

// PlanEditOp validates the new values against the whole history of the position and refuses
// when the linked cash transaction is reconciled.
func (s *Service) PlanEditOp(ctx context.Context, req EditOpRequest) (EditOpPlan, error) {
	if err := s.gate(""); err != nil {
		return EditOpPlan{}, err
	}
	var out EditOpPlan
	err := s.store.Read(ctx, func(r app.Repos) error {
		var err error
		out, err = s.buildEditOp(ctx, r, req)
		return err
	})
	return out, err
}

// applyOpEdits parses the set fields of req into op and validates the result.
func (s *Service) applyOpEdits(op domain.SecurityOp, sec domain.Security, req EditOpRequest) (domain.SecurityOp, error) {
	var err error
	if req.Date != nil {
		if op.Date, err = s.date(*req.Date); err != nil {
			return domain.SecurityOp{}, err
		}
	}
	if req.Quantity != nil {
		if op.Quantity, err = domain.ParseQuantity(*req.Quantity, sec.QuantityScale); err != nil {
			return domain.SecurityOp{}, fmt.Errorf("quantity: %w", err)
		}
	}
	if req.Price != nil {
		if op.UnitPrice, err = domain.ParsePrice(*req.Price, sec.Currency, sec.PriceScale, s.cfg.Locale); err != nil {
			return domain.SecurityOp{}, fmt.Errorf("price: %w", err)
		}
	}
	if req.Fees != nil {
		op.Fees = domain.Money{}
		if strings.TrimSpace(*req.Fees) != "" {
			if op.Fees, err = domain.ParseAmount(*req.Fees, s.cfg.Locale, sec.Currency); err != nil {
				return domain.SecurityOp{}, fmt.Errorf("fees: %w", err)
			}
		}
	}
	return domain.NewSecurityOp(op)
}

func (s *Service) buildEditOp(ctx context.Context, r app.Repos, req EditOpRequest) (EditOpPlan, error) {
	op, err := r.Securities().GetOp(ctx, req.ID)
	if err != nil {
		return EditOpPlan{}, fmt.Errorf("security operation %d: %w", req.ID, err)
	}
	sec, err := r.Securities().Get(ctx, op.Security)
	if err != nil {
		return EditOpPlan{}, fmt.Errorf("security: %w", err)
	}
	if err := s.gate(sec.Kind); err != nil {
		return EditOpPlan{}, err
	}
	if op, err = s.applyOpEdits(op, sec, req); err != nil {
		return EditOpPlan{}, err
	}
	if err := checkHeld(ctx, r, sec, op.Account, &op, op.ID); err != nil {
		return EditOpPlan{}, err
	}
	plan := EditOpPlan{Op: op, Plan: app.Plan{Changes: []app.Change{{
		Op: app.OpUpdate, Entity: EntitySecurityOp, Summary: fmt.Sprintf("update %s %s of %s", op.Kind, op.Quantity, sec.Name),
		Fields: priceFields(map[string]any{
			"id": int64(op.ID), "kind": string(op.Kind), "date": op.Date.String(), "security": sec.Name, "quantity": op.Quantity.String(),
		}, "unit_price", op.UnitPrice),
	}}}}
	if op.CashTx == 0 {
		return plan, nil
	}
	tx, err := s.editedCashTx(ctx, r, sec, op)
	if err != nil {
		return EditOpPlan{}, err
	}
	plan.CashTx = &tx
	plan.Changes = append(plan.Changes, app.Change{
		Op: app.OpUpdate, Entity: app.EntityTransaction, Summary: fmt.Sprintf("update %s %s", tx.Kind, tx.Amount),
		Fields: moneyFields(map[string]any{"id": int64(tx.ID), "date": tx.Date.String()}, "amount", tx.Amount),
	})
	return plan, nil
}

// editedCashTx rebuilds the amount and date of the cash transaction linked to op, keeping the
// rest (category, payee, notes) of the stored one.
func (s *Service) editedCashTx(ctx context.Context, r app.Repos, sec domain.Security, op domain.SecurityOp) (domain.Transaction, error) {
	tx, err := r.Transactions().Get(ctx, op.CashTx)
	if err != nil {
		return domain.Transaction{}, fmt.Errorf("cash transaction: %w", err)
	}
	if tx.Status == domain.StatusReconciled {
		return domain.Transaction{}, domain.ErrReconciled
	}
	cash, err := r.Accounts().Get(ctx, tx.AccountID)
	if err != nil {
		return domain.Transaction{}, fmt.Errorf("cash account: %w", err)
	}
	built, err := cashTx(sec, cash, op)
	if err != nil {
		return domain.Transaction{}, err
	}
	tx.Amount, tx.Date = built.Amount, built.Date
	return tx, nil
}

// ApplyEditOp stores the operation and its cash transaction atomically, re-checking the held
// quantity and the reconciliation status against the current data.
func (s *Service) ApplyEditOp(ctx context.Context, p EditOpPlan) (domain.SecurityOp, error) {
	if err := s.gate(""); err != nil {
		return domain.SecurityOp{}, err
	}
	var out domain.SecurityOp
	err := s.write(ctx, func(r app.Repos) error {
		cur, err := r.Securities().GetOp(ctx, p.Op.ID)
		if err != nil {
			return fmt.Errorf("security operation %d: %w", p.Op.ID, err)
		}
		sec, err := r.Securities().Get(ctx, cur.Security)
		if err != nil {
			return fmt.Errorf("security: %w", err)
		}
		if err := s.gate(sec.Kind); err != nil {
			return err
		}
		op := p.Op
		op.Account, op.Security, op.Kind, op.CashTx, op.ImportBatchID = cur.Account, cur.Security, cur.Kind, cur.CashTx, cur.ImportBatchID
		if err := checkHeld(ctx, r, sec, op.Account, &op, op.ID); err != nil {
			return err
		}
		if op.CashTx != 0 {
			tx, err := s.editedCashTx(ctx, r, sec, op)
			if err != nil {
				return err
			}
			if err := r.Transactions().Update(ctx, tx); err != nil {
				return fmt.Errorf("update cash transaction: %w", err)
			}
		}
		if err := r.Securities().UpdateOp(ctx, op); err != nil {
			return fmt.Errorf("update operation: %w", err)
		}
		out = op
		return nil
	})
	return out, err
}

// SecurityPositionLine is one security aggregated over every account, valued in the base
// currency. Value and Unrealized are meaningful only when HasPrice; Accounts counts the accounts
// that still hold it.
type SecurityPositionLine struct {
	Security   domain.Security
	Quantity   domain.Quantity
	CostBasis  domain.Money
	Value      domain.Money
	Unrealized domain.Money
	HasPrice   bool
	Accounts   int
}

// ListSecurityPositions sums the per-account positions of each security with operations up to
// the date (default today), ordered by security ID; crypto is hidden while the crypto module is off.
func (s *Service) ListSecurityPositions(ctx context.Context, date string) ([]SecurityPositionLine, error) {
	if err := s.gate(""); err != nil {
		return nil, err
	}
	d, err := s.date(date)
	if err != nil {
		return nil, err
	}
	var out []SecurityPositionLine
	err = s.store.Read(ctx, func(r app.Repos) error {
		accts, err := r.Accounts().List(ctx)
		if err != nil {
			return fmt.Errorf("list accounts: %w", err)
		}
		secs, err := r.Securities().List(ctx)
		if err != nil {
			return fmt.Errorf("list securities: %w", err)
		}
		for _, sec := range secs {
			if !s.visible(sec.Kind) {
				continue
			}
			lines, err := s.securityLines(ctx, r, sec, accts, 0, d)
			if err != nil {
				return err
			}
			if len(lines) == 0 {
				continue
			}
			agg, err := s.aggregate(ctx, r, sec, lines, d)
			if err != nil {
				return err
			}
			out = append(out, agg)
		}
		return nil
	})
	return out, err
}

func (s *Service) aggregate(ctx context.Context, r app.Repos, sec domain.Security, lines []PositionLine, d domain.Date) (SecurityPositionLine, error) {
	qty, err := domain.NewQuantity(0, sec.QuantityScale)
	if err != nil {
		return SecurityPositionLine{}, err
	}
	zero, err := domain.NewMoney(0, s.cfg.BaseCurrency)
	if err != nil {
		return SecurityPositionLine{}, fmt.Errorf("base currency: %w", err)
	}
	agg := SecurityPositionLine{Security: sec, Quantity: qty, CostBasis: zero, Value: zero, Unrealized: zero, HasPrice: true}
	for _, l := range lines {
		if l.Position.Quantity.IsZero() {
			continue
		}
		agg.Accounts++
		if agg.Quantity, err = agg.Quantity.Add(l.Position.Quantity); err != nil {
			return SecurityPositionLine{}, fmt.Errorf("sum quantity: %w", err)
		}
		cost, err := s.toBase(ctx, r, l.Position.CostBasis, d)
		if err != nil {
			return SecurityPositionLine{}, err
		}
		if agg.CostBasis, err = agg.CostBasis.Add(cost); err != nil {
			return SecurityPositionLine{}, fmt.Errorf("sum cost basis: %w", err)
		}
		if !l.Position.HasPrice {
			agg.HasPrice = false
			continue
		}
		if agg.Value, err = agg.Value.Add(l.Value); err != nil {
			return SecurityPositionLine{}, fmt.Errorf("sum value: %w", err)
		}
	}
	if agg.Accounts == 0 || !agg.HasPrice {
		agg.HasPrice = false
		agg.Value, agg.Unrealized = zero, zero
		return agg, nil
	}
	if agg.Unrealized, err = agg.Value.Sub(agg.CostBasis); err != nil {
		return SecurityPositionLine{}, fmt.Errorf("unrealized: %w", err)
	}
	return agg, nil
}
