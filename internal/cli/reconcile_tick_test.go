package cli

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/config"
	"github.com/0xballo/fui/internal/domain"
)

func TestReconcileTickFlow(t *testing.T) {
	h := newStandalone(t, domain.LocaleENUS, config.LocaleUS)
	h.seedAccount(t, "A")
	one := h.seedTx(t, "A", "-10", "One", "2026-10-01", "booked")
	two := h.seedTx(t, "A", "-20", "Two", "2026-10-02", "booked")
	id := func(tx domain.Transaction) string { return strconv.FormatInt(int64(tx.ID), 10) }
	base := []string{"reconcile", "A", "--balance", "-30", "--date", "2026-10-10", "--json"}
	run := func(extra ...string) error {
		return h.run(newReconcileCmd(h.deps), append(append([]string{}, base...), extra...)...)
	}

	// --dry-run never stores ticks.
	if err := run("--tick", id(one), "--dry-run"); err != nil {
		t.Fatalf("dry-run tick: %v", err)
	}
	if err := run("--dry-run"); err != nil {
		t.Fatalf("plain reconcile: %v", err)
	}
	if strings.Contains(h.out.String(), `"ticked"`) {
		t.Fatalf("dry-run persisted ticks: %s", h.out.String())
	}

	// A partial tick shows the difference and is stored without --yes.
	if err := run("--tick", id(one)); err != nil {
		t.Fatalf("partial tick: %v", err)
	}
	for _, want := range []string{`"ticked":1`, `"can_close":false`, `"difference":-2000`} {
		if !strings.Contains(h.out.String(), want) {
			t.Errorf("partial tick output missing %s: %s", want, h.out.String())
		}
	}

	// Ticks persist across runs; ticking the rest reaches zero and a close needs --yes.
	err := run("--tick", id(two))
	var derr *domain.Error
	if !errors.As(err, &derr) || derr.Code != app.CodeConfirmationRequired {
		t.Fatalf("closing tick without --yes err = %v, want confirmation_required", err)
	}
	if err := run("--tick", id(one)+","+id(two), "--yes"); err != nil {
		t.Fatalf("closing tick: %v", err)
	}
	view, err := h.app.ShowTx(t.Context(), two.ID)
	if err != nil {
		t.Fatalf("ShowTx: %v", err)
	}
	if view.Transaction.Status != domain.StatusReconciled {
		t.Fatalf("status = %s, want reconciled", view.Transaction.Status)
	}
}

func TestReconcileUntickAndTable(t *testing.T) {
	h := newStandalone(t, domain.LocaleENUS, config.LocaleUS)
	h.seedAccount(t, "A")
	one := h.seedTx(t, "A", "-10", "One", "2026-10-01", "booked")
	id := strconv.FormatInt(int64(one.ID), 10)
	run := func(extra ...string) error {
		args := append([]string{"reconcile", "A", "--balance", "-10", "--date", "2026-10-10"}, extra...)
		return h.run(newReconcileCmd(h.deps), args...)
	}

	if err := run("--tick", id, "--dry-run"); err != nil {
		t.Fatalf("tick: %v", err)
	}
	for _, want := range []string{"Ticked transactions: 1", "Unreconciled transactions: 0", "pass --yes"} {
		if !strings.Contains(h.out.String(), want) {
			t.Errorf("table output missing %q: %s", want, h.out.String())
		}
	}

	if err := run("--tick", id); err == nil {
		t.Fatal("zero-difference tick without --yes should require confirmation")
	}
	// Nothing was stored (the close was refused), so store a partial state with another balance.
	if err := h.run(newReconcileCmd(h.deps), "reconcile", "A", "--balance", "-5", "--date", "2026-10-10", "--tick", id); err != nil {
		t.Fatalf("partial tick: %v", err)
	}
	if !strings.Contains(h.out.String(), "Ticks saved.") {
		t.Errorf("missing save note: %s", h.out.String())
	}
	if err := h.run(newReconcileCmd(h.deps), "reconcile", "A", "--balance", "-5", "--date", "2026-10-10", "--untick", id, "--json"); err != nil {
		t.Fatalf("untick: %v", err)
	}
	if !strings.Contains(h.out.String(), `"ticked":0`) || !strings.Contains(h.out.String(), `"unreconciled":[{`) {
		t.Errorf("untick output = %s", h.out.String())
	}
	if err := run("--tick", "x"); err == nil {
		t.Fatal("invalid tick id should fail")
	}
	if err := run("--untick", "x"); err == nil {
		t.Fatal("invalid untick id should fail")
	}
}
