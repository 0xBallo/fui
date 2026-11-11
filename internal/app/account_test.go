package app_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

func TestInit(t *testing.T) {
	e := newEnv(t)
	ctx := t.Context()
	e.category(t, "housing", "expense") // already there, in another case

	p, err := e.svc.PlanInit(ctx)
	wantNoErr(t, err)
	if want := len(domain.DefaultCategories()) - 1; len(p.Categories) != want || len(p.Changes) != want {
		t.Fatalf("plan has %d categories and %d changes, want %d", len(p.Categories), len(p.Changes), want)
	}
	if p.Bulk {
		t.Error("init must not be bulk")
	}
	res, err := e.svc.ApplyInit(ctx, p)
	wantNoErr(t, err)
	if res.Created != len(p.Categories) {
		t.Errorf("Created = %d, want %d", res.Created, len(p.Categories))
	}
	again, err := e.svc.PlanInit(ctx)
	wantNoErr(t, err)
	if len(again.Changes) != 0 {
		t.Errorf("second init plans %d changes, want 0 (idempotent)", len(again.Changes))
	}
}

func TestAddAccount(t *testing.T) {
	tests := []struct {
		name    string
		req     app.AddAccountRequest
		wantErr *domain.Error
		check   func(t *testing.T, a domain.Account)
	}{
		{
			name: "defaults opening balance and date",
			req:  app.AddAccountRequest{Name: " Fineco ", Kind: "Checking", Currency: "eur"},
			check: func(t *testing.T, a domain.Account) {
				if a.Name != "Fineco" || a.OpeningBalance.Minor() != 0 || a.OpenedOn.String() != "2026-11-11" {
					t.Errorf("account = %+v", a)
				}
			},
		},
		{
			name: "opening balance in the account currency",
			req:  app.AddAccountRequest{Name: "Yen", Kind: "cash", Currency: "JPY", OpeningBalance: "1,500", OpenedOn: "2026-01-02"},
			check: func(t *testing.T, a domain.Account) {
				if a.OpeningBalance.Minor() != 1500 {
					t.Errorf("opening = %v, want 1500 JPY", a.OpeningBalance)
				}
			},
		},
		{name: "unknown kind", req: app.AddAccountRequest{Name: "X", Kind: "wallet", Currency: "EUR"}, wantErr: domain.ErrInvalidValue},
		{name: "missing kind", req: app.AddAccountRequest{Name: "X", Currency: "EUR"}, wantErr: domain.ErrMissingField},
		{name: "unknown currency", req: app.AddAccountRequest{Name: "X", Kind: "cash", Currency: "ZZZ"}, wantErr: domain.ErrInvalidValue},
		{name: "bad opening balance", req: app.AddAccountRequest{Name: "X", Kind: "cash", Currency: "EUR", OpeningBalance: "abc"}, wantErr: domain.ErrInvalidAmount},
		{name: "bad date", req: app.AddAccountRequest{Name: "X", Kind: "cash", Currency: "EUR", OpenedOn: "someday"}, wantErr: domain.ErrInvalidDate},
		{name: "empty name", req: app.AddAccountRequest{Name: "  ", Kind: "cash", Currency: "EUR"}, wantErr: domain.ErrMissingField},
		{name: "duplicate name", req: app.AddAccountRequest{Name: "cash", Kind: "cash", Currency: "EUR"}, wantErr: app.ErrAlreadyExists},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newEnv(t)
			e.account(t, "Cash", "cash", "EUR", "0")
			p, err := e.svc.PlanAddAccount(t.Context(), tt.req)
			if tt.wantErr != nil {
				wantCode(t, err, tt.wantErr)
				return
			}
			wantNoErr(t, err)
			if len(p.Changes) != 1 || p.Changes[0].Op != app.OpCreate || p.Changes[0].Entity != app.EntityAccount {
				t.Errorf("changes = %+v", p.Changes)
			}
			a, err := e.svc.ApplyAddAccount(t.Context(), p)
			wantNoErr(t, err)
			if a.ID == 0 {
				t.Error("no ID assigned")
			}
			tt.check(t, a)
		})
	}
}

func TestListAndShowAccounts(t *testing.T) {
	e := newEnv(t)
	ctx := t.Context()
	fineco := e.account(t, "Fineco", "checking", "EUR", "1000.00")
	old := e.account(t, "Old", "savings", "EUR", "50")
	e.tx(t, app.AddTxRequest{Account: "Fineco", Amount: "-12.50", Description: "A", Date: "2026-03-01"})
	e.tx(t, app.AddTxRequest{Account: "Fineco", Amount: "100", Description: "B", Date: "2026-06-01"})
	p, err := e.svc.PlanArchiveAccount(ctx, "old")
	wantNoErr(t, err)
	_, err = e.svc.ApplyArchiveAccount(ctx, p)
	wantNoErr(t, err)

	tests := []struct {
		name string
		req  app.ListAccountsRequest
		want map[string]int64
	}{
		{"today, archived hidden", app.ListAccountsRequest{}, map[string]int64{"Fineco": 108750}},
		{"before the second movement", app.ListAccountsRequest{At: "2026-05-31"}, map[string]int64{"Fineco": 98750}},
		{"before any movement", app.ListAccountsRequest{At: "2026-01-01"}, map[string]int64{"Fineco": 100000}},
		{"archived included", app.ListAccountsRequest{IncludeArchived: true}, map[string]int64{"Fineco": 108750, "Old": 5000}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := e.svc.ListAccounts(ctx, tt.req)
			wantNoErr(t, err)
			have := map[string]int64{}
			for _, ab := range got {
				have[ab.Account.Name] = ab.Balance.Minor()
			}
			if !reflect.DeepEqual(have, tt.want) {
				t.Errorf("balances = %v, want %v", have, tt.want)
			}
		})
	}
	ab, err := e.svc.ShowAccount(ctx, "old", "")
	wantNoErr(t, err)
	if ab.Account.ID != old.ID || !ab.Account.Archived || ab.Balance.Minor() != 5000 {
		t.Errorf("ShowAccount(old) = %+v", ab)
	}
	ab, err = e.svc.ShowAccount(ctx, "fin", "2026-03-31")
	wantNoErr(t, err)
	if ab.Account.ID != fineco.ID || ab.Balance.Minor() != 98750 {
		t.Errorf("ShowAccount(fin) = %+v", ab)
	}
	_, err = e.svc.ShowAccount(ctx, "nope", "")
	wantCode(t, err, domain.ErrNotFound)
	_, err = e.svc.ListAccounts(ctx, app.ListAccountsRequest{At: "x"})
	wantCode(t, err, domain.ErrInvalidDate)
}

func TestEditAndArchiveAccount(t *testing.T) {
	e := newEnv(t)
	ctx := t.Context()
	e.account(t, "Fineco", "checking", "EUR", "0")
	e.account(t, "Fineco Risparmio", "savings", "EUR", "0")

	_, err := e.svc.PlanEditAccount(ctx, app.EditAccountRequest{Account: "fineco"})
	wantCode(t, err, domain.ErrMissingField)
	_, err = e.svc.PlanEditAccount(ctx, app.EditAccountRequest{Account: "fineco risp", Name: ptr("FINECO")})
	wantCode(t, err, app.ErrAlreadyExists)
	_, err = e.svc.PlanEditAccount(ctx, app.EditAccountRequest{Account: "fine", Name: ptr("x")})
	wantCode(t, err, domain.ErrAmbiguousName)
}

func TestEditAccountApplied(t *testing.T) {
	e := newEnv(t)
	ctx := t.Context()
	e.account(t, "Fineco", "checking", "EUR", "0")
	p, err := e.svc.PlanEditAccount(ctx, app.EditAccountRequest{
		Account: "fineco", Name: ptr("Fineco Main"), Description: ptr("daily"), Kind: ptr("savings"), OpeningBalance: ptr("10.5"),
	})
	wantNoErr(t, err)
	a, err := e.svc.ApplyEditAccount(ctx, p)
	wantNoErr(t, err)
	if a.Name != "Fineco Main" || a.Description != "daily" || a.Kind != domain.AccountSavings || a.OpeningBalance.Minor() != 1050 {
		t.Errorf("edited = %+v", a)
	}
	got, err := e.svc.ShowAccount(ctx, "main", "")
	wantNoErr(t, err)
	if got.Account != a {
		t.Errorf("stored %+v, want %+v", got.Account, a)
	}
	ap, err := e.svc.PlanArchiveAccount(ctx, "Fineco Main")
	wantNoErr(t, err)
	_, err = e.svc.ApplyArchiveAccount(ctx, ap)
	wantNoErr(t, err)
	_, err = e.svc.PlanArchiveAccount(ctx, "Fineco Main")
	wantCode(t, err, domain.ErrNotFound) // archived accounts are no longer addressable for writes
}

func TestTodayFollowsTheConfiguredZone(t *testing.T) {
	kiritimati, err := time.LoadLocation("Pacific/Kiritimati")
	if err != nil {
		t.Skipf("no tzdata: %v", err)
	}
	e := newEnv(t, func(c *app.Config) { c.Location = kiritimati })
	got, err := e.svc.Today()
	wantNoErr(t, err)
	if got.String() != "2026-11-12" { // 10:00 UTC is already the 12th at UTC+14
		t.Errorf("Today = %s, want 2026-11-12", got)
	}
}
