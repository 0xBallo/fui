package assetting_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/assetting"
	"github.com/0xballo/fui/internal/app/lending"
	"github.com/0xballo/fui/internal/clock"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
	"github.com/0xballo/fui/internal/testutil/memstore"
)

type spyBackup struct{ n int }

func (b *spyBackup) Backup(context.Context, string) (string, error) { b.n++; return "b", nil }
func (b *spyBackup) Restore(context.Context, string) error          { return nil }

type env struct {
	svc    *assetting.Service
	store  *memstore.Store
	backup *spyBackup
	on     module.Set
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

func date(s string) domain.Date { return must(domain.ParseDate(s)) }

func newEnv(t *testing.T) env {
	t.Helper()
	e := env{
		store: memstore.New(), backup: &spyBackup{},
		on: must(module.Validate([]module.ID{module.Core, module.Recurring, module.Loans, module.Assets})),
	}
	eur := must(domain.ParseCurrency("EUR"))
	clk := clock.NewFixed(date("2026-03-15").In(time.UTC))
	e.svc = assetting.New(e.store, clk, e.backup, assetting.Config{BaseCurrency: eur})
	ctx := context.Background()
	err := e.store.WithTx(ctx, func(r app.Repos) error {
		_, err := r.Accounts().Insert(ctx, domain.Account{
			Name: "Main", Kind: domain.AccountChecking, Currency: eur,
			OpeningBalance: must(domain.NewMoney(5_000_000, eur)), OpenedOn: date("2025-01-01"),
		})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	lend := lending.New(e.store, clk, e.backup, lending.Config{})
	p, err := lend.PlanAdd(ctx, e.on, lending.AddRequest{
		Name: "Mortgage", Kind: "mortgage", Principal: "12000", Disbursed: "2026-01-01", TermMonths: 12,
		RateType: "fixed", Rate: "3,5", Debit: "Main",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := lend.ApplyAdd(ctx, p); err != nil {
		t.Fatal(err)
	}
	return e
}

func (e env) add(t *testing.T, req assetting.AddRequest) domain.Asset {
	t.Helper()
	ctx := context.Background()
	p, err := e.svc.PlanAdd(ctx, e.on, req)
	if err != nil {
		t.Fatalf("PlanAdd: %v", err)
	}
	a, err := e.svc.ApplyAdd(ctx, p)
	if err != nil {
		t.Fatalf("ApplyAdd: %v", err)
	}
	return a
}

func TestRequiresModule(t *testing.T) {
	e := newEnv(t)
	off := must(module.Validate([]module.ID{module.Core}))
	ctx := context.Background()
	if _, err := e.svc.PlanAdd(ctx, off, assetting.AddRequest{}); !errors.Is(err, app.ErrModuleDisabled) {
		t.Errorf("PlanAdd err = %v, want ErrModuleDisabled", err)
	}
	if _, err := e.svc.PlanValue(ctx, off, assetting.ValueRequest{}); !errors.Is(err, app.ErrModuleDisabled) {
		t.Errorf("PlanValue err = %v, want ErrModuleDisabled", err)
	}
	if _, err := e.svc.List(ctx, off, ""); !errors.Is(err, app.ErrModuleDisabled) {
		t.Errorf("List err = %v, want ErrModuleDisabled", err)
	}
	if _, err := e.svc.Show(ctx, off, "x", ""); !errors.Is(err, app.ErrModuleDisabled) {
		t.Errorf("Show err = %v, want ErrModuleDisabled", err)
	}
}

func TestAddDryRunAndApply(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	req := assetting.AddRequest{Name: "Flat", Kind: "property", Value: "200000", PurchaseDate: "2026-01-01", Loan: "mort"}
	before := e.store.Snapshot()
	p, err := e.svc.PlanAdd(ctx, e.on, req)
	if err != nil {
		t.Fatal(err)
	}
	if e.store.Snapshot() != before {
		t.Error("PlanAdd wrote to the store")
	}
	if len(p.Changes) != 1 || p.Changes[0].Op != app.OpCreate || p.Changes[0].Entity != assetting.EntityAsset ||
		p.Changes[0].Fields["purchase_value"] != int64(200_000_00) || p.Asset.LoanID == 0 {
		t.Errorf("plan = %+v", p)
	}
	a := e.add(t, req)
	if a.ID == 0 || a.PurchaseValue.Currency().Code() != "EUR" {
		t.Errorf("asset = %+v", a)
	}
}

func TestAddErrors(t *testing.T) {
	e := newEnv(t)
	e.add(t, assetting.AddRequest{Name: "Car", Kind: "vehicle", Value: "10000"})
	tests := []struct {
		name string
		req  assetting.AddRequest
		want error
	}{
		{"duplicate name", assetting.AddRequest{Name: "car", Kind: "vehicle", Value: "1"}, app.ErrAlreadyExists},
		{"unknown loan", assetting.AddRequest{Name: "X", Kind: "other", Value: "1", Loan: "nope"}, domain.ErrNotFound},
		{"loan currency mismatch", assetting.AddRequest{Name: "X", Kind: "other", Value: "1", Loan: "Mortgage", Currency: "USD"}, domain.ErrCurrencyMismatch},
		{"bad kind", assetting.AddRequest{Name: "X", Kind: "boat", Value: "1"}, domain.ErrInvalidValue},
		{"negative value", assetting.AddRequest{Name: "X", Kind: "other", Value: "-1"}, domain.ErrInvalidValue},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := e.svc.PlanAdd(context.Background(), e.on, tt.req)
			if !errors.Is(err, tt.want) {
				t.Errorf("err = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestAddNeedsCurrency(t *testing.T) {
	e := newEnv(t)
	svc := assetting.New(e.store, clock.NewFixed(time.Unix(0, 0)), e.backup, assetting.Config{})
	_, err := svc.PlanAdd(context.Background(), e.on, assetting.AddRequest{Name: "X", Kind: "other", Value: "1"})
	if !errors.Is(err, domain.ErrInvalidValue) {
		t.Errorf("err = %v, want ErrInvalidValue", err)
	}
	p, err := svc.PlanAdd(context.Background(), e.on, assetting.AddRequest{Name: "X", Kind: "other", Value: "1", Currency: "usd"})
	if err != nil || p.Asset.PurchaseValue.Currency().Code() != "USD" {
		t.Errorf("explicit currency: %+v, %v", p.Asset, err)
	}
}

func TestValuationUpsert(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	e.add(t, assetting.AddRequest{Name: "Flat", Kind: "property", Value: "200000", PurchaseDate: "2026-01-01"})
	value := func(at, v string) assetting.ValuePlan {
		p, err := e.svc.PlanValue(ctx, e.on, assetting.ValueRequest{Asset: "flat", Date: at, Value: v})
		if err != nil {
			t.Fatalf("PlanValue: %v", err)
		}
		return p
	}

	p := value("2026-02-01", "210000")
	if p.Overwrite || p.Bulk || p.Changes[0].Op != app.OpCreate {
		t.Errorf("first plan = %+v, want plain create", p.Plan)
	}
	before := e.store.Snapshot()
	value("2026-02-01", "1")
	if e.store.Snapshot() != before {
		t.Error("PlanValue wrote to the store")
	}
	if err := e.svc.ApplyValue(ctx, p); err != nil {
		t.Fatal(err)
	}
	if e.backup.n != 0 {
		t.Errorf("backups = %d, want 0 for a new valuation", e.backup.n)
	}

	p = value("2026-02-01", "215000")
	if !p.Overwrite || !p.Bulk || p.Previous.Minor() != 210_000_00 || p.Changes[0].Op != app.OpUpdate ||
		p.Changes[0].Fields["previous"] != int64(210_000_00) {
		t.Errorf("overwrite plan = %+v", p)
	}
	if err := e.svc.ApplyValue(ctx, p); err != nil {
		t.Fatal(err)
	}
	if e.backup.n != 1 {
		t.Errorf("backups = %d, want 1 for an overwrite", e.backup.n)
	}

	v, err := e.svc.Show(ctx, e.on, "flat", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(v.History) != 1 || v.History[0].Value.Minor() != 215_000_00 || v.Latest.Value.Minor() != 215_000_00 {
		t.Errorf("view = %+v", v)
	}
}

func TestValueErrors(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	e.add(t, assetting.AddRequest{Name: "Flat", Kind: "property", Value: "1000"})
	if _, err := e.svc.PlanValue(ctx, e.on, assetting.ValueRequest{Asset: "zzz", Value: "1"}); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("unknown asset err = %v", err)
	}
	if _, err := e.svc.PlanValue(ctx, e.on, assetting.ValueRequest{Asset: "flat", Value: "-5"}); !errors.Is(err, domain.ErrInvalidValue) {
		t.Errorf("negative value err = %v", err)
	}
	if _, err := e.svc.PlanValue(ctx, e.on, assetting.ValueRequest{Asset: "flat", Date: "garbage", Value: "5"}); err == nil {
		t.Error("bad date accepted")
	}
}

func TestListNetOfDebt(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	e.add(t, assetting.AddRequest{Name: "Flat", Kind: "property", Value: "200000", PurchaseDate: "2026-01-01", Loan: "Mortgage"})
	e.add(t, assetting.AddRequest{Name: "Car", Kind: "vehicle", Value: "10000", PurchaseDate: "2026-01-01"})
	p, err := e.svc.PlanValue(ctx, e.on, assetting.ValueRequest{Asset: "Flat", Date: "2026-02-01", Value: "220000"})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.svc.ApplyValue(ctx, p); err != nil {
		t.Fatal(err)
	}

	items, err := e.svc.List(ctx, e.on, "")
	if err != nil {
		t.Fatal(err)
	}
	type row struct {
		Name              string
		Latest, Debt, Net int64
		Date              string
	}
	var got []row
	for _, it := range items {
		got = append(got, row{it.Asset.Name, it.Latest.Value.Minor(), it.Debt.Minor(), it.Net.Minor(), it.Latest.Date.String()})
	}
	want := []row{
		{"Flat", 220_000_00, 12_000_00, 208_000_00, "2026-02-01"},
		{"Car", 10_000_00, 0, 10_000_00, "2026-01-01"},
	}
	if d := cmp.Diff(want, got); d != "" {
		t.Errorf("List mismatch (-want +got):\n%s", d)
	}

	// Before the first valuation the purchase value is the latest value.
	items, err = e.svc.List(ctx, e.on, "2026-01-15")
	if err != nil || items[0].Latest.Value.Minor() != 200_000_00 {
		t.Errorf("List at purchase = %+v, %v", items, err)
	}
}

func TestEmptyList(t *testing.T) {
	e := newEnv(t)
	items, err := e.svc.List(context.Background(), e.on, "")
	if err != nil || items == nil || len(items) != 0 {
		t.Errorf("List = %v, %v; want empty non-nil", items, err)
	}
}
