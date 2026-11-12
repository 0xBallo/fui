// Package settings implements the TUI screen that manages enabled modules and the active theme.
package settings

import (
	"context"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/module"
)

// UseCases is the subset of app.Service this screen needs, received by interface so tests can
// supply a stub store-free implementation.
type UseCases interface {
	ListModules(ctx context.Context, enabled module.Set) []app.ModuleInfo
	PlanEnableModule(ctx context.Context, enabled module.Set, id module.ID) (app.EnableModulePlan, error)
	ApplyEnableModule(ctx context.Context, writer app.ModuleConfigWriter, p app.EnableModulePlan) error
	PlanDisableModule(ctx context.Context, enabled module.Set, id module.ID) (app.DisableModulePlan, error)
	ApplyDisableModule(ctx context.Context, writer app.ModuleConfigWriter, p app.DisableModulePlan) error
}

// ModuleConfig is the narrow configuration port the screen needs to read and persist the enabled
// module set; *config.Config satisfies it, without this package importing internal/config.
type ModuleConfig interface {
	ModulesEnabled() (module.Set, error)
	app.ModuleConfigWriter
}

// ThemeConfig is the narrow configuration port the screen needs to read and persist the active
// theme; *config.Config satisfies it, without this package importing internal/config.
type ThemeConfig interface {
	Get(key string) (string, error)
	Set(key, value string) error
}
