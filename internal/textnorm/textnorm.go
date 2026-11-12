// Package textnorm normalizes free text for search matching.
package textnorm

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// Normalize lower-cases, removes diacritics, collapses whitespace and trims.
func Normalize(s string) string {
	decomposed := norm.NFD.String(strings.ToLower(strings.TrimSpace(s)))
	var out strings.Builder
	out.Grow(len(decomposed))
	lastSpace := false
	for _, r := range decomposed {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		if unicode.IsSpace(r) {
			if !lastSpace {
				out.WriteByte(' ')
				lastSpace = true
			}
			continue
		}
		out.WriteRune(r)
		lastSpace = false
	}
	return strings.TrimSpace(out.String())
}

// SearchText joins normalized description, payee and notes with one space.
func SearchText(description, payee, notes string) string {
	parts := make([]string, 0, 3)
	for _, raw := range []string{description, payee, notes} {
		if norm := Normalize(raw); norm != "" {
			parts = append(parts, norm)
		}
	}
	return strings.Join(parts, " ")
}
