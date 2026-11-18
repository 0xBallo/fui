package app_test

import (
	"errors"
	"testing"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

func TestTxTotals(t *testing.T) {
	ctx := t.Context()

	t.Run("mixed signs, transfer excluded, paged", func(t *testing.T) {
		e := newEnv(t)
		e.account(t, "A", "checking", "EUR", "0")
		e.account(t, "B", "checking", "EUR", "0")
		e.tx(t, app.AddTxRequest{Account: "A", Amount: "100", Date: "2026-11-01"})
		e.tx(t, app.AddTxRequest{Account: "A", Amount: "50", Date: "2026-11-02"})
		e.tx(t, app.AddTxRequest{Account: "A", Amount: "-30", Date: "2026-11-03"})
		p, err := e.svc.PlanTransfer(ctx, app.TransferRequest{From: "A", To: "B", Amount: "70", Date: "2026-11-04"})
		wantNoErr(t, err)
		_, err = e.svc.ApplyTransfer(ctx, p)
		wantNoErr(t, err)

		got, err := e.svc.TxTotals(ctx, app.ListTxRequest{Limit: 1})
		wantNoErr(t, err)
		if got.Income.Minor() != 15000 || got.Expense.Minor() != -3000 || got.Count != 3 || got.Base.Code() != "EUR" {
			t.Errorf("TxTotals = %+v, want income 150.00 expense -30.00 count 3 EUR", got)
		}
	})

	t.Run("foreign currency converted at its date", func(t *testing.T) {
		e := newEnv(t)
		e.account(t, "Usd", "checking", "USD", "0")
		e.setRate(t, "USD", "2026-10-01", "0.9")
		e.setRate(t, "USD", "2026-11-05", "0.8")
		e.tx(t, app.AddTxRequest{Account: "Usd", Amount: "100", Date: "2026-11-01"})
		e.tx(t, app.AddTxRequest{Account: "Usd", Amount: "-100", Date: "2026-11-06"})
		got, err := e.svc.TxTotals(ctx, app.ListTxRequest{})
		wantNoErr(t, err)
		if got.Income.Minor() != 9000 || got.Expense.Minor() != -8000 {
			t.Errorf("TxTotals = %+v, want income 90.00 expense -80.00", got)
		}
	})

	t.Run("missing rate", func(t *testing.T) {
		e := newEnv(t)
		e.account(t, "Usd", "checking", "USD", "0")
		e.tx(t, app.AddTxRequest{Account: "Usd", Amount: "100", Date: "2026-11-01"})
		_, err := e.svc.TxTotals(ctx, app.ListTxRequest{})
		var de *domain.Error
		if !errors.As(err, &de) || de.Code != app.CodeRateMissing {
			t.Errorf("TxTotals = %v, want rate_missing", err)
		}
	})

	t.Run("no base currency", func(t *testing.T) {
		e := newEnv(t, func(c *app.Config) { c.BaseCurrency = domain.Currency{} })
		_, err := e.svc.TxTotals(ctx, app.ListTxRequest{})
		var de *domain.Error
		if !errors.As(err, &de) || de.Code != domain.CodeMissingField {
			t.Errorf("TxTotals = %v, want missing_field", err)
		}
	})

	t.Run("empty result", func(t *testing.T) {
		e := newEnv(t)
		got, err := e.svc.TxTotals(ctx, app.ListTxRequest{})
		wantNoErr(t, err)
		if got.Income.Minor() != 0 || got.Expense.Minor() != 0 || got.Count != 0 || got.Income.Currency().Code() != "EUR" {
			t.Errorf("TxTotals = %+v, want zero EUR amounts", got)
		}
	})
}
