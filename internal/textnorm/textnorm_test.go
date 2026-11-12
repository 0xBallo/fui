package textnorm

import "testing"

func TestNormalizeItalianAccents(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"  Caffè  ", "caffe"},
		{"Èlite ÀÈÌÒÙ", "elite aeiou"},
		{"perché già più", "perche gia piu"},
		{"  bonifico\tSEPA\n", "bonifico sepa"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := Normalize(tt.in); got != tt.want {
			t.Fatalf("Normalize(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestSearchText(t *testing.T) {
	got := SearchText("  Caffè al BAR ", " Èlite ", "\n  pagamento  ")
	want := "caffe al bar elite pagamento"
	if got != want {
		t.Fatalf("SearchText = %q, want %q", got, want)
	}
}
