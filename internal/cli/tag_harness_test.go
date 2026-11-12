package cli

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/config"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
	"github.com/0xballo/fui/internal/testutil/memstore"
)

// internalFixedClock is a deterministic app.Clock for the internal (white-box) command tests in
// tag_test.go, payee_test.go and tx_split_test.go, which build Deps directly over a memstore
// instead of going through the full command tree (these commands are not yet registered in
// root.go; see V32).
type internalFixedClock struct{ now time.Time }

func (c internalFixedClock) Now() time.Time { return c.now }

// internalFakeBackup is a no-op app.Backuper, sufficient for the bulk-plan tests (tag/payee
// merge and rm) that need one to apply.
type internalFakeBackup struct{}

func (internalFakeBackup) Backup(_ context.Context, _ string) (string, error) { return "", nil }
func (internalFakeBackup) Restore(_ context.Context, _ string) error          { return nil }

// newTestDeps builds Deps over a fresh, empty memstore.
func newTestDeps(t *testing.T) *Deps {
	t.Helper()
	store := memstore.New()
	clock := internalFixedClock{time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC)}
	eur, err := domain.ParseCurrency("EUR")
	if err != nil {
		t.Fatalf("ParseCurrency(EUR): %v", err)
	}
	svc := app.New(store, clock, internalFakeBackup{}, app.Config{
		Locale: domain.LocaleENUS, Location: time.UTC, BaseCurrency: eur,
	})
	cfg := &config.Config{
		Currency: "EUR", Locale: config.LocaleUS, MonthStartDay: 1,
		Modules: config.ModulesConfig{Enabled: module.Default().IDs()},
	}
	return &Deps{
		Service: svc, Config: cfg, Stdin: &bytes.Buffer{}, Stdout: io.Discard, Stderr: io.Discard,
		Version: "test",
	}
}

// runCmd executes cmd with args against a root carrying the shared persistent flags, capturing
// stdout/stderr through deps (every command under test writes there directly, never to
// cmd.OutOrStdout). It returns the error ExecuteContext produced, if any.
func runCmd(deps *Deps, cmd *cobra.Command, args []string) error {
	root := &cobra.Command{Use: "fui", SilenceUsage: true, SilenceErrors: true}
	registerPersistentFlags(root)
	root.AddCommand(cmd)
	root.SetArgs(append([]string{cmd.Name()}, args...))
	root.SetIn(deps.Stdin)
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	return root.ExecuteContext(context.Background())
}

// mustRunCmd runs cmd and fails the test unless it succeeds.
func mustRunCmd(t *testing.T, deps *Deps, cmd *cobra.Command, args []string) {
	t.Helper()
	if err := runCmd(deps, cmd, args); err != nil {
		t.Fatalf("run(%v): %v", args, err)
	}
}

// outBuf resets deps.Stdout to a fresh buffer and returns it, so each assertion only sees the
// output of the command it ran.
func outBuf(deps *Deps) *bytes.Buffer {
	buf := &bytes.Buffer{}
	deps.Stdout = buf
	return buf
}

// seedAccount creates an account directly through the service, bypassing the CLI.
func seedAccount(t *testing.T, deps *Deps, name string) domain.Account {
	t.Helper()
	ctx := context.Background()
	p, err := deps.Service.PlanAddAccount(ctx, app.AddAccountRequest{
		Name: name, Kind: "checking", Currency: "EUR", OpeningBalance: "0", OpenedOn: "2026-01-01",
	})
	if err != nil {
		t.Fatalf("PlanAddAccount(%s): %v", name, err)
	}
	a, err := deps.Service.ApplyAddAccount(ctx, p)
	if err != nil {
		t.Fatalf("ApplyAddAccount(%s): %v", name, err)
	}
	return a
}

// seedCategory creates a category directly through the service.
func seedCategory(t *testing.T, deps *Deps, name string) domain.Category {
	t.Helper()
	ctx := context.Background()
	p, err := deps.Service.PlanAddCategory(ctx, app.AddCategoryRequest{Name: name, Kind: "expense"})
	if err != nil {
		t.Fatalf("PlanAddCategory(%s): %v", name, err)
	}
	c, err := deps.Service.ApplyAddCategory(ctx, p)
	if err != nil {
		t.Fatalf("ApplyAddCategory(%s): %v", name, err)
	}
	return c
}

// seedTx creates a transaction directly through the service.
func seedTx(t *testing.T, deps *Deps, account, category, amount string) domain.Transaction {
	t.Helper()
	ctx := context.Background()
	p, err := deps.Service.PlanAddTx(ctx, app.AddTxRequest{
		Account: account, Category: category, Amount: amount, Date: "2026-01-10", Description: "seed",
	})
	if err != nil {
		t.Fatalf("PlanAddTx: %v", err)
	}
	tx, err := deps.Service.ApplyAddTx(ctx, p)
	if err != nil {
		t.Fatalf("ApplyAddTx: %v", err)
	}
	return tx
}
