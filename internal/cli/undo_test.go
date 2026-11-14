package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

func undoTxCount(t *testing.T, deps *Deps) int {
	t.Helper()
	page, err := deps.Service.ListTx(context.Background(), app.ListTxRequest{})
	if err != nil {
		t.Fatalf("ListTx: %v", err)
	}
	return len(page.Items)
}

func TestUndoCommand(t *testing.T) {
	deps := newTestDeps(t)
	seedAccount(t, deps, "Main")
	seedCategory(t, deps, "Food")
	seedTx(t, deps, "Main", "Food", "-5")

	out := outBuf(deps)
	mustRunCmd(t, deps, newUndoCmd(deps), []string{"--dry-run"})
	if !strings.Contains(out.String(), "undo") || undoTxCount(t, deps) != 1 {
		t.Fatalf("dry-run must preview only; out=%q", out.String())
	}

	err := runCmd(deps, newUndoCmd(deps), nil)
	if ExitCode(err) != ExitUserError {
		t.Fatalf("no --yes without terminal: want exit %d, got %v", ExitUserError, err)
	}

	out = outBuf(deps)
	mustRunCmd(t, deps, newUndoCmd(deps), []string{"--yes", "--json"})
	var got struct {
		Kind     string `json:"kind"`
		Reverted int    `json:"reverted"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("json: %v: %s", err, out.String())
	}
	if got.Kind != app.OpKindTxAdd || got.Reverted != 1 || undoTxCount(t, deps) != 0 {
		t.Fatalf("got %+v, txs=%d", got, undoTxCount(t, deps))
	}

	err = runCmd(deps, newUndoCmd(deps), []string{"--yes"})
	if ExitCode(err) != ExitUserError || !strings.Contains(err.Error(), "nothing to undo") {
		t.Fatalf("empty journal: %v", err)
	}
}

func TestUndoReconciledExitsUserError(t *testing.T) {
	deps := newTestDeps(t)
	seedAccount(t, deps, "Main")
	seedCategory(t, deps, "Food")
	seedTx(t, deps, "Main", "Food", "-5")
	ctx := context.Background()
	enabled, err := enabledModulesFromDeps(deps)
	if err != nil {
		t.Fatal(err)
	}
	p, err := deps.Service.PlanReconcile(ctx, enabled, app.ReconcileRequest{Account: "Main", StatementBalance: "-5", Date: "2026-01-31"})
	if err != nil {
		t.Fatalf("PlanReconcile: %v", err)
	}
	if err := deps.Service.ApplyReconcile(ctx, enabled, p); err != nil {
		t.Fatalf("ApplyReconcile: %v", err)
	}
	deps.Stdout = &bytes.Buffer{}
	err = runCmd(deps, newUndoCmd(deps), []string{"--yes"})
	if ExitCode(err) != ExitUserError || !domainIs(err, domain.CodeReconciled) {
		t.Fatalf("want reconciled user error, got %v", err)
	}
}

func domainIs(err error, code domain.Code) bool {
	var de *domain.Error
	return err != nil && errors.As(err, &de) && de.Code == code
}
