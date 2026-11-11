package cli

import (
	"fmt"

	"github.com/charmbracelet/huh"

	"github.com/0xballo/fui/internal/domain"
)

// promptString returns value when it is not empty. Otherwise, on a terminal without --no-input
// (and outside agent mode) it asks for label; everywhere else a missing value fails with
// missing_field, exactly as the technical spec requires.
func promptString(deps *Deps, rt runtimeFlags, field, label, value string) (string, error) {
	if value != "" {
		return value, nil
	}
	if !deps.IsTerminal || rt.noInput || rt.agent {
		return "", missingFieldError(field)
	}
	var out string
	form := huh.NewForm(huh.NewGroup(huh.NewInput().Title(label).Value(&out))).
		WithAccessible(true).WithInput(deps.Stdin).WithOutput(deps.Stdout)
	if err := form.Run(); err != nil {
		return "", fmt.Errorf("prompt %s: %w", field, err)
	}
	if out == "" {
		return "", missingFieldError(field)
	}
	return out, nil
}

func missingFieldError(field string) error {
	return &domain.Error{
		Code: domain.ErrMissingField.Code, Message: fmt.Sprintf("missing field %q", field),
		Details: map[string]any{"field": field},
	}
}
