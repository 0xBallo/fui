package clock_test

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/0xballo/fui/internal/clock"
)

var (
	_ clock.Clock = clock.System{}
	_ clock.Clock = (*clock.Fixed)(nil)
)

func TestSystemClock(t *testing.T) {
	c := clock.NewSystem()
	before := time.Now()
	got := c.Now()
	after := time.Now()

	if got.Before(before) || got.After(after) {
		t.Errorf("System.Now() = %v; want between %v and %v", got, before, after)
	}
}

func TestFixedClock(t *testing.T) {
	ref := time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
	fixed := clock.NewFixed(ref)

	if diff := cmp.Diff(ref, fixed.Now()); diff != "" {
		t.Errorf("unexpected time (-want +got):\n%s", diff)
	}

	delta := 15 * time.Minute
	fixed.Advance(delta)
	wantAdvanced := ref.Add(delta)
	if diff := cmp.Diff(wantAdvanced, fixed.Now()); diff != "" {
		t.Errorf("unexpected advanced time (-want +got):\n%s", diff)
	}

	newRef := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)
	fixed.Set(newRef)
	if diff := cmp.Diff(newRef, fixed.Now()); diff != "" {
		t.Errorf("unexpected set time (-want +got):\n%s", diff)
	}
}
