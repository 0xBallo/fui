package cli

import (
	"strings"
	"testing"
)

func TestPayeeAddListAliasRename(t *testing.T) {
	deps := newTestDeps(t)

	mustRunCmd(t, deps, newPayeeAddCmd(deps), []string{"Amazon", "--alias", "AMZN Mktp", "--json"})

	buf := outBuf(deps)
	mustRunCmd(t, deps, newPayeeListCmd(deps), []string{"--json"})
	if !strings.Contains(buf.String(), `"name":"Amazon"`) || !strings.Contains(buf.String(), "AMZN Mktp") {
		t.Fatalf("payee list = %q, want Amazon with its alias", buf.String())
	}

	buf = outBuf(deps)
	mustRunCmd(t, deps, newPayeeAliasCmd(deps), []string{"Amazon", "Amazon.it", "--json"})
	if !strings.Contains(buf.String(), "AMZN Mktp") || !strings.Contains(buf.String(), "Amazon.it") {
		t.Fatalf("payee alias = %q, want both aliases kept", buf.String())
	}

	buf = outBuf(deps)
	mustRunCmd(t, deps, newPayeeAliasCmd(deps), []string{"Amazon", "AMZN Mktp", "--json"})
	if strings.Count(buf.String(), "AMZN Mktp") != 1 {
		t.Fatalf("payee alias (duplicate) = %q, want AMZN Mktp listed once", buf.String())
	}

	buf = outBuf(deps)
	mustRunCmd(t, deps, newPayeeRenameCmd(deps), []string{"Amazon", "Amazon EU", "--json"})
	if !strings.Contains(buf.String(), `"name":"Amazon EU"`) {
		t.Fatalf("payee rename = %q, want Amazon EU", buf.String())
	}
}

// TestPayeeGroupDispatch covers newPayeeCmd routing its subcommands (add/list), the group
// command that V32 will register under the root tree.
func TestPayeeGroupDispatch(t *testing.T) {
	deps := newTestDeps(t)
	mustRunCmd(t, deps, newPayeeCmd(deps), []string{"add", "Amazon"})

	buf := outBuf(deps)
	mustRunCmd(t, deps, newPayeeCmd(deps), []string{"list", "--json"})
	if !strings.Contains(buf.String(), `"name":"Amazon"`) {
		t.Fatalf("payee list via group = %q, want Amazon", buf.String())
	}
}

func TestPayeeAddDuplicateFails(t *testing.T) {
	deps := newTestDeps(t)
	mustRunCmd(t, deps, newPayeeAddCmd(deps), []string{"Amazon"})
	if err := runCmd(deps, newPayeeAddCmd(deps), []string{"amazon"}); err == nil {
		t.Fatal("expected an error adding a duplicate payee (case-insensitive)")
	}
}

// TestPayeeMerge covers that a payee merge keeps the source's name as an alias of the target and
// removes the source.
func TestPayeeMerge(t *testing.T) {
	deps := newTestDeps(t)
	mustRunCmd(t, deps, newPayeeAddCmd(deps), []string{"Amazon"})
	mustRunCmd(t, deps, newPayeeAddCmd(deps), []string{"Netflix"})

	buf := outBuf(deps)
	mustRunCmd(t, deps, newPayeeMergeCmd(deps), []string{"Amazon", "Netflix", "--yes", "--json"})
	if !strings.Contains(buf.String(), `"moved":0`) {
		t.Fatalf("payee merge = %q, want moved:0 (no transaction references Amazon)", buf.String())
	}

	buf = outBuf(deps)
	mustRunCmd(t, deps, newPayeeListCmd(deps), []string{"--json"})
	if strings.Contains(buf.String(), `"name":"Amazon"`) {
		t.Fatalf("payee list after merge = %q, want Amazon gone as a payee", buf.String())
	}
	if !strings.Contains(buf.String(), `"name":"Netflix"`) || !strings.Contains(buf.String(), `"aliases":["Amazon"]`) {
		t.Fatalf("payee list after merge = %q, want Netflix with Amazon kept as an alias", buf.String())
	}
}

func TestPayeeAliasTooFewArgsIsUsageError(t *testing.T) {
	deps := newTestDeps(t)
	mustRunCmd(t, deps, newPayeeAddCmd(deps), []string{"Amazon"})
	err := runCmd(deps, newPayeeAliasCmd(deps), []string{"Amazon"})
	if err == nil {
		t.Fatal("expected a usage error with no alias given")
	}
	if ExitCode(err) != ExitUserError {
		t.Fatalf("ExitCode = %d, want ExitUserError", ExitCode(err))
	}
}

func TestPayeeAliasUnknownPayeeFails(t *testing.T) {
	deps := newTestDeps(t)
	if err := runCmd(deps, newPayeeAliasCmd(deps), []string{"Ghost", "alias"}); err == nil {
		t.Fatal("expected a not_found error aliasing an unknown payee")
	}
}
