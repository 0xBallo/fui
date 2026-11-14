package app

import (
	"context"
	"fmt"

	"github.com/0xballo/fui/internal/domain"
)

// TransferRequest moves Amount (typed in the source currency) from one account to another.
// Fee is charged on the source account on top of Amount. Across currencies the destination
// amount is DestAmount if given, else Amount converted at the rates of Date (both rates must
// exist); a cross-currency transfer cannot carry a fee.
type TransferRequest struct {
	From        string
	To          string
	Amount      string
	DestAmount  string
	Fee         string
	Date        string
	Description string
	Status      string
}

// TransferPlan is the plan to create the group and its two legs.
type TransferPlan struct {
	Plan
	Group domain.TransferGroup
	Out   domain.Transaction
	In    domain.Transaction
}

// TransferResult is the persisted transfer.
type TransferResult struct {
	Group domain.TransferGroup
	Out   domain.Transaction
	In    domain.Transaction
}

// PlanTransfer resolves the accounts and computes both legs without writing.
func (s *Service) PlanTransfer(ctx context.Context, req TransferRequest) (TransferPlan, error) {
	var out TransferPlan
	err := s.store.Read(ctx, func(r Repos) error {
		accts, err := r.Accounts().List(ctx)
		if err != nil {
			return fmt.Errorf("list accounts: %w", err)
		}
		accts = activeAccounts(accts)
		from, err := pickAccount(accts, req.From)
		if err != nil {
			return err
		}
		to, err := pickAccount(accts, req.To)
		if err != nil {
			return err
		}
		out, err = s.buildTransfer(ctx, r, req, from, to)
		return err
	})
	return out, err
}

func (s *Service) buildTransfer(ctx context.Context, r Repos, req TransferRequest, from, to domain.Account) (TransferPlan, error) {
	date, err := s.date(req.Date)
	if err != nil {
		return TransferPlan{}, err
	}
	amount, err := s.amount(req.Amount, from.Currency)
	if err != nil {
		return TransferPlan{}, err
	}
	if amount.Sign() <= 0 {
		return TransferPlan{}, newError(domain.ErrInvalidValue, map[string]any{"field": "amount"}, "transfer amount must be positive")
	}
	in, err := s.destAmount(ctx, r, req, amount, to.Currency, date)
	if err != nil {
		return TransferPlan{}, err
	}
	fee, err := s.transferFee(req.Fee, from.Currency)
	if err != nil {
		return TransferPlan{}, err
	}
	total, err := amount.Add(fee)
	if err != nil {
		return TransferPlan{}, fmt.Errorf("transfer total: %w", err)
	}
	negTotal, err := total.Neg()
	if err != nil {
		return TransferPlan{}, fmt.Errorf("transfer total: %w", err)
	}
	var groupFee domain.Money
	if !fee.IsZero() {
		groupFee = fee
	}
	group, err := domain.NewTransferGroup(domain.TransferGroup{
		Out: domain.TransferLeg{AccountID: from.ID, Amount: negTotal},
		In:  domain.TransferLeg{AccountID: to.ID, Amount: in},
		Fee: groupFee,
	})
	if err != nil {
		return TransferPlan{}, fmt.Errorf("transfer: %w", err)
	}
	return s.transferPlan(req, group, from, to, date)
}

func (s *Service) transferPlan(req TransferRequest, g domain.TransferGroup, from, to domain.Account, date domain.Date) (TransferPlan, error) {
	status, err := domain.ParseTxStatus(orDefault(req.Status, string(domain.StatusBooked)))
	if err != nil {
		return TransferPlan{}, fmt.Errorf("status: %w", err)
	}
	leg := func(l domain.TransferLeg, desc string) (domain.Transaction, error) {
		t, err := domain.NewTransaction(domain.Transaction{
			AccountID: l.AccountID, Date: date, Amount: l.Amount, Status: status, Kind: domain.KindTransfer,
			Description: orDefault(req.Description, desc),
		})
		if err != nil {
			return domain.Transaction{}, fmt.Errorf("transfer leg: %w", err)
		}
		return t, nil
	}
	out, err := leg(g.Out, "Transfer to "+to.Name)
	if err != nil {
		return TransferPlan{}, err
	}
	in, err := leg(g.In, "Transfer from "+from.Name)
	if err != nil {
		return TransferPlan{}, err
	}
	f := map[string]any{"from": from.Name, "to": to.Name, "date": date.String()}
	moneyFields(f, "out_amount", g.Out.Amount)
	moneyFields(f, "in_amount", g.In.Amount)
	summary := fmt.Sprintf("transfer %s from %q to %q on %s", g.In.Amount, from.Name, to.Name, date)
	if !g.Fee.Currency().IsZero() {
		moneyFields(f, "fee", g.Fee)
		summary += fmt.Sprintf(" (fee %s)", g.Fee)
	}
	ch := Change{Op: OpCreate, Entity: EntityTransfer, Summary: summary, Fields: f}
	return TransferPlan{Plan: Plan{Changes: []Change{ch}}, Group: g, Out: out, In: in}, nil
}

// destAmount is the amount arriving on the destination account.
func (s *Service) destAmount(ctx context.Context, r Repos, req TransferRequest, amount domain.Money, to domain.Currency, date domain.Date) (domain.Money, error) {
	if amount.Currency() == to {
		if req.DestAmount != "" {
			return domain.Money{}, newError(domain.ErrInvalidValue, map[string]any{"field": "dest_amount"},
				"both accounts are in %s: a destination amount makes no sense", to)
		}
		return amount, nil
	}
	if req.DestAmount != "" {
		return s.amount(req.DestAmount, to)
	}
	src, err := s.rateOn(ctx, r, amount.Currency(), date)
	if err != nil {
		return domain.Money{}, err
	}
	dst, err := s.rateOn(ctx, r, to, date)
	if err != nil {
		return domain.Money{}, err
	}
	return convertVia(amount, to, src, dst)
}

func (s *Service) transferFee(in string, c domain.Currency) (domain.Money, error) {
	if in == "" {
		return domain.NewMoney(0, c)
	}
	return s.amount(in, c)
}

// ApplyTransfer stores the group and its two legs in one transaction.
func (s *Service) ApplyTransfer(ctx context.Context, p TransferPlan) (TransferResult, error) {
	var res TransferResult
	err := s.apply(ctx, p.Bulk, func(r Repos) error {
		g, err := r.Transfers().Insert(ctx, p.Group)
		if err != nil {
			return fmt.Errorf("create transfer: %w", err)
		}
		p.Out.TransferGroupID, p.In.TransferGroupID = g.ID, g.ID
		if res.Out, err = r.Transactions().Insert(ctx, p.Out); err != nil {
			return fmt.Errorf("create transfer out leg: %w", err)
		}
		if res.In, err = r.Transactions().Insert(ctx, p.In); err != nil {
			return fmt.Errorf("create transfer in leg: %w", err)
		}
		g.Out = domain.TransferLeg{TransactionID: res.Out.ID, AccountID: res.Out.AccountID, Amount: res.Out.Amount}
		g.In = domain.TransferLeg{TransactionID: res.In.ID, AccountID: res.In.AccountID, Amount: res.In.Amount}
		res.Group = g
		after, err := snapshot(ctx, r, res.Out.ID, res.In.ID)
		if err != nil {
			return err
		}
		return s.journal(ctx, r, OpKindTransfer, undoPayload{
			Group: int64(g.ID), FeeMinor: g.Fee.Minor(), FeeCurrency: g.Fee.Currency().Code(), After: after,
		})
	})
	return res, err
}
