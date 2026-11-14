package simulating

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/clock"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
	"github.com/0xballo/fui/internal/testutil/memstore"
)

func TestRunRequiresSimulationsModule(t *testing.T) {
	e := newEnv(t)
	_, err := e.svc.Run(context.Background(), module.Set{}, RunRequest{Kind: KindInvestment, Investment: &InvestmentParams{
		InitialMinor: 100_00, Years: 1, Compounding: "annual", Rates: Rates{},
	}})
	if !errors.Is(err, app.ErrModuleDisabled) {
		t.Fatalf("err = %v, want app.ErrModuleDisabled", err)
	}
}

func TestRunWithoutSavingWritesNoTransactions(t *testing.T) {
	e := newEnv(t)
	mustAccount(t, e.store, domain.Account{
		Name: "Broker", Kind: domain.AccountInvestment, Currency: eur(t), OpeningBalance: money(t, 100_00),
		OpenedOn: date(t, 2025, 1, 1),
	})
	before := e.store.Snapshot()
	_, err := e.svc.Run(context.Background(), e.enabled, RunRequest{Kind: KindInvestment, Investment: &InvestmentParams{
		InitialCurrentPositions: true, Years: 1, Compounding: "annual", Rates: Rates{},
	}})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	after := e.store.Snapshot()
	if before != after {
		t.Fatalf("snapshot changed on run without save")
	}
	if e.spy.transactionWrites != 0 {
		t.Fatalf("transaction writes = %d, want 0", e.spy.transactionWrites)
	}
}

func TestRunInvestmentUsesCurrentPositionsValue(t *testing.T) {
	e := newEnv(t)
	acc := mustAccount(t, e.store, domain.Account{
		Name: "Broker", Kind: domain.AccountInvestment, Currency: eur(t), OpeningBalance: money(t, 100_00),
		OpenedOn: date(t, 2025, 1, 1),
	})
	sec := mustSecurity(t, e.store, domain.Security{
		Name: "Global ETF", Ticker: "VWCE", Kind: domain.SecurityETF, Currency: eur(t), QuantityScale: 0,
	})
	mustSecurityOp(t, e.store, domain.SecurityOp{
		Date: date(t, 2026, 1, 10), Account: acc.ID, Security: sec.ID, Kind: domain.OpBuy,
		Quantity: quantity(t, 2, 0), UnitPrice: money(t, 40_00), Fees: money(t, 0),
	})
	mustSecurityPrice(t, e.store, domain.SecurityPrice{
		Security: sec.ID, Date: date(t, 2026, 1, 12), Price: money(t, 75_00),
	})
	got, err := e.svc.Run(context.Background(), e.enabled, RunRequest{Kind: KindInvestment, Investment: &InvestmentParams{
		InitialCurrentPositions: true, Years: 1, Compounding: "annual", Rates: Rates{},
	}})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got.Investment == nil {
		t.Fatalf("investment result missing")
	}
	want := int64(150_00)
	if got.Investment.Base.Nominal != want || got.Investment.Pessimistic.Nominal != want || got.Investment.Optimistic.Nominal != want {
		t.Fatalf("nominals = %+v, want %d for all rates", got.Investment, want)
	}
}

func TestRunInvestmentCurrentPositionsConvertsFX(t *testing.T) {
	e := newEnv(t)
	acc := mustAccount(t, e.store, domain.Account{
		Name: "USD broker", Kind: domain.AccountInvestment, Currency: usd(t), OpeningBalance: moneyIn(t, 100_00, "USD"),
		OpenedOn: date(t, 2025, 1, 1),
	})
	sec := mustSecurity(t, e.store, domain.Security{
		Name: "US stock", Ticker: "USS", Kind: domain.SecurityStock, Currency: usd(t), QuantityScale: 0,
	})
	mustSecurityOp(t, e.store, domain.SecurityOp{
		Date: date(t, 2026, 1, 10), Account: acc.ID, Security: sec.ID, Kind: domain.OpBuy,
		Quantity: quantity(t, 5, 0), UnitPrice: moneyIn(t, 20_00, "USD"), Fees: moneyIn(t, 0, "USD"),
	})
	mustSecurityPrice(t, e.store, domain.SecurityPrice{
		Security: sec.ID, Date: date(t, 2026, 1, 12), Price: moneyIn(t, 30_00, "USD"),
	})
	mustFX(t, e.store, domain.FXRate{
		Currency: usd(t),
		Date:     date(t, 2026, 1, 15),
		Rate:     rate(t, 900_000_000),
	})
	got, err := e.svc.Run(context.Background(), e.enabled, RunRequest{Kind: KindInvestment, Investment: &InvestmentParams{
		InitialCurrentPositions: true, Years: 1, Compounding: "annual", Rates: Rates{},
	}})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got.Investment.Base.Nominal != 135_00 {
		t.Fatalf("nominal = %d, want 13500", got.Investment.Base.Nominal)
	}
}

func TestRunInvestmentCurrentPositionsMissingRate(t *testing.T) {
	e := newEnv(t)
	acc := mustAccount(t, e.store, domain.Account{
		Name: "USD broker", Kind: domain.AccountInvestment, Currency: usd(t), OpeningBalance: moneyIn(t, 100_00, "USD"),
		OpenedOn: date(t, 2025, 1, 1),
	})
	sec := mustSecurity(t, e.store, domain.Security{
		Name: "US stock", Ticker: "USS", Kind: domain.SecurityStock, Currency: usd(t), QuantityScale: 0,
	})
	mustSecurityOp(t, e.store, domain.SecurityOp{
		Date: date(t, 2026, 1, 10), Account: acc.ID, Security: sec.ID, Kind: domain.OpBuy,
		Quantity: quantity(t, 1, 0), UnitPrice: moneyIn(t, 10_00, "USD"), Fees: moneyIn(t, 0, "USD"),
	})
	mustSecurityPrice(t, e.store, domain.SecurityPrice{
		Security: sec.ID, Date: date(t, 2026, 1, 12), Price: moneyIn(t, 12_00, "USD"),
	})
	_, err := e.svc.Run(context.Background(), e.enabled, RunRequest{Kind: KindInvestment, Investment: &InvestmentParams{
		InitialCurrentPositions: true, Years: 1, Compounding: "annual", Rates: Rates{},
	}})
	var de *domain.Error
	if !errors.As(err, &de) || de.Code != app.CodeRateMissing {
		t.Fatalf("err = %v, want app.CodeRateMissing", err)
	}
}

func TestRunInvestmentCurrentPositionsUsesCostBasisWithoutPrice(t *testing.T) {
	e := newEnv(t)
	acc := mustAccount(t, e.store, domain.Account{
		Name: "Broker", Kind: domain.AccountInvestment, Currency: eur(t), OpeningBalance: money(t, 0),
		OpenedOn: date(t, 2025, 1, 1),
	})
	sec := mustSecurity(t, e.store, domain.Security{
		Name: "Global ETF", Ticker: "VWCE", Kind: domain.SecurityETF, Currency: eur(t), QuantityScale: 0,
	})
	mustSecurityOp(t, e.store, domain.SecurityOp{
		Date: date(t, 2026, 1, 10), Account: acc.ID, Security: sec.ID, Kind: domain.OpBuy,
		Quantity: quantity(t, 2, 0), UnitPrice: money(t, 40_00), Fees: money(t, 0),
	})
	got, err := e.svc.Run(context.Background(), e.enabled, RunRequest{Kind: KindInvestment, Investment: &InvestmentParams{
		InitialCurrentPositions: true, Years: 1, Compounding: "annual", Rates: Rates{},
	}})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got.Investment.Base.Nominal != 80_00 {
		t.Fatalf("nominal = %d, want 8000", got.Investment.Base.Nominal)
	}
}

func TestRunInvestmentRejectsUnknownCompounding(t *testing.T) {
	e := newEnv(t)
	_, err := e.svc.Run(context.Background(), e.enabled, RunRequest{Kind: KindInvestment, Investment: &InvestmentParams{
		InitialMinor: 100_00, Years: 1, Compounding: "weekly", Rates: Rates{},
	}})
	var de *domain.Error
	if !errors.As(err, &de) || de.Code != domain.CodeInvalidValue {
		t.Fatalf("err = %v, want domain.CodeInvalidValue", err)
	}
}

func TestRunInvestmentCurrentPositionsSkipsFutureOpenedAccount(t *testing.T) {
	e := newEnv(t)
	mustAccount(t, e.store, domain.Account{
		Name: "Future broker", Kind: domain.AccountInvestment, Currency: eur(t), OpeningBalance: money(t, 50_00),
		OpenedOn: date(t, 2027, 1, 1),
	})
	got, err := e.svc.Run(context.Background(), e.enabled, RunRequest{Kind: KindInvestment, Investment: &InvestmentParams{
		InitialCurrentPositions: true, Years: 1, Compounding: "annual", Rates: Rates{},
	}})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got.Investment.Base.Nominal != 0 {
		t.Fatalf("nominal = %d, want 0", got.Investment.Base.Nominal)
	}
}

func TestRunPACRejectsUnknownFrequency(t *testing.T) {
	e := newEnv(t)
	_, err := e.svc.Run(context.Background(), e.enabled, RunRequest{Kind: KindPAC, PAC: &PACParams{
		ContributionMinor: 10_00,
		Frequency:         "weekly",
		Years:             1,
		Compounding:       "monthly",
		Rates:             Rates{},
	}})
	var de *domain.Error
	if !errors.As(err, &de) || de.Code != domain.CodeInvalidValue {
		t.Fatalf("err = %v, want domain.CodeInvalidValue", err)
	}
}

func TestRunMissingParamsAndUnknownKind(t *testing.T) {
	e := newEnv(t)
	if _, err := e.svc.Run(context.Background(), e.enabled, RunRequest{Kind: KindInvestment}); !errors.Is(err, domain.ErrMissingField) {
		t.Fatalf("missing params err = %v, want domain.ErrMissingField", err)
	}
	_, err := e.svc.Run(context.Background(), e.enabled, RunRequest{Kind: "nope"})
	var de *domain.Error
	if !errors.As(err, &de) || de.Code != domain.CodeInvalidValue {
		t.Fatalf("unknown kind err = %v, want domain.CodeInvalidValue", err)
	}
}

func TestRunCashFlowStartsFromForecast(t *testing.T) {
	e := newEnv(t)
	acc := mustAccount(t, e.store, domain.Account{
		Name: "Main", Kind: domain.AccountChecking, Currency: eur(t), OpeningBalance: money(t, 100_00),
		OpenedOn: date(t, 2025, 1, 1),
	})
	mustTx(t, e.store, domain.Transaction{
		AccountID: acc.ID, Date: date(t, 2026, 1, 20), Amount: money(t, -10_00),
		Status: domain.StatusBooked, Kind: domain.KindPayment,
	})
	got, err := e.svc.Run(context.Background(), e.enabled, RunRequest{
		Kind:     KindCashFlow,
		CashFlow: &CashFlowParams{Months: 1},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got.CashFlow == nil {
		t.Fatalf("cashflow result missing")
	}
	if got.CashFlow.OpeningMinor != 100_00 {
		t.Fatalf("opening = %d, want 10000", got.CashFlow.OpeningMinor)
	}
	if len(got.CashFlow.Months) != 1 {
		t.Fatalf("months = %d, want 1", len(got.CashFlow.Months))
	}
	row := got.CashFlow.Months[0]
	if row.BaseMinor != 90_00 || row.ScenarioMinor != 90_00 {
		t.Fatalf("month row = %+v, want base=scenario=9000", row)
	}
}

func TestRunCashFlowParsesAllEventKinds(t *testing.T) {
	e := newEnv(t)
	mustAccount(t, e.store, domain.Account{
		Name: "Main", Kind: domain.AccountChecking, Currency: eur(t), OpeningBalance: money(t, 0),
		OpenedOn: date(t, 2025, 1, 1),
	})
	got, err := e.svc.Run(context.Background(), e.enabled, RunRequest{
		Kind: KindCashFlow,
		CashFlow: &CashFlowParams{
			Months: 2,
			Events: []CashFlowEvent{
				{Kind: "one_off", Date: "2026-01-20", AmountMinor: -10_00},
				{Kind: "recurring_change", Date: "2026-01-25", AmountMinor: 5_00},
				{Kind: "stop_income", Date: "2026-01-26"},
				{Kind: "new_loan", Date: "2026-01-27", AmountMinor: 100_00, RateBP: 300, Years: 1},
			},
		},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got.CashFlow == nil || len(got.CashFlow.Months) != 2 {
		t.Fatalf("cashflow result = %+v", got.CashFlow)
	}
}

func TestRunCashFlowRejectsInvalidMonthsAndEventKind(t *testing.T) {
	e := newEnv(t)
	if _, err := e.svc.Run(context.Background(), e.enabled, RunRequest{
		Kind:     KindCashFlow,
		CashFlow: &CashFlowParams{Months: 0},
	}); err == nil {
		t.Fatal("expected invalid months error")
	}
	_, err := e.svc.Run(context.Background(), e.enabled, RunRequest{
		Kind: KindCashFlow,
		CashFlow: &CashFlowParams{
			Months: 1,
			Events: []CashFlowEvent{{Kind: "unknown", Date: "2026-01-20"}},
		},
	})
	var de *domain.Error
	if !errors.As(err, &de) || de.Code != domain.CodeInvalidValue {
		t.Fatalf("err = %v, want domain.CodeInvalidValue", err)
	}
}

func TestSaveListRerunRemoveAndParamsRoundTrip(t *testing.T) {
	e := newEnv(t)
	req := RunRequest{
		Kind: KindPAC,
		PAC: &PACParams{
			InitialMinor:            50_00,
			ContributionMinor:       20_00,
			Frequency:               "monthly",
			ContributionGrowthBP:    -50,
			Years:                   5,
			Compounding:             "monthly",
			Rates:                   Rates{PessimisticBP: 100, BaseBP: 300, OptimisticBP: 500},
			CostBP:                  10,
			TaxBP:                   26_00,
			InflationBP:             200,
			InitialCurrentPositions: false,
		},
	}
	plan, err := e.svc.PlanSave(context.Background(), e.enabled, "Plan A", req)
	if err != nil {
		t.Fatalf("PlanSave: %v", err)
	}
	raw, _ := json.Marshal(req.PAC)
	if plan.Simulation.ParamsJSON != string(raw) {
		t.Fatalf("params_json = %s, want %s", plan.Simulation.ParamsJSON, string(raw))
	}
	if _, err := e.svc.ApplySave(context.Background(), plan); err != nil {
		t.Fatalf("ApplySave: %v", err)
	}
	list, err := e.svc.List(context.Background(), e.enabled)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 || list[0].Name != "Plan A" {
		t.Fatalf("list = %+v, want one Plan A", list)
	}
	rerun, err := e.svc.Rerun(context.Background(), e.enabled, "plan")
	if err != nil {
		t.Fatalf("Rerun: %v", err)
	}
	if rerun.RunResult.Kind != KindPAC || rerun.RunResult.PAC == nil {
		t.Fatalf("rerun result = %+v", rerun.RunResult)
	}
	rm, err := e.svc.PlanRemove(context.Background(), e.enabled, "plan")
	if err != nil {
		t.Fatalf("PlanRemove: %v", err)
	}
	if err := e.svc.ApplyRemove(context.Background(), rm); err != nil {
		t.Fatalf("ApplyRemove: %v", err)
	}
	list, err = e.svc.List(context.Background(), e.enabled)
	if err != nil {
		t.Fatalf("List after remove: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("list after remove = %+v, want empty", list)
	}
}

func TestPlanSaveRejectsDuplicateName(t *testing.T) {
	e := newEnv(t)
	req := RunRequest{Kind: KindInvestment, Investment: &InvestmentParams{
		InitialMinor: 100_00, Years: 1, Compounding: "annual", Rates: Rates{},
	}}
	p, err := e.svc.PlanSave(context.Background(), e.enabled, "Plan A", req)
	if err != nil {
		t.Fatalf("PlanSave: %v", err)
	}
	if _, err := e.svc.ApplySave(context.Background(), p); err != nil {
		t.Fatalf("ApplySave: %v", err)
	}
	_, err = e.svc.PlanSave(context.Background(), e.enabled, " plan a ", req)
	var de *domain.Error
	if !errors.As(err, &de) || de.Code != app.CodeAlreadyExists {
		t.Fatalf("err = %v, want app.CodeAlreadyExists", err)
	}
}

func TestRerunFailsOnInvalidSavedParamsJSON(t *testing.T) {
	e := newEnv(t)
	err := e.store.WithTx(context.Background(), func(r app.Repos) error {
		_, err := r.SavedSimulations().Insert(context.Background(), app.SavedSimulation{
			Name:       "Bad",
			Kind:       KindInvestment,
			ParamsJSON: "{",
		})
		return err
	})
	if err != nil {
		t.Fatalf("seed saved simulation: %v", err)
	}
	if _, err := e.svc.Rerun(context.Background(), e.enabled, "bad"); err == nil {
		t.Fatal("expected rerun decode error")
	}
}

func TestRerunFailsOnUnknownSavedKind(t *testing.T) {
	e := newEnv(t)
	err := e.store.WithTx(context.Background(), func(r app.Repos) error {
		_, err := r.SavedSimulations().Insert(context.Background(), app.SavedSimulation{
			Name:       "BadKind",
			Kind:       "weird",
			ParamsJSON: "{}",
		})
		return err
	})
	if err != nil {
		t.Fatalf("seed saved simulation: %v", err)
	}
	_, err = e.svc.Rerun(context.Background(), e.enabled, "badkind")
	var de *domain.Error
	if !errors.As(err, &de) || de.Code != domain.CodeInvalidValue {
		t.Fatalf("err = %v, want domain.CodeInvalidValue", err)
	}
}

func TestCompareRejectsInvalidCounts(t *testing.T) {
	e := newEnv(t)
	for _, refs := range [][]string{{"one"}, {"a", "b", "c", "d"}} {
		_, err := e.svc.Compare(context.Background(), e.enabled, refs)
		var de *domain.Error
		if !errors.As(err, &de) || de.Code != domain.CodeInvalidValue {
			t.Fatalf("refs=%v err=%v, want domain.CodeInvalidValue", refs, err)
		}
	}
}

func TestCompareRerunsTwoSavedScenarios(t *testing.T) {
	e := newEnv(t)
	mustAccount(t, e.store, domain.Account{
		Name: "Main", Kind: domain.AccountChecking, Currency: eur(t), OpeningBalance: money(t, 100_00),
		OpenedOn: date(t, 2025, 1, 1),
	})
	save := func(name string, req RunRequest) {
		t.Helper()
		p, err := e.svc.PlanSave(context.Background(), e.enabled, name, req)
		if err != nil {
			t.Fatalf("PlanSave(%s): %v", name, err)
		}
		if _, err := e.svc.ApplySave(context.Background(), p); err != nil {
			t.Fatalf("ApplySave(%s): %v", name, err)
		}
	}
	save("Inv", RunRequest{Kind: KindInvestment, Investment: &InvestmentParams{
		InitialMinor: 100_00, Years: 1, Compounding: "annual", Rates: Rates{},
	}})
	save("Flow", RunRequest{Kind: KindCashFlow, CashFlow: &CashFlowParams{Months: 1}})
	got, err := e.svc.Compare(context.Background(), e.enabled, []string{"inv", "flow"})
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	if len(got.Items) != 2 {
		t.Fatalf("items = %d, want 2", len(got.Items))
	}
	if got.Items[0].Simulation.Name != "Inv" || got.Items[1].Simulation.Name != "Flow" {
		t.Fatalf("names = [%s, %s], want [Inv, Flow]", got.Items[0].Simulation.Name, got.Items[1].Simulation.Name)
	}
	if e.spy.transactionWrites != 0 {
		t.Fatalf("transaction writes = %d, want 0", e.spy.transactionWrites)
	}
}

type env struct {
	store   *memstore.Store
	spy     *spyStore
	svc     *Service
	enabled module.Set
}

func newEnv(t *testing.T) env {
	t.Helper()
	store := memstore.New()
	spy := &spyStore{Store: store}
	fixed := clock.NewFixed(date(t, 2026, 1, 15).In(time.UTC))
	enabled, err := module.Validate([]module.ID{module.Core, module.Recurring, module.Investments, module.Simulations})
	if err != nil {
		t.Fatal(err)
	}
	return env{
		store:   store,
		spy:     spy,
		svc:     New(spy, fixed, Config{BaseCurrency: eur(t), Location: time.UTC}),
		enabled: enabled,
	}
}

type spyStore struct {
	app.Store
	transactionWrites int
}

func (s *spyStore) Read(ctx context.Context, fn func(app.Repos) error) error {
	return s.Store.Read(ctx, func(r app.Repos) error { return fn(spyRepos{Repos: r, writes: &s.transactionWrites}) })
}

func (s *spyStore) WithTx(ctx context.Context, fn func(app.Repos) error) error {
	return s.Store.WithTx(ctx, func(r app.Repos) error { return fn(spyRepos{Repos: r, writes: &s.transactionWrites}) })
}

type spyRepos struct {
	app.Repos
	writes *int
}

func (r spyRepos) Transactions() app.TransactionRepo {
	return spyTransactions{TransactionRepo: r.Repos.Transactions(), writes: r.writes}
}

type spyTransactions struct {
	app.TransactionRepo
	writes *int
}

func (t spyTransactions) Insert(ctx context.Context, tx domain.Transaction) (domain.Transaction, error) {
	(*t.writes)++
	return t.TransactionRepo.Insert(ctx, tx)
}

func (t spyTransactions) Update(ctx context.Context, tx domain.Transaction) error {
	(*t.writes)++
	return t.TransactionRepo.Update(ctx, tx)
}

func (t spyTransactions) Delete(ctx context.Context, id domain.TransactionID) error {
	(*t.writes)++
	return t.TransactionRepo.Delete(ctx, id)
}

func mustAccount(t *testing.T, store *memstore.Store, acct domain.Account) domain.Account {
	t.Helper()
	var out domain.Account
	err := store.WithTx(context.Background(), func(r app.Repos) (err error) {
		out, err = r.Accounts().Insert(context.Background(), acct)
		return err
	})
	if err != nil {
		t.Fatalf("insert account: %v", err)
	}
	return out
}

func mustTx(t *testing.T, store *memstore.Store, tx domain.Transaction) domain.Transaction {
	t.Helper()
	var out domain.Transaction
	err := store.WithTx(context.Background(), func(r app.Repos) (err error) {
		out, err = r.Transactions().Insert(context.Background(), tx)
		return err
	})
	if err != nil {
		t.Fatalf("insert tx: %v", err)
	}
	return out
}

func mustSecurity(t *testing.T, store *memstore.Store, sec domain.Security) domain.Security {
	t.Helper()
	in, err := domain.NewSecurity(sec)
	if err != nil {
		t.Fatalf("new security: %v", err)
	}
	var out domain.Security
	err = store.WithTx(context.Background(), func(r app.Repos) (err error) {
		out, err = r.Securities().Insert(context.Background(), in)
		return err
	})
	if err != nil {
		t.Fatalf("insert security: %v", err)
	}
	return out
}

func mustSecurityOp(t *testing.T, store *memstore.Store, op domain.SecurityOp) domain.SecurityOp {
	t.Helper()
	in, err := domain.NewSecurityOp(op)
	if err != nil {
		t.Fatalf("new security operation: %v", err)
	}
	var out domain.SecurityOp
	err = store.WithTx(context.Background(), func(r app.Repos) (err error) {
		out, err = r.Securities().InsertOp(context.Background(), in)
		return err
	})
	if err != nil {
		t.Fatalf("insert security operation: %v", err)
	}
	return out
}

func mustSecurityPrice(t *testing.T, store *memstore.Store, sp domain.SecurityPrice) {
	t.Helper()
	if err := store.WithTx(context.Background(), func(r app.Repos) error {
		return r.Securities().UpsertPrice(context.Background(), sp)
	}); err != nil {
		t.Fatalf("insert security price: %v", err)
	}
}

func mustFX(t *testing.T, store *memstore.Store, fx domain.FXRate) {
	t.Helper()
	if err := store.WithTx(context.Background(), func(r app.Repos) error {
		return r.FXRates().Upsert(context.Background(), fx)
	}); err != nil {
		t.Fatalf("insert fx: %v", err)
	}
}

func eur(t *testing.T) domain.Currency {
	t.Helper()
	c, err := domain.ParseCurrency("EUR")
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func usd(t *testing.T) domain.Currency {
	t.Helper()
	c, err := domain.ParseCurrency("USD")
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func money(t *testing.T, minor int64) domain.Money {
	t.Helper()
	m, err := domain.NewMoney(minor, eur(t))
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func moneyIn(t *testing.T, minor int64, code string) domain.Money {
	t.Helper()
	c, err := domain.ParseCurrency(code)
	if err != nil {
		t.Fatal(err)
	}
	m, err := domain.NewMoney(minor, c)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func date(t *testing.T, y, m, d int) domain.Date {
	t.Helper()
	day, err := domain.NewDate(y, m, d)
	if err != nil {
		t.Fatal(err)
	}
	return day
}

func rate(t *testing.T, scaled int64) domain.Rate {
	t.Helper()
	r, err := domain.NewRate(scaled)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func quantity(t *testing.T, scaled int64, scale int) domain.Quantity {
	t.Helper()
	q, err := domain.NewQuantity(scaled, scale)
	if err != nil {
		t.Fatal(err)
	}
	return q
}
