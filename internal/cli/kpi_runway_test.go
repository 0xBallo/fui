package cli_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/0xballo/fui/internal/app/investing"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
)

func TestKPIRunwayIncludeInvestments(t *testing.T) {
	h := newHarness(t)
	enableModules(t, h, module.Investments, module.Crypto)
	seedSpendingHistory(h)
	h.account("Broker", "investment", "EUR", "0")

	inv := newInvestingService(t, h, investingModules(t, true))
	ctx := context.Background()
	secPlan, err := inv.PlanAddSecurity(ctx, investing.AddSecurityRequest{
		Name: "VWCE ETF", Kind: "etf", Ticker: "VWCE", Currency: "EUR",
	})
	if err != nil {
		t.Fatalf("PlanAddSecurity: %v", err)
	}
	if _, err := inv.ApplyAddSecurity(ctx, secPlan); err != nil {
		t.Fatalf("ApplyAddSecurity: %v", err)
	}
	pricePlan, err := inv.PlanSetPrice(ctx, investing.SetPriceRequest{Security: "VWCE", Date: "2026-11-10", Price: "50"})
	if err != nil {
		t.Fatalf("PlanSetPrice: %v", err)
	}
	if err := inv.ApplySetPrice(ctx, pricePlan); err != nil {
		t.Fatalf("ApplySetPrice: %v", err)
	}
	buyPlan, err := inv.PlanBuy(ctx, investing.OpRequest{
		Kind: domain.OpBuy, Account: "Broker", Security: "VWCE", Date: "2026-11-10", Quantity: "10", Price: "40", Fees: "0",
	})
	if err != nil {
		t.Fatalf("PlanBuy: %v", err)
	}
	if _, err := inv.ApplyOp(ctx, buyPlan); err != nil {
		t.Fatalf("ApplyOp buy: %v", err)
	}

	baseOut, _ := h.mustRun("kpi", "runway", "--at", "2026-11-10", "--json")
	withInvOut, _ := h.mustRun("kpi", "runway", "--at", "2026-11-10", "--include-investments", "--json")

	var base struct {
		Liquidity int64 `json:"liquidity"`
	}
	if err := json.Unmarshal([]byte(baseOut), &base); err != nil {
		t.Fatalf("unmarshal base runway json: %v", err)
	}
	var withInv struct {
		Liquidity int64 `json:"liquidity"`
	}
	if err := json.Unmarshal([]byte(withInvOut), &withInv); err != nil {
		t.Fatalf("unmarshal runway include investments json: %v", err)
	}
	if delta := withInv.Liquidity - base.Liquidity; delta != 50000 {
		t.Fatalf("liquidity delta = %d, want 50000", delta)
	}
}
