package textnorm

import (
	"testing"
	"unicode"
)

func FuzzNormalize(f *testing.F) {
	for _, s := range []string{"Caffè", "  BONIFICO   SEPA  ", "Èlite", "a\u0301", "", "Straße"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		norm := Normalize(s)
		if Normalize(norm) != norm {
			t.Fatalf("Normalize is not idempotent: %q -> %q", s, norm)
		}
		for _, r := range norm {
			if unicode.IsUpper(r) {
				t.Fatalf("uppercase rune left: %q in %q", r, norm)
			}
			if unicode.Is(unicode.Mn, r) {
				t.Fatalf("combining mark left: %q in %q", r, norm)
			}
		}
	})
}
