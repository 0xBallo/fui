package app_test

import (
	"testing"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

func transferEnv(t *testing.T) env {
	t.Helper()
	e := newEnv(t)
	for _, a := range [][3]string{
		{"Fineco", "checking", "EUR"},
		{"Risparmio", "savings", "EUR"},
		{"Dollars", "checking", "USD"},
		{"Pounds", "checking", "GBP"},
		{"Yen", "cash", "JPY"},
	} {
		e.account(t, a[0], a[1], a[2], "1000")
	}
	return e
}

func (e env) setRate(t *testing.T, cur, day, rate string) {
	t.Helper()
	p, err := e.svc.PlanSetFXRate(t.Context(), app.SetFXRateRequest{Currency: cur, Date: day, Rate: rate})
	wantNoErr(t, err)
	_, err = e.svc.ApplySetFXRate(t.Context(), p)
	wantNoErr(t, err)
}

func TestTransfer(t *testing.T) {
	tests := []struct {
		name    string
		req     app.TransferRequest
		wantErr *domain.Error
		out, in int64 // expected leg amounts in minor units
	}{
		{name: "same currency", req: app.TransferRequest{From: "fineco", To: "risp", Amount: "500"}, out: -50000, in: 50000},
		{name: "same currency with fee", req: app.TransferRequest{From: "fineco", To: "risp", Amount: "500", Fee: "1.5"}, out: -50150, in: 50000},
		{name: "cross currency, explicit destination", req: app.TransferRequest{From: "fineco", To: "dollars", Amount: "100", DestAmount: "108.40"}, out: -10000, in: 10840},
		{name: "cross currency via rates", req: app.TransferRequest{From: "fineco", To: "dollars", Amount: "90"}, out: -9000, in: 10000},
		{name: "cross currency, none is the base", req: app.TransferRequest{From: "dollars", To: "yen", Amount: "100"}, out: -10000, in: 15000},
		{name: "cross currency, rate missing", req: app.TransferRequest{From: "fineco", To: "pounds", Amount: "10"}, wantErr: app.ErrRateMissing},
		{name: "cross currency with a fee", req: app.TransferRequest{From: "fineco", To: "dollars", Amount: "90", Fee: "1"}, wantErr: domain.ErrInvalidValue},
		{name: "destination amount between same currencies", req: app.TransferRequest{From: "fineco", To: "risp", Amount: "5", DestAmount: "5"}, wantErr: domain.ErrInvalidValue},
		{name: "same account", req: app.TransferRequest{From: "fineco", To: "Fineco", Amount: "5"}, wantErr: domain.ErrInvalidValue},
		{name: "negative amount", req: app.TransferRequest{From: "fineco", To: "risp", Amount: "-5"}, wantErr: domain.ErrInvalidValue},
		{name: "negative fee", req: app.TransferRequest{From: "fineco", To: "risp", Amount: "5", Fee: "-1"}, wantErr: domain.ErrInvalidValue},
		{name: "unknown account", req: app.TransferRequest{From: "fineco", To: "zzz", Amount: "5"}, wantErr: domain.ErrNotFound},
		{name: "bad status", req: app.TransferRequest{From: "fineco", To: "risp", Amount: "5", Status: "x"}, wantErr: domain.ErrInvalidValue},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := transferEnv(t)
			e.setRate(t, "USD", "2026-10-01", "0.9")
			e.setRate(t, "JPY", "2026-10-01", "0.006")
			p, err := e.svc.PlanTransfer(t.Context(), tt.req)
			if tt.wantErr != nil {
				wantCode(t, err, tt.wantErr)
				return
			}
			wantNoErr(t, err)
			if len(p.Changes) != 1 || p.Changes[0].Entity != app.EntityTransfer || p.Bulk {
				t.Errorf("plan = %+v", p.Plan)
			}
			res, err := e.svc.ApplyTransfer(t.Context(), p)
			wantNoErr(t, err)
			if res.Out.Amount.Minor() != tt.out || res.In.Amount.Minor() != tt.in {
				t.Errorf("legs = %v / %v, want %d / %d", res.Out.Amount, res.In.Amount, tt.out, tt.in)
			}
			if res.Out.TransferGroupID == 0 || res.Out.TransferGroupID != res.In.TransferGroupID || res.Group.Out.TransactionID != res.Out.ID {
				t.Errorf("legs not linked: %+v", res)
			}
			if res.Out.Kind != domain.KindTransfer || res.Out.Date.String() != "2026-11-11" || res.Out.Status != domain.StatusBooked {
				t.Errorf("out leg = %+v", res.Out)
			}
		})
	}
}

func TestTransferShowAndRemove(t *testing.T) {
	e := transferEnv(t)
	ctx := t.Context()
	p, err := e.svc.PlanTransfer(ctx, app.TransferRequest{From: "fineco", To: "risp", Amount: "200", Fee: "2", Description: "Savings"})
	wantNoErr(t, err)
	res, err := e.svc.ApplyTransfer(ctx, p)
	wantNoErr(t, err)

	v, err := e.svc.ShowTx(ctx, res.In.ID)
	wantNoErr(t, err)
	if v.Transfer == nil || v.Transfer.Fee.Minor() != 200 || v.Transfer.Out.TransactionID != res.Out.ID || v.Account != "Risparmio" {
		t.Errorf("ShowTx(in leg) = %+v", v)
	}
	_, err = e.svc.PlanEditTx(ctx, app.EditTxRequest{ID: res.Out.ID, Amount: ptr("-1")})
	wantCode(t, err, domain.ErrInvalidValue)
	pe, err := e.svc.PlanEditTx(ctx, app.EditTxRequest{ID: res.Out.ID, Notes: ptr("ok")})
	wantNoErr(t, err)
	_, err = e.svc.ApplyEditTx(ctx, pe)
	wantNoErr(t, err)

	rp, err := e.svc.PlanRemoveTx(ctx, app.RemoveTxRequest{ID: res.Out.ID})
	wantNoErr(t, err)
	if len(rp.Changes) != 2 || rp.GroupID != res.Group.ID {
		t.Errorf("remove plan = %+v, want both legs and the group", rp)
	}
	wantNoErr(t, e.svc.ApplyRemoveTx(ctx, rp))
	for _, id := range []domain.TransactionID{res.Out.ID, res.In.ID} {
		if _, err := e.svc.ShowTx(ctx, id); err == nil {
			t.Errorf("leg %d survived the removal", id)
		}
	}
	accts, err := e.svc.ListAccounts(ctx, app.ListAccountsRequest{})
	wantNoErr(t, err)
	if accts[0].Balance.Minor() != 100000 || accts[1].Balance.Minor() != 100000 {
		t.Errorf("balances after removing the transfer = %v, %v; want the opening 1000", accts[0].Balance, accts[1].Balance)
	}
}

func TestTransferWithoutBaseCurrency(t *testing.T) {
	e := newEnv(t, func(c *app.Config) { c.BaseCurrency = domain.Currency{} })
	e.account(t, "A", "checking", "EUR", "0")
	e.account(t, "B", "checking", "USD", "0")
	_, err := e.svc.PlanTransfer(t.Context(), app.TransferRequest{From: "A", To: "B", Amount: "1"})
	wantCode(t, err, domain.ErrMissingField)
	_, err = e.svc.PlanTransfer(t.Context(), app.TransferRequest{From: "A", To: "B", Amount: "1", DestAmount: "1.1"})
	wantNoErr(t, err) // an explicit destination amount needs no rate
}
