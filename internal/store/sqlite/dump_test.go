package sqlite

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestDumpEveryTablePresentWithRows(t *testing.T) {
	store := fixtureAllTables(t)
	dump, err := store.DumpArchive(context.Background())
	if err != nil {
		t.Fatalf("DumpArchive: %v", err)
	}
	if dump.SchemaVersion != int(currentSchemaVersion) {
		t.Errorf("SchemaVersion = %d, want %d", dump.SchemaVersion, currentSchemaVersion)
	}
	if len(dump.Tables) == 0 {
		t.Fatal("no tables dumped")
	}
	for i, table := range dump.Tables {
		if len(table.Rows) == 0 {
			t.Errorf("table %s has no rows in the fixture dump", table.Name)
		}
		if i > 0 && dump.Tables[i-1].Name >= table.Name {
			t.Errorf("tables not sorted by name: %s before %s", dump.Tables[i-1].Name, table.Name)
		}
		for _, row := range table.Rows {
			if len(row) != len(table.Columns) {
				t.Errorf("table %s: row has %d cells, want %d", table.Name, len(row), len(table.Columns))
			}
		}
	}
}

func TestDumpCoversEveryTable(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()

	dump, err := store.DumpArchive(ctx)
	if err != nil {
		t.Fatalf("DumpArchive: %v", err)
	}
	dumped := map[string]bool{}
	for _, table := range dump.Tables {
		dumped[table.Name] = true
	}

	rows, err := store.readDB.QueryContext(ctx, "SELECT name FROM sqlite_master WHERE type = 'table'")
	if err != nil {
		t.Fatalf("list tables: %v", err)
	}
	defer func() { _ = rows.Close() }()
	existing := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan: %v", err)
		}
		existing[name] = true
		if !dumped[name] && !isDumpExcluded(name) {
			t.Errorf("table %q is neither dumped nor in dumpExcluded: extend DumpArchive (or dumpExcluded if it is derived) so backups and exports do not silently lose it", name)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}
	for name := range dumpExcluded {
		if !existing[name] {
			t.Errorf("dumpExcluded lists %q but no such table exists: remove it", name)
		}
	}
}

func TestDumpDeterministicAndReadOnly(t *testing.T) {
	store := fixtureAllTables(t)
	ctx := context.Background()

	first, err := store.DumpArchive(ctx)
	if err != nil {
		t.Fatalf("first dump: %v", err)
	}
	second, err := store.DumpArchive(ctx)
	if err != nil {
		t.Fatalf("second dump: %v", err)
	}
	if diff := cmp.Diff(first, second); diff != "" {
		t.Errorf("dumps differ (-first +second):\n%s", diff)
	}
}
