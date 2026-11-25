package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/importing"
	"github.com/0xballo/fui/internal/cli/render"
)

// newRestoreCmd replaces the archive with a backup file (.db) or a JSON dump (.json); it is always
// a bulk write (the plan sets Bulk), and the store takes a safety backup of the current archive first.
func newRestoreCmd(deps *Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "restore PATH",
		Short: "Restore the archive from a backup (.db) or a dump (.json)",
		Args:  exactArgs(1),
		Example: "  fui restore ~/.local/state/fui/backups/fui-2026-01-01.db --yes\n" +
			"  fui restore ./dump/dump.json --yes\n" +
			"  fui restore ./dump/dump.json --with-config --yes",
		RunE: func(cmd *cobra.Command, args []string) error {
			rt, err := runtimeOf(cmd, deps)
			if err != nil {
				return err
			}
			withConfig, _ := cmd.Flags().GetBool("with-config")
			if strings.EqualFold(filepath.Ext(args[0]), ".json") {
				return runRestoreDump(cmd, deps, rt, args[0], withConfig)
			}
			if withConfig {
				return newUsageError(errors.New("--with-config only applies to a .json dump"))
			}
			plan, err := deps.Service.PlanRestore(app.RestoreRequest{Path: args[0]})
			if err != nil {
				return err
			}
			return writeChange(cmd, deps, rt, plan.Plan, func() error {
				if err := deps.Service.ApplyRestore(cmd.Context(), plan); err != nil {
					return err
				}
				return writeRestored(deps, rt, plan.Path)
			})
		},
	}
	cmd.Flags().Bool("with-config", false, "also write the config.toml embedded in a .json dump (the current one is kept as config.toml.bak)")
	return cmd
}

func runRestoreDump(cmd *cobra.Command, deps *Deps, rt runtimeFlags, path string, withConfig bool) error {
	f, err := os.Open(path) // #nosec G304 -- path is the dump the user asked to restore.
	if err != nil {
		return fmt.Errorf("open dump: %w", err)
	}
	d, config, err := importing.DecodeDump(f)
	_ = f.Close() // read-only file: nothing to flush
	if err != nil {
		return err
	}
	plan, err := deps.Service.PlanRestoreDump(d, path)
	if err != nil {
		return err
	}
	return writeChange(cmd, deps, rt, plan.Plan, func() error {
		if err := deps.Service.ApplyRestoreDump(cmd.Context(), d); err != nil {
			return err
		}
		if withConfig && config != "" {
			if err := writeConfigText(deps, config); err != nil {
				return err
			}
		}
		return writeRestored(deps, rt, path)
	})
}

// writeConfigText replaces config.toml with text, first copying an existing file to config.toml.bak.
func writeConfigText(deps *Deps, text string) error {
	path := deps.Config.ConfigFile
	if path == "" {
		return errors.New("write config: no config file path")
	}
	old, err := os.ReadFile(path) // #nosec G304 -- the resolved config path.
	switch {
	case err == nil:
		if err := os.WriteFile(path+".bak", old, 0o600); err != nil { // #nosec G703 -- derived from the resolved config path.
			return fmt.Errorf("back up config: %w", err)
		}
	case !errors.Is(err, fs.ErrNotExist):
		return fmt.Errorf("read config file: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

func writeRestored(deps *Deps, rt runtimeFlags, path string) error {
	if rt.format == render.JSON {
		return render.WriteJSON(deps.Stdout, struct {
			Restored string `json:"restored"`
		}{path})
	}
	if _, err := fmt.Fprintf(deps.Stdout, "Restored from %s.\n", path); err != nil {
		return fmt.Errorf("write restore confirmation: %w", err)
	}
	return nil
}
