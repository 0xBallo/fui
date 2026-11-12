package domain

import (
	"errors"
	"testing"

	"pgregory.net/rapid"
)

func TestNewPayee(t *testing.T) {
	p, err := NewPayee("  Esselunga  ", "  ", " Esse SPA ", "")
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "Esselunga" {
		t.Fatalf("Name = %q", p.Name)
	}
	if len(p.Aliases) != 1 || p.Aliases[0] != "Esse SPA" {
		t.Fatalf("Aliases = %#v", p.Aliases)
	}
	if _, err := NewPayee("   "); !errors.Is(err, ErrMissingField) {
		t.Fatalf("NewPayee(empty) err = %v", err)
	}
}

func TestNormalizePayee(t *testing.T) {
	got := NormalizePayee("  Esse   LUNGA\tSPA  ")
	if got != "esse lunga spa" {
		t.Fatalf("got %q", got)
	}
}

func TestPayeeMatchesAlias(t *testing.T) {
	p, err := NewPayee("Esselunga", "Esse SPA")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		desc string
		want bool
	}{
		{"ESSELUNGA", true},
		{"  esselunga  ", true},
		{"Esse   SPA", true},
		{"Other Shop", false},
	}
	for _, tt := range tests {
		if got := p.MatchesAlias(tt.desc); got != tt.want {
			t.Errorf("MatchesAlias(%q) = %v, want %v", tt.desc, got, tt.want)
		}
	}
}

// TestNormalizePayeeIdempotent checks that normalizing twice is the same as
// normalizing once, for any input.
func TestNormalizePayeeIdempotent(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		s := rapid.String().Draw(t, "s")
		once := NormalizePayee(s)
		twice := NormalizePayee(once)
		if once != twice {
			t.Fatalf("NormalizePayee not idempotent: %q -> %q -> %q", s, once, twice)
		}
	})
}
