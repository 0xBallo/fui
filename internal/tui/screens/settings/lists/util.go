package lists

import "strings"

// equalFold reports whether a and b are the same name, ignoring case (matching domain's
// case-insensitive name comparison).
func equalFold(a, b string) bool { return strings.EqualFold(a, b) }
