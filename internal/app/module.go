package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
)

// ModuleConfigWriter persists the enabled-module list to configuration.
type ModuleConfigWriter interface {
	SetModulesEnabled(ctx context.Context, ids []module.ID) error
}

// EnableModulePlan describes the effect of enabling one module.
type EnableModulePlan struct {
	Plan
	Target module.ID
	Before module.Set
	After  module.Set
	Added  []module.ID
}

// DisableModulePlan describes the effect of disabling one module.
type DisableModulePlan struct {
	Plan
	Target               module.ID
	Before               module.Set
	After                module.Set
	DependentsDisabled   []module.ID
	RequiresConfirmation bool
}

// ModuleInfo is one module row returned by ListModules.
type ModuleInfo struct {
	ID        module.ID
	Enabled   bool
	Default   bool
	Deps      []module.ID
	InExpense bool
	InFull    bool
}

// PlanEnableModule plans enabling id from currently enabled modules.
func (s *Service) PlanEnableModule(_ context.Context, enabled module.Set, id module.ID) (EnableModulePlan, error) {
	after, added, err := module.Enable(enabled, id)
	if err != nil {
		return EnableModulePlan{}, &domain.Error{
			Code:    domain.CodeInvalidValue,
			Message: err.Error(),
			Details: map[string]any{"field": "module", "value": string(id)},
		}
	}
	plan := EnableModulePlan{
		Target: id, Before: enabled, After: after, Added: added,
	}
	if len(added) == 0 {
		return plan, nil
	}
	for _, mid := range added {
		plan.Changes = append(plan.Changes, Change{
			Op:      OpUpdate,
			Entity:  "module",
			Summary: fmt.Sprintf("enable module %s", mid),
			Fields: map[string]any{
				"module_id": mid,
				"enabled":   true,
			},
		})
	}
	return plan, nil
}

// ApplyEnableModule writes the enabled module list to configuration.
func (s *Service) ApplyEnableModule(ctx context.Context, writer ModuleConfigWriter, p EnableModulePlan) error {
	if writer == nil {
		return errors.New("module config writer is required")
	}
	if err := writer.SetModulesEnabled(ctx, p.After.IDs()); err != nil {
		return fmt.Errorf("set modules.enabled: %w", err)
	}
	return nil
}

// PlanDisableModule plans disabling id and every currently enabled dependent.
func (s *Service) PlanDisableModule(_ context.Context, enabled module.Set, id module.ID) (DisableModulePlan, error) {
	next, dependents, err := module.Disable(enabled, id)
	if err != nil {
		return DisableModulePlan{}, err
	}
	if !enabled.Has(id) {
		return DisableModulePlan{Target: id, Before: enabled, After: next}, nil
	}

	plan := DisableModulePlan{
		Target:               id,
		Before:               enabled,
		After:                next,
		DependentsDisabled:   slicesClone(dependents),
		RequiresConfirmation: len(dependents) > 0,
	}
	idsToDisable := append(slicesClone(dependents), id)
	for _, mid := range idsToDisable {
		plan.Changes = append(plan.Changes, Change{
			Op:      OpUpdate,
			Entity:  "module",
			Summary: fmt.Sprintf("disable module %s", mid),
			Fields: map[string]any{
				"module_id": mid,
				"enabled":   false,
			},
		})
	}
	plan.Bulk = len(idsToDisable) > 1
	return plan, nil
}

// ApplyDisableModule writes the enabled module list to configuration.
func (s *Service) ApplyDisableModule(ctx context.Context, writer ModuleConfigWriter, p DisableModulePlan) error {
	if writer == nil {
		return errors.New("module config writer is required")
	}
	if err := writer.SetModulesEnabled(ctx, p.After.IDs()); err != nil {
		return fmt.Errorf("set modules.enabled: %w", err)
	}
	return nil
}

// ListModules returns metadata and enabled state for all modules.
func (s *Service) ListModules(_ context.Context, enabled module.Set) []ModuleInfo {
	defs := module.Definitions()
	out := make([]ModuleInfo, 0, len(defs))
	for _, def := range defs {
		out = append(out, ModuleInfo{
			ID:        def.ID,
			Enabled:   enabled.Has(def.ID),
			Default:   def.DefaultEnabled,
			Deps:      def.Dependencies,
			InExpense: def.InExpense,
			InFull:    true,
		})
	}
	return out
}

// WrapModuleError maps module package errors to app error codes.
func WrapModuleError(err error) error {
	if errors.Is(err, module.ErrDisabled) {
		return newError(ErrModuleDisabled, map[string]any{"reason": err.Error()}, "%s", err.Error())
	}
	return err
}

func slicesClone(ids []module.ID) []module.ID {
	if len(ids) == 0 {
		return nil
	}
	out := make([]module.ID, len(ids))
	copy(out, ids)
	return out
}
