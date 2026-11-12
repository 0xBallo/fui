package cli

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/lending"
	"github.com/0xballo/fui/internal/app/recurring"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/pkg/fuiproto"
)

func newLendingStandalone(t *testing.T) *standalone {
	t.Helper()
	h := newStandalone(t, domain.LocaleENUS, "us")
	h.deps.Config.Modules.Enabled = append(h.deps.Config.Modules.Enabled, "loans")
	clk := fixedClock{now: time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC)}
	h.deps.Recurring = recurring.New(h.store, clk, noopBackup{}, recurring.Config{Locale: domain.LocaleENUS, Location: time.UTC})
	h.deps.Loans = lending.New(h.store, clk, noopBackup{}, lending.Config{Locale: domain.LocaleENUS, Location: time.UTC})
	return h
}

func TestRecurringCommands(t *testing.T) {
	h := newLendingStandalone(t)
	h.seedAccount(t, "Checking")
	h.seedCategory(t, "Subscriptions")

	if err := h.run(newRecurringCmd(h.deps),
		"recurring", "add", "--account", "Checking", "--category", "Subscriptions",
		"--description", "Netflix", "--amount", "-9.99", "--every", "1", "--unit", "months",
		"--start", "2026-01-01", "--json",
	); err != nil {
		t.Fatalf("recurring add: %v (stderr=%q)", err, h.err.String())
	}
	if !strings.Contains(h.out.String(), `"description":"Netflix"`) {
		t.Fatalf("recurring add output = %q", h.out.String())
	}

	if err := h.run(newRecurringCmd(h.deps), "recurring", "list", "--json"); err != nil {
		t.Fatalf("recurring list: %v (stderr=%q)", err, h.err.String())
	}
	if !strings.Contains(h.out.String(), `"description":"Netflix"`) {
		t.Fatalf("recurring list output = %q", h.out.String())
	}

	if err := h.run(newRecurringCmd(h.deps), "recurring", "run", "--until", "2026-01-01", "--yes", "--json"); err != nil {
		t.Fatalf("recurring run: %v (stderr=%q)", err, h.err.String())
	}
	if !strings.Contains(h.out.String(), `"created":1`) {
		t.Fatalf("recurring run output = %q", h.out.String())
	}

	ctx := context.Background()
	page, err := h.app.ListTx(ctx, app.ListTxRequest{Accounts: []string{"Checking"}, Statuses: []string{"pending"}})
	if err != nil {
		t.Fatalf("ListTx: %v", err)
	}
	if len(page.Items) != 1 {
		t.Fatalf("pending tx count = %d, want 1", len(page.Items))
	}
	txID := page.Items[0].Transaction.ID

	if err := h.run(newRecurringCmd(h.deps), "recurring", "confirm", strconv.FormatInt(int64(txID), 10), "--json"); err != nil {
		t.Fatalf("recurring confirm: %v (stderr=%q)", err, h.err.String())
	}
	got, err := h.app.ShowTx(ctx, txID)
	if err != nil {
		t.Fatalf("ShowTx: %v", err)
	}
	if got.Transaction.Status != domain.StatusBooked {
		t.Fatalf("tx status after confirm = %s, want booked", got.Transaction.Status)
	}

	// A second recurrence due later than --until above is skipped without creating a pending tx.
	if err := h.run(newRecurringCmd(h.deps),
		"recurring", "add", "--account", "Checking", "--amount", "-1.00",
		"--every", "1", "--unit", "months", "--start", "2027-01-01", "--json",
	); err != nil {
		t.Fatalf("recurring add (2nd): %v (stderr=%q)", err, h.err.String())
	}
	if err := h.run(newRecurringCmd(h.deps), "recurring", "skip", "2", "--json"); err != nil {
		t.Fatalf("recurring skip: %v (stderr=%q)", err, h.err.String())
	}
	if !strings.Contains(h.out.String(), `"next_date":"2027-02-01"`) {
		t.Fatalf("recurring skip output = %q", h.out.String())
	}
}

func TestLoanAddImport(t *testing.T) {
	h := newLendingStandalone(t)
	h.seedAccount(t, "Checking")

	path := filepath.Join(t.TempDir(), "schedule.fui.jsonl")
	fh, err := os.Create(path)
	if err != nil {
		t.Fatalf("create %s: %v", path, err)
	}
	enc := fuiproto.NewJSONLEncoder(fh)
	installments := []fuiproto.InstallmentRecord{
		{Account: "Checking", DueDate: "2027-01-01", Principal: 4900, Interest: 100, Remaining: 5100},
		{Account: "Checking", DueDate: "2027-02-01", Principal: 4950, Interest: 50, Remaining: 150},
		{Account: "Checking", DueDate: "2027-03-01", Principal: 150, Interest: 0, Remaining: 0},
	}
	for _, inst := range installments {
		rec := fuiproto.Record{Type: fuiproto.RecordTypeInstallment, Installment: &inst}
		if err := enc.Encode(rec); err != nil {
			t.Fatalf("encode installment: %v", err)
		}
	}
	if err := fh.Close(); err != nil {
		t.Fatalf("close %s: %v", path, err)
	}

	if err := h.run(newLoanCmd(h.deps),
		"loan", "add", "Prestito", "--kind", "loan", "--principal", "100", "--term-months", "3",
		"--rate-type", "fixed", "--rate", "3.00", "--debit", "Checking", "--import", path, "--json",
	); err != nil {
		t.Fatalf("loan add --import: %v (stderr=%q)", err, h.err.String())
	}
	if !strings.Contains(h.out.String(), `"name":"Prestito"`) {
		t.Fatalf("loan add --import output = %q", h.out.String())
	}

	if err := h.run(newLoanCmd(h.deps), "loan", "schedule", "Prestito", "--json"); err != nil {
		t.Fatalf("loan schedule: %v (stderr=%q)", err, h.err.String())
	}
	if !strings.Contains(h.out.String(), `"origin":"imported"`) {
		t.Fatalf("loan schedule output = %q", h.out.String())
	}
}

func TestLoanConfirmWarnsOnMismatch(t *testing.T) {
	h := newLendingStandalone(t)
	h.seedAccount(t, "Checking")

	if err := h.run(newLoanCmd(h.deps),
		"loan", "add", "Mutuo", "--kind", "mortgage", "--principal", "12000", "--term-months", "12",
		"--rate-type", "fixed", "--rate", "3.00", "--debit", "Checking", "--computed", "--json",
	); err != nil {
		t.Fatalf("loan add: %v (stderr=%q)", err, h.err.String())
	}
	if err := h.run(newLoanCmd(h.deps), "loan", "run", "--loan", "Mutuo", "--until", "2027-01-01", "--yes", "--json"); err != nil {
		t.Fatalf("loan run: %v (stderr=%q)", err, h.err.String())
	}

	ctx := context.Background()
	page, err := h.app.ListTx(ctx, app.ListTxRequest{Accounts: []string{"Checking"}, Statuses: []string{"pending"}})
	if err != nil {
		t.Fatalf("ListTx: %v", err)
	}
	if len(page.Items) != 1 {
		t.Fatalf("pending tx count = %d, want 1", len(page.Items))
	}
	tx := page.Items[0].Transaction
	tx.Amount, err = tx.Amount.Sub(mustMoney(t, 500, tx.Amount.Currency()))
	if err != nil {
		t.Fatalf("Sub: %v", err)
	}
	if err := h.store.WithTx(ctx, func(r app.Repos) error { return r.Transactions().Update(ctx, tx) }); err != nil {
		t.Fatalf("update tx: %v", err)
	}

	if err := h.run(newLoanCmd(h.deps), "loan", "confirm", strconv.FormatInt(int64(tx.ID), 10), "--json"); err != nil {
		t.Fatalf("loan confirm: %v (stderr=%q)", err, h.err.String())
	}
	if !strings.Contains(h.out.String(), `"warnings":[`) || !strings.Contains(h.out.String(), `"proposed_rate":`) {
		t.Fatalf("loan confirm output = %q, want warnings and a proposed rate", h.out.String())
	}
}

func mustMoney(t *testing.T, minor int64, cur domain.Currency) domain.Money {
	t.Helper()
	m, err := domain.NewMoney(minor, cur)
	if err != nil {
		t.Fatalf("NewMoney: %v", err)
	}
	return m
}

func TestLoanCommands(t *testing.T) {
	h := newLendingStandalone(t)
	h.seedAccount(t, "Checking")

	if err := h.run(newLoanCmd(h.deps),
		"loan", "add", "Mutuo", "--kind", "mortgage", "--principal", "12000", "--term-months", "12",
		"--rate-type", "fixed", "--rate", "3.00", "--debit", "Checking", "--computed", "--json",
	); err != nil {
		t.Fatalf("loan add: %v (stderr=%q)", err, h.err.String())
	}
	if !strings.Contains(h.out.String(), `"name":"Mutuo"`) || !strings.Contains(h.out.String(), `"debit_account":"Checking"`) {
		t.Fatalf("loan add output = %q", h.out.String())
	}

	if err := h.run(newLoanCmd(h.deps), "loan", "show", "Mutuo", "--json"); err != nil {
		t.Fatalf("loan show: %v (stderr=%q)", err, h.err.String())
	}
	if !strings.Contains(h.out.String(), `"liability_account":"Mutuo"`) {
		t.Fatalf("loan show output = %q", h.out.String())
	}

	if err := h.run(newLoanCmd(h.deps), "loan", "schedule", "Mutuo", "--json"); err != nil {
		t.Fatalf("loan schedule: %v (stderr=%q)", err, h.err.String())
	}
	if !strings.Contains(h.out.String(), `"installments":[`) {
		t.Fatalf("loan schedule output = %q", h.out.String())
	}

	if err := h.run(newLoanCmd(h.deps),
		"loan", "event", "Mutuo", "--kind", "rate_change", "--date", "2026-12-01", "--rate", "2.50", "--json",
	); err != nil {
		t.Fatalf("loan event: %v (stderr=%q)", err, h.err.String())
	}
	if !strings.Contains(h.out.String(), `"rate_bp":250`) {
		t.Fatalf("loan event output = %q", h.out.String())
	}

	if err := h.run(newLoanCmd(h.deps), "loan", "run", "--loan", "Mutuo", "--until", "2027-12-31", "--yes", "--json"); err != nil {
		t.Fatalf("loan run: %v (stderr=%q)", err, h.err.String())
	}
	if !strings.Contains(h.out.String(), `"created":12`) {
		t.Fatalf("loan run output = %q", h.out.String())
	}

	ctx := context.Background()
	page, err := h.app.ListTx(ctx, app.ListTxRequest{Accounts: []string{"Checking"}, Statuses: []string{"pending"}})
	if err != nil {
		t.Fatalf("ListTx: %v", err)
	}
	if len(page.Items) == 0 {
		t.Fatal("no pending installment transactions found")
	}
	txID := page.Items[0].Transaction.ID

	if err := h.run(newLoanCmd(h.deps), "loan", "confirm", strconv.FormatInt(int64(txID), 10), "--json"); err != nil {
		t.Fatalf("loan confirm: %v (stderr=%q)", err, h.err.String())
	}
	if !strings.Contains(h.out.String(), `"transaction"`) {
		t.Fatalf("loan confirm output = %q", h.out.String())
	}
}
