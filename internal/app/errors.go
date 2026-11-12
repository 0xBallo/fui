package app

import (
	"fmt"

	"github.com/0xballo/fui/internal/domain"
)

// App-level error codes. They extend the domain codes (see domain.Codes) and
// are carried by *domain.Error so the CLI maps every failure the same way.
const (
	// CodeConfirmationRequired is raised by UIs that must not write without an explicit
	// confirmation (agent mode without --yes, bulk plans without --yes).
	CodeConfirmationRequired domain.Code = "confirmation_required"
	// CodeRateMissing means a currency conversion needs an exchange rate that was never set.
	CodeRateMissing domain.Code = "rate_missing"
	// CodeAlreadyExists means a unique name or external id is already taken.
	CodeAlreadyExists domain.Code = "already_exists"
	// CodeInUse means an entity cannot be removed because other data still references it.
	CodeInUse domain.Code = "in_use"
	// CodeModuleDisabled means a command requires a disabled module.
	CodeModuleDisabled domain.Code = "module_disabled"
)

// Codes returns every app-level error code, for the CLI's exhaustive exit-code mapping test.
func Codes() []domain.Code {
	return []domain.Code{CodeConfirmationRequired, CodeRateMissing, CodeAlreadyExists, CodeInUse}
}

// Sentinels for errors.Is; they match every error carrying the same code.
var (
	ErrConfirmationRequired = &domain.Error{Code: CodeConfirmationRequired, Message: "confirmation required"}
	ErrRateMissing          = &domain.Error{Code: CodeRateMissing, Message: "exchange rate missing"}
	ErrAlreadyExists        = &domain.Error{Code: CodeAlreadyExists, Message: "already exists"}
	ErrInUse                = &domain.Error{Code: CodeInUse, Message: "in use"}
	ErrModuleDisabled       = &domain.Error{Code: CodeModuleDisabled, Message: "module disabled"}
)

// newError builds an error with the code of sentinel, a formatted message and structured details.
func newError(sentinel *domain.Error, details map[string]any, format string, args ...any) *domain.Error {
	return &domain.Error{Code: sentinel.Code, Message: fmt.Sprintf(format, args...), Details: details}
}
