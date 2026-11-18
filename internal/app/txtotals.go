package app

import (
	"context"
	"fmt"

	"github.com/0xballo/fui/internal/domain"
)

// TxTotals are the income and expense sums of every transaction matching a filter, in the base currency.
// Expense is negative. Count is the number of transactions summed (transfers, investments and exchanges excluded).
type TxTotals struct {
	Base    domain.Currency
	Income  domain.Money
	Expense domain.Money
	Count   int
}

// TxTotals walks every page of ListTx(req) (After and Limit are ignored), skips transfers, investments and exchanges (not income or expense) and converts
// each amount at its own date. A missing rate fails with ErrRateMissing, like NetWorth.
func (s *Service) TxTotals(ctx context.Context, req ListTxRequest) (TxTotals, error) {
	if err := s.needBase(); err != nil {
		return TxTotals{}, err
	}
	base := s.cfg.BaseCurrency
	zero, err := domain.NewMoney(0, base)
	if err != nil {
		return TxTotals{}, fmt.Errorf("tx totals: %w", err)
	}
	out := TxTotals{Base: base, Income: zero, Expense: zero}

	rates := map[txRateKey]domain.Rate{}
	req.Limit = 0 // ponytail: pages through ListTx; add a SQL SUM query if 100k-row filters get slow.
	req.After = nil
	for {
		page, err := s.ListTx(ctx, req)
		if err != nil {
			return TxTotals{}, err
		}
		err = s.store.Read(ctx, func(r Repos) error { return s.addTotals(ctx, r, &out, page.Items, rates) })
		if err != nil {
			return TxTotals{}, err
		}
		if page.Next == nil {
			return out, nil
		}
		req.After = page.Next
	}
}

type txRateKey struct {
	cur  domain.Currency
	date domain.Date
}

func (s *Service) addTotals(ctx context.Context, r Repos, out *TxTotals, items []TxView, rates map[txRateKey]domain.Rate) error {
	for _, v := range items {
		t := v.Transaction
		if t.Kind == domain.KindTransfer || t.Kind == domain.KindInvestment || t.Kind == domain.KindExchange {
			continue
		}
		k := txRateKey{t.Amount.Currency(), t.Date}
		rate, ok := rates[k]
		if !ok {
			var err error
			if rate, err = s.rateOn(ctx, r, k.cur, k.date); err != nil {
				return err
			}
			rates[k] = rate
		}
		conv, err := domain.Convert(t.Amount, rate, out.Base)
		if err != nil {
			return fmt.Errorf("tx totals: %w", err)
		}
		switch {
		case conv.Sign() > 0:
			out.Income, err = out.Income.Add(conv)
		case conv.Sign() < 0:
			out.Expense, err = out.Expense.Add(conv)
		}
		if err != nil {
			return fmt.Errorf("tx totals: %w", err)
		}
		out.Count++
	}
	return nil
}
