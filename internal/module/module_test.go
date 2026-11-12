package module_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"pgregory.net/rapid"

	"github.com/0xballo/fui/internal/module"
)

func TestPreset(t *testing.T) {
	expense, err := module.Preset("expense")
	if err != nil {
		t.Fatalf("Preset(expense): %v", err)
	}
	if got := expense.IDs(); !slices.Equal(got, []module.ID{module.Budget, module.Core, module.Recurring}) {
		t.Fatalf("expense IDs = %v", got)
	}

	full, err := module.Preset("full")
	if err != nil {
		t.Fatalf("Preset(full): %v", err)
	}
	if got := full.IDs(); len(got) != 10 {
		t.Fatalf("full IDs count = %d, want 10", len(got))
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		ids     []module.ID
		wantErr string
	}{
		{name: "ok", ids: []module.ID{module.Core, module.Budget}},
		{name: "unknown", ids: []module.ID{module.Core, "unknown"}, wantErr: "unknown module"},
		{name: "missing core", ids: []module.ID{module.Budget}, wantErr: "required"},
		{name: "missing dependency", ids: []module.ID{module.Core, module.Crypto}, wantErr: "requires"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := module.Validate(tt.ids)
			if tt.wantErr == "" && err != nil {
				t.Fatalf("Validate(%v): %v", tt.ids, err)
			}
			if tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)) {
				t.Fatalf("Validate(%v) error = %v, want substring %q", tt.ids, err, tt.wantErr)
			}
		})
	}
}

func TestEnable(t *testing.T) {
	base, err := module.Validate([]module.ID{module.Core, module.Recurring})
	if err != nil {
		t.Fatalf("Validate(base): %v", err)
	}

	next, added, err := module.Enable(base, module.AI)
	if err != nil {
		t.Fatalf("Enable: %v", err)
	}
	if got := next.IDs(); !slices.Equal(got, []module.ID{module.AI, module.Core, module.Recurring, module.Simulations}) {
		t.Fatalf("Enable result IDs = %v", got)
	}
	if !slices.Equal(added, []module.ID{module.AI, module.Simulations}) {
		t.Fatalf("Enable added = %v", added)
	}

	_, _, err = module.Enable(base, "unknown")
	if err == nil {
		t.Fatal("Enable unknown expected error")
	}
}

func TestDisable(t *testing.T) {
	s, err := module.Validate([]module.ID{module.Core, module.Recurring, module.Simulations, module.AI})
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	next, dependents, err := module.Disable(s, module.Recurring)
	if err != nil {
		t.Fatalf("Disable recurring: %v", err)
	}
	if !slices.Equal(dependents, []module.ID{module.AI, module.Simulations}) {
		t.Fatalf("dependents = %v", dependents)
	}
	if got := next.IDs(); !slices.Equal(got, []module.ID{module.Core}) {
		t.Fatalf("Disable should remove target and transitive dependents, got %v", got)
	}

	_, _, err = module.Disable(s, module.Core)
	if err == nil {
		t.Fatal("Disable core expected error")
	}
}

func TestDisableTransitiveChain(t *testing.T) {
	s, err := module.Validate([]module.ID{module.Core, module.Recurring, module.Simulations, module.AI})
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	next, dependents, err := module.Disable(s, module.Recurring)
	if err != nil {
		t.Fatalf("Disable recurring: %v", err)
	}
	if !slices.Equal(dependents, []module.ID{module.AI, module.Simulations}) {
		t.Fatalf("dependents = %v", dependents)
	}
	if got := next.IDs(); !slices.Equal(got, []module.ID{module.Core}) {
		t.Fatalf("Disable recurring should leave only core, got %v", got)
	}
}

func TestRequire(t *testing.T) {
	s, err := module.Validate([]module.ID{module.Core, module.Budget})
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if err := s.Require(module.Budget); err != nil {
		t.Fatalf("Require(enabled): %v", err)
	}
	err = s.Require(module.Crypto)
	if !errors.Is(err, module.ErrDisabled) {
		t.Fatalf("Require(disabled) err = %v", err)
	}
	if !strings.Contains(err.Error(), "fui module enable crypto") {
		t.Fatalf("Require(disabled) message = %q", err.Error())
	}
}

func TestEnableDisableSequenceProperty(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		s := module.Default()
		ops := rapid.SliceOfN(rapid.Custom(func(rt *rapid.T) struct {
			enable bool
			id     module.ID
		} {
			ids := []module.ID{
				module.Core, module.Budget, module.Recurring, module.Loans, module.Investments,
				module.Crypto, module.Assets, module.Analytics, module.Simulations, module.AI,
			}
			return struct {
				enable bool
				id     module.ID
			}{
				enable: rapid.Bool().Draw(rt, "enable"),
				id:     ids[rapid.IntRange(0, len(ids)-1).Draw(rt, "idx")],
			}
		}), 1, 100).Draw(rt, "ops")

		for _, op := range ops {
			if op.enable {
				var added []module.ID
				next, added, err := module.Enable(s, op.id)
				if err != nil {
					continue
				}
				s = next
				_ = added
				continue
			}
			next, dependents, err := module.Disable(s, op.id)
			if err != nil {
				continue
			}
			if s.Has(op.id) {
				if next.Has(op.id) {
					rt.Fatalf("Disable(%s) left target enabled", op.id)
				}
				for _, dep := range dependents {
					if next.Has(dep) {
						rt.Fatalf("Disable(%s) left dependent %s enabled", op.id, dep)
					}
				}
			}
			s = next
		}

		if !s.Has(module.Core) {
			rt.Fatalf("core must stay enabled: %v", s.IDs())
		}
		if _, err := module.Validate(s.IDs()); err != nil {
			rt.Fatalf("set must stay dependency-closed: %v", err)
		}
	})
}
