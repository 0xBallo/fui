package cli

import (
	"slices"
	"testing"

	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/module"
)

func TestConfigureModuleCommandsRejectsUnannotatedTopLevel(t *testing.T) {
	root := &cobra.Command{Use: "fui"}
	root.AddCommand(&cobra.Command{Use: "annotated"}, &cobra.Command{Use: "missing"})
	annotateModule(root.Commands()[0], module.Core)

	err := configureModuleCommands(root, nil)
	if err == nil {
		t.Fatal("expected missing-annotation error, got nil")
	}
}

func TestCommandModuleInheritsFromAncestor(t *testing.T) {
	root := &cobra.Command{Use: "fui"}
	parent := &cobra.Command{Use: "asset"}
	child := &cobra.Command{Use: "list"}
	parent.AddCommand(child)
	root.AddCommand(parent)
	annotateModule(parent, module.Assets)

	got, err := commandModule(child)
	if err != nil {
		t.Fatalf("commandModule(child): %v", err)
	}
	if got != module.Assets {
		t.Fatalf("commandModule(child) = %q, want %q", got, module.Assets)
	}
}

func TestConfigureModuleCommandsHidesInheritedDisabledSubcommands(t *testing.T) {
	root := &cobra.Command{Use: "fui"}
	coreCmd := &cobra.Command{Use: "corecmd"}
	assetCmd := &cobra.Command{Use: "asset"}
	assetChild := &cobra.Command{Use: "list"}
	assetCmd.AddCommand(assetChild)
	root.AddCommand(coreCmd, assetCmd)
	annotateModule(coreCmd, module.Core)
	annotateModule(assetCmd, module.Assets)

	if err := configureModuleCommands(root, nil); err != nil {
		t.Fatalf("configureModuleCommands: %v", err)
	}
	if coreCmd.Hidden {
		t.Fatal("core command should remain visible")
	}
	if !assetCmd.Hidden || !assetChild.Hidden {
		t.Fatalf("assets command tree should be hidden when assets module is disabled: parent=%v child=%v", assetCmd.Hidden, assetChild.Hidden)
	}
}

func TestValidateTopLevelModuleAnnotationsAcceptsKnownModules(t *testing.T) {
	root := &cobra.Command{Use: "fui"}
	a := &cobra.Command{Use: "a"}
	b := &cobra.Command{Use: "b"}
	root.AddCommand(a, b)
	annotateModule(a, module.Core)
	annotateModule(b, module.Assets)

	if err := validateTopLevelModuleAnnotations(root); err != nil {
		t.Fatalf("validateTopLevelModuleAnnotations: %v", err)
	}
}

func TestModuleIDsToStringsEmptyIsStableArray(t *testing.T) {
	got := moduleIDsToStrings(nil)
	if got == nil {
		t.Fatal("moduleIDsToStrings(nil) must return an empty slice, not nil")
	}
	if !slices.Equal(got, []string{}) {
		t.Fatalf("moduleIDsToStrings(nil) = %v, want []", got)
	}
}
