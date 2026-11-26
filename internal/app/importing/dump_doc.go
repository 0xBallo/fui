package importing

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

// DumpFileName is the name of the full-dump document written by DumpAll in JSON format.
const DumpFileName = "dump.json"

const dumpFormatName = "fui-dump"

// dumpDoc is the wire form of a full dump (version 2). Tables is a map so that encoding/json
// emits its keys sorted, which makes the output deterministic.
type dumpDoc struct {
	Format        string                `json:"format"`
	Version       int                   `json:"version"`
	SchemaVersion int                   `json:"schema_version"`
	Config        string                `json:"config"`
	Tables        map[string]*dumpTable `json:"tables"`
}

type dumpTable struct {
	Columns []string `json:"columns"`
	Rows    [][]any  `json:"rows"`
}

// EncodeDump writes d and the raw text of config.toml (which may be empty) as the dump
// document: indented, with tables sorted by name, so equal inputs give equal bytes.
func EncodeDump(w io.Writer, d app.ArchiveDump, config string) error {
	doc := dumpDoc{
		Format: dumpFormatName, Version: app.DumpSchemaVersion, SchemaVersion: d.SchemaVersion,
		Config: config, Tables: make(map[string]*dumpTable, len(d.Tables)),
	}
	for _, t := range d.Tables {
		if _, dup := doc.Tables[t.Name]; dup {
			return newError(domain.ErrInvalidValue, map[string]any{"table": t.Name}, "dump table %q appears twice", t.Name)
		}
		cols, rows := t.Columns, t.Rows
		if cols == nil {
			cols = []string{}
		}
		if rows == nil {
			rows = [][]any{}
		}
		for i, row := range rows {
			if len(row) != len(cols) {
				return newError(domain.ErrInvalidValue, map[string]any{"table": t.Name, "row": i},
					"dump table %q row %d has %d cells, want %d", t.Name, i, len(row), len(cols))
			}
			for j, c := range row {
				switch c.(type) {
				case nil, int64, string:
				default:
					return newError(domain.ErrInvalidValue, map[string]any{"table": t.Name, "row": i},
						"dump table %q row %d column %d: unsupported cell type %T", t.Name, i, j, c)
				}
			}
		}
		doc.Tables[t.Name] = &dumpTable{Columns: cols, Rows: rows}
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(doc); err != nil {
		return fmt.Errorf("write dump: %w", err)
	}
	return nil
}

// DecodeDump reads a dump document. It rejects anything that is not a restorable version 1 or
// 2 dump: wrong format, unknown version, no tables (version 1 was the per-entity export),
// ragged rows, duplicate tables and non-integer numbers. Tables come back sorted by name.
func DecodeDump(r io.Reader) (d app.ArchiveDump, config string, err error) {
	dec := json.NewDecoder(r)
	dec.UseNumber()
	var raw struct {
		Format        string          `json:"format"`
		Version       int             `json:"version"`
		SchemaVersion int             `json:"schema_version"`
		Config        string          `json:"config"`
		Tables        json.RawMessage `json:"tables"`
	}
	if err := dec.Decode(&raw); err != nil {
		return app.ArchiveDump{}, "", invalidDump(err, "read dump: %v", err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return app.ArchiveDump{}, "", invalidDump(nil, "the dump has trailing data after the document")
	}
	if raw.Format != dumpFormatName {
		return app.ArchiveDump{}, "", invalidDump(nil, "not a FUI dump: format is %q, want %q", raw.Format, dumpFormatName)
	}
	if raw.Version < 1 || raw.Version > app.DumpSchemaVersion {
		return app.ArchiveDump{}, "", invalidDump(nil, "unsupported dump version %d (this build reads up to %d)", raw.Version, app.DumpSchemaVersion)
	}
	if len(raw.Tables) == 0 || bytes.Equal(raw.Tables, []byte("null")) {
		return app.ArchiveDump{}, "", invalidDump(nil, "the dump is not restorable: it has no tables (a version 1 per-entity export cannot rebuild the archive)")
	}
	if raw.SchemaVersion <= 0 {
		return app.ArchiveDump{}, "", invalidDump(nil, "the dump has no schema version")
	}
	tables, err := decodeTables(raw.Tables)
	if err != nil {
		return app.ArchiveDump{}, "", err
	}
	return app.ArchiveDump{SchemaVersion: raw.SchemaVersion, Tables: tables}, raw.Config, nil
}

func invalidDump(cause error, format string, args ...any) error {
	if cause != nil {
		return caused(domain.ErrInvalidValue, cause, format, args...)
	}
	return newError(domain.ErrInvalidValue, nil, format, args...)
}

// decodeTables walks the "tables" object token by token to detect duplicate names, which a
// map decode would silently merge.
func decodeTables(data json.RawMessage) ([]app.TableDump, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		return nil, invalidDump(nil, "the dump \"tables\" is not an object")
	}
	var out []app.TableDump
	seen := map[string]bool{}
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, invalidDump(err, "read dump tables: %v", err)
		}
		name, _ := tok.(string)
		if seen[name] {
			return nil, invalidDump(nil, "dump table %q appears twice", name)
		}
		seen[name] = true
		var t struct {
			Columns []string `json:"columns"`
			Rows    [][]any  `json:"rows"`
		}
		if err := dec.Decode(&t); err != nil {
			return nil, invalidDump(err, "dump table %q: %v", name, err)
		}
		td := app.TableDump{Name: name, Columns: t.Columns, Rows: make([][]any, len(t.Rows))}
		if td.Columns == nil {
			td.Columns = []string{}
		}
		for i, row := range t.Rows {
			if len(row) != len(t.Columns) {
				return nil, invalidDump(nil, "dump table %q row %d has %d cells, want %d", name, i, len(row), len(t.Columns))
			}
			for j, c := range row {
				v, err := dumpCell(c)
				if err != nil {
					return nil, invalidDump(nil, "dump table %q row %d column %d: %v", name, i, j, err)
				}
				row[j] = v
			}
			td.Rows[i] = row
		}
		out = append(out, td)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// dumpCell converts a decoded JSON value to nil, int64 or string.
func dumpCell(c any) (any, error) {
	switch v := c.(type) {
	case nil, string:
		return v, nil
	case json.Number:
		n, err := strconv.ParseInt(v.String(), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%q is not an integer", v.String())
		}
		return n, nil
	default:
		return nil, fmt.Errorf("unsupported cell type %T", c)
	}
}
