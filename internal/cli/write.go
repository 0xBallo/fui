package cli

import (
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/cli/render"
	"github.com/0xballo/fui/internal/domain"
)

// confirmationRequired builds the error a write returns when it cannot ask for confirmation
// (agent mode, --no-input, or no terminal) and the caller did not pass --yes.
func confirmationRequired(reason string) error {
	return &domain.Error{Code: app.CodeConfirmationRequired, Message: "confirmation required: " + reason}
}

// writeChange runs the Plan/Apply pattern shared by every write command: --dry-run renders the
// plan and returns without applying; a bulk plan is applied only with --yes or, on a terminal,
// after an interactive confirmation; agent mode (FUI_AGENT=1) always requires --yes. apply is
// called only once the plan is accepted, and renders its own result.
func writeChange(cmd *cobra.Command, deps *Deps, rt runtimeFlags, plan app.Plan, apply func() error) error {
	if rt.dryRun {
		return render.WritePlan(deps.Stdout, rt.format, plan)
	}
	if rt.agent && !rt.yes {
		return confirmationRequired("FUI_AGENT=1 requires --yes to write")
	}
	if plan.Bulk && !rt.yes {
		ok, err := confirmBulk(cmd, deps, rt, plan)
		if err != nil {
			return err
		}
		if !ok {
			if _, ferr := fmt.Fprintln(deps.Stdout, "Cancelled: nothing was changed."); ferr != nil {
				return fmt.Errorf("write cancellation note: %w", ferr)
			}
			return nil
		}
	}
	return apply()
}

// confirmBulk shows the plan and asks for confirmation; it fails with confirmation_required
// instead of asking when there is no terminal or --no-input was given.
func confirmBulk(_ *cobra.Command, deps *Deps, rt runtimeFlags, plan app.Plan) (bool, error) {
	if !deps.IsTerminal || rt.noInput {
		return false, confirmationRequired("a bulk change needs --yes without a terminal or with --no-input")
	}
	if err := render.WritePlan(deps.Stdout, rt.format, plan); err != nil {
		return false, err
	}
	var ok bool
	form := huh.NewForm(huh.NewGroup(
		huh.NewConfirm().Title("Apply this bulk change?").Value(&ok),
	)).WithAccessible(true).WithInput(deps.Stdin).WithOutput(deps.Stdout)
	if err := form.Run(); err != nil {
		return false, fmt.Errorf("confirm bulk change: %w", err)
	}
	return ok, nil
}
