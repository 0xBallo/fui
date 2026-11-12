package main

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// sandbox points HOME and every XDG_*_HOME variable at fresh subdirectories of t.TempDir(), so
// run() never touches the real user environment (see AGENTS.md: no paths outside the worktree).
func sandbox(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(dir, "data"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(dir, "state"))
	t.Setenv("FUI_TEST_TODAY", "2026-01-15")
}

// runFUI runs the composition root for args against the sandboxed environment and returns the
// exit code, stdout and stderr.
func runFUI(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code = run(context.Background(), args, strings.NewReader(""), &out, &errOut)
	return code, out.String(), errOut.String()
}

// TestRunEndToEnd exercises the composition root the way a user would: init, add an account and
// a transaction, then read them back through reporting and account-list. A failure here means
// the wiring in bootstrap/wireStore/newReporting is broken, not any one package in isolation.
func TestRunEndToEnd(t *testing.T) {
	sandbox(t)

	if code, _, stderr := runFUI(t, "init", "--currency", "EUR"); code != 0 {
		t.Fatalf("init: code=%d stderr=%s", code, stderr)
	}
	if code, _, stderr := runFUI(t, "account", "add", "Checking", "--kind", "checking", "--currency", "EUR"); code != 0 {
		t.Fatalf("account add: code=%d stderr=%s", code, stderr)
	}
	if code, _, stderr := runFUI(t, "add", "-12,50", "Esselunga", "-c", "Groceries", "-a", "Checking"); code != 0 {
		t.Fatalf("add: code=%d stderr=%s", code, stderr)
	}

	code, stdout, stderr := runFUI(t, "account", "list", "--json")
	if code != 0 {
		t.Fatalf("account list: code=%d stderr=%s", code, stderr)
	}
	var accounts []struct {
		Name    string `json:"name"`
		Balance int64  `json:"balance"`
	}
	if err := json.Unmarshal([]byte(stdout), &accounts); err != nil {
		t.Fatalf("decode account list %q: %v", stdout, err)
	}
	if len(accounts) != 1 || accounts[0].Name != "Checking" || accounts[0].Balance != -1250 {
		t.Fatalf("account list = %+v, want one Checking account with balance -1250", accounts)
	}

	if code, stdout, stderr := runFUI(t, "report", "networth", "--period", "2026-01", "--json"); code != 0 {
		t.Fatalf("report networth: code=%d stderr=%s stdout=%s", code, stderr, stdout)
	}
}

// TestRunStoreFreeCommandsNeedNoArchive asserts that help, --version, completion, config and
// skill show all succeed without FUI_TEST_TODAY or any writable XDG directory: they must not
// open the store (requiresStore's contract).
func TestRunStoreFreeCommandsNeedNoArchive(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	// A non-writable, nonexistent data directory: any attempt to open the archive fails loudly.
	t.Setenv("XDG_DATA_HOME", filepath.Join(dir, "does", "not", "exist"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "does", "not", "exist"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(dir, "does", "not", "exist"))

	for _, args := range [][]string{
		{"--help"},
		{"help", "--json"},
		{"--version"},
		{"completion", "bash"},
		{"config", "get", "currency"},
		{"skill", "show"},
	} {
		if code, _, stderr := runFUI(t, args...); code != 0 {
			t.Errorf("run(%v): code=%d stderr=%s", args, code, stderr)
		}
	}
}

// TestRunUnknownCommandExitsNonZero is a smoke check that errors are rendered and mapped to a
// non-zero exit code end-to-end, not just within internal/cli's own tests.
func TestRunUnknownCommandExitsNonZero(t *testing.T) {
	sandbox(t)
	if code, _, stderr := runFUI(t, "not-a-real-command"); code == 0 || stderr == "" {
		t.Fatalf("run(not-a-real-command): code=%d stderr=%q, want nonzero code and a message", code, stderr)
	}
}

func TestResolveClock(t *testing.T) {
	tests := []struct {
		name    string
		env     string
		wantNow string
		wantErr bool
	}{
		{name: "unset uses system clock", env: ""},
		{name: "RFC3339", env: "2026-03-01T10:00:00Z", wantNow: "2026-03-01T10:00:00Z"},
		{name: "bare date", env: "2026-03-01", wantNow: "2026-03-01T00:00:00Z"},
		{name: "invalid", env: "not-a-date", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clk, err := resolveClock(func(string) string { return tt.env })
			if tt.wantErr {
				if err == nil {
					t.Fatalf("resolveClock(%q) = nil error, want one", tt.env)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveClock(%q): %v", tt.env, err)
			}
			if tt.wantNow != "" {
				want, _ := time.Parse(time.RFC3339, tt.wantNow)
				if !clk.Now().Equal(want) {
					t.Errorf("resolveClock(%q).Now() = %v, want %v", tt.env, clk.Now(), want)
				}
			}
		})
	}
}

func TestRequiresStore(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want bool
	}{
		{name: "bare invocation launches the TUI, needs the store", args: nil, want: true},
		{name: "help", args: []string{"help"}, want: false},
		{name: "help json", args: []string{"help", "--json"}, want: false},
		{name: "completion", args: []string{"completion", "bash"}, want: false},
		{name: "config get", args: []string{"config", "get", "currency"}, want: false},
		{name: "skill show", args: []string{"skill", "show"}, want: false},
		{name: "skill install", args: []string{"skill", "install"}, want: false},
		{name: "global help flag before verb", args: []string{"--help", "account", "list"}, want: false},
		{name: "global help flag after verb", args: []string{"account", "list", "-h"}, want: false},
		{name: "global version flag", args: []string{"--version"}, want: false},
		{name: "account list needs the store", args: []string{"account", "list"}, want: true},
		{name: "db flag before verb is skipped", args: []string{"--db", "config", "account", "list"}, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := requiresStore(tt.args); got != tt.want {
				t.Errorf("requiresStore(%v) = %v, want %v", tt.args, got, tt.want)
			}
		})
	}
}

func TestPrescanGlobalFlags(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantDB      string
		wantVerbose bool
	}{
		{name: "none", args: []string{"account", "list"}},
		{name: "db space-separated", args: []string{"--db", "/tmp/a.db", "account", "list"}, wantDB: "/tmp/a.db"},
		{name: "db equals form", args: []string{"--db=/tmp/a.db"}, wantDB: "/tmp/a.db"},
		{name: "verbose long", args: []string{"--verbose"}, wantVerbose: true},
		{name: "verbose short", args: []string{"-v"}, wantVerbose: true},
		{name: "negative amount is not a flag", args: []string{"add", "--", "-12,50", "Desc"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, verbose := prescanGlobalFlags(tt.args)
			if db != tt.wantDB || verbose != tt.wantVerbose {
				t.Errorf("prescanGlobalFlags(%v) = (%q, %v), want (%q, %v)", tt.args, db, verbose, tt.wantDB, tt.wantVerbose)
			}
		})
	}
}

func TestEarlyJSON(t *testing.T) {
	t.Run("json flag", func(t *testing.T) {
		if !earlyJSON([]string{"account", "list", "--json"}) {
			t.Error("earlyJSON with --json = false, want true")
		}
	})
	t.Run("agent mode env", func(t *testing.T) {
		t.Setenv("FUI_AGENT", "1")
		if !earlyJSON([]string{"account", "list"}) {
			t.Error("earlyJSON with FUI_AGENT=1 = false, want true")
		}
	})
	t.Run("neither", func(t *testing.T) {
		t.Setenv("FUI_AGENT", "")
		if earlyJSON([]string{"account", "list"}) {
			t.Error("earlyJSON with neither = true, want false")
		}
	})
}
