package app_test

import (
	"reflect"
	"testing"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

func TestBackup(t *testing.T) {
	e := newEnv(t)
	path, err := e.svc.Backup(t.Context())
	wantNoErr(t, err)
	if path != "/backups/manual.db" || !reflect.DeepEqual(e.backup.reasons, []string{app.BackupReasonManual}) {
		t.Errorf("path = %q, reasons = %v", path, e.backup.reasons)
	}
}

func TestRestore(t *testing.T) {
	e := newEnv(t)
	_, err := e.svc.PlanRestore(app.RestoreRequest{})
	wantCode(t, err, domain.ErrMissingField)

	p, err := e.svc.PlanRestore(app.RestoreRequest{Path: "/b/old.db"})
	wantNoErr(t, err)
	if !p.Bulk || len(p.Changes) != 1 || p.Changes[0].Entity != app.EntityArchive {
		t.Errorf("plan = %+v, want a bulk plan with one archive change", p)
	}
	if len(e.backup.reasons)+len(e.backup.restored) != 0 {
		t.Fatal("planning must not touch the archive")
	}
	wantNoErr(t, e.svc.ApplyRestore(t.Context(), p))
	if !reflect.DeepEqual(e.backup.reasons, []string{app.BackupReasonRestore}) || !reflect.DeepEqual(e.backup.restored, []string{"/b/old.db"}) {
		t.Errorf("reasons = %v, restored = %v; want a restore backup first, then the restore", e.backup.reasons, e.backup.restored)
	}
}

func TestAppCodes(t *testing.T) {
	seen := map[domain.Code]bool{}
	for _, c := range domain.Codes() {
		seen[c] = true
	}
	for _, c := range app.Codes() {
		if seen[c] {
			t.Errorf("app code %q duplicates a domain code", c)
		}
		seen[c] = true
	}
}
