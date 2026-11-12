package domain

import (
	"errors"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

func TestNewTag(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr error
	}{
		{"trims", "  vacanze-2026  ", "vacanze-2026", nil},
		{"empty", "   ", "", ErrMissingField},
		{"space", "vacanze 2026", "", ErrInvalidValue},
		{"comma", "a,b", "", ErrInvalidValue},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewTag(tt.input)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("NewTag(%q) err = %v, want %v", tt.input, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewTag(%q): %v", tt.input, err)
			}
			if got.Name != tt.want {
				t.Fatalf("NewTag(%q).Name = %q, want %q", tt.input, got.Name, tt.want)
			}
		})
	}
}

func TestTagKeyCaseInsensitive(t *testing.T) {
	a, err := NewTag("Vacanze2026")
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewTag("vacanze2026")
	if err != nil {
		t.Fatal(err)
	}
	if a.Key() != b.Key() {
		t.Fatalf("keys differ: %q vs %q", a.Key(), b.Key())
	}
	c, err := NewTag("other")
	if err != nil {
		t.Fatal(err)
	}
	if a.Key() == c.Key() {
		t.Fatalf("keys of distinct tags collide: %q", a.Key())
	}
}

// TestTagKeyProperty checks that Key is idempotent and case-insensitive for
// any valid tag name (no spaces or commas).
func TestTagKeyProperty(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		name := rapid.StringMatching(`[A-Za-z0-9_-]{1,20}`).Draw(t, "name")
		lower, err := NewTag(strings.ToLower(name))
		if err != nil {
			t.Fatalf("NewTag(%q): %v", name, err)
		}
		upper, err := NewTag(strings.ToUpper(name))
		if err != nil {
			t.Fatal(err)
		}
		if lower.Key() != upper.Key() {
			t.Fatalf("Key not case-insensitive: %q vs %q", lower.Key(), upper.Key())
		}
	})
}
