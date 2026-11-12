package cli_test

import (
	"context"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/cli"
	"github.com/0xballo/fui/internal/module"
)

func TestTopLevelCommandsHaveValidModuleAnnotation(t *testing.T) {
	h := newHarness(t)
	root := cli.NewRoot(h.deps(nil, nil))

	known := map[string]bool{}
	for _, def := range module.Definitions() {
		known[string(def.ID)] = true
	}

	for _, cmd := range root.Commands() {
		if strings.HasPrefix(cmd.Name(), "__complete") {
			continue
		}
		ann := cmd.Annotations["module"]
		if ann == "" {
			t.Fatalf("top-level command %q missing module annotation", cmd.CommandPath())
		}
		if !known[ann] {
			t.Fatalf("top-level command %q has unknown module annotation %q", cmd.CommandPath(), ann)
		}
	}
}

func TestUnannotatedTopLevelCommandFails(t *testing.T) {
	h := newHarness(t)
	root := cli.NewRoot(h.deps(nil, nil))
	root.AddCommand(&cobra.Command{
		Use:  "fake",
		Args: cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error { return nil },
	})
	root.SetArgs([]string{"fake"})
	root.SetIn(h.stdin)
	root.SetOut(h.deps(nil, nil).Stdout)
	root.SetErr(h.deps(nil, nil).Stderr)

	err := root.ExecuteContext(context.Background())
	if err == nil {
		t.Fatal("expected error for unannotated top-level command, got nil")
	}
	if !strings.Contains(err.Error(), "missing") || !strings.Contains(err.Error(), "module") {
		t.Fatalf("error = %v, want missing-module-annotation error", err)
	}
}
