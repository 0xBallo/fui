package cli

import (
	"strconv"
	"strings"
	"testing"
)

func TestTagAddListRenameRemove(t *testing.T) {
	deps := newTestDeps(t)

	mustRunCmd(t, deps, newTagAddCmd(deps), []string{"vacanze-2026"})

	buf := outBuf(deps)
	mustRunCmd(t, deps, newTagListCmd(deps), []string{"--json"})
	if !strings.Contains(buf.String(), `"name":"vacanze-2026"`) {
		t.Fatalf("tag list = %q, want vacanze-2026", buf.String())
	}

	buf = outBuf(deps)
	mustRunCmd(t, deps, newTagRenameCmd(deps), []string{"vacanze-2026", "ferie-2026", "--json"})
	if !strings.Contains(buf.String(), `"name":"ferie-2026"`) {
		t.Fatalf("tag rename = %q, want ferie-2026", buf.String())
	}

	buf = outBuf(deps)
	mustRunCmd(t, deps, newTagRemoveCmd(deps), []string{"ferie-2026", "--json"})
	if !strings.Contains(buf.String(), `"detached":0`) {
		t.Fatalf("tag rm = %q, want detached:0", buf.String())
	}

	buf = outBuf(deps)
	mustRunCmd(t, deps, newTagListCmd(deps), []string{"--json"})
	if strings.Contains(buf.String(), "ferie-2026") {
		t.Fatalf("tag list after rm = %q, want ferie-2026 gone", buf.String())
	}
}

// TestTagGroupDispatch covers newTagCmd routing its subcommands (add/list), the group command
// that V32 will register under the root tree.
func TestTagGroupDispatch(t *testing.T) {
	deps := newTestDeps(t)
	mustRunCmd(t, deps, newTagCmd(deps), []string{"add", "vacanze"})

	buf := outBuf(deps)
	mustRunCmd(t, deps, newTagCmd(deps), []string{"list", "--json"})
	if !strings.Contains(buf.String(), `"name":"vacanze"`) {
		t.Fatalf("tag list via group = %q, want vacanze", buf.String())
	}
}

func TestTagAddDuplicateFails(t *testing.T) {
	deps := newTestDeps(t)
	mustRunCmd(t, deps, newTagAddCmd(deps), []string{"vacanze"})
	if err := runCmd(deps, newTagAddCmd(deps), []string{"vacanze"}); err == nil {
		t.Fatal("expected an error adding a duplicate tag")
	}
}

func TestTagMerge(t *testing.T) {
	deps := newTestDeps(t)
	seedAccount(t, deps, "Checking")
	seedCategory(t, deps, "Food")
	tx := seedTx(t, deps, "Checking", "Food", "-10")
	mustRunCmd(t, deps, newTagAddCmd(deps), []string{"vacanze"})
	mustRunCmd(t, deps, newTagAddCmd(deps), []string{"ferie"})
	mustRunCmd(t, deps, newTxTagCmd(deps), []string{strconv.FormatInt(int64(tx.ID), 10), "vacanze"})

	buf := outBuf(deps)
	mustRunCmd(t, deps, newTagMergeCmd(deps), []string{"vacanze", "ferie", "--yes", "--json"})
	if !strings.Contains(buf.String(), `"moved":1`) {
		t.Fatalf("tag merge = %q, want moved:1", buf.String())
	}

	buf = outBuf(deps)
	mustRunCmd(t, deps, newTagListCmd(deps), []string{"--json"})
	if strings.Contains(buf.String(), "vacanze") || !strings.Contains(buf.String(), "ferie") {
		t.Fatalf("tag list after merge = %q, want only ferie", buf.String())
	}
}

func TestTxTagAndUntag(t *testing.T) {
	deps := newTestDeps(t)
	seedAccount(t, deps, "Checking")
	seedCategory(t, deps, "Food")
	tx := seedTx(t, deps, "Checking", "Food", "-10")
	mustRunCmd(t, deps, newTagAddCmd(deps), []string{"vacanze"})
	mustRunCmd(t, deps, newTagAddCmd(deps), []string{"ferie"})
	id := strconv.FormatInt(int64(tx.ID), 10)

	buf := outBuf(deps)
	mustRunCmd(t, deps, newTxTagCmd(deps), []string{id, "vacanze", "ferie", "--json"})
	if !strings.Contains(buf.String(), `"tags":["ferie","vacanze"]`) {
		t.Fatalf("tx tag = %q, want both tags sorted", buf.String())
	}

	buf = outBuf(deps)
	mustRunCmd(t, deps, newTxUntagCmd(deps), []string{id, "vacanze", "--json"})
	if !strings.Contains(buf.String(), `"tags":["ferie"]`) {
		t.Fatalf("tx untag = %q, want only ferie left", buf.String())
	}
}

func TestTxTagUnknownTagFails(t *testing.T) {
	deps := newTestDeps(t)
	seedAccount(t, deps, "Checking")
	seedCategory(t, deps, "Food")
	tx := seedTx(t, deps, "Checking", "Food", "-10")
	id := strconv.FormatInt(int64(tx.ID), 10)
	if err := runCmd(deps, newTxTagCmd(deps), []string{id, "nope"}); err == nil {
		t.Fatal("expected an error tagging with an unknown tag")
	}
}

func TestTxTagTooFewArgsIsUsageError(t *testing.T) {
	deps := newTestDeps(t)
	err := runCmd(deps, newTxTagCmd(deps), []string{"1"})
	if err == nil {
		t.Fatal("expected a usage error with no tags given")
	}
	if ExitCode(err) != ExitUserError {
		t.Fatalf("ExitCode = %d, want ExitUserError", ExitCode(err))
	}
}
