package domain

import (
	"errors"
	"slices"
	"testing"
)

func TestResolve(t *testing.T) {
	names := []string{"Fineco", "Fineco Risparmio", "Widiba", "Revolut EUR", "Revolut USD"}
	tests := []struct {
		name    string
		query   string
		want    int
		wantErr error
		cands   []string
	}{
		{name: "exact wins over prefix", query: "fineco", want: 0},
		{name: "exact case-insensitive", query: "WIDIBA", want: 2},
		{name: "unique prefix", query: "fineco r", want: 1},
		{name: "unique substring", query: "diba", want: 2},
		{name: "ambiguous prefix", query: "revolut", wantErr: ErrAmbiguousName, cands: []string{"Revolut EUR", "Revolut USD"}},
		{name: "ambiguous substring", query: "e", wantErr: ErrAmbiguousName, cands: []string{"Fineco", "Fineco Risparmio", "Revolut EUR", "Revolut USD"}},
		{name: "prefix beats substring", query: "w", want: 2},
		{name: "not found", query: "zzz", wantErr: ErrNotFound},
		{name: "empty", query: "  ", wantErr: ErrMissingField},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Resolve(tt.query, names)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want %v", err, tt.wantErr)
				}
				if tt.cands != nil {
					var de *Error
					if !errors.As(err, &de) {
						t.Fatal("not a domain.Error")
					}
					if got, _ := de.Details["candidates"].([]string); !slices.Equal(got, tt.cands) {
						t.Fatalf("candidates = %v, want %v", got, tt.cands)
					}
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("Resolve(%q) = %d, %v; want %d", tt.query, got, err, tt.want)
			}
		})
	}
}

func TestResolveNoCandidates(t *testing.T) {
	if _, err := Resolve("a", nil); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v", err)
	}
}
