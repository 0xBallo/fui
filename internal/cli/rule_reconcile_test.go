package cli

import (
	"bytes"
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/rules"
	"github.com/0xballo/fui/internal/config"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
	"github.com/0xballo/fui/internal/testutil/memstore"
)

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

type noopBackup struct{}

func (noopBackup) Backup(context.Context, string) (string, error) { return "b", nil }
func (noopBackup) Restore(context.Context, string) error          { return nil }

type standalone struct {
	store *memstore.Store
	app   *app.Service
	deps  *Deps
	in    *bytes.Buffer
	out   *bytes.Buffer
	err   *bytes.Buffer
}

func newStandalone(t *testing.T, loc domain.Locale, cfgLocale string) *standalone {
	t.Helper()
	store := memstore.New()
	clk := fixedClock{now: time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC)}
	cur, err := domain.ParseCurrency("EUR")
	if err != nil {
		t.Fatalf("ParseCurrency: %v", err)
	}
	backup := noopBackup{}
	svc := app.New(store, clk, backup, app.Config{Locale: loc, Location: time.UTC, BaseCurrency: cur})
	in, out, errw := &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{}
	deps := &Deps{
		Service: svc,
		Rules:   rules.New(store, backup),
		Config: &config.Config{
			Currency: "EUR",
			Locale:   cfgLocale,
			Modules:  config.ModulesConfig{Enabled: module.Default().IDs()},
		},
		Stdin: in, Stdout: out, Stderr: errw, IsTerminal: false, Version: "test",
	}
	return &standalone{store: store, app: svc, deps: deps, in: in, out: out, err: errw}
}

func (h *standalone) run(cmd *cobra.Command, args ...string) error {
	h.out.Reset()
	h.err.Reset()
	root := &cobra.Command{Use: "fui", SilenceErrors: true, SilenceUsage: true}
	registerPersistentFlags(root)
	root.AddCommand(cmd)
	root.SetIn(h.in)
	root.SetOut(h.out)
	root.SetErr(h.err)
	root.SetArgs(args)
	return root.ExecuteContext(context.Background())
}

func (h *standalone) seedAccount(t *testing.T, name string) {
	t.Helper()
	ctx := context.Background()
	p, err := h.app.PlanAddAccount(ctx, app.AddAccountRequest{
		Name: name, Kind: "checking", Currency: "EUR", OpeningBalance: "0", OpenedOn: "2026-01-01",
	})
	if err != nil {
		t.Fatalf("PlanAddAccount: %v", err)
	}
	if _, err := h.app.ApplyAddAccount(ctx, p); err != nil {
		t.Fatalf("ApplyAddAccount: %v", err)
	}
}

func (h *standalone) seedCategory(t *testing.T, name string) {
	t.Helper()
	ctx := context.Background()
	p, err := h.app.PlanAddCategory(ctx, app.AddCategoryRequest{Name: name, Kind: "expense"})
	if err != nil {
		t.Fatalf("PlanAddCategory: %v", err)
	}
	if _, err := h.app.ApplyAddCategory(ctx, p); err != nil {
		t.Fatalf("ApplyAddCategory: %v", err)
	}
}

func (h *standalone) seedPayee(t *testing.T, name string) {
	t.Helper()
	ctx := context.Background()
	p, err := h.app.PlanAddPayee(ctx, app.AddPayeeRequest{Name: name})
	if err != nil {
		t.Fatalf("PlanAddPayee: %v", err)
	}
	if _, err := h.app.ApplyAddPayee(ctx, p); err != nil {
		t.Fatalf("ApplyAddPayee: %v", err)
	}
}

func (h *standalone) seedTag(t *testing.T, name string) {
	t.Helper()
	ctx := context.Background()
	p, err := h.app.PlanAddTag(ctx, app.AddTagRequest{Name: name})
	if err != nil {
		t.Fatalf("PlanAddTag: %v", err)
	}
	if _, err := h.app.ApplyAddTag(ctx, p); err != nil {
		t.Fatalf("ApplyAddTag: %v", err)
	}
}

func (h *standalone) seedTx(t *testing.T, account, amount, desc, date, status string) domain.Transaction {
	t.Helper()
	ctx := context.Background()
	p, err := h.app.PlanAddTx(ctx, app.AddTxRequest{
		Account: account, Amount: amount, Description: desc, Date: date, Status: status,
	})
	if err != nil {
		t.Fatalf("PlanAddTx: %v", err)
	}
	tx, err := h.app.ApplyAddTx(ctx, p)
	if err != nil {
		t.Fatalf("ApplyAddTx: %v", err)
	}
	return tx
}

func TestRuleCommands(t *testing.T) {
	h := newStandalone(t, domain.LocaleITIT, config.LocaleIT)
	h.seedAccount(t, "Checking")
	h.seedCategory(t, "Food")
	h.seedCategory(t, "Other")
	h.seedPayee(t, "Shop")
	h.seedTag(t, "weekly")

	if err := h.run(newRuleCmd(h.deps),
		"rule", "add", "groceries",
		"--contains", "super", "--min", "10,50", "--sign", "outflow",
		"--account", "Checking", "--category", "Food", "--payee", "Shop", "--tag", "weekly",
		"--priority", "10", "--json",
	); err != nil {
		t.Fatalf("rule add: %v (stderr=%q)", err, h.err.String())
	}
	if !strings.Contains(h.out.String(), `"min_abs_minor":1050`) || !strings.Contains(h.out.String(), `"name":"groceries"`) {
		t.Fatalf("rule add output = %q", h.out.String())
	}

	if err := h.run(newRuleCmd(h.deps), "rule", "edit", "groceries", "--set-priority", "--priority", "5", "--set-actions", "--category", "Other", "--json"); err != nil {
		t.Fatalf("rule edit: %v (stderr=%q)", err, h.err.String())
	}
	if !strings.Contains(h.out.String(), `"priority":5`) || !strings.Contains(h.out.String(), `"category":"Other"`) {
		t.Fatalf("rule edit output = %q", h.out.String())
	}

	if err := h.run(newRuleCmd(h.deps), "rule", "list", "--json"); err != nil {
		t.Fatalf("rule list: %v (stderr=%q)", err, h.err.String())
	}
	if !strings.Contains(h.out.String(), `"name":"groceries"`) {
		t.Fatalf("rule list output = %q", h.out.String())
	}

	if err := h.run(newRuleCmd(h.deps), "rule", "rm", "groceries", "--json"); err != nil {
		t.Fatalf("rule rm: %v (stderr=%q)", err, h.err.String())
	}
	if !strings.Contains(h.out.String(), `"removed":"groceries"`) {
		t.Fatalf("rule rm output = %q", h.out.String())
	}
}

func TestRuleTestAndApply(t *testing.T) {
	h := newStandalone(t, domain.LocaleITIT, config.LocaleIT)
	h.seedAccount(t, "Checking")
	h.seedCategory(t, "Food")
	h.seedPayee(t, "Shop")
	h.seedTag(t, "weekly")
	tx := h.seedTx(t, "Checking", "-3,00", "Bar Roma", "2026-10-01", "booked")
	_ = h.seedTx(t, "Checking", "-1,00", "Cinema", "2026-10-02", "booked")

	if err := h.run(newRuleCmd(h.deps), "rule", "add", "bar", "--contains", "bar", "--category", "Food", "--payee", "Shop", "--tag", "weekly", "--json"); err != nil {
		t.Fatalf("rule add: %v", err)
	}
	if err := h.run(newRuleCmd(h.deps), "rule", "test", "Bar Roma", "--account", "Checking", "--amount", "-3,00", "--json"); err != nil {
		t.Fatalf("rule test: %v (stderr=%q)", err, h.err.String())
	}
	if !strings.Contains(h.out.String(), `"category_rule":"bar"`) || !strings.Contains(h.out.String(), `"payee_rule":"bar"`) {
		t.Fatalf("rule test output = %q", h.out.String())
	}
	if err := h.run(newRuleCmd(h.deps), "rule", "apply", "--month", "2026-10", "--min", "2,50", "--dry-run", "--json"); err != nil {
		t.Fatalf("rule apply dry-run: %v (stderr=%q)", err, h.err.String())
	}
	if !strings.Contains(h.out.String(), `"id":`+strconv.FormatInt(int64(tx.ID), 10)) {
		t.Fatalf("rule apply dry-run output = %q", h.out.String())
	}
	if err := h.run(newRuleCmd(h.deps), "rule", "apply", "--month", "2026-10", "--min", "2,50", "--yes", "--json"); err != nil {
		t.Fatalf("rule apply: %v (stderr=%q)", err, h.err.String())
	}
	got, err := h.app.ShowTx(context.Background(), tx.ID)
	if err != nil {
		t.Fatalf("ShowTx: %v", err)
	}
	if got.Transaction.CategoryID == 0 || got.Transaction.PayeeID == 0 {
		t.Fatalf("tx after apply = %+v", got.Transaction)
	}
}

func TestReconcileCommand(t *testing.T) {
	h := newStandalone(t, domain.LocaleENUS, config.LocaleUS)
	ctx := context.Background()
	p, err := h.app.PlanAddAccount(ctx, app.AddAccountRequest{
		Name: "A", Kind: "checking", Currency: "EUR", OpeningBalance: "100", OpenedOn: "2026-01-01",
	})
	if err != nil {
		t.Fatalf("PlanAddAccount: %v", err)
	}
	if _, err := h.app.ApplyAddAccount(ctx, p); err != nil {
		t.Fatalf("ApplyAddAccount: %v", err)
	}
	tx := h.seedTx(t, "A", "-10", "One", "2026-10-01", "booked")

	if err := h.run(newReconcileCmd(h.deps), "reconcile", "A", "--balance", "84", "--date", "2026-10-10", "--json"); err != nil {
		t.Fatalf("reconcile non-zero: %v (stderr=%q)", err, h.err.String())
	}
	if !strings.Contains(h.out.String(), `"difference":-600`) || !strings.Contains(h.out.String(), `"can_close":false`) {
		t.Fatalf("reconcile non-zero output = %q", h.out.String())
	}

	err = h.run(newReconcileCmd(h.deps), "reconcile", "A", "--balance", "90", "--date", "2026-10-10", "--json")
	var derr *domain.Error
	if !errors.As(err, &derr) || derr.Code != app.CodeConfirmationRequired {
		t.Fatalf("reconcile without --yes err = %v, want confirmation_required", err)
	}

	if err := h.run(newReconcileCmd(h.deps), "reconcile", "A", "--balance", "90", "--date", "2026-10-10", "--yes", "--json"); err != nil {
		t.Fatalf("reconcile with --yes: %v (stderr=%q)", err, h.err.String())
	}
	view, err := h.app.ShowTx(ctx, tx.ID)
	if err != nil {
		t.Fatalf("ShowTx: %v", err)
	}
	if view.Transaction.Status != domain.StatusReconciled {
		t.Fatalf("tx status after reconcile = %s, want reconciled", view.Transaction.Status)
	}
}
