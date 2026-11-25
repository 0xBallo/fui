package main

import (
	"path/filepath"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"
)

func TestSkipsCatchup(t *testing.T) {
	tests := []struct {
		args []string
		want bool
	}{
		{[]string{"help"}, true},
		{[]string{"completion", "bash"}, true},
		{[]string{"__complete", "tx", ""}, true},
		{[]string{"init"}, true},
		{[]string{"reset"}, true},
		{[]string{"restore"}, true},
		{[]string{"backup", "create"}, true},
		{[]string{"version"}, true},
		{[]string{"config", "show"}, true},
		{[]string{"skill", "show"}, true},
		{[]string{"--help"}, true},
		{[]string{"-h"}, true},
		{[]string{"--version"}, true},
		{[]string{"tx", "list", "--dry-run"}, true},
		{[]string{"--db", "x.db", "init"}, true},
		{[]string{"tx", "list"}, false},
		{[]string{"--db", "x.db", "account", "list"}, false},
		{nil, false}, // bare invocation starts the TUI
	}
	for _, tt := range tests {
		if got := SkipsCatchup(tt.args); got != tt.want {
			t.Errorf("SkipsCatchup(%q) = %v, want %v", tt.args, got, tt.want)
		}
	}
}

// TestCatchupE2E runs the startup catch-up scripts in testdata (cmd/fui): a fixed clock, a fresh
// archive and sandboxed XDG directories per script.
func TestCatchupE2E(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir:                 "testdata",
		RequireExplicitExec: true,
		Setup: func(env *testscript.Env) error {
			env.Setenv("XDG_CONFIG_HOME", filepath.Join(env.WorkDir, "xdg", "config"))
			env.Setenv("XDG_DATA_HOME", filepath.Join(env.WorkDir, "xdg", "data"))
			env.Setenv("XDG_STATE_HOME", filepath.Join(env.WorkDir, "xdg", "state"))
			env.Setenv("FUI_TEST_TODAY", "2026-01-15")
			env.Setenv("TZ", "UTC")
			return nil
		},
	})
}
