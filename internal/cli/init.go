package cli

import (
	"fmt"
	"slices"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/cli/render"
	"github.com/0xballo/fui/internal/module"
)

// newInitCmd creates the default categories FUI needs before the first transaction; running it
// again is harmless. --currency only sets the base currency in config.toml on success (it is not
// part of the plan, so --dry-run never changes it).
func newInitCmd(deps *Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "init",
		Short:   "Create the default categories",
		Example: "  fui init --currency EUR --preset expense\n  fui init --preset custom --modules core,budget,recurring",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runInit(cmd, deps)
		},
	}
	cmd.Flags().String("currency", "", "set the base currency (ISO 4217), e.g. EUR")
	cmd.Flags().String("preset", "", "module preset: expense|full|custom")
	cmd.Flags().String("modules", "", "comma-separated modules to enable (implies --preset custom)")
	return cmd
}

func runInit(cmd *cobra.Command, deps *Deps) error {
	rt, err := runtimeOf(cmd, deps)
	if err != nil {
		return err
	}
	ctx := cmd.Context()
	plan, err := deps.Service.PlanInit(ctx)
	if err != nil {
		return err
	}
	modules, err := resolveInitModules(cmd, deps, rt)
	if err != nil {
		return err
	}
	currency, _ := cmd.Flags().GetString("currency")
	return writeChange(cmd, deps, rt, plan.Plan, func() error {
		res, err := deps.Service.ApplyInit(ctx, plan)
		if err != nil {
			return err
		}
		if err := deps.Config.SetModulesEnabled(ctx, modules.IDs()); err != nil {
			return err
		}
		if currency != "" {
			if err := deps.Config.Set("currency", currency); err != nil {
				return err
			}
		}
		if rt.format == render.JSON {
			return render.WriteJSON(deps.Stdout, struct {
				Created int `json:"created"`
			}{res.Created})
		}
		_, err = fmt.Fprintf(deps.Stdout, "Created %d categories.\n", res.Created)
		if err != nil {
			return fmt.Errorf("write created-count note: %w", err)
		}
		return nil
	})
}

func resolveInitModules(cmd *cobra.Command, deps *Deps, rt runtimeFlags) (module.Set, error) {
	preset, modulesRaw := initModuleFlags(cmd)
	prompted := preset == ""
	var err error
	if preset == "" {
		preset, err = chooseInitPreset(deps, rt)
		if err != nil {
			return module.Set{}, err
		}
		if preset == "" {
			return module.Default(), nil
		}
	}
	if err := validateInitModuleFlagCombination(preset, modulesRaw, prompted); err != nil {
		return module.Set{}, err
	}
	if preset == "custom" {
		if modulesRaw == "" && prompted {
			return chooseInitCustomModules(deps, rt)
		}
		return parseCustomModules(modulesRaw)
	}
	set, err := module.Preset(preset)
	if err == nil {
		return set, nil
	}
	return module.Set{}, newUsageError(fmt.Errorf("invalid --preset %q (want expense, full, or custom)", preset))
}

func initModuleFlags(cmd *cobra.Command) (preset, modulesRaw string) {
	preset, _ = cmd.Flags().GetString("preset")
	modulesRaw, _ = cmd.Flags().GetString("modules")
	preset = strings.ToLower(strings.TrimSpace(preset))
	modulesRaw = strings.TrimSpace(modulesRaw)
	if modulesRaw != "" && preset == "" {
		preset = "custom"
	}
	return preset, modulesRaw
}

func chooseInitPreset(deps *Deps, rt runtimeFlags) (string, error) {
	if !deps.IsTerminal || rt.noInput || rt.agent {
		return "", nil
	}
	preset := "expense"
	err := promptSelect(deps, rt, "preset", "Choose module profile", []huh.Option[string]{
		huh.NewOption("Expense tracker", "expense"),
		huh.NewOption("Full", "full"),
		huh.NewOption("Custom", "custom"),
	}, &preset)
	return preset, err
}

func validateInitModuleFlagCombination(preset, modulesRaw string, prompted bool) error {
	if modulesRaw != "" && preset != "custom" {
		return newUsageError(fmt.Errorf("--modules requires --preset custom"))
	}
	if preset == "custom" && modulesRaw == "" && !prompted {
		return newUsageError(fmt.Errorf("--preset custom requires --modules"))
	}
	return nil
}

func parseCustomModules(modulesRaw string) (module.Set, error) {
	ids := strings.Split(modulesRaw, ",")
	parsed := make([]module.ID, 0, len(ids))
	for _, raw := range ids {
		id := module.ID(strings.TrimSpace(raw))
		if id == "" {
			return module.Set{}, newUsageError(fmt.Errorf("invalid --modules: empty module id"))
		}
		parsed = append(parsed, id)
	}
	return module.Validate(parsed)
}

func chooseInitCustomModules(deps *Deps, rt runtimeFlags) (module.Set, error) {
	defs := module.Definitions()
	options := make([]huh.Option[string], 0, len(defs))
	for _, def := range defs {
		if def.ID == module.Core {
			continue
		}
		options = append(options, huh.NewOption(string(def.ID), string(def.ID)))
	}
	selected := []string{}
	if err := promptMultiSelect(
		deps, rt, "modules", "Choose additional modules (core is always enabled)", options, &selected,
	); err != nil {
		return module.Set{}, err
	}
	custom, addedByDeps, err := closeCustomSelection(selected)
	if err != nil {
		return module.Set{}, err
	}
	if len(addedByDeps) > 0 {
		_, werr := fmt.Fprintf(deps.Stdout, "Warning: dependencies were enabled automatically: %s\n", strings.Join(addedByDeps, ", "))
		if werr != nil {
			return module.Set{}, fmt.Errorf("write dependency warning: %w", werr)
		}
	}
	return custom, nil
}

func closeCustomSelection(selected []string) (module.Set, []string, error) {
	set, err := module.Validate([]module.ID{module.Core})
	if err != nil {
		return module.Set{}, nil, err
	}
	explicit := map[module.ID]bool{}
	addedByDeps := map[module.ID]bool{}
	for _, raw := range selected {
		id := module.ID(strings.TrimSpace(raw))
		if id == "" {
			continue
		}
		beforeHas := set.Has(id)
		explicit[id] = true
		next, added, err := module.Enable(set, id)
		if err != nil {
			return module.Set{}, nil, err
		}
		set = next
		if !beforeHas {
			for _, mid := range added {
				if mid != id && !explicit[mid] {
					addedByDeps[mid] = true
				}
			}
		}
	}
	warnings := make([]string, 0, len(addedByDeps))
	for id := range addedByDeps {
		warnings = append(warnings, string(id))
	}
	slices.Sort(warnings)
	return set, warnings, nil
}
