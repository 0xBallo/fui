package cli

import (
	"errors"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/cli/render"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
)

// newModuleCmd groups module management commands.
func newModuleCmd(deps *Deps) *cobra.Command {
	cmd := &cobra.Command{Use: "module", Short: "List, enable or disable modules"}
	cmd.AddCommand(newModuleListCmd(deps), newModuleEnableCmd(deps), newModuleDisableCmd(deps))
	return cmd
}

func newModuleListCmd(deps *Deps) *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Short:   "List modules and their state",
		Args:    noArgs(),
		Example: "  fui module list --json",
		RunE: func(cmd *cobra.Command, _ []string) error {
			rt, err := runtimeOf(cmd, deps)
			if err != nil {
				return err
			}
			enabled, err := enabledModulesFromDeps(deps)
			if err != nil {
				return err
			}
			rows := listModuleRows(deps.Service.ListModules(cmd.Context(), enabled))
			if rt.format == render.JSON {
				return render.WriteJSON(deps.Stdout, rows)
			}
			return writeModuleRows(deps, rows)
		},
	}
}

func newModuleEnableCmd(deps *Deps) *cobra.Command {
	return &cobra.Command{
		Use:     "enable ID",
		Short:   "Enable a module (including required dependencies)",
		Args:    exactArgs(1),
		Example: "  fui module enable investments",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runModuleEnable(cmd, deps, module.ID(args[0]))
		},
	}
}

func newModuleDisableCmd(deps *Deps) *cobra.Command {
	return &cobra.Command{
		Use:     "disable ID",
		Short:   "Disable a module (and any enabled dependents)",
		Args:    exactArgs(1),
		Example: "  fui module disable recurring",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runModuleDisable(cmd, deps, module.ID(args[0]))
		},
	}
}

func runModuleEnable(cmd *cobra.Command, deps *Deps, id module.ID) error {
	rt, err := runtimeOf(cmd, deps)
	if err != nil {
		return err
	}
	enabled, err := enabledModulesFromDeps(deps)
	if err != nil {
		return err
	}
	plan, err := deps.Service.PlanEnableModule(cmd.Context(), enabled, id)
	if err != nil {
		return err
	}
	preview := func() error { return writeEnablePlan(deps, rt, plan) }
	return writeChangePreview(cmd, deps, rt, plan.Plan, preview, func() error {
		if err := deps.Service.ApplyEnableModule(cmd.Context(), deps.Config, plan); err != nil {
			return err
		}
		return writeModuleSetResult(deps, rt, plan.After)
	})
}

func runModuleDisable(cmd *cobra.Command, deps *Deps, id module.ID) error {
	rt, err := runtimeOf(cmd, deps)
	if err != nil {
		return err
	}
	enabled, err := enabledModulesFromDeps(deps)
	if err != nil {
		return err
	}
	plan, err := deps.Service.PlanDisableModule(cmd.Context(), enabled, id)
	if err != nil {
		return mapModuleError(err, id)
	}
	preview := func() error { return writeDisablePlan(deps, rt, plan) }
	return writeChangePreview(cmd, deps, rt, plan.Plan, preview, func() error {
		if err := deps.Service.ApplyDisableModule(cmd.Context(), deps.Config, plan); err != nil {
			return err
		}
		return writeModuleSetResult(deps, rt, plan.After)
	})
}

func mapModuleError(err error, id module.ID) error {
	var derr *domain.Error
	if errors.As(err, &derr) {
		return err
	}
	return &domain.Error{
		Code:    domain.CodeInvalidValue,
		Message: err.Error(),
		Details: map[string]any{"field": "module", "value": string(id)},
	}
}

type moduleListRow struct {
	ID      string   `json:"id"`
	Enabled bool     `json:"enabled"`
	Default bool     `json:"default"`
	Depends []string `json:"depends_on"`
	Presets []string `json:"presets"`
}

func listModuleRows(infos []app.ModuleInfo) []moduleListRow {
	rows := make([]moduleListRow, 0, len(infos))
	for _, info := range infos {
		row := moduleListRow{
			ID:      string(info.ID),
			Enabled: info.Enabled,
			Default: info.Default,
			Depends: moduleIDsToStrings(info.Deps),
		}
		if info.InExpense {
			row.Presets = append(row.Presets, "expense")
		}
		if info.InFull {
			row.Presets = append(row.Presets, "full")
		}
		rows = append(rows, row)
	}
	return rows
}

func writeModuleRows(deps *Deps, rows []moduleListRow) error {
	tw := tabwriter.NewWriter(deps.Stdout, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "ID\tENABLED\tDEFAULT\tDEPENDS ON\tPRESETS"); err != nil {
		return fmt.Errorf("write module table header: %w", err)
	}
	for _, row := range rows {
		if _, err := fmt.Fprintf(tw, "%s\t%t\t%t\t%s\t%s\n",
			row.ID, row.Enabled, row.Default, strings.Join(row.Depends, ","), strings.Join(row.Presets, ","),
		); err != nil {
			return fmt.Errorf("write module table row: %w", err)
		}
	}
	if err := tw.Flush(); err != nil {
		return fmt.Errorf("flush module table: %w", err)
	}
	return nil
}

func writeEnablePlan(deps *Deps, rt runtimeFlags, plan app.EnableModulePlan) error {
	if rt.format == render.JSON {
		return render.WriteJSON(deps.Stdout, struct {
			Target string   `json:"target"`
			Added  []string `json:"added"`
		}{
			Target: string(plan.Target),
			Added:  moduleIDsToStrings(plan.Added),
		})
	}
	if len(plan.Added) == 0 {
		_, err := fmt.Fprintf(deps.Stdout, "Module %q is already enabled.\n", plan.Target)
		if err != nil {
			return fmt.Errorf("write enable no-op message: %w", err)
		}
		return nil
	}
	if _, err := fmt.Fprintf(deps.Stdout, "Would enable modules: %s\n", strings.Join(moduleIDsToStrings(plan.Added), ", ")); err != nil {
		return fmt.Errorf("write enable plan: %w", err)
	}
	var depsAdded []string
	for _, id := range plan.Added {
		if id != plan.Target {
			depsAdded = append(depsAdded, string(id))
		}
	}
	if len(depsAdded) > 0 {
		if _, err := fmt.Fprintf(deps.Stdout, "Warning: enabling %q also enables dependencies: %s\n",
			plan.Target, strings.Join(depsAdded, ", ")); err != nil {
			return fmt.Errorf("write enable dependency warning: %w", err)
		}
	}
	return nil
}

func writeDisablePlan(deps *Deps, rt runtimeFlags, plan app.DisableModulePlan) error {
	if rt.format == render.JSON {
		return render.WriteJSON(deps.Stdout, struct {
			Target     string   `json:"target"`
			Dependents []string `json:"dependents_disabled,omitempty"`
		}{
			Target:     string(plan.Target),
			Dependents: moduleIDsToStrings(plan.DependentsDisabled),
		})
	}
	if !plan.Before.Has(plan.Target) {
		_, err := fmt.Fprintf(deps.Stdout, "Module %q is already disabled.\n", plan.Target)
		if err != nil {
			return fmt.Errorf("write disable no-op message: %w", err)
		}
		return nil
	}
	disabled := append(moduleIDsToStrings(plan.DependentsDisabled), string(plan.Target))
	if _, err := fmt.Fprintf(deps.Stdout, "Would disable modules: %s\n", strings.Join(disabled, ", ")); err != nil {
		return fmt.Errorf("write disable plan: %w", err)
	}
	if len(plan.DependentsDisabled) > 0 {
		if _, err := fmt.Fprintf(deps.Stdout, "Warning: active dependent modules will also be disabled: %s\n",
			strings.Join(moduleIDsToStrings(plan.DependentsDisabled), ", ")); err != nil {
			return fmt.Errorf("write disable dependents warning: %w", err)
		}
	}
	return nil
}

func writeModuleSetResult(deps *Deps, rt runtimeFlags, enabled module.Set) error {
	ids := moduleIDsToStrings(enabled.IDs())
	if rt.format == render.JSON {
		return render.WriteJSON(deps.Stdout, struct {
			Enabled []string `json:"enabled"`
		}{Enabled: ids})
	}
	_, err := fmt.Fprintf(deps.Stdout, "Enabled modules: %s\n", strings.Join(ids, ", "))
	if err != nil {
		return fmt.Errorf("write enabled modules result: %w", err)
	}
	return nil
}

func moduleIDsToStrings(ids []module.ID) []string {
	if len(ids) == 0 {
		return []string{}
	}
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = string(id)
	}
	return out
}
