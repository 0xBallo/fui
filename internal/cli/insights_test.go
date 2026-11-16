package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/recurring"
	"github.com/0xballo/fui/internal/app/reporting"
	"github.com/0xballo/fui/internal/clock"
	"github.com/0xballo/fui/internal/config"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
	"github.com/0xballo/fui/internal/testutil/memstore"
)

type noBackup struct{}

func (noBackup) Backup(context.Context, string) (string, error) { return "", nil }
func (noBackup) Restore(context.Context, string) error          { return nil }

func insDate(t *testing.T, s string) domain.Date {
	t.Helper()
	d, err := domain.ParseDate(s)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func insMoney(t *testing.T, minor int64, c domain.Currency) domain.Money {
	t.Helper()
	m, err := domain.NewMoney(minor, c)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// insightEnv seeds a recurring Netflix charge plus one Grocer outlier, as the reporting tests do.
func insightEnv(t *testing.T) (*Deps, *bytes.Buffer) {
	t.Helper()
	ctx := context.Background()
	eur, err := domain.ParseCurrency("EUR")
	if err != nil {
		t.Fatal(err)
	}
	d := func(s string) domain.Date { return insDate(t, s) }
	store := memstore.New()
	now := d("2026-03-15").In(time.UTC)
	clk := clock.NewFixed(now)
	mods, err := module.Validate([]module.ID{module.Core, module.Recurring, module.Analytics})
	if err != nil {
		t.Fatal(err)
	}
	rc := reporting.Config{BaseCurrency: eur, Location: time.UTC, Modules: mods, Locale: domain.LocaleENUS}
	analytics := reporting.NewAnalytics(store, clk, nil, rc)
	err = store.WithTx(ctx, func(r app.Repos) error {
		acct, err := r.Accounts().Insert(ctx, domain.Account{
			Name: "Main", Kind: domain.AccountChecking, Currency: eur,
			OpeningBalance: insMoney(t, 100_000, eur), OpenedOn: d("2025-12-01"),
		})
		if err != nil {
			return err
		}
		nf, err := r.Payees().Insert(ctx, domain.Payee{Name: "Netflix"})
		if err != nil {
			return err
		}
		gr, err := r.Payees().Insert(ctx, domain.Payee{Name: "Grocer"})
		if err != nil {
			return err
		}
		cat, err := r.Categories().Insert(ctx, domain.Category{Name: "Media", Kind: domain.CategoryExpense})
		if err != nil {
			return err
		}
		add := func(date string, minor int64, payee domain.PayeeID, c domain.CategoryID) error {
			_, err := r.Transactions().Insert(ctx, domain.Transaction{
				AccountID: acct.ID, Date: d(date), Amount: insMoney(t, minor, eur),
				Status: domain.StatusBooked, Kind: domain.KindPayment, PayeeID: payee, CategoryID: c,
			})
			return err
		}
		for _, s := range []string{"2025-12-20", "2026-01-20", "2026-02-20"} {
			if err := add(s, -1_299, nf.ID, cat.ID); err != nil {
				return err
			}
		}
		for _, s := range []string{"2025-12-03", "2026-01-04", "2026-02-02"} {
			if err := add(s, -3_000, gr.ID, 0); err != nil {
				return err
			}
		}
		return add("2026-03-10", -9_000, gr.ID, 0)
	})
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	return &Deps{
		Reporting: Reporting{Analytics: analytics},
		Recurring: recurring.New(store, clk, noBackup{}, recurring.Config{Locale: domain.LocaleENUS, Location: time.UTC}),
		Config: &config.Config{
			Currency: "EUR", Locale: config.LocaleUS, MonthStartDay: 1,
			Modules: config.ModulesConfig{Enabled: mods.IDs()},
		},
		Stdout: &out, Stderr: &bytes.Buffer{},
	}, &out
}

func runInsightCmd(t *testing.T, deps *Deps, build func(*Deps) *cobra.Command, args ...string) error {
	t.Helper()
	root := &cobra.Command{Use: "fui", SilenceUsage: true, SilenceErrors: true}
	registerPersistentFlags(root)
	root.AddCommand(build(deps))
	root.SetArgs(args)
	return root.ExecuteContext(context.Background())
}

func TestInsightsRunDismissAccept(t *testing.T) {
	deps, out := insightEnv(t)
	if err := runInsightCmd(t, deps, NewInsightsCmd, "insights", "--json"); err != nil {
		t.Fatal(err)
	}
	var views []reporting.InsightView
	if err := json.Unmarshal(out.Bytes(), &views); err != nil {
		t.Fatalf("json: %v\n%s", err, out)
	}
	keys := map[string]string{}
	for _, v := range views {
		keys[v.Kind] = v.Key
	}
	if keys["anomaly"] == "" || keys["subscription"] == "" {
		t.Fatalf("views = %+v", views)
	}

	out.Reset()
	if err := runInsightCmd(t, deps, NewInsightsCmd, "insights"); err != nil || !strings.Contains(out.String(), "Netflix") {
		t.Fatalf("table = %q, %v", out, err)
	}

	out.Reset()
	if err := runInsightCmd(t, deps, NewInsightsCmd, "insights", "--dismiss", keys["anomaly"], "--dry-run", "--json"); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := runInsightCmd(t, deps, NewInsightsCmd, "insights", "--dismiss", keys["anomaly"], "--json"); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := runInsightCmd(t, deps, NewInsightsCmd, "insights", "--accept", keys["subscription"], "--json"); err != nil {
		t.Fatal(err)
	}
	var res map[string]any
	if err := json.Unmarshal(out.Bytes(), &res); err != nil || res["created"] != true {
		t.Fatalf("accept = %s, %v", out, err)
	}
	out.Reset()
	if err := runInsightCmd(t, deps, NewInsightsCmd, "insights", "--json"); err != nil || strings.TrimSpace(out.String()) != "[]" {
		t.Fatalf("after = %q, %v", out, err)
	}

	err := runInsightCmd(t, deps, NewInsightsCmd, "insights", "--dismiss", "a", "--accept", "b")
	if err == nil {
		t.Fatal("dismiss+accept must fail")
	}
	if err := runInsightCmd(t, deps, NewInsightsCmd, "insights", "--dismiss", "nope"); err == nil {
		t.Fatal("unknown key must fail")
	}
}

func TestInsightConfigFrom(t *testing.T) {
	if got := InsightConfigFrom(&config.Config{}); got != domain.DefaultInsightConfig() {
		t.Fatalf("zero config = %+v", got)
	}
	if got := InsightConfigFrom(nil); got != domain.DefaultInsightConfig() {
		t.Fatalf("nil config = %+v", got)
	}
	cfg := config.Defaults()
	if got := InsightConfigFrom(&cfg); got != domain.DefaultInsightConfig() {
		t.Fatalf("defaults = %+v", got)
	}
	cfg.Insights = config.InsightsConfig{AnomalyFactorPermille: 4000, SubscriptionTolPermil: 10, SubscriptionMinCount: 5, TrendPermille: 500, TrendMinMinor: 9000}
	got := InsightConfigFrom(&cfg)
	want := domain.DefaultInsightConfig()
	want.AnomalyFactorPermille, want.SubscriptionTolPermil, want.SubscriptionMinCount, want.TrendPermille, want.TrendMinMinor = 4000, 10, 5, 500, 9000
	if got != want {
		t.Fatalf("custom = %+v, want %+v", got, want)
	}
}

func TestReportCompare(t *testing.T) {
	deps, out := insightEnv(t)
	if err := runInsightCmd(t, deps, NewReportCompareCmd, "compare", "--a", "last-month", "--b", "this-month", "--by", "category", "--json"); err != nil {
		t.Fatal(err)
	}
	var rep reporting.CompareReport
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil || len(rep.Rows) == 0 {
		t.Fatalf("compare = %s, %v", out, err)
	}
	out.Reset()
	if err := runInsightCmd(t, deps, NewReportCompareCmd, "compare"); err != nil || !strings.Contains(out.String(), "TOTAL") {
		t.Fatalf("table = %q, %v", out, err)
	}
	out.Reset()
	if err := runInsightCmd(t, deps, NewReportCompareCmd, "compare", "--csv", "--by", "category"); err != nil || !strings.HasPrefix(out.String(), "CATEGORY,") {
		t.Fatalf("csv = %q, %v", out, err)
	}
	if err := runInsightCmd(t, deps, NewReportCompareCmd, "compare", "--by", "payee"); err == nil {
		t.Fatal("bad --by must fail")
	}
	if err := runInsightCmd(t, deps, NewReportCompareCmd, "compare", "--a", "bogus"); err == nil {
		t.Fatal("bad period must fail")
	}
}

func TestPermilleText(t *testing.T) {
	neg, pos := int64(-125), int64(1005)
	for want, in := range map[string]*int64{"-": nil, "-12.5%": &neg, "100.5%": &pos} {
		if got := permilleText(in); got != want {
			t.Errorf("permilleText = %q, want %q", got, want)
		}
	}
}
