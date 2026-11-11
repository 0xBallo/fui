package cli_test

import (
	"testing"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/cli"
	"github.com/0xballo/fui/internal/domain"
)

// TestExitCodeKnownCodesAreUserErrors enumerates every known error code (domain.Codes plus
// app.Codes) through cli.ExitCode: none of them is a data/IO code yet (those, e.g. db_locked,
// belong to a store/connector layer not built in this task), so every one must map to
// ExitUserError.
func TestExitCodeKnownCodesAreUserErrors(t *testing.T) {
	codes := append(append([]domain.Code{}, domain.Codes()...), app.Codes()...)
	if len(codes) == 0 {
		t.Fatal("no known codes to enumerate")
	}
	for _, code := range codes {
		err := &domain.Error{Code: code, Message: "boom"}
		if got := cli.ExitCode(err); got != cli.ExitUserError {
			t.Errorf("ExitCode(%q) = %d, want %d (ExitUserError)", code, got, cli.ExitUserError)
		}
	}
}

// TestExitCodeTable covers the boundary cases: success, an unknown domain.Error code (a future
// data/IO error), and a plain error without a code (e.g. a cobra flag-parsing failure).
func TestExitCodeTable(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"nil", nil, cli.ExitOK},
		{"unknown domain code", &domain.Error{Code: "db_locked", Message: "locked"}, cli.ExitDataError},
		{"plain error", errFlagParse, cli.ExitUserError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cli.ExitCode(tt.err); got != tt.want {
				t.Errorf("ExitCode(%v) = %d, want %d", tt.err, got, tt.want)
			}
		})
	}
}

var errFlagParse = &plainError{"unknown flag"}

type plainError struct{ msg string }

func (e *plainError) Error() string { return e.msg }
