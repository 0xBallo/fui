package importing_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"pgregory.net/rapid"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/importing"
	"github.com/0xballo/fui/internal/domain"
)

const goodDoc = `{"format":"fui-dump","version":2,"schema_version":3,"config":"","tables":{"t":{"columns":["a","b"],"rows":[[1,"x"],[null,"y"]]}}}`

func TestDecodeDumpErrors(t *testing.T) {
	tests := []struct{ name, doc, want string }{
		{"bad format", strings.Replace(goodDoc, "fui-dump", "other", 1), "not a FUI dump"},
		{"version 0", strings.Replace(goodDoc, `"version":2`, `"version":0`, 1), "unsupported dump version"},
		{"version 3", strings.Replace(goodDoc, `"version":2`, `"version":3`, 1), "unsupported dump version"},
		{"v1 no tables", `{"format":"fui-dump","version":1,"schema_version":3}`, "not restorable"},
		{"null tables", `{"format":"fui-dump","version":2,"schema_version":3,"tables":null}`, "not restorable"},
		{"no schema version", strings.Replace(goodDoc, `"schema_version":3`, `"schema_version":0`, 1), "schema version"},
		{"ragged row", strings.Replace(goodDoc, `[null,"y"]`, `[null]`, 1), "row 1 has 1 cells"},
		{"duplicate table", `{"format":"fui-dump","version":2,"schema_version":3,"tables":{"t":{"columns":[],"rows":[]},"t":{"columns":[],"rows":[]}}}`, "appears twice"},
		{"float", strings.Replace(goodDoc, `[1,"x"]`, `[1.5,"x"]`, 1), "not an integer"},
		{"exponent", strings.Replace(goodDoc, `[1,"x"]`, `[1e3,"x"]`, 1), "not an integer"},
		{"bool cell", strings.Replace(goodDoc, `[1,"x"]`, `[true,"x"]`, 1), "unsupported cell type"},
		{"tables not object", `{"format":"fui-dump","version":2,"schema_version":3,"tables":[]}`, "not an object"},
		{"trailing data", goodDoc + ` {}`, "trailing data"},
		{"not json", `nope`, "read dump"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := importing.DecodeDump(strings.NewReader(tc.doc))
			if !errors.Is(err, domain.ErrInvalidValue) {
				t.Fatalf("err = %v, want ErrInvalidValue", err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %q, want it to contain %q", err, tc.want)
			}
		})
	}
	if _, _, err := importing.DecodeDump(strings.NewReader(goodDoc)); err != nil {
		t.Fatalf("good doc: %v", err)
	}
}

func TestEncodeDumpErrors(t *testing.T) {
	bad := []app.TableDump{
		{Name: "t", Columns: []string{"a"}, Rows: [][]any{{1.5}}},
		{Name: "t", Columns: []string{"a"}, Rows: [][]any{{int64(1), int64(2)}}},
	}
	for _, tb := range bad {
		if err := importing.EncodeDump(&bytes.Buffer{}, app.ArchiveDump{SchemaVersion: 1, Tables: []app.TableDump{tb}}, ""); !errors.Is(err, domain.ErrInvalidValue) {
			t.Errorf("%v: err = %v", tb, err)
		}
	}
	dup := app.ArchiveDump{SchemaVersion: 1, Tables: []app.TableDump{{Name: "t"}, {Name: "t"}}}
	if err := importing.EncodeDump(&bytes.Buffer{}, dup, ""); !errors.Is(err, domain.ErrInvalidValue) {
		t.Errorf("dup: err = %v", err)
	}
}

func genDump() *rapid.Generator[app.ArchiveDump] {
	cell := rapid.OneOf(rapid.Just[any](nil), rapid.Map(rapid.Int64(), func(n int64) any { return n }), rapid.Map(rapid.String(), func(s string) any { return s }))
	table := rapid.Custom(func(t *rapid.T) app.TableDump {
		cols := rapid.SliceOfNDistinct(rapid.StringMatching(`[a-z]{1,6}`), 0, 4, rapid.ID).Draw(t, "cols")
		rows := rapid.SliceOfN(rapid.SliceOfN(cell, len(cols), len(cols)), 0, 4).Draw(t, "rows")
		return app.TableDump{Columns: cols, Rows: rows}
	})
	return rapid.Custom(func(t *rapid.T) app.ArchiveDump {
		names := rapid.SliceOfNDistinct(rapid.StringMatching(`[a-z_]{1,8}`), 1, 4, rapid.ID).Draw(t, "names")
		d := app.ArchiveDump{SchemaVersion: rapid.IntRange(1, 100).Draw(t, "schema")}
		for _, n := range names {
			tb := table.Draw(t, "table")
			tb.Name = n
			d.Tables = append(d.Tables, tb)
		}
		return d
	})
}

func normalize(d app.ArchiveDump) app.ArchiveDump {
	out := app.ArchiveDump{SchemaVersion: d.SchemaVersion}
	for _, t := range d.Tables {
		if t.Columns == nil {
			t.Columns = []string{}
		}
		if t.Rows == nil {
			t.Rows = [][]any{}
		}
		out.Tables = append(out.Tables, t)
	}
	for i := range out.Tables { // insertion sort by name keeps the helper dependency-free
		for j := i; j > 0 && out.Tables[j].Name < out.Tables[j-1].Name; j-- {
			out.Tables[j], out.Tables[j-1] = out.Tables[j-1], out.Tables[j]
		}
	}
	return out
}

func TestDumpRoundTrip(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		d := genDump().Draw(t, "dump")
		cfg := rapid.String().Draw(t, "config")
		var a, b bytes.Buffer
		if err := importing.EncodeDump(&a, d, cfg); err != nil {
			t.Fatal(err)
		}
		got, gotCfg, err := importing.DecodeDump(bytes.NewReader(a.Bytes()))
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(normalize(d), got); diff != "" {
			t.Fatalf("round trip (-want +got):\n%s", diff)
		}
		if gotCfg != cfg {
			t.Fatalf("config %q != %q", gotCfg, cfg)
		}
		if err := importing.EncodeDump(&b, got, gotCfg); err != nil || !bytes.Equal(a.Bytes(), b.Bytes()) {
			t.Fatalf("re-encoding is not byte-identical (err %v)", err)
		}
	})
}

func FuzzDecodeDump(f *testing.F) {
	f.Add(goodDoc)
	f.Add(`{"format":"fui-dump","version":1,"schema_version":1}`)
	f.Add(`{"format":"fui-dump","version":2,"schema_version":1,"tables":{"a":{"columns":["x"],"rows":[[9223372036854775808]]}}}`)
	f.Add(`{"tables":{"a":null}}`)
	f.Add(`{"format":"fui-dump","version":2,"schema_version":1,"tables":{"a":{"columns":null,"rows":[[]]}}}`)
	f.Add(`[`)
	f.Fuzz(func(t *testing.T, doc string) {
		d, _, err := importing.DecodeDump(strings.NewReader(doc))
		if err != nil {
			return
		}
		var buf bytes.Buffer
		if err := importing.EncodeDump(&buf, d, ""); err != nil {
			t.Fatalf("decoded dump does not re-encode: %v", err)
		}
	})
}
