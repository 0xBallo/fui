package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/cli/render"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
)

func TestMapModuleError(t *testing.T) {
	got := mapModuleError(errors.New("module \"core\" cannot be disabled"), module.Core)
	var derr *domain.Error
	if !errors.As(got, &derr) {
		t.Fatalf("mapModuleError() = %T, want *domain.Error", got)
	}
	if derr.Code != domain.CodeInvalidValue {
		t.Fatalf("mapModuleError() code = %q, want %q", derr.Code, domain.CodeInvalidValue)
	}

	original := &domain.Error{Code: domain.CodeNotFound, Message: "boom"}
	got = mapModuleError(original, module.Core)
	if !errors.Is(got, original) {
		t.Fatalf("mapModuleError(domain error) = %#v, want original error code", got)
	}
	var gotDomain *domain.Error
	if !errors.As(got, &gotDomain) || gotDomain.Code != original.Code {
		t.Fatalf("mapModuleError(domain error) = %#v, want unchanged code %q", got, original.Code)
	}
}

func TestWriteDisablePlanBranches(t *testing.T) {
	var out bytes.Buffer
	deps := &Deps{Stdout: &out}
	rt := runtimeFlags{format: render.Table}
	before, err := module.Validate([]module.ID{module.Core})
	if err != nil {
		t.Fatalf("module.Validate(before): %v", err)
	}

	err = writeDisablePlan(deps, rt, app.DisableModulePlan{Target: module.Budget, Before: before})
	if err != nil {
		t.Fatalf("writeDisablePlan no-op: %v", err)
	}
	if !strings.Contains(out.String(), "already disabled") {
		t.Fatalf("writeDisablePlan no-op output = %q, want already disabled note", out.String())
	}

	out.Reset()
	beforeWithRecurring, err := module.Validate([]module.ID{module.Core, module.Recurring, module.Simulations, module.AI})
	if err != nil {
		t.Fatalf("module.Validate(beforeWithRecurring): %v", err)
	}
	err = writeDisablePlan(deps, rt, app.DisableModulePlan{
		Target:             module.Recurring,
		Before:             beforeWithRecurring,
		DependentsDisabled: []module.ID{module.AI, module.Simulations},
	})
	if err != nil {
		t.Fatalf("writeDisablePlan text: %v", err)
	}
	if !strings.Contains(out.String(), "Would disable modules: ai, simulations, recurring") ||
		!strings.Contains(out.String(), "Warning: active dependent modules will also be disabled") {
		t.Fatalf("writeDisablePlan text output = %q", out.String())
	}

	out.Reset()
	rt.format = render.JSON
	err = writeDisablePlan(deps, rt, app.DisableModulePlan{
		Target:             module.Recurring,
		DependentsDisabled: []module.ID{module.AI, module.Simulations},
	})
	if err != nil {
		t.Fatalf("writeDisablePlan json: %v", err)
	}
	if !strings.Contains(out.String(), `"dependents_disabled":["ai","simulations"]`) {
		t.Fatalf("writeDisablePlan json output = %q", out.String())
	}
}

func TestWriteModuleSetResultBranches(t *testing.T) {
	enabled, err := module.Validate([]module.ID{module.Core, module.Budget})
	if err != nil {
		t.Fatalf("module.Validate(enabled): %v", err)
	}
	var out bytes.Buffer
	deps := &Deps{Stdout: &out}

	if err := writeModuleSetResult(deps, runtimeFlags{}, enabled); err != nil {
		t.Fatalf("writeModuleSetResult text: %v", err)
	}
	if !strings.Contains(out.String(), "Enabled modules:") {
		t.Fatalf("writeModuleSetResult text output = %q, want label", out.String())
	}

	out.Reset()
	if err := writeModuleSetResult(deps, runtimeFlags{format: render.JSON}, enabled); err != nil {
		t.Fatalf("writeModuleSetResult json: %v", err)
	}
	if !strings.Contains(out.String(), `"enabled":["budget","core"]`) {
		t.Fatalf("writeModuleSetResult json output = %q", out.String())
	}
}
