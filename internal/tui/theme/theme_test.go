package theme_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/0xballo/fui/internal/tui/theme"
)

func TestByName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantName string
	}{
		{name: "empty defaults to default", input: "", wantName: "default"},
		{name: "default by name", input: "default", wantName: "default"},
		{name: "high-contrast by name", input: "high-contrast", wantName: "high-contrast"},
		{name: "unknown defaults to default", input: "unknown", wantName: "default"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := theme.ByName(tt.input)
			if diff := cmp.Diff(tt.wantName, got.Name); diff != "" {
				t.Errorf("Theme.Name mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
