package app_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

type loaderBackup struct {
	spyBackup
	got app.ArchiveDump
	err error
}

func (b *loaderBackup) LoadArchive(_ context.Context, d app.ArchiveDump) error {
	b.got = d
	return b.err
}

func sampleDump() app.ArchiveDump {
	return app.ArchiveDump{SchemaVersion: 2, Tables: []app.TableDump{
		{Name: "tags", Columns: []string{"id", "name"}, Rows: [][]any{{int64(1), "a"}, {int64(2), "b"}}},
		{Name: "payees", Columns: []string{"id", "name"}},
	}}
}

func TestPlanRestoreDump(t *testing.T) {
	e := newEnv(t)
	plan, err := e.svc.PlanRestoreDump(sampleDump(), "dump.json")
	if err != nil {
		t.Fatalf("PlanRestoreDump: %v", err)
	}
	if !plan.Bulk || len(plan.Changes) != 1 || plan.Changes[0].Entity != app.EntityArchive || plan.Path != "dump.json" {
		t.Errorf("unexpected plan: %+v", plan)
	}
	if diff := cmp.Diff(map[string]any{"tags": 2, "payees": 0}, plan.Changes[0].Fields); diff != "" {
		t.Errorf("fields (-want +got):\n%s", diff)
	}

	for name, tc := range map[string]struct {
		d      app.ArchiveDump
		source string
		want   *domain.Error
	}{
		"no source":  {sampleDump(), "", domain.ErrMissingField},
		"no tables":  {app.ArchiveDump{SchemaVersion: 2}, "x", domain.ErrInvalidValue},
		"no version": {app.ArchiveDump{Tables: sampleDump().Tables}, "x", domain.ErrInvalidValue},
	} {
		if _, err := e.svc.PlanRestoreDump(tc.d, tc.source); !errors.Is(err, tc.want) {
			t.Errorf("%s: err = %v, want %v", name, err, tc.want)
		}
	}
}

func TestApplyRestoreDump(t *testing.T) {
	ctx := context.Background()
	cfg := app.Config{Locale: domain.LocaleENUS, Location: time.UTC, BaseCurrency: currency(t, "EUR")}
	clk := fixedClock{time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC)}

	lb := &loaderBackup{}
	e := newEnv(t)
	svc := app.New(e.store, clk, lb, cfg)
	if err := svc.ApplyRestoreDump(ctx, sampleDump()); err != nil {
		t.Fatalf("ApplyRestoreDump: %v", err)
	}
	if diff := cmp.Diff(sampleDump(), lb.got); diff != "" {
		t.Errorf("loader got (-want +got):\n%s", diff)
	}

	boom := errors.New("boom")
	svc = app.New(e.store, clk, &loaderBackup{err: boom}, cfg)
	if err := svc.ApplyRestoreDump(ctx, sampleDump()); !errors.Is(err, boom) {
		t.Errorf("err = %v, want wrapped boom", err)
	}

	if err := e.svc.ApplyRestoreDump(ctx, sampleDump()); err == nil {
		t.Error("ApplyRestoreDump succeeded without an ArchiveLoader")
	}
}
