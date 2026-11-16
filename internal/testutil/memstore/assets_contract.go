package memstore

import (
	"context"
	"errors"
	"testing"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

func contractAssets(t *testing.T, f *fixture) {
	ctx := context.Background()
	loanAcc := f.loanAccount(t, "Mortgage account")
	loan := insertContractLoan(t, f, f.account(t, "Debit").ID, loanAcc.ID)
	var home domain.Asset
	f.do(t, func(r app.Repos) (err error) {
		home, err = r.Assets().Insert(ctx, domain.Asset{
			Name: "Home", Kind: domain.AssetKindProperty, PurchaseDate: mustDate(t, "2026-01-01"),
			PurchaseValue: mustMoney(t, 200_000_00), LoanID: loan.ID,
		})
		return err
	})
	if home.ID == 0 {
		t.Fatal("inserted asset has zero ID")
	}
	assetErrors(t, f, home, loan)
	assetValuations(t, f, home)
	assetLifecycle(t, f, home, loan)
}

func assetErrors(t *testing.T, f *fixture, home domain.Asset, loan domain.Loan) {
	ctx := context.Background()
	f.do(t, func(r app.Repos) error {
		_, err := r.Assets().Insert(ctx, domain.Asset{
			Name: "HOME", Kind: domain.AssetKindOther, PurchaseDate: mustDate(t, "2026-01-01"), PurchaseValue: mustMoney(t, 1),
		})
		if !errors.Is(err, app.ErrAlreadyExists) {
			t.Errorf("duplicate name err = %v, want ErrAlreadyExists", err)
		}
		_, err = r.Assets().Insert(ctx, domain.Asset{
			Name: "Car", Kind: domain.AssetKindVehicle, PurchaseDate: mustDate(t, "2026-01-01"),
			PurchaseValue: mustMoney(t, 1), LoanID: loan.ID + 99,
		})
		if !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("missing loan err = %v, want ErrNotFound", err)
		}
		if err := r.Assets().SetValuation(ctx, domain.Valuation{AssetID: home.ID + 99, Date: mustDate(t, "2026-02-01"), Value: mustMoney(t, 1)}); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("valuation of missing asset err = %v, want ErrNotFound", err)
		}
		return nil
	})
}

func assetValuations(t *testing.T, f *fixture, home domain.Asset) {
	ctx := context.Background()
	f.do(t, func(r app.Repos) error {
		for _, v := range []struct {
			date  string
			minor int64
		}{{"2026-03-01", 210_000_00}, {"2026-02-01", 205_000_00}, {"2026-03-01", 220_000_00}} {
			if err := r.Assets().SetValuation(ctx, domain.Valuation{AssetID: home.ID, Date: mustDate(t, v.date), Value: mustMoney(t, v.minor)}); err != nil {
				t.Fatalf("SetValuation: %v", err)
			}
		}
		vs, err := r.Assets().ListValuations(ctx, home.ID)
		if err != nil || len(vs) != 2 || vs[0].Value.Minor() != 205_000_00 || vs[1].Value.Minor() != 220_000_00 {
			t.Errorf("ListValuations = %+v, %v", vs, err)
		}
		if err := r.Assets().DeleteValuation(ctx, home.ID, mustDate(t, "2026-02-01")); err != nil {
			t.Errorf("DeleteValuation: %v", err)
		}
		if err := r.Assets().DeleteValuation(ctx, home.ID, mustDate(t, "2026-02-01")); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("DeleteValuation again err = %v, want ErrNotFound", err)
		}
		return nil
	})
}

func assetLifecycle(t *testing.T, f *fixture, home domain.Asset, loan domain.Loan) {
	ctx := context.Background()
	f.do(t, func(r app.Repos) error {
		if err := r.Loans().Delete(ctx, loan.ID); err != nil {
			t.Fatalf("delete loan: %v", err)
		}
		got, err := r.Assets().Get(ctx, home.ID)
		if err != nil || got.LoanID != 0 {
			t.Errorf("after loan delete Get = %+v, %v; want unlinked", got, err)
		}
		got.Name = "House"
		if err := r.Assets().Update(ctx, got); err != nil {
			t.Errorf("Update: %v", err)
		}
		if l, err := r.Assets().List(ctx); err != nil || len(l) != 1 || l[0].Name != "House" {
			t.Errorf("List = %+v, %v", l, err)
		}
		if err := r.Assets().Delete(ctx, home.ID); err != nil {
			t.Errorf("Delete: %v", err)
		}
		if _, err := r.Assets().Get(ctx, home.ID); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("Get deleted err = %v, want ErrNotFound", err)
		}
		if err := r.Assets().Delete(ctx, home.ID); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("Delete again err = %v, want ErrNotFound", err)
		}
		if err := r.Assets().Update(ctx, got); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("Update deleted err = %v, want ErrNotFound", err)
		}
		return nil
	})
}
