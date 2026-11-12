package domain

import (
	"fmt"
)

// Code is a stable, machine-readable error code. It becomes the "code" key of
// the CLI's JSON error and determines the process exit code.
type Code string

// Stable error codes. Never rename or reuse a code: scripts and agents match on them.
const (
	CodeAmbiguousName    Code = "ambiguous_name"
	CodeNotFound         Code = "not_found"
	CodeCurrencyMismatch Code = "currency_mismatch"
	CodeReconciled       Code = "reconciled"
	CodeInvalidFilter    Code = "invalid_filter"
	CodeOverflow         Code = "overflow"
	CodeInvalidAmount    Code = "invalid_amount"
	CodeInvalidDate      Code = "invalid_date"
	CodeMissingField     Code = "missing_field"
	CodeInvalidValue     Code = "invalid_value"
	CodeNotEnoughData    Code = "not_enough_data"
)

// Codes returns every domain error code, so the CLI can test that its
// code → exit-code mapping is exhaustive.
func Codes() []Code {
	return []Code{
		CodeAmbiguousName, CodeNotFound, CodeCurrencyMismatch, CodeReconciled,
		CodeInvalidFilter, CodeOverflow, CodeInvalidAmount, CodeInvalidDate, CodeMissingField, CodeInvalidValue,
		CodeNotEnoughData,
	}
}

// Error is a domain error with a stable code and optional structured details.
// Two Errors match under errors.Is when their codes are equal, so the Err*
// sentinels match every error carrying that code.
type Error struct {
	Code    Code
	Message string
	Details map[string]any
}

// Error implements the error interface.
func (e *Error) Error() string { return e.Message }

// Is matches any *Error with the same Code, which makes the sentinels usable with errors.Is.
func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	return ok && t.Code == e.Code
}

// Sentinels, one per code, for use with errors.Is.
var (
	ErrAmbiguousName    = &Error{Code: CodeAmbiguousName, Message: "ambiguous name"}
	ErrNotFound         = &Error{Code: CodeNotFound, Message: "not found"}
	ErrCurrencyMismatch = &Error{Code: CodeCurrencyMismatch, Message: "currency mismatch"}
	ErrReconciled       = &Error{Code: CodeReconciled, Message: "transaction is reconciled"}
	ErrInvalidFilter    = &Error{Code: CodeInvalidFilter, Message: "invalid filter"}
	ErrOverflow         = &Error{Code: CodeOverflow, Message: "arithmetic overflow"}
	ErrInvalidAmount    = &Error{Code: CodeInvalidAmount, Message: "invalid amount"}
	ErrInvalidDate      = &Error{Code: CodeInvalidDate, Message: "invalid date"}
	ErrMissingField     = &Error{Code: CodeMissingField, Message: "missing field"}
	ErrInvalidValue     = &Error{Code: CodeInvalidValue, Message: "invalid value"}
	ErrNotEnoughData    = &Error{Code: CodeNotEnoughData, Message: "not enough data"}
)

// newError builds an Error with the code of sentinel and a formatted message.
func newError(sentinel *Error, details map[string]any, format string, args ...any) *Error {
	return &Error{Code: sentinel.Code, Message: fmt.Sprintf(format, args...), Details: details}
}

func missingField(field string) error {
	return newError(ErrMissingField, map[string]any{"field": field}, "missing field %q", field)
}

func invalidValue(field, value string) error {
	return newError(ErrInvalidValue, map[string]any{"field": field, "value": value},
		"invalid %s %q", field, value)
}
