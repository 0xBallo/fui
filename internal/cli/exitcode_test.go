package cli_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/importing"
	"github.com/0xballo/fui/internal/cli"
	"github.com/0xballo/fui/internal/domain"
)

// wantExit is the expected cli.ExitCode for every code domain.Codes, app.Codes and
// importing.Codes can return. It is deliberately exhaustive and hand-written (not derived from
// cli.ExitCode itself): TestExitCodeExhaustive fails any code missing from this map, so a new
// code added to one of those packages forces a classification decision here too.
var wantExit = map[domain.Code]int{
	domain.CodeAmbiguousName:       cli.ExitUserError,
	domain.CodeNotFound:            cli.ExitUserError,
	domain.CodeCurrencyMismatch:    cli.ExitUserError,
	domain.CodeReconciled:          cli.ExitUserError,
	domain.CodeInvalidFilter:       cli.ExitUserError,
	domain.CodeOverflow:            cli.ExitDataError,
	domain.CodeInvalidAmount:       cli.ExitUserError,
	domain.CodeInvalidDate:         cli.ExitUserError,
	domain.CodeMissingField:        cli.ExitUserError,
	domain.CodeInvalidValue:        cli.ExitUserError,
	domain.CodeNotEnoughData:       cli.ExitUserError,
	app.CodeConfirmationRequired:   cli.ExitUserError,
	app.CodeRateMissing:            cli.ExitUserError,
	app.CodeAlreadyExists:          cli.ExitUserError,
	app.CodeInUse:                  cli.ExitUserError,
	importing.CodeUnmappedAccounts: cli.ExitUserError,
	importing.CodeInvalidStream:    cli.ExitDataError,
	importing.CodeConnectorFailed:  cli.ExitDataError,
}

// TestExitCodeExhaustive enumerates every code domain.Codes, app.Codes and importing.Codes
// return and checks cli.ExitCode against wantExit, failing loudly on any code wantExit does not
// cover, so a future code added to one of those packages cannot silently fall through to the
// wrong exit code.
func TestExitCodeExhaustive(t *testing.T) {
	all := append(append(append([]domain.Code{}, domain.Codes()...), app.Codes()...), importing.Codes()...)
	if len(all) == 0 {
		t.Fatal("no known codes to enumerate")
	}
	seen := make(map[domain.Code]bool, len(all))
	for _, code := range all {
		seen[code] = true
		want, ok := wantExit[code]
		if !ok {
			t.Fatalf("code %q has no expected exit code in wantExit; classify it in exitcode.go and wantExit", code)
		}
		err := &domain.Error{Code: code, Message: "boom"}
		if got := cli.ExitCode(err); got != want {
			t.Errorf("ExitCode(%q) = %d, want %d", code, got, want)
		}
	}
	for code := range wantExit {
		if !seen[code] {
			t.Errorf("wantExit has stale code %q no longer returned by any Codes()", code)
		}
	}
}

// TestExitCodeBoundaries covers success, an unmapped *domain.Error code (e.g. a future store
// code such as db_locked, unreachable from this package), and a plain non-domain error: all data
// errors except nil, since only an explicit user-input code or a cobra usage error is
// ExitUserError.
func TestExitCodeBoundaries(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"nil", nil, cli.ExitOK},
		{"unmapped domain code", &domain.Error{Code: "db_locked", Message: "locked"}, cli.ExitDataError},
		{"plain non-domain error", errors.New("open fui.db: permission denied"), cli.ExitDataError},
		{"wrapped plain error", errWrapped, cli.ExitDataError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cli.ExitCode(tt.err); got != tt.want {
				t.Errorf("ExitCode(%v) = %d, want %d", tt.err, got, tt.want)
			}
		})
	}
}

var errWrapped = fmt.Errorf("write backup: %w", errors.New("disk full"))

// TestExitCodeCobraUsageErrors runs real cobra usage mistakes through cli.Execute and checks they
// exit 1 (ExitUserError), not the ExitDataError a plain non-domain error defaults to: an unknown
// flag (caught by cobra's flag parsing), the wrong number of positional arguments (caught by an
// Args validator), an unknown top-level command, and the --json/--csv conflict this package
// checks itself.
func TestExitCodeCobraUsageErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"unknown flag", []string{"account", "list", "--does-not-exist"}},
		{"wrong arg count", []string{"account", "show"}},
		{"unknown command", []string{"bogus"}},
		{"json and csv conflict", []string{"account", "list", "--json", "--csv"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t)
			_, _, code := h.run(tt.args...)
			if code != cli.ExitUserError {
				t.Errorf("run(%v) exit = %d, want %d (ExitUserError)", tt.args, code, cli.ExitUserError)
			}
		})
	}
}
