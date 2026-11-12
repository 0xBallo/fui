package cli

import (
	"slices"
	"testing"

	"github.com/0xballo/fui/internal/module"
)

func TestValidateInitModuleFlagCombination(t *testing.T) {
	tests := []struct {
		name      string
		preset    string
		modules   string
		prompted  bool
		wantError bool
	}{
		{name: "custom with modules", preset: "custom", modules: "core,budget"},
		{name: "prompted custom without modules", preset: "custom", prompted: true},
		{name: "explicit custom without modules", preset: "custom", wantError: true},
		{name: "modules without custom", preset: "expense", modules: "core,budget", wantError: true},
		{name: "full without modules", preset: "full"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateInitModuleFlagCombination(tt.preset, tt.modules, tt.prompted)
			if (err != nil) != tt.wantError {
				t.Fatalf("validateInitModuleFlagCombination() error=%v wantError=%v", err, tt.wantError)
			}
		})
	}
}

func TestCloseCustomSelectionIncludesCoreAndDependencies(t *testing.T) {
	got, warnings, err := closeCustomSelection([]string{string(module.AI)})
	if err != nil {
		t.Fatalf("closeCustomSelection: %v", err)
	}
	want, err := module.Validate([]module.ID{module.Core, module.Recurring, module.Simulations, module.AI})
	if err != nil {
		t.Fatalf("module.Validate(want): %v", err)
	}
	if !slices.Equal(got.IDs(), want.IDs()) {
		t.Fatalf("selected set = %v, want %v", got.IDs(), want.IDs())
	}
	if !slices.Equal(warnings, []string{string(module.Recurring), string(module.Simulations)}) {
		t.Fatalf("warnings = %v, want recurring and simulations", warnings)
	}
}

func TestCloseCustomSelectionEmptyKeepsCore(t *testing.T) {
	got, warnings, err := closeCustomSelection(nil)
	if err != nil {
		t.Fatalf("closeCustomSelection(nil): %v", err)
	}
	want, err := module.Validate([]module.ID{module.Core})
	if err != nil {
		t.Fatalf("module.Validate(core): %v", err)
	}
	if !slices.Equal(got.IDs(), want.IDs()) {
		t.Fatalf("selected set = %v, want %v", got.IDs(), want.IDs())
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings = %v, want none", warnings)
	}
}

func TestParseCustomModulesRejectsEmptyID(t *testing.T) {
	_, err := parseCustomModules("core,,budget")
	if err == nil {
		t.Fatal("expected parseCustomModules to fail on empty module id")
	}
}
