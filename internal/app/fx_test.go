package app_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

func TestParseRate(t *testing.T) {
	tests := []struct {
		in      string
		want    int64
		wantErr bool
	}{
		{in: "1", want: 1_000_000_000},
		{in: "1.0842", want: 1_084_200_000},
		{in: "0,92", want: 920_000_000},
		{in: " .5 ", want: 500_000_000},
		{in: "5.", want: 5_000_000_000},
		{in: "0.000000001", want: 1},
		{in: "123456789.123456789", want: 123_456_789_123_456_789},
		{in: "0.0000000001", wantErr: true}, // more than 9 decimals
		{in: "1234567890", wantErr: true},   // would overflow
		{in: "0", wantErr: true},
		{in: "", wantErr: true},
		{in: "-1", wantErr: true},
		{in: "1.2.3", wantErr: true},
		{in: "1,2,3", wantErr: true},
		{in: "abc", wantErr: true},
		{in: ".", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := app.ParseRate(tt.in)
			if tt.wantErr {
				wantCode(t, err, domain.ErrInvalidValue)
				return
			}
			wantNoErr(t, err)
			if got.Scaled() != tt.want {
				t.Errorf("ParseRate(%q) = %d, want %d", tt.in, got.Scaled(), tt.want)
			}
		})
	}
}

func TestSetListLookupFXRate(t *testing.T) {
	e := newEnv(t)
	ctx := t.Context()

	p, err := e.svc.PlanSetFXRate(ctx, app.SetFXRateRequest{Currency: "usd", Date: "2026-10-01", Rate: "0.90"})
	wantNoErr(t, err)
	if p.Changes[0].Op != app.OpCreate {
		t.Errorf("first rate op = %s, want create", p.Changes[0].Op)
	}
	_, err = e.svc.ApplySetFXRate(ctx, p)
	wantNoErr(t, err)
	p, err = e.svc.PlanSetFXRate(ctx, app.SetFXRateRequest{Currency: "USD", Date: "2026-10-01", Rate: "0.91"})
	wantNoErr(t, err)
	if p.Changes[0].Op != app.OpUpdate {
		t.Errorf("same day op = %s, want update", p.Changes[0].Op)
	}
	_, err = e.svc.ApplySetFXRate(ctx, p)
	wantNoErr(t, err)
	e.setRate(t, "USD", "2026-11-05", "0.95")
	e.setRate(t, "GBP", "", "1.17") // today

	all, err := e.svc.ListFXRates(ctx, "")
	wantNoErr(t, err)
	if len(all) != 3 || all[0].Currency.Code() != "GBP" || all[1].Rate.Scaled() != 910_000_000 {
		t.Errorf("ListFXRates = %+v", all)
	}
	usd, err := e.svc.ListFXRates(ctx, "usd")
	wantNoErr(t, err)
	if len(usd) != 2 {
		t.Errorf("ListFXRates(usd) = %+v", usd)
	}

	for date, want := range map[string]int64{"2026-10-31": 910_000_000, "2026-11-05": 950_000_000, "": 950_000_000} {
		got, err := e.svc.LookupFXRate(ctx, "USD", date)
		wantNoErr(t, err)
		if got.Rate.Scaled() != want {
			t.Errorf("Lookup(%q) = %d, want %d", date, got.Rate.Scaled(), want)
		}
	}
	base, err := e.svc.LookupFXRate(ctx, "EUR", "")
	wantNoErr(t, err)
	if base.Rate.Scaled() != domain.RateScale {
		t.Errorf("base rate = %d, want 1.0", base.Rate.Scaled())
	}
	_, err = e.svc.LookupFXRate(ctx, "USD", "2026-09-30")
	wantCode(t, err, app.ErrRateMissing)
	_, err = e.svc.LookupFXRate(ctx, "JPY", "")
	wantCode(t, err, app.ErrRateMissing)
}

func TestSetFXRateErrors(t *testing.T) {
	tests := []struct {
		name    string
		req     app.SetFXRateRequest
		noBase  bool
		wantErr *domain.Error
	}{
		{name: "base currency", req: app.SetFXRateRequest{Currency: "EUR", Rate: "1"}, wantErr: domain.ErrInvalidValue},
		{name: "unknown currency", req: app.SetFXRateRequest{Currency: "ZZZ", Rate: "1"}, wantErr: domain.ErrInvalidValue},
		{name: "bad rate", req: app.SetFXRateRequest{Currency: "USD", Rate: "x"}, wantErr: domain.ErrInvalidValue},
		{name: "bad date", req: app.SetFXRateRequest{Currency: "USD", Rate: "1", Date: "x"}, wantErr: domain.ErrInvalidDate},
		{name: "no base configured", req: app.SetFXRateRequest{Currency: "USD", Rate: "1"}, noBase: true, wantErr: domain.ErrMissingField},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newEnv(t, func(c *app.Config) {
				if tt.noBase {
					c.BaseCurrency = domain.Currency{}
				}
			})
			_, err := e.svc.PlanSetFXRate(t.Context(), tt.req)
			wantCode(t, err, tt.wantErr)
		})
	}
}

func TestNetWorth(t *testing.T) {
	e := newEnv(t)
	ctx := t.Context()
	e.account(t, "Eur", "checking", "EUR", "1000")
	e.account(t, "Usd", "checking", "USD", "200")
	e.account(t, "Yen", "cash", "JPY", "10000")
	e.account(t, "Old", "cash", "GBP", "5")
	ap, err := e.svc.PlanArchiveAccount(ctx, "Old")
	wantNoErr(t, err)
	_, err = e.svc.ApplyArchiveAccount(ctx, ap)
	wantNoErr(t, err)
	e.tx(t, app.AddTxRequest{Account: "Usd", Amount: "-100", Date: "2026-11-01"})

	_, err = e.svc.NetWorth(ctx, app.NetWorthRequest{})
	var de *domain.Error
	if !errors.As(err, &de) || de.Code != app.CodeRateMissing || fmt.Sprint(de.Details["currencies"]) != "[JPY USD]" {
		t.Fatalf("NetWorth without rates = %v (%+v), want rate_missing listing JPY and USD", err, de)
	}

	e.setRate(t, "USD", "2026-10-01", "0.9")
	e.setRate(t, "USD", "2026-11-05", "0.8")
	e.setRate(t, "JPY", "2026-10-01", "0.006")
	nw, err := e.svc.NetWorth(ctx, app.NetWorthRequest{}) // today 2026-11-11: USD 100 × 0.8, JPY 10000 × 0.006
	wantNoErr(t, err)
	if nw.Total.Minor() != 100000+8000+6000 || nw.Base.Code() != "EUR" || len(nw.Lines) != 3 {
		t.Errorf("NetWorth today = %v with %d lines, want 1140.00 EUR", nw.Total, len(nw.Lines))
	}
	nw, err = e.svc.NetWorth(ctx, app.NetWorthRequest{At: "2026-10-31"}) // before the USD movement and the second rate
	wantNoErr(t, err)
	if nw.Total.Minor() != 100000+18000+6000 {
		t.Errorf("NetWorth at 2026-10-31 = %v, want 1240.00 EUR", nw.Total)
	}
	_, err = e.svc.NetWorth(ctx, app.NetWorthRequest{IncludeArchived: true})
	wantCode(t, err, app.ErrRateMissing) // the archived GBP account counts when asked

	noBase := newEnv(t, func(c *app.Config) { c.BaseCurrency = domain.Currency{} })
	_, err = noBase.svc.NetWorth(ctx, app.NetWorthRequest{})
	wantCode(t, err, domain.ErrMissingField)
}
