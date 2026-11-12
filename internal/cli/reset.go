package cli

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/cli/render"
)

// resetConfirmWord is what a user must type on a terminal to confirm a reset.
const resetConfirmWord = "reset"

// newResetCmd brings the archive back to its initial state. Unlike other bulk writes, a terminal
// confirmation needs the typed word "reset" (not y/N); without a terminal, with --no-input or in
// agent mode it needs --yes. The config file is never touched.
func newResetCmd(deps *Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "reset",
		Short:   "Reset the archive to its initial state (a backup is taken first)",
		Args:    noArgs(),
		Example: "  fui reset --keep-settings --yes\n  fui reset --dry-run",
		RunE: func(cmd *cobra.Command, _ []string) error {
			rt, err := runtimeOf(cmd, deps)
			if err != nil {
				return err
			}
			keep, _ := cmd.Flags().GetBool("keep-settings")
			plan, err := deps.Service.PlanReset(cmd.Context(), app.ResetRequest{KeepSettings: keep})
			if err != nil {
				return err
			}
			preview := func() error { return render.WritePlan(deps.Stdout, rt.format, plan.Plan) }
			apply := func() error { return applyReset(cmd, deps, rt, plan) }
			if plan.Init != nil {
				// No archive yet: this is init, which is not a bulk change.
				return writeChangePreview(cmd, deps, rt, plan.Plan, preview, apply)
			}
			if rt.dryRun {
				return preview()
			}
			if !rt.yes {
				if rt.agent {
					return confirmationRequired("FUI_AGENT=1 requires --yes to write")
				}
				ok, err := confirmReset(deps, rt, preview)
				if err != nil {
					return err
				}
				if !ok {
					_, err := fmt.Fprintln(deps.Stdout, "Cancelled: nothing was changed.")
					if err != nil {
						return fmt.Errorf("write cancellation note: %w", err)
					}
					return nil
				}
			}
			return apply()
		},
	}
	cmd.Flags().Bool("keep-settings", false, "keep categories and other settings; delete everything else")
	return cmd
}

func applyReset(cmd *cobra.Command, deps *Deps, rt runtimeFlags, plan app.ResetPlan) error {
	res, err := deps.Service.ApplyReset(cmd.Context(), plan)
	if err != nil {
		return err
	}
	if rt.format == render.JSON {
		return render.WriteJSON(deps.Stdout, struct {
			Reset        bool `json:"reset"`
			KeepSettings bool `json:"keep_settings"`
			Initialized  bool `json:"initialized"`
		}{!res.Initialized, plan.KeepSettings, res.Initialized})
	}
	msg := "Archive reset."
	if res.Initialized {
		msg = fmt.Sprintf("No archive found: created %d categories.", res.Created)
	}
	if _, err := fmt.Fprintln(deps.Stdout, msg); err != nil {
		return fmt.Errorf("write reset confirmation: %w", err)
	}
	return nil
}

// confirmReset shows the plan and asks the user to type the confirmation word; it fails with
// confirmation_required instead of asking when there is no terminal or --no-input was given.
func confirmReset(deps *Deps, rt runtimeFlags, preview func() error) (bool, error) {
	if !deps.IsTerminal || rt.noInput {
		return false, confirmationRequired("reset needs --yes without a terminal or with --no-input")
	}
	if err := preview(); err != nil {
		return false, err
	}
	var typed string
	form := huh.NewForm(huh.NewGroup(
		huh.NewInput().Title(fmt.Sprintf("Type %q to confirm", resetConfirmWord)).Value(&typed),
	)).WithAccessible(true).WithInput(deps.Stdin).WithOutput(deps.Stdout)
	if err := form.Run(); err != nil {
		return false, fmt.Errorf("confirm reset: %w", err)
	}
	return strings.TrimSpace(typed) == resetConfirmWord, nil
}
