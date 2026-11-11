package cli

import (
	"errors"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

// Exit codes, per the technical spec: 0 success (including --dry-run), 1 a user input error,
// 2 a data or I/O error.
const (
	ExitOK        = 0
	ExitUserError = 1
	ExitDataError = 2
)

// userInputCodes are the stable error codes caused by the user, hence ExitUserError. Every other
// *domain.Error, including codes not yet defined (e.g. a future store code such as
// "db_corrupted"), is ExitDataError; so is any plain error without a code, such as a cobra
// flag-parsing failure, since in this layer that always means bad CLI usage.
var userInputCodes = buildUserInputCodes()

func buildUserInputCodes() map[domain.Code]bool {
	m := make(map[domain.Code]bool)
	for _, c := range domain.Codes() {
		m[c] = true
	}
	for _, c := range app.Codes() {
		m[c] = true
	}
	return m
}

// ExitCode maps err to the process exit code fui returns. The mapping is the single function
// required by the technical spec, so a test can enumerate every known code against it.
func ExitCode(err error) int {
	if err == nil {
		return ExitOK
	}
	var derr *domain.Error
	if errors.As(err, &derr) && !userInputCodes[derr.Code] {
		return ExitDataError
	}
	return ExitUserError
}
