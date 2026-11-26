package cli_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/0xballo/fui/internal/app/assetting"
	"github.com/0xballo/fui/internal/app/investing"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
)

// TestKPINetWorthJSON covers the UC-04 "net worth" KPI in the single-currency case: the total
// equals the sum of account balances and each line carries its own and converted amount.
func TestKPINetWorthJSON(t *testing.T) {
	h := newHarness(t)
	h.account("Checking", "checking", "EUR", "1000")
	h.account("Savings", "savings", "EUR", "500")

	out, _ := h.mustRun("kpi", "networth", "--json")
	if !strings.Contains(out, `"currency":"EUR"`) || !strings.Contains(out, `"total":150000`) {
		t.Fatalf("stdout = %q, want currency EUR and total 150000", out)
	}
	if !strings.Contains(out, `"name":"Checking"`) || !strings.Contains(out, `"name":"Savings"`) {
		t.Fatalf("stdout = %q, want both account lines", out)
	}
}

// TestKPINetWorthMultiCurrency covers a cross-currency account converted at the fx rate in
// force, summed into the base currency total.
func TestKPINetWorthMultiCurrency(t *testing.T) {
	h := newHarness(t)
	h.account("Checking", "checking", "EUR", "1000")
	h.account("Revolut", "checking", "USD", "1000")
	h.mustRun("fx", "set", "USD", "2", "--date", "2026-01-01")

	out, _ := h.mustRun("kpi", "networth", "--json")
	if !strings.Contains(out, `"total":300000`) {
		t.Fatalf("stdout = %q, want total 300000 (1000 EUR + 1000 USD converted at rate 2 = 2000 EUR)", out)
	}
}

// TestKPINetWorthRateMissing covers the "rate_missing" error (exit 1) when a foreign-currency
// account has no fx rate recorded by the requested date.
func TestKPINetWorthRateMissing(t *testing.T) {
	h := newHarness(t)
	h.account("Revolut", "checking", "USD", "1000")

	_, stderr, code := h.run("kpi", "networth", "--json")
	if code != 1 {
		t.Fatalf("exit = %d, want 1; stderr=%s", code, stderr)
	}
	if !strings.Contains(stderr, "rate_missing") {
		t.Fatalf("stderr = %q, want rate_missing", stderr)
	}
}

// TestKPINetWorthTableAndCSV covers the table and --csv renderers; --csv must stay in machine
// format (dot decimal) even under the it-IT locale.
func TestKPINetWorthTableAndCSV(t *testing.T) {
	h := newHarness(t)
	h.useItalianLocale()
	h.account("Checking", "checking", "EUR", "1000")

	table, _ := h.mustRun("kpi", "networth")
	if !strings.Contains(table, "Checking") || !strings.Contains(table, "1.000,00") {
		t.Fatalf("table = %q, want it-IT formatted balance 1.000,00", table)
	}

	csv, _ := h.mustRun("kpi", "networth", "--csv")
	if !strings.Contains(csv, "1000.00") || strings.Contains(csv, "1.000,00") {
		t.Fatalf("csv = %q, want machine-format balance 1000.00, not locale-formatted", csv)
	}
}

// TestKPINetWorthAt covers the --at flag.
func TestKPINetWorthAt(t *testing.T) {
	h := newHarness(t)
	h.account("Checking", "checking", "EUR", "1000")

	out, _ := h.mustRun("kpi", "networth", "--at", "2026-11-10", "--json")
	if !strings.Contains(out, `"as_of":"2026-11-10"`) {
		t.Fatalf("stdout = %q, want as_of 2026-11-10", out)
	}
}

func TestKPINetWorthIncludesLoanDebtWhenLoansEnabled(t *testing.T) {
	h := newHarness(t)
	enableModules(t, h, module.Loans)
	h.account("Checking", "checking", "EUR", "1000")
	h.account("Car Loan", "loan", "EUR", "-300")

	out, _ := h.mustRun("kpi", "networth", "--json")
	if !strings.Contains(out, `"total":70000`) {
		t.Fatalf("stdout = %q, want total 70000 with loan debt included", out)
	}
}

func TestKPINetWorthIncludesPositionsAndAssetsComponents(t *testing.T) {
	h := newHarness(t)
	enableModules(t, h, module.Investments, module.Crypto, module.Assets)
	h.account("Broker", "investment", "EUR", "0")
	h.account("Cash", "checking", "EUR", "1000")

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
	pricePlan, err := inv.PlanSetPrice(ctx, investing.SetPriceRequest{Security: "VWCE", Date: "2026-03-01", Price: "50"})
	if err != nil {
		t.Fatalf("PlanSetPrice: %v", err)
	}
	if err := inv.ApplySetPrice(ctx, pricePlan); err != nil {
		t.Fatalf("ApplySetPrice: %v", err)
	}
	buyPlan, err := inv.PlanBuy(ctx, investing.OpRequest{
		Kind: domain.OpBuy, Account: "Broker", Security: "VWCE", Date: "2026-03-01", Quantity: "10", Price: "40", Fees: "0",
	})
	if err != nil {
		t.Fatalf("PlanBuy: %v", err)
	}
	if _, err := inv.ApplyOp(ctx, buyPlan); err != nil {
		t.Fatalf("ApplyOp buy: %v", err)
	}

	enabled, err := h.cfg.ModulesEnabled()
	if err != nil {
		t.Fatalf("ModulesEnabled: %v", err)
	}
	assetSvc := assetting.New(h.store, fixedClock{time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC)}, h.backup, assetting.Config{
		Locale: "en-US", Location: time.UTC, BaseCurrency: currency(t, "EUR"),
	})
	addPlan, err := assetSvc.PlanAdd(ctx, enabled, assetting.AddRequest{
		Name: "House", Kind: "property", Value: "200000", PurchaseDate: "2026-01-01",
	})
	if err != nil {
		t.Fatalf("PlanAdd asset: %v", err)
	}
	if _, err := assetSvc.ApplyAdd(ctx, addPlan); err != nil {
		t.Fatalf("ApplyAdd asset: %v", err)
	}
	valPlan, err := assetSvc.PlanValue(ctx, enabled, assetting.ValueRequest{
		Asset: "House", Date: "2026-03-01", Value: "210000",
	})
	if err != nil {
		t.Fatalf("PlanValue asset: %v", err)
	}
	if err := assetSvc.ApplyValue(ctx, valPlan); err != nil {
		t.Fatalf("ApplyValue asset: %v", err)
	}

	out, _ := h.mustRun("kpi", "networth", "--at", "2026-03-01", "--json")
	if !strings.Contains(out, `"positions":{"minor":50000,"currency":"EUR"}`) {
		t.Fatalf("stdout = %q, want positions component", out)
	}
	if !strings.Contains(out, `"assets":{"minor":21000000,"currency":"EUR"}`) {
		t.Fatalf("stdout = %q, want assets component", out)
	}
	if !strings.Contains(out, `"total":21150000`) {
		t.Fatalf("stdout = %q, want total 21150000 with positions and assets", out)
	}
}

// TestKPINetWorthItemizesAssets covers the per-asset itemization: JSON asset_lines, extra table
// and CSV rows, unchanged totals, and no trace when the assets module is off.
func TestKPINetWorthItemizesAssets(t *testing.T) {
	h := newHarness(t)
	enableModules(t, h, module.Assets)
	h.account("Cash", "checking", "EUR", "1000")
	ctx := context.Background()
	enabled, err := h.cfg.ModulesEnabled()
	if err != nil {
		t.Fatalf("ModulesEnabled: %v", err)
	}
	svc := newAssetService(t, h)
	plan, err := svc.PlanAdd(ctx, enabled, assetting.AddRequest{
		Name: "House", Kind: "property", Value: "200000", PurchaseDate: "2026-01-01",
	})
	if err != nil {
		t.Fatalf("PlanAdd: %v", err)
	}
	if _, err := svc.ApplyAdd(ctx, plan); err != nil {
		t.Fatalf("ApplyAdd: %v", err)
	}

	args := []string{"kpi", "networth", "--at", "2026-03-01"}
	out, _, err := runWithAssetCommands(h, enabled, append(args, "--json")...)
	if err != nil {
		t.Fatalf("json: %v", err)
	}
	want := `"asset_lines":[{"name":"House","currency":"EUR","value":20000000,"debt":0,"net":20000000,"loan":""}]`
	if !strings.Contains(out, want) {
		t.Fatalf("stdout = %q, want %s", out, want)
	}
	if !strings.Contains(out, `"total":20100000`) {
		t.Fatalf("stdout = %q, want total 20100000 (cash + assets, itemization adds nothing)", out)
	}
	csv, _, err := runWithAssetCommands(h, enabled, append(args, "--csv")...)
	if err != nil || !strings.Contains(csv, "ASSET House,EUR,,,200000.00,0.00,200000.00") {
		t.Fatalf("csv = %q err=%v, want ASSET row", csv, err)
	}
	table, _, err := runWithAssetCommands(h, enabled, args...)
	if err != nil || !strings.Contains(table, "ASSET House") {
		t.Fatalf("table = %q err=%v, want ASSET row", table, err)
	}

	off, _, err := runWithAssetCommands(h, module.Default(), append(args, "--json")...)
	if err != nil {
		t.Fatalf("off: %v", err)
	}
	if strings.Contains(off, "asset_lines") {
		t.Fatalf("stdout = %q, want no asset_lines with assets off", off)
	}
}
