package connector_test

import (
	"context"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/0xballo/fui/internal/app/importing"
	"github.com/0xballo/fui/internal/connector"
)

var update = flag.Bool("update", false, "rewrite the golden files")

func TestDetectCSVGolden(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("testdata", "csv_detect", "*.csv"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no fixtures: %v", err)
	}
	for _, f := range files {
		name := strings.TrimSuffix(filepath.Base(f), ".csv")
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			sug, err := connector.DetectCSV(data)
			if err != nil {
				t.Fatal(err)
			}
			got, err := json.MarshalIndent(sug, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			golden := strings.TrimSuffix(f, ".csv") + ".golden.json"
			if *update {
				if err := os.WriteFile(golden, append(got, '\n'), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(strings.TrimSpace(string(want)), string(got)); diff != "" {
				t.Errorf("suggestion mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestDetectCSV(t *testing.T) {
	tests := []struct {
		name       string
		file       string
		delim      rune
		dateFormat string
		decimal    rune
		header     bool
		roles      []string
		skip       int
		warn       string // substring of a warning, empty for none
	}{
		{
			name: "italian bank export", file: "it_bank.csv", delim: ';', dateFormat: "DD/MM/YYYY", decimal: ',', header: true, skip: 2,
			roles: []string{"date", "other", "description", "amount", "balance"},
		},
		{
			name: "english export", file: "en_export.csv", delim: ',', dateFormat: "YYYY-MM-DD", decimal: '.', header: true,
			roles: []string{"date", "description", "amount", "id"},
		},
		{
			name: "debit and credit", file: "debit_credit.csv", delim: ';', dateFormat: "DD-MM-YY", decimal: ',', header: true,
			roles: []string{"date", "description", "debit", "credit"},
		},
		{
			name: "tab separated", file: "tab.csv", delim: '\t', dateFormat: "DD.MM.YYYY", decimal: '.', header: true,
			roles: []string{"date", "description", "amount"},
		},
		{
			name: "no header", file: "no_header.csv", delim: ',', dateFormat: "YYYY-MM-DD", decimal: '.',
			roles: []string{"date", "description", "amount"}, warn: "no header row",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("testdata", "csv_detect", tt.file))
			if err != nil {
				t.Fatal(err)
			}
			sug, err := connector.DetectCSV(data)
			if err != nil {
				t.Fatal(err)
			}
			if sug.Delimiter != tt.delim || sug.DateFormat != tt.dateFormat || sug.DecimalSep != tt.decimal ||
				sug.HasHeader != tt.header || sug.Profile.SkipRows != tt.skip {
				t.Errorf("got delim=%q date=%q dec=%q header=%v skip=%d", sug.Delimiter, sug.DateFormat, sug.DecimalSep, sug.HasHeader, sug.Profile.SkipRows)
			}
			var roles []string
			for _, c := range sug.Columns {
				roles = append(roles, c.Role)
			}
			if diff := cmp.Diff(tt.roles, roles); diff != "" {
				t.Errorf("roles (-want +got):\n%s", diff)
			}
			if tt.warn == "" && len(sug.Warnings) != 0 {
				t.Errorf("unexpected warnings %q", sug.Warnings)
			}
			if tt.warn != "" && !strings.Contains(strings.Join(sug.Warnings, "|"), tt.warn) {
				t.Errorf("warnings %q lack %q", sug.Warnings, tt.warn)
			}
			if len(sug.Sample) == 0 || len(sug.Sample) > 5 {
				t.Errorf("sample has %d rows", len(sug.Sample))
			}
		})
	}
}

func TestDetectCSVEdgeCases(t *testing.T) {
	for name, in := range map[string]string{"empty": "", "blank": " \n\n"} {
		if _, err := connector.DetectCSV([]byte(in)); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}

	// Windows-1252 input is recognized and the profile says so.
	sug, err := connector.DetectCSV([]byte("Data;Descrizione;Importo\n02/10/2026;Caff\xe8;-1,20\n03/10/2026;Pizza;-8,00\n"))
	if err != nil {
		t.Fatal(err)
	}
	if sug.Profile.Encoding != "windows-1252" || len(sug.Warnings) != 0 {
		t.Errorf("encoding=%q warnings=%q", sug.Profile.Encoding, sug.Warnings)
	}

	// Rows that the profile cannot parse are reported.
	sug, err = connector.DetectCSV([]byte("Date,Description,Amount\n2026-10-02,A,1.00\n2026-10-03,B,1.00\n2026-10-04,C,oops\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(sug.Warnings, "|"), "1 of 3 rows") {
		t.Errorf("warnings %q", sug.Warnings)
	}

	// Two unnamed numeric columns filled in different rows read as debit and credit.
	sug, err = connector.DetectCSV([]byte("2026-10-02,A,5.00,\n2026-10-03,B,,7.00\n2026-10-04,C,1.00,\n"))
	if err != nil {
		t.Fatal(err)
	}
	if sug.Columns[2].Role != "debit" || sug.Columns[3].Role != "credit" {
		t.Errorf("columns %+v", sug.Columns)
	}

	// A single-column file has no delimiter: a comma is assumed and the columns are not found.
	sug, err = connector.DetectCSV([]byte("hello\nworld\n"))
	if err != nil || len(sug.Warnings) == 0 {
		t.Errorf("err=%v warnings=%q", err, sug.Warnings)
	}
}

func TestCSVFileDetector(t *testing.T) {
	file := filepath.Join("testdata", "csv_detect", "en_export.csv")
	sug, err := connector.CSVFileDetector{}.DetectCSV(context.Background(), file)
	if err != nil || sug.Profile.AmountColumn != "Amount" {
		t.Fatalf("sug=%+v err=%v", sug.Profile, err)
	}
	if _, err := (connector.CSVFileDetector{}).DetectCSV(context.Background(), file+".missing"); err == nil {
		t.Error("missing file: want an error")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := (connector.CSVFileDetector{}).DetectCSV(ctx, file); err == nil {
		t.Error("canceled context: want an error")
	}
	var _ importing.CSVDetector = connector.CSVFileDetector{}
}

func FuzzDetectCSV(f *testing.F) {
	files, _ := filepath.Glob(filepath.Join("testdata", "csv_detect", "*.csv"))
	for _, name := range files {
		if data, err := os.ReadFile(name); err == nil {
			f.Add(data)
		}
	}
	f.Add([]byte(""))
	f.Add([]byte("a;b\n\"unterminated,1\n"))
	f.Add([]byte("\xef\xbb\xbfDate,Amount,Description\n01/02/03,(1.234,50),x\n"))
	f.Fuzz(func(t *testing.T, data []byte) {
		a, errA := connector.DetectCSV(data)
		b, errB := connector.DetectCSV(data)
		if (errA == nil) != (errB == nil) || !reflect.DeepEqual(a, b) {
			t.Fatalf("not deterministic: %+v / %+v", a, b)
		}
	})
}
