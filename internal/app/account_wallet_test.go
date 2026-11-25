package app_test

import (
	"context"
	"testing"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/module"
)

func TestWalletAccountNeedsCryptoModule(t *testing.T) {
	ctx := context.Background()
	req := app.AddAccountRequest{Name: "Ledger", Kind: "wallet", Currency: "EUR"}

	off := newEnv(t)
	_, err := off.svc.PlanAddAccount(ctx, req)
	wantCode(t, err, app.ErrModuleDisabled)

	set, err := module.Validate(append(module.Default().IDs(), module.Investments, module.Crypto))
	wantNoErr(t, err)
	on := newEnv(t, func(c *app.Config) { c.Modules = set })
	_, err = on.svc.PlanAddAccount(ctx, req)
	wantNoErr(t, err)

	off.account(t, "Plain", "cash", "EUR", "0")
	_, err = off.svc.PlanEditAccount(ctx, app.EditAccountRequest{Account: "Plain", Kind: ptr("wallet")})
	wantCode(t, err, app.ErrModuleDisabled)
}
