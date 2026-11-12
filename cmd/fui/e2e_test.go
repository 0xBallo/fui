package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"
)

// TestMain lets testscript re-exec this test binary as the real "fui" command for every e2e
// script in testdata/e2e: each `fui ...` line in a script runs in its own process through the
// same run() the composition root's main() calls, with the normal argv/stdio wiring.
func TestMain(m *testing.M) {
	testscript.Main(m, map[string]func(){
		"fui": func() {
			os.Exit(run(context.Background(), os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
		},
	})
}

// TestE2E runs every .txtar script in testdata/e2e (repo root) against the real fui binary: at
// least one script per MVP use case (docs/specs/funzionali.md "Casi d'uso"), on a fresh archive
// per script, with a fixed clock and sandboxed XDG directories for determinism.
func TestE2E(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir:                 filepath.Join("..", "..", "testdata", "e2e"),
		RequireExplicitExec: true,
		Setup: func(env *testscript.Env) error {
			env.Setenv("XDG_CONFIG_HOME", filepath.Join(env.WorkDir, "xdg", "config"))
			env.Setenv("XDG_DATA_HOME", filepath.Join(env.WorkDir, "xdg", "data"))
			env.Setenv("XDG_STATE_HOME", filepath.Join(env.WorkDir, "xdg", "state"))
			env.Setenv("FUI_TEST_TODAY", "2026-01-15")
			// Forward the outer TZ, so CI's UTC/Pacific-Kiritimati matrix also exercises the
			// scripts; fall back to UTC so a developer's local zone cannot make them flaky.
			tz := os.Getenv("TZ")
			if tz == "" {
				tz = "UTC"
			}
			env.Setenv("TZ", tz)
			return nil
		},
	})
}
