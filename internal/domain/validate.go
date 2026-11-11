package domain

import (
	"strings"
)

// requireName trims a user-visible name and rejects an empty one.
func requireName(field, name string) (string, error) {
	n := strings.TrimSpace(name)
	if n == "" {
		return "", missingField(field)
	}
	return n, nil
}

// enumError reports an empty value as a missing field and anything else as invalid.
func enumError(field, value string) error {
	if value == "" {
		return missingField(field)
	}
	return invalidValue(field, value)
}

// parseEnum lower-cases and trims s and returns it if valid reports true.
func parseEnum[T ~string](field, s string, valid func(T) bool) (T, error) {
	v := T(strings.ToLower(strings.TrimSpace(s)))
	if !valid(v) {
		return "", enumError(field, s)
	}
	return v, nil
}
