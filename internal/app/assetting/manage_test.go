package assetting_test

import (
	"context"
	"errors"
	"testing"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/assetting"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
)

func ptr(s string) *string { return &s }

func (e env) value(t *testing.T, asset, day, v string) {
	t.Helper()
	ctx := context.Background()
	p, err := e.svc.PlanValue(ctx, e.on, assetting.ValueRequest{Asset: asset, Date: day, Value: v})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.svc.ApplyValue(ctx, p); err != nil {
		t.Fatal(err)
	}
}

func TestManageRequiresModule(t *testing.T) {
	e := newEnv(t)
	off := must(module.Validate([]module.ID{module.Core}))
	ctx := context.Background()
	calls := map[string]error{}
	_, calls["edit"] = e.svc.PlanEdit(ctx, off, assetting.EditRequest{})
	_, calls["rm"] = e.svc.PlanRemove(ctx, off, "x")
	_, calls["rmval"] = e.svc.PlanRemoveValuation(ctx, off, "x", "")
	_, calls["link"] = e.svc.PlanLink(ctx, off, "x", "y")
	_, calls["unlink"] = e.svc.PlanUnlink(ctx, off, "x")
	for name, err := range calls {
		if !errors.Is(err, app.ErrModuleDisabled) {
			t.Errorf("%s err = %v, want ErrModuleDisabled", name, err)
		}
	}
}

func TestEdit(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	e.add(t, assetting.AddRequest{Name: "Flat", Kind: "property", Value: "1000"})
	e.add(t, assetting.AddRequest{Name: "Car", Kind: "vehicle", Value: "500"})
	e.add(t, assetting.AddRequest{Name: "Bike", Kind: "other", Value: "100", Currency: "USD"})
	e.value(t, "car", "2026-02-01", "400")

	tests := []struct {
		name    string
		req     assetting.EditRequest
		wantErr error
		check   func(domain.Asset) bool
	}{
		{
			"rename and kind",
			assetting.EditRequest{Ref: "flat", Name: ptr(" House "), Kind: ptr("OTHER")},
			nil, func(a domain.Asset) bool { return a.Name == "House" && a.Kind == domain.AssetKindOther },
		},
		{
			"value and date",
			assetting.EditRequest{Ref: "house", PurchaseValue: ptr("2000"), PurchaseDate: ptr("2025-12-31")},
			nil, func(a domain.Asset) bool {
				return a.PurchaseValue.Minor() == 200000 && a.PurchaseDate.String() == "2025-12-31"
			},
		},
		{
			"currency without valuations",
			assetting.EditRequest{Ref: "bike", Currency: ptr("EUR")},
			nil, func(a domain.Asset) bool {
				return a.PurchaseValue.Currency().Code() == "EUR" && a.PurchaseValue.Minor() == 10000
			},
		},
		{
			"currency and value",
			assetting.EditRequest{Ref: "bike", Currency: ptr("GBP"), PurchaseValue: ptr("7")},
			nil, func(a domain.Asset) bool {
				return a.PurchaseValue.Currency().Code() == "GBP" && a.PurchaseValue.Minor() == 700
			},
		},
		{
			"same currency ok with valuations",
			assetting.EditRequest{Ref: "car", Currency: ptr("eur")},
			nil,
			func(a domain.Asset) bool { return a.Name == "Car" },
		},
		{"currency with valuations", assetting.EditRequest{Ref: "car", Currency: ptr("USD")}, app.ErrInUse, nil},
		{"duplicate name", assetting.EditRequest{Ref: "house", Name: ptr("car")}, app.ErrAlreadyExists, nil},
		{"unknown ref", assetting.EditRequest{Ref: "zzz", Name: ptr("a")}, domain.ErrNotFound, nil},
		{"nothing", assetting.EditRequest{Ref: "house"}, domain.ErrInvalidValue, nil},
		{"bad kind", assetting.EditRequest{Ref: "house", Kind: ptr("boat")}, domain.ErrInvalidValue, nil},
		{"empty name", assetting.EditRequest{Ref: "house", Name: ptr(" ")}, domain.ErrMissingField, nil},
		{"bad currency", assetting.EditRequest{Ref: "bike", Currency: ptr("??")}, domain.ErrInvalidValue, nil},
		{"negative value", assetting.EditRequest{Ref: "house", PurchaseValue: ptr("-1")}, domain.ErrInvalidValue, nil},
		{"bad date", assetting.EditRequest{Ref: "house", PurchaseDate: ptr("garbage")}, domain.ErrInvalidDate, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			before := e.store.Snapshot()
			p, err := e.svc.PlanEdit(ctx, e.on, tc.req)
			if e.store.Snapshot() != before {
				t.Fatal("PlanEdit wrote")
			}
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("err = %v, want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			got, err := e.svc.ApplyEdit(ctx, p)
			if err != nil {
				t.Fatal(err)
			}
			if !tc.check(got) {
				t.Errorf("asset = %+v", got)
			}
		})
	}
}

func TestEditLinkedLoanCurrency(t *testing.T) {
	e := newEnv(t)
	e.add(t, assetting.AddRequest{Name: "Flat", Kind: "property", Value: "1000", Loan: "mort"})
	_, err := e.svc.PlanEdit(context.Background(), e.on, assetting.EditRequest{Ref: "flat", Currency: ptr("USD")})
	if !errors.Is(err, domain.ErrCurrencyMismatch) {
		t.Errorf("err = %v, want ErrCurrencyMismatch", err)
	}
}

func TestRemoveCascades(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	a := e.add(t, assetting.AddRequest{Name: "Flat", Kind: "property", Value: "1000"})
	e.value(t, "flat", "2026-01-01", "1100")
	e.value(t, "flat", "2026-02-01", "1200")
	if _, err := e.svc.PlanRemove(ctx, e.on, "zzz"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("unknown err = %v", err)
	}
	p, err := e.svc.PlanRemove(ctx, e.on, "flat")
	if err != nil {
		t.Fatal(err)
	}
	if p.Valuations != 2 || !p.Bulk || p.Changes[0].Op != app.OpDelete {
		t.Errorf("plan = %+v", p)
	}
	if err := e.svc.ApplyRemove(ctx, p); err != nil {
		t.Fatal(err)
	}
	if e.backup.n != 1 {
		t.Errorf("backups = %d", e.backup.n)
	}
	err = e.store.Read(ctx, func(r app.Repos) error {
		if _, err := r.Assets().Get(ctx, a.ID); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("asset still there: %v", err)
		}
		if v, _ := r.Assets().ListValuations(ctx, a.ID); len(v) != 0 {
			t.Errorf("valuations left: %v", v)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.svc.ApplyRemove(ctx, p); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("second remove err = %v", err)
	}
}

func TestRemoveValuation(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	e.add(t, assetting.AddRequest{Name: "Flat", Kind: "property", Value: "1000"})
	e.value(t, "flat", "2026-02-01", "1200")
	if _, err := e.svc.PlanRemoveValuation(ctx, e.on, "flat", "2026-02-02"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("missing date err = %v", err)
	}
	if _, err := e.svc.PlanRemoveValuation(ctx, e.on, "zzz", "2026-02-01"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("unknown asset err = %v", err)
	}
	if _, err := e.svc.PlanRemoveValuation(ctx, e.on, "flat", "garbage"); err == nil {
		t.Error("bad date accepted")
	}
	p, err := e.svc.PlanRemoveValuation(ctx, e.on, "flat", "2026-02-01")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.svc.ApplyRemoveValuation(ctx, p); err != nil {
		t.Fatal(err)
	}
	v, err := e.svc.Show(ctx, e.on, "flat", "")
	if err != nil || len(v.History) != 0 {
		t.Errorf("history = %v err = %v", v.History, err)
	}
	if err := e.svc.ApplyRemoveValuation(ctx, p); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("second delete err = %v", err)
	}
}

func TestLinkUnlink(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	e.add(t, assetting.AddRequest{Name: "Flat", Kind: "property", Value: "1000"})
	e.add(t, assetting.AddRequest{Name: "Villa", Kind: "property", Value: "1000", Currency: "USD"})
	e.add(t, assetting.AddRequest{Name: "Garage", Kind: "property", Value: "1000"})

	if _, err := e.svc.PlanUnlink(ctx, e.on, "flat"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("unlink without link err = %v", err)
	}
	if _, err := e.svc.PlanLink(ctx, e.on, "flat", "zzz"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("unknown loan err = %v", err)
	}
	if _, err := e.svc.PlanLink(ctx, e.on, "zzz", "mort"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("unknown asset err = %v", err)
	}
	if _, err := e.svc.PlanLink(ctx, e.on, "villa", "mort"); !errors.Is(err, domain.ErrCurrencyMismatch) {
		t.Errorf("currency err = %v", err)
	}
	p, err := e.svc.PlanLink(ctx, e.on, "flat", "mort")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.svc.ApplyLink(ctx, p); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.PlanLink(ctx, e.on, "garage", "mort"); !errors.Is(err, app.ErrInUse) {
		t.Errorf("second link err = %v", err)
	}
	if _, err := e.svc.PlanLink(ctx, e.on, "flat", "mort"); err != nil {
		t.Errorf("relink same asset err = %v", err)
	}
	v, err := e.svc.Show(ctx, e.on, "flat", "")
	if err != nil || v.Loan != "Mortgage" || v.Debt.Sign() <= 0 || v.Net.Minor() != v.Latest.Value.Minor()-v.Debt.Minor() {
		t.Errorf("view = %+v err = %v", v.Item, err)
	}

	u, err := e.svc.PlanUnlink(ctx, e.on, "flat")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.svc.ApplyUnlink(ctx, u); err != nil {
		t.Fatal(err)
	}
	v, err = e.svc.Show(ctx, e.on, "flat", "")
	if err != nil || v.Loan != "" || v.Debt.Sign() != 0 || v.Asset.LoanID != 0 {
		t.Errorf("after unlink = %+v err = %v", v.Item, err)
	}
	if _, err := e.svc.PlanLink(ctx, e.on, "garage", "mort"); err != nil {
		t.Errorf("link after unlink err = %v", err)
	}
}
