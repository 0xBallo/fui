package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/cli/render"
	"github.com/0xballo/fui/internal/skill"
)

// defaultSkillTargetDir is Claude Code's default skills directory, used when --target is empty.
const defaultSkillTargetDir = ".claude/skills"

// skillInstallResult is the --json shape of `fui skill install`.
type skillInstallResult struct {
	Path    string `json:"path"`
	DryRun  bool   `json:"dry_run"`
	Written bool   `json:"written"`
}

// newSkillInstallCmd writes the agent skill document as <target>/fui/SKILL.md, creating the
// directory (0700) and the file (0600) so only the user can read it. It needs neither the
// archive nor a terminal: it is a plain filesystem write, not an app.Plan/Apply change.
func newSkillInstallCmd(deps *Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "install",
		Short:   "Install the agent skill into a skills directory",
		Args:    noArgs(),
		Example: "  fui skill install --target ~/.claude/skills",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runSkillInstall(cmd, deps)
		},
	}
	cmd.Flags().String("target", "", "skills directory to install into (default: ~/.claude/skills)")
	return cmd
}

func runSkillInstall(cmd *cobra.Command, deps *Deps) error {
	rt, err := runtimeOf(cmd, deps)
	if err != nil {
		return err
	}
	target, _ := cmd.Flags().GetString("target")
	if target == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("resolve home directory: %w", err)
		}
		target = filepath.Join(home, defaultSkillTargetDir)
	}
	dir := filepath.Join(target, "fui")
	path := filepath.Join(dir, "SKILL.md")

	if !rt.dryRun {
		if err := writeSkillFile(dir, path, skill.Render(deps.Version)); err != nil {
			return err
		}
	}
	return renderSkillInstallResult(deps, rt, skillInstallResult{Path: path, DryRun: rt.dryRun, Written: !rt.dryRun})
}

// writeSkillFile creates dir (0700) and writes the skill document to path (0600).
func writeSkillFile(dir, path, content string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create skill directory %q: %w", dir, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return fmt.Errorf("write skill document %q: %w", path, err)
	}
	return nil
}

func renderSkillInstallResult(deps *Deps, rt runtimeFlags, res skillInstallResult) error {
	if rt.format == render.JSON {
		return render.WriteJSON(deps.Stdout, res)
	}
	verb := "Installed"
	if res.DryRun {
		verb = "Would install"
	}
	if _, err := fmt.Fprintf(deps.Stdout, "%s skill at %s.\n", verb, res.Path); err != nil {
		return fmt.Errorf("write skill install note: %w", err)
	}
	return nil
}
