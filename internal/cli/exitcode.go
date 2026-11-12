package cli

import (
	"errors"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/importing"
	"github.com/0xballo/fui/internal/domain"
)

// Exit codes, per the technical spec: 0 success (including --dry-run), 1 a user input error,
// 2 a data or I/O error.
const (
	ExitOK        = 0
	ExitUserError = 1
	ExitDataError = 2
)

// userInputCodes is an explicit allow-list of the stable error codes caused by bad user input,
// hence ExitUserError. It is deliberately not derived from domain.Codes()+app.Codes()+
// importing.Codes(): every code those return still has to be listed here, by name, for ExitCode
// to classify it as ExitUserError; anything left out -- a known code such as "overflow",
// "invalid_stream" or "connector_failed", a future one such as a store code like "db_locked", or
// an unrecognised one -- falls through to ExitDataError. The accompanying test enumerates every
// code from the three Codes() functions so a newly added code must be classified here or the
// test fails.
var userInputCodes = map[domain.Code]bool{
	domain.CodeAmbiguousName:       true,
	domain.CodeNotFound:            true,
	domain.CodeCurrencyMismatch:    true,
	domain.CodeReconciled:          true,
	domain.CodeInvalidFilter:       true,
	domain.CodeInvalidAmount:       true,
	domain.CodeInvalidDate:         true,
	domain.CodeMissingField:        true,
	domain.CodeInvalidValue:        true,
	domain.CodeNotEnoughData:       true,
	app.CodeConfirmationRequired:   true,
	app.CodeRateMissing:            true,
	app.CodeAlreadyExists:          true,
	app.CodeInUse:                  true,
	importing.CodeUnmappedAccounts: true,
}

// usageError marks err as caused by invalid CLI usage -- an unknown flag, the wrong number of
// positional arguments, or a conflicting flag combination -- rather than by the domain/app layer.
// ExitCode treats it like a user-input *domain.Error (ExitUserError) instead of defaulting it to
// ExitDataError, which is where every other non-domain error (plain I/O, context, wrapped store
// errors) lands.
type usageError struct{ err error }

// newUsageError wraps err as a usageError, or returns nil if err is nil.
func newUsageError(err error) error {
	if err == nil {
		return nil
	}
	return &usageError{err: err}
}

// Error implements error.
func (e *usageError) Error() string { return e.err.Error() }

// Unwrap lets errors.Is/As see through the wrapper to err.
func (e *usageError) Unwrap() error { return e.err }

// ExitCode maps err to the process exit code fui returns. It is the single function required by
// the technical spec: cobra/pflag usage errors and *domain.Error codes in userInputCodes are
// ExitUserError; every other error, domain-coded or not, is ExitDataError.
func ExitCode(err error) int {
	if err == nil {
		return ExitOK
	}
	var derr *domain.Error
	if errors.As(err, &derr) {
		if userInputCodes[derr.Code] {
			return ExitUserError
		}
		return ExitDataError
	}
	var uerr *usageError
	if errors.As(err, &uerr) {
		return ExitUserError
	}
	return ExitDataError
}
