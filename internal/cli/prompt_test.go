package cli

import "testing"

func TestPromptSelectWithoutTerminalFailsMissingField(t *testing.T) {
	deps := &Deps{IsTerminal: false}
	rt := runtimeFlags{}
	value := ""
	err := promptSelect(deps, rt, "preset", "Choose", nil, &value)
	if err == nil {
		t.Fatal("expected promptSelect to fail without terminal")
	}
}

func TestPromptMultiSelectWithoutTerminalFailsMissingField(t *testing.T) {
	deps := &Deps{IsTerminal: false}
	rt := runtimeFlags{}
	values := []string{}
	err := promptMultiSelect(deps, rt, "modules", "Choose", nil, &values)
	if err == nil {
		t.Fatal("expected promptMultiSelect to fail without terminal")
	}
}
