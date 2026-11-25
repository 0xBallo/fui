package main

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/lending"
	"github.com/0xballo/fui/internal/app/recurring"
	"github.com/0xballo/fui/internal/clock"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
	"github.com/0xballo/fui/internal/store/sqlite"
)

type fakeTxLookup struct {
	recurrence domain.RecurrenceID
	removed    []domain.TransactionID
}

func (f *fakeTxLookup) ShowTx(_ context.Context, id domain.TransactionID) (app.TxView, error) {
	return app.TxView{Transaction: domain.Transaction{ID: id, RecurrenceID: f.recurrence}}, nil
}

func (f *fakeTxLookup) PlanRemoveTx(_ context.Context, req app.RemoveTxRequest) (app.RemoveTxPlan, error) {
	f.removed = append(f.removed, req.ID)
	return app.RemoveTxPlan{}, nil
}

func (f *fakeTxLookup) ApplyRemoveTx(context.Context, app.RemoveTxPlan) error { return nil }

type fakeRecurring struct{ calls []string }

func (f *fakeRecurring) PlanConfirm(context.Context, module.Set, domain.TransactionID) (recurring.ConfirmPlan, error) {
	f.calls = append(f.calls, "plan-confirm")
	return recurring.ConfirmPlan{}, nil
}

func (f *fakeRecurring) ApplyConfirm(context.Context, recurring.ConfirmPlan) error {
	f.calls = append(f.calls, "apply-confirm")
	return nil
}

type fakeLoans struct {
	calls []string
	err   error
}

func (f *fakeLoans) PlanConfirm(context.Context, module.Set, domain.TransactionID) (lending.ConfirmPlan, error) {
	f.calls = append(f.calls, "plan-confirm")
	return lending.ConfirmPlan{}, f.err
}

func (f *fakeLoans) ApplyConfirm(context.Context, lending.ConfirmPlan) error {
	f.calls = append(f.calls, "apply-confirm")
	return nil
}

func TestPendingActionsDispatch(t *testing.T) {
	notLoan := &domain.Error{
		Code: domain.CodeInvalidValue, Message: "transaction is not a loan installment",
		Details: map[string]any{"field": "transaction"},
	}
	moduleOff := errors.New("module loans is disabled")
	tests := []struct {
		name        string
		skip        bool
		recurrence  domain.RecurrenceID
		loanErr     error
		wantNotice  string
		wantErr     bool
		wantRecur   []string
		wantLoanLog []string
		wantRemoved []domain.TransactionID
	}{
		{name: "confirm recurring", recurrence: 7, wantNotice: noticeRecurBooked, wantRecur: []string{"plan-confirm", "apply-confirm"}},
		{name: "confirm loan", wantNotice: noticeLoanBooked, wantLoanLog: []string{"plan-confirm", "apply-confirm"}},
		{name: "confirm unlinked", loanErr: notLoan, wantNotice: noticeNotPending, wantLoanLog: []string{"plan-confirm"}},
		{name: "confirm loan module off", loanErr: moduleOff, wantErr: true, wantLoanLog: []string{"plan-confirm"}},
		{name: "skip recurring removes the pending tx", skip: true, recurrence: 7, wantNotice: noticeRecurSkip, wantRemoved: []domain.TransactionID{1}},
		{name: "skip loan or unlinked", skip: true, wantNotice: noticeNoSkip},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec, loans, txs := &fakeRecurring{}, &fakeLoans{err: tt.loanErr}, &fakeTxLookup{recurrence: tt.recurrence}
			a := pendingActions{txs: txs, recurring: rec, loans: loans, modules: module.Default}
			act := a.Confirm
			if tt.skip {
				act = a.Skip
			}
			notice, err := act(context.Background(), 1)
			if (err != nil) != tt.wantErr || notice != tt.wantNotice {
				t.Fatalf("got notice %q err %v, want %q (err=%v)", notice, err, tt.wantNotice, tt.wantErr)
			}
			if !slices.Equal(rec.calls, tt.wantRecur) || !slices.Equal(loans.calls, tt.wantLoanLog) || !slices.Equal(txs.removed, tt.wantRemoved) {
				t.Fatalf("recurring %v (want %v), loan %v (want %v), removed %v (want %v)",
					rec.calls, tt.wantRecur, loans.calls, tt.wantLoanLog, txs.removed, tt.wantRemoved)
			}
		})
	}
}

// TestPendingSkipDiscardsRowKeepsRecurrence runs Skip over a real archive: after catch-up the
// pending row exists and the recurrence already points past it, so skipping must delete the row
// and leave the recurrence alone.
func TestPendingSkipDiscardsRowKeepsRecurrence(t *testing.T) {
	ctx := context.Background()
	clk := clock.NewFixed(time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC))
	store, err := sqlite.Open(ctx, sqlite.Options{Path: filepath.Join(t.TempDir(), "fui.db"), BackupDir: t.TempDir(), BackupKeep: 3, Clock: clk})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	eur, err := domain.ParseCurrency("EUR")
	if err != nil {
		t.Fatal(err)
	}
	svc := app.New(store, clk, store, app.Config{Locale: domain.LocaleENUS, Location: time.UTC, BaseCurrency: eur})
	rec := recurring.New(store, clk, store, recurring.Config{Locale: domain.LocaleENUS})
	enabled := module.Default()
	if !enabled.Has(module.Recurring) {
		full, err := module.Preset("full")
		if err != nil {
			t.Fatal(err)
		}
		enabled = full
	}

	acct, err := svc.PlanAddAccount(ctx, app.AddAccountRequest{Name: "Checking", Kind: "checking", Currency: "EUR", OpeningBalance: "1000", OpenedOn: "2025-12-01"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ApplyAddAccount(ctx, acct); err != nil {
		t.Fatal(err)
	}
	plan, err := rec.PlanAdd(ctx, enabled, recurring.AddRequest{
		Account: "Checking", Description: "Gym", Amount: "-20", Every: 1, Unit: "months", DayOfMonth: 10, Start: "2026-01-10",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rec.ApplyAdd(ctx, plan); err != nil {
		t.Fatal(err)
	}
	today, err := domain.NewDate(2026, 1, 15)
	if err != nil {
		t.Fatal(err)
	}
	if n, err := rec.CatchUp(ctx, enabled, today); err != nil || n != 1 {
		t.Fatalf("CatchUp = %d, %v; want 1 pending row", n, err)
	}
	before, err := rec.List(ctx, enabled)
	if err != nil || len(before) != 1 {
		t.Fatalf("List = %v, %v", before, err)
	}
	page, err := svc.ListTx(ctx, app.ListTxRequest{Limit: 10})
	if err != nil || len(page.Items) != 1 || page.Items[0].Transaction.Status != domain.StatusPending {
		t.Fatalf("expected one pending row, got %+v (%v)", page.Items, err)
	}

	a := pendingActions{txs: svc, recurring: rec, modules: func() module.Set { return enabled }}
	notice, err := a.Skip(ctx, page.Items[0].Transaction.ID)
	if err != nil || notice != noticeRecurSkip {
		t.Fatalf("Skip = %q, %v", notice, err)
	}
	if page, err = svc.ListTx(ctx, app.ListTxRequest{Limit: 10}); err != nil || len(page.Items) != 0 {
		t.Fatalf("the pending row should be gone, got %+v (%v)", page.Items, err)
	}
	after, err := rec.List(ctx, enabled)
	if err != nil || len(after) != 1 || after[0].NextDate != before[0].NextDate {
		t.Fatalf("recurrence changed: before %+v after %+v (%v)", before, after, err)
	}
}
