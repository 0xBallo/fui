package app_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
)

type moduleConfigSpy struct {
	writes [][]module.ID
	err    error
}

func (m *moduleConfigSpy) SetModulesEnabled(_ context.Context, ids []module.ID) error {
	if m.err != nil {
		return m.err
	}
	m.writes = append(m.writes, slices.Clone(ids))
	return nil
}

func TestPlanEnableModule(t *testing.T) {
	e := newEnv(t)
	start, err := module.Validate([]module.ID{module.Core, module.Recurring})
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	plan, err := e.svc.PlanEnableModule(t.Context(), start, module.AI)
	if err != nil {
		t.Fatalf("PlanEnableModule: %v", err)
	}
	if !plan.After.Has(module.Simulations) || !plan.After.Has(module.AI) {
		t.Fatalf("enabled set missing closure: %v", plan.After.IDs())
	}
	if !slices.Equal(plan.Added, []module.ID{module.AI, module.Simulations}) {
		t.Fatalf("added = %v", plan.Added)
	}
}

func TestPlanEnableModuleUnknownIsUserInputError(t *testing.T) {
	e := newEnv(t)
	start, err := module.Validate([]module.ID{module.Core, module.Recurring})
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	_, err = e.svc.PlanEnableModule(t.Context(), start, module.ID("unknown"))
	if !errors.Is(err, domain.ErrInvalidValue) {
		t.Fatalf("PlanEnableModule unknown error = %v", err)
	}
}

func TestApplyEnableModule(t *testing.T) {
	e := newEnv(t)
	start, err := module.Validate([]module.ID{module.Core, module.Recurring})
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	plan, err := e.svc.PlanEnableModule(t.Context(), start, module.AI)
	if err != nil {
		t.Fatalf("PlanEnableModule: %v", err)
	}
	spy := &moduleConfigSpy{}
	if err := e.svc.ApplyEnableModule(t.Context(), spy, plan); err != nil {
		t.Fatalf("ApplyEnableModule: %v", err)
	}
	if len(spy.writes) != 1 || !slices.Equal(spy.writes[0], plan.After.IDs()) {
		t.Fatalf("writes = %v", spy.writes)
	}
}

func TestPlanDisableModule(t *testing.T) {
	e := newEnv(t)
	start, err := module.Validate([]module.ID{module.Core, module.Recurring, module.Simulations, module.AI})
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	plan, err := e.svc.PlanDisableModule(t.Context(), start, module.Recurring)
	if err != nil {
		t.Fatalf("PlanDisableModule: %v", err)
	}
	if !plan.RequiresConfirmation {
		t.Fatal("expected confirmation for dependent modules")
	}
	if !slices.Equal(plan.DependentsDisabled, []module.ID{module.AI, module.Simulations}) {
		t.Fatalf("dependents = %v", plan.DependentsDisabled)
	}
	if plan.After.Has(module.Recurring) || plan.After.Has(module.Simulations) || plan.After.Has(module.AI) {
		t.Fatalf("after should disable target and dependents: %v", plan.After.IDs())
	}
}

func TestApplyDisableModule(t *testing.T) {
	e := newEnv(t)
	start, err := module.Validate([]module.ID{module.Core, module.Budget})
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	plan, err := e.svc.PlanDisableModule(t.Context(), start, module.Budget)
	if err != nil {
		t.Fatalf("PlanDisableModule: %v", err)
	}
	spy := &moduleConfigSpy{}
	if err := e.svc.ApplyDisableModule(t.Context(), spy, plan); err != nil {
		t.Fatalf("ApplyDisableModule: %v", err)
	}
	if len(spy.writes) != 1 || !slices.Equal(spy.writes[0], plan.After.IDs()) {
		t.Fatalf("writes = %v", spy.writes)
	}
}

func TestListModules(t *testing.T) {
	e := newEnv(t)
	enabled, err := module.Validate([]module.ID{module.Core, module.Budget, module.Recurring})
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	list := e.svc.ListModules(t.Context(), enabled)
	if len(list) != 10 {
		t.Fatalf("len(list) = %d, want 10", len(list))
	}
	if list[0].ID != module.Core || !list[0].Enabled || !list[0].InExpense {
		t.Fatalf("unexpected first row: %+v", list[0])
	}
}

func TestWrapModuleError(t *testing.T) {
	err := app.WrapModuleError(errors.New("plain"))
	if err == nil || err.Error() != "plain" {
		t.Fatalf("WrapModuleError(plain) = %v", err)
	}

	err = app.WrapModuleError(module.ErrDisabled)
	if !errors.Is(err, app.ErrModuleDisabled) {
		t.Fatalf("WrapModuleError(module.ErrDisabled) = %v", err)
	}
}
