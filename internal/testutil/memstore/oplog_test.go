package memstore

import (
	"testing"
	"time"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

func TestOpLog(t *testing.T) {
	s := New()
	ctx := t.Context()
	err := s.WithTx(ctx, func(r app.Repos) error {
		o := r.OpLog()
		if _, err := o.Latest(ctx); err == nil {
			t.Error("Latest on empty journal succeeded")
		}
		a, _ := o.Append(ctx, app.OpEntry{Kind: "a"})
		b, _ := o.Append(ctx, app.OpEntry{Kind: "b"})
		if err := o.MarkUndone(ctx, b.ID, time.Time{}.Add(time.Hour)); err != nil {
			t.Error(err)
		}
		if got, _ := o.Latest(ctx); got.ID != a.ID {
			t.Errorf("Latest = %d, want %d", got.ID, a.ID)
		}
		if err := o.MarkUndone(ctx, b.ID, time.Time{}); err == nil {
			t.Error("double MarkUndone succeeded")
		}
		if err := o.RestoreTransaction(ctx, domain.Transaction{ID: 5, AccountID: 1}); err == nil {
			t.Error("restore without account succeeded")
		}
		if err := o.RestoreTransferGroup(ctx, 3, domain.Money{}); err != nil {
			t.Error(err)
		}
		if err := o.RestoreTransferGroup(ctx, 3, domain.Money{}); err == nil {
			t.Error("duplicate group restore succeeded")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
