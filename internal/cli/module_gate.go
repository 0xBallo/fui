package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
)

const moduleAnnotationKey = "module"

func annotateModule(cmd *cobra.Command, id module.ID) {
	if cmd.Annotations == nil {
		cmd.Annotations = map[string]string{}
	}
	cmd.Annotations[moduleAnnotationKey] = string(id)
}

func configureModuleCommands(root *cobra.Command, deps *Deps) error {
	if err := validateTopLevelModuleAnnotations(root); err != nil {
		return err
	}

	enabled, err := enabledModulesFromDeps(deps)
	if err != nil {
		return err
	}
	return walkCommands(root, func(cmd *cobra.Command) error {
		if cmd == root {
			return nil
		}
		id, err := commandModule(cmd)
		if err != nil {
			return err
		}
		if !enabled.Has(id) {
			cmd.Hidden = true
		}
		return nil
	})
}

func moduleGate(deps *Deps) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, _ []string) error {
		if cmd == cmd.Root() {
			return nil
		}
		enabled, err := enabledModulesFromDeps(deps)
		if err != nil {
			return err
		}
		id, err := commandModule(cmd)
		if err != nil {
			return err
		}
		if enabled.Has(id) {
			return nil
		}
		return &domain.Error{
			Code:    app.CodeModuleDisabled,
			Message: fmt.Sprintf("module %q is disabled; run fui module enable %s", id, id),
			Details: map[string]any{"module": id, "command": cmd.CommandPath()},
		}
	}
}

func enabledModulesFromDeps(deps *Deps) (module.Set, error) {
	if deps == nil || deps.Config == nil {
		return module.Default(), nil
	}
	enabled, err := deps.Config.ModulesEnabled()
	if err != nil {
		return module.Set{}, fmt.Errorf("resolve modules.enabled: %w", err)
	}
	return enabled, nil
}

func commandModule(cmd *cobra.Command) (module.ID, error) {
	for current := cmd; current != nil; current = current.Parent() {
		raw := strings.TrimSpace(current.Annotations[moduleAnnotationKey])
		if raw == "" {
			continue
		}
		id := module.ID(raw)
		for _, def := range module.Definitions() {
			if def.ID == id {
				return id, nil
			}
		}
		return "", fmt.Errorf("command %q has unknown module annotation %q", current.CommandPath(), raw)
	}
	return "", fmt.Errorf("command %q is missing %q annotation", cmd.CommandPath(), moduleAnnotationKey)
}

func validateTopLevelModuleAnnotations(root *cobra.Command) error {
	for _, cmd := range root.Commands() {
		if isCobraInternalCommand(cmd) {
			continue
		}
		ann := strings.TrimSpace(cmd.Annotations[moduleAnnotationKey])
		if ann == "" {
			return fmt.Errorf("top-level command %q is missing %q annotation", cmd.CommandPath(), moduleAnnotationKey)
		}
		if _, err := commandModule(cmd); err != nil {
			return err
		}
	}
	return nil
}

func isCobraInternalCommand(cmd *cobra.Command) bool {
	return strings.HasPrefix(cmd.Name(), "__complete")
}

func walkCommands(root *cobra.Command, visit func(*cobra.Command) error) error {
	var walk func(*cobra.Command) error
	walk = func(cmd *cobra.Command) error {
		if err := visit(cmd); err != nil {
			return err
		}
		for _, child := range cmd.Commands() {
			if err := walk(child); err != nil {
				return err
			}
		}
		return nil
	}
	return walk(root)
}
