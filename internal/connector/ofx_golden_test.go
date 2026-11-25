package connector

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/0xballo/fui/internal/app/importing"
)

// goldenJSONL renders the fui/1 stream of a source as JSON lines.
func goldenJSONL(t *testing.T, src importing.Source) string {
	t.Helper()
	recs, err := collect(t, src.Records(ctx()))
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	for _, r := range recs {
		line, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		b.Write(line)
		b.WriteByte('\n')
	}
	return b.String()
}

func TestGoldenFUI1(t *testing.T) {
	// The external csv_detect test registers -update in the same test binary.
	update := flag.Lookup("update")
	cases := []struct {
		name string
		src  importing.Source
	}{
		{"ofx-bank-v1.ofx", NewOFX(filepath.Join("testdata", "ofx-bank-v1.ofx"), "")},
		{"ofx-errors.ofx", NewOFX(filepath.Join("testdata", "ofx-errors.ofx"), "")},
		{"camt/v02.xml", NewCAMT(filepath.Join("testdata", "camt", "v02.xml"), CAMTOptions{})},
		{"camt/v08.xml", NewCAMT(filepath.Join("testdata", "camt", "v08.xml"), CAMTOptions{})},
		{"camt/batch-mismatch.xml", NewCAMT(filepath.Join("testdata", "camt", "batch-mismatch.xml"), CAMTOptions{})},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := goldenJSONL(t, tc.src)
			golden := filepath.Join("testdata", tc.name+".golden.jsonl")
			if update != nil && update.Value.String() == "true" {
				if err := os.WriteFile(golden, []byte(got), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(string(want), got); diff != "" {
				t.Errorf("fui/1 stream mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
