package app_test

import (
	"context"
	"errors"
	"testing"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
)

// fakeSource creates left transactions on its first call and none afterwards, like a real
// idempotent source.
type fakeSource struct {
	left int
	err  error
}

func (f *fakeSource) CatchUp(context.Context, module.Set, domain.Date) (int, error) {
	n := f.left
	f.left = 0
	return n, f.err
}

func TestCatchup(t *testing.T) {
	boom := errors.New("boom")
	tests := []struct {
		name    string
		srcs    []app.CatchupSource
		want    []int
		wantErr error
	}{
		{"none", nil, []int{0, 0}, nil},
		{"sums and is idempotent", []app.CatchupSource{&fakeSource{left: 2}, &fakeSource{left: 3}}, []int{5, 0}, nil},
		{"error stops and wraps", []app.CatchupSource{&fakeSource{left: 1, err: boom}, &fakeSource{left: 9}}, []int{1}, boom},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for i, want := range tt.want {
				got, err := app.Catchup(context.Background(), module.Default(), domain.Date{}, tt.srcs...)
				if !errors.Is(err, tt.wantErr) || got != want {
					t.Fatalf("call %d = %d, %v, want %d, %v", i+1, got, err, want, tt.wantErr)
				}
			}
		})
	}
}
