package app_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/testutil/memstore"
)

// resetBackup is a Backuper that is also a Resetter, delegating Reset to the memstore.
type resetBackup struct {
	spyBackup
	store *memstore.Store
}

func (b *resetBackup) Reset(ctx context.Context, o app.ResetOptions) error {
	return b.store.Reset(ctx, o)
}

func resetEnv(t *testing.T) env {
	t.Helper()
	e := newEnv(t)
	rb := &resetBackup{store: e.store}
	cfg := app.Config{Locale: domain.LocaleENUS, Location: time.UTC, BaseCurrency: currency(t, "EUR")}
	e.svc = app.New(e.store, fixedClock{time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC)}, rb, cfg)
	return e
}

func seedReset(t *testing.T, e env) {
	t.Helper()
	ctx := context.Background()
	wantNoErr(t, func() error {
		p, err := e.svc.PlanInit(ctx)
		if err != nil {
			return err
		}
		_, err = e.svc.ApplyInit(ctx, p)
		return err
	}())
	e.category(t, "Custom", "expense")
	e.account(t, "Checking", "checking", "EUR", "100")
}

func TestPlanResetCounts(t *testing.T) {
	e := resetEnv(t)
	seedReset(t, e)
	ctx := context.Background()

	full, err := e.svc.PlanReset(ctx, app.ResetRequest{})
	wantNoErr(t, err)
	if !full.Bulk || full.Init != nil {
		t.Fatalf("plan = %+v, want bulk without init", full)
	}
	var sums []string
	for _, c := range full.Changes {
		sums = append(sums, string(c.Op)+" "+c.Summary)
	}
	got := strings.Join(sums, "|")
	for _, want := range []string{"delete 1 accounts", "delete 21 categories", "create 20 default categories"} {
		if !strings.Contains(got, want) {
			t.Errorf("plan %q lacks %q", got, want)
		}
	}

	keep, err := e.svc.PlanReset(ctx, app.ResetRequest{KeepSettings: true})
	wantNoErr(t, err)
	var kept bool
	for _, c := range keep.Changes {
		kept = kept || (c.Op == app.OpKeep && c.Entity == app.EntityCategory && c.Fields["count"] == 21)
		if c.Op == app.OpDelete && c.Entity == app.EntityCategory {
			t.Errorf("keep-settings plan deletes categories: %+v", c)
		}
	}
	if !kept {
		t.Errorf("keep plan lacks the kept categories: %+v", keep.Changes)
	}
}

func TestPlanResetWritesNothing(t *testing.T) {
	e := resetEnv(t)
	seedReset(t, e)
	before := e.store.Snapshot()
	_, err := e.svc.PlanReset(context.Background(), app.ResetRequest{})
	wantNoErr(t, err)
	if e.store.Snapshot() != before {
		t.Error("PlanReset changed the store")
	}
}

func TestApplyReset(t *testing.T) {
	for _, keep := range []bool{false, true} {
		e := resetEnv(t)
		seedReset(t, e)
		ctx := context.Background()
		p, err := e.svc.PlanReset(ctx, app.ResetRequest{KeepSettings: keep})
		wantNoErr(t, err)
		res, err := e.svc.ApplyReset(ctx, p)
		wantNoErr(t, err)
		if res.Initialized {
			t.Errorf("keep=%v: reset was delegated to init", keep)
		}
		after, err := e.svc.PlanReset(ctx, app.ResetRequest{KeepSettings: keep})
		wantNoErr(t, err)
		for _, c := range after.Changes {
			if c.Op == app.OpDelete && c.Entity != app.EntityCategory {
				t.Errorf("keep=%v: data left after reset: %+v", keep, c)
			}
		}
		wantCats := 20
		if keep {
			wantCats = 21
		}
		wantCount(t, e, wantCats)
	}
}

func wantCount(t *testing.T, e env, want int) {
	t.Helper()
	p, err := e.svc.PlanReset(context.Background(), app.ResetRequest{KeepSettings: true})
	wantNoErr(t, err)
	for _, c := range p.Changes {
		if c.Op == app.OpKeep && c.Entity == app.EntityCategory && c.Fields["count"] != want {
			t.Errorf("categories = %v, want %d", c.Fields["count"], want)
		}
	}
}

func TestResetOnEmptyArchiveDelegatesToInit(t *testing.T) {
	e := resetEnv(t)
	ctx := context.Background()
	p, err := e.svc.PlanReset(ctx, app.ResetRequest{})
	wantNoErr(t, err)
	if p.Init == nil || p.Bulk {
		t.Fatalf("plan = %+v, want non-bulk init plan", p)
	}
	res, err := e.svc.ApplyReset(ctx, p)
	wantNoErr(t, err)
	if !res.Initialized || res.Created != 20 {
		t.Errorf("result = %+v, want initialized with 20 categories", res)
	}
}

func TestApplyResetWithoutResetter(t *testing.T) {
	e := newEnv(t)
	e.category(t, "Custom", "expense")
	ctx := context.Background()
	p, err := e.svc.PlanReset(ctx, app.ResetRequest{})
	wantNoErr(t, err)
	if _, err := e.svc.ApplyReset(ctx, p); err == nil {
		t.Fatal("ApplyReset succeeded without a Resetter")
	}
}
