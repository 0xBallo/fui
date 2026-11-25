package memstore_test

import (
	"context"
	"errors"
	"testing"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/testutil/memstore"
)

func TestSecurityRepo(t *testing.T) {
	ctx := context.Background()
	eur, _ := domain.ParseCurrency("EUR")
	d1, _ := domain.ParseDate("2026-01-01")
	d2, _ := domain.ParseDate("2026-02-01")
	price := func(n int64) domain.Price { p, _ := domain.NewPrice(n, 2, eur); return p }
	money := func(n int64) domain.Money { m, _ := domain.NewMoney(n, eur); return m }
	qty, _ := domain.NewQuantity(1, 0)

	err := memstore.New().WithTx(ctx, func(r app.Repos) error {
		repo := r.Securities()
		a, err := repo.Insert(ctx, domain.Security{Name: "A", Ticker: "a", Kind: domain.SecurityETF, Currency: eur})
		if err != nil {
			return err
		}
		if _, err := repo.Insert(ctx, domain.Security{Name: "a", Ticker: "Z", Kind: domain.SecurityETF, Currency: eur}); !errors.Is(err, app.ErrAlreadyExists) {
			t.Errorf("duplicate name: %v", err)
		}
		if _, err := repo.Insert(ctx, domain.Security{Name: "B", Ticker: "A", Kind: domain.SecurityETF, Currency: eur}); !errors.Is(err, app.ErrAlreadyExists) {
			t.Errorf("duplicate ticker: %v", err)
		}
		a.Name = "Alpha"
		if err := repo.Update(ctx, a); err != nil {
			t.Error(err)
		}
		if err := repo.Update(ctx, domain.Security{ID: 99, Name: "X", Ticker: "X", Kind: domain.SecurityETF, Currency: eur}); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("update missing: %v", err)
		}
		for _, p := range []domain.SecurityPrice{{Security: a.ID, Date: d2, Price: price(20)}, {Security: a.ID, Date: d1, Price: price(10)}} {
			if err := repo.UpsertPrice(ctx, p); err != nil {
				return err
			}
		}
		if ps, _ := repo.ListPrices(ctx, a.ID); len(ps) != 2 || ps[0].Date != d1 {
			t.Errorf("prices = %+v", ps)
		}
		if p, err := repo.PriceOnOrBefore(ctx, a.ID, d2); err != nil || p.Price.Minor() != 20 {
			t.Errorf("on or before = %+v, %v", p, err)
		}
		if _, err := repo.PriceOnOrBefore(ctx, a.ID, domain.Date{}); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("no price: %v", err)
		}
		if err := repo.Delete(ctx, a.ID); !errors.Is(err, app.ErrInUse) {
			t.Errorf("delete with prices: %v", err)
		}
		if err := repo.DeletePrice(ctx, a.ID, d2); err != nil {
			t.Error(err)
		}
		if err := repo.DeletePrice(ctx, a.ID, d2); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("delete missing price: %v", err)
		}
		acct, err := r.Accounts().Insert(ctx, domain.Account{Name: "Br", Kind: domain.AccountInvestment, Currency: eur, OpeningBalance: money(0), OpenedOn: d1})
		if err != nil {
			return err
		}
		op, err := repo.InsertOp(ctx, domain.SecurityOp{Date: d2, Account: acct.ID, Security: a.ID, Kind: domain.OpBuy, Quantity: qty, UnitPrice: price(10)})
		if err != nil {
			return err
		}
		if _, err := repo.InsertOp(ctx, domain.SecurityOp{Date: d2, Account: 99, Security: a.ID, Kind: domain.OpBuy, Quantity: qty, UnitPrice: price(10)}); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("op on missing account: %v", err)
		}
		op.Date = d1
		if err := repo.UpdateOp(ctx, op); err != nil {
			t.Error(err)
		}
		if got, err := repo.GetOp(ctx, op.ID); err != nil || got.Date != d1 {
			t.Errorf("get op = %+v, %v", got, err)
		}
		if ops, _ := repo.ListOpsBySecurity(ctx, a.ID, d2, domain.Date{}); len(ops) != 0 {
			t.Errorf("range: %+v", ops)
		}
		if ops, _ := repo.ListOpsBySecurity(ctx, a.ID, domain.Date{}, d2); len(ops) != 1 {
			t.Errorf("range: %+v", ops)
		}
		if err := repo.Delete(ctx, a.ID); !errors.Is(err, app.ErrInUse) {
			t.Errorf("delete with ops: %v", err)
		}
		if err := repo.DeleteOp(ctx, op.ID); err != nil {
			t.Error(err)
		}
		if err := repo.DeleteOp(ctx, op.ID); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("delete missing op: %v", err)
		}
		if err := repo.DeletePrice(ctx, a.ID, d1); err != nil {
			t.Error(err)
		}
		if err := repo.Delete(ctx, a.ID); err != nil {
			t.Error(err)
		}
		if _, err := repo.Get(ctx, a.ID); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("get deleted: %v", err)
		}
		if l, _ := repo.List(ctx); len(l) != 0 {
			t.Errorf("list: %+v", l)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := memstore.New().Read(ctx, func(r app.Repos) error {
		return r.Securities().Delete(ctx, 1)
	}); err == nil {
		t.Fatal("read-only delete must fail")
	}
}

func TestSecurityRepoSubCentPrice(t *testing.T) {
	ctx := context.Background()
	usd, _ := domain.ParseCurrency("USD")
	day, _ := domain.ParseDate("2026-05-01")
	err := memstore.New().WithTx(ctx, func(r app.Repos) error {
		repo := r.Securities()
		acct, err := r.Accounts().Insert(ctx, domain.Account{Name: "W", Kind: domain.AccountWallet, Currency: usd, OpeningBalance: domain.Money{}, OpenedOn: day})
		if err != nil {
			return err
		}
		sec, err := repo.Insert(ctx, domain.Security{Name: "Shib", Ticker: "shib", Kind: domain.SecurityCrypto, Currency: usd})
		if err != nil {
			return err
		}
		if sec.PriceScale != 8 {
			t.Errorf("PriceScale = %d, want 8", sec.PriceScale)
		}
		price, _ := domain.NewPrice(1234, 8, usd)
		if err := repo.UpsertPrice(ctx, domain.SecurityPrice{Security: sec.ID, Date: day, Price: price}); err != nil {
			return err
		}
		if p, err := repo.PriceOnOrBefore(ctx, sec.ID, day); err != nil || p.Price.String() != "0.00001234" {
			t.Errorf("price = %v, %v", p.Price, err)
		}
		wrong, _ := domain.NewPrice(12, 2, usd)
		if err := repo.UpsertPrice(ctx, domain.SecurityPrice{Security: sec.ID, Date: day, Price: wrong}); !errors.Is(err, domain.ErrInvalidValue) {
			t.Errorf("wrong scale price: %v", err)
		}
		qty, _ := domain.NewQuantity(5, 0)
		if _, err := repo.InsertOp(ctx, domain.SecurityOp{Date: day, Account: acct.ID, Security: sec.ID, Kind: domain.OpBuy, Quantity: qty, UnitPrice: wrong}); !errors.Is(err, domain.ErrInvalidValue) {
			t.Errorf("wrong scale op: %v", err)
		}
		if _, err := repo.InsertOp(ctx, domain.SecurityOp{Date: day, Account: acct.ID, Security: sec.ID, Kind: domain.OpBuy, Quantity: qty, UnitPrice: price}); err != nil {
			t.Errorf("sub-cent op: %v", err)
		}
		other, _ := domain.ParseCurrency("EUR")
		eurPrice, _ := domain.NewPrice(1, 8, other)
		if err := repo.UpsertPrice(ctx, domain.SecurityPrice{Security: sec.ID, Date: day, Price: eurPrice}); !errors.Is(err, domain.ErrCurrencyMismatch) {
			t.Errorf("currency mismatch: %v", err)
		}
		sec.PriceScale = 3
		if err := repo.Update(ctx, sec); err != nil {
			return err
		}
		if got, _ := repo.Get(ctx, sec.ID); got.PriceScale != 8 {
			t.Errorf("PriceScale after update = %d, want 8", got.PriceScale)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
