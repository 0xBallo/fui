package sqlite

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

func restoreBackups(t *testing.T, s *Store) int {
	t.Helper()
	entries, err := os.ReadDir(s.backupDir)
	if err != nil {
		t.Fatalf("read backup dir: %v", err)
	}
	n := 0
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), "-restore.db") {
			n++
		}
	}
	return n
}

func TestLoadArchiveRoundTrip(t *testing.T) {
	ctx := context.Background()
	src := fixtureAllTables(t)
	want, err := src.DumpArchive(ctx)
	if err != nil {
		t.Fatalf("DumpArchive: %v", err)
	}

	dst, _ := newTestStore(t)
	// Pre-existing data must be replaced, not merged.
	if _, err := dst.writeDB.ExecContext(ctx, "INSERT INTO categories (id, name, kind) VALUES (5, 'Old', 'expense')"); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := dst.LoadArchive(ctx, want); err != nil {
		t.Fatalf("LoadArchive: %v", err)
	}
	got, err := dst.DumpArchive(ctx)
	if err != nil {
		t.Fatalf("DumpArchive after load: %v", err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("round trip mismatch (-want +got):\n%s", diff)
	}

	if restoreBackups(t, dst) != 1 {
		t.Errorf("want one restore backup")
	}
	if n := count(t, dst, "categories"); n != 2 {
		t.Errorf("categories = %d, want 2 (ids 901, 902 only)", n)
	}
	var id int64
	if err := dst.readDB.QueryRowContext(ctx, "SELECT id FROM categories WHERE name = 'Fixture salary'").Scan(&id); err != nil || id != 902 {
		t.Errorf("category id = %d, err %v; want 902", id, err)
	}
	var hits int
	if err := dst.readDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM transactions_fts WHERE transactions_fts MATCH 'fixture'").Scan(&hits); err != nil || hits != 1 {
		t.Errorf("fts hits = %d, err %v; want 1", hits, err)
	}
	cats, err := listCategories(ctx, dst)
	if err != nil || len(cats) != 2 {
		t.Errorf("repo read: %d categories, err %v", len(cats), err)
	}
	// The store stays writable after the swap.
	if _, err := dst.writeDB.ExecContext(ctx, "INSERT INTO tags (name) VALUES ('after')"); err != nil {
		t.Errorf("write after load: %v", err)
	}
}

func listCategories(ctx context.Context, s *Store) ([]domain.Category, error) {
	var cats []domain.Category
	err := s.Read(ctx, func(r app.Repos) (err error) {
		cats, err = r.Categories().List(ctx)
		return err
	})
	return cats, err
}

func TestLoadArchiveInvalidLeavesArchiveUntouched(t *testing.T) {
	ctx := context.Background()
	good, err := fixtureAllTables(t).DumpArchive(ctx)
	if err != nil {
		t.Fatalf("DumpArchive: %v", err)
	}
	clone := func() app.ArchiveDump {
		d := app.ArchiveDump{SchemaVersion: good.SchemaVersion}
		for _, tb := range good.Tables {
			c := app.TableDump{Name: tb.Name, Columns: append([]string(nil), tb.Columns...)}
			for _, r := range tb.Rows {
				c.Rows = append(c.Rows, append([]any(nil), r...))
			}
			d.Tables = append(d.Tables, c)
		}
		return d
	}
	table := func(d *app.ArchiveDump, name string) *app.TableDump {
		for i := range d.Tables {
			if d.Tables[i].Name == name {
				return &d.Tables[i]
			}
		}
		t.Fatalf("no table %s", name)
		return nil
	}

	tests := []struct {
		name   string
		mutate func(d *app.ArchiveDump)
	}{
		{"newer schema", func(d *app.ArchiveDump) { d.SchemaVersion = int(currentSchemaVersion) + 1 }},
		{"unknown table", func(d *app.ArchiveDump) {
			d.Tables = append(d.Tables, app.TableDump{Name: "nope", Columns: []string{"a"}})
		}},
		{"duplicate table", func(d *app.ArchiveDump) { d.Tables = append(d.Tables, *table(d, "tags")) }},
		{"unknown column", func(d *app.ArchiveDump) { table(d, "tags").Columns[1] = "nope" }},
		{"duplicate column", func(d *app.ArchiveDump) { table(d, "tags").Columns[1] = "id" }},
		{"no columns", func(d *app.ArchiveDump) { table(d, "tags").Columns = nil }},
		{"ragged row", func(d *app.ArchiveDump) { table(d, "tags").Rows[0] = []any{int64(1)} }},
		{"float cell", func(d *app.ArchiveDump) { table(d, "tags").Rows[0][0] = 1.5 }},
		{"bad foreign key", func(d *app.ArchiveDump) { table(d, "payee_aliases").Rows[0][1] = int64(999) }},
		{"constraint violation", func(d *app.ArchiveDump) { table(d, "accounts").Rows[0][1] = nil }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dst := fixtureAllTables(t)
			beforeDump, err := dst.DumpArchive(ctx)
			if err != nil {
				t.Fatalf("DumpArchive: %v", err)
			}

			d := clone()
			tc.mutate(&d)
			err = dst.LoadArchive(ctx, d)
			if err == nil {
				t.Fatal("LoadArchive succeeded on an invalid dump")
			}
			if tc.name != "constraint violation" && !errors.Is(err, domain.ErrInvalidValue) {
				t.Errorf("err = %v, want ErrInvalidValue", err)
			}

			afterDump, err := dst.DumpArchive(ctx)
			if err != nil {
				t.Fatalf("archive unusable after failure: %v", err)
			}
			if diff := cmp.Diff(beforeDump, afterDump); diff != "" {
				t.Errorf("archive changed (-before +after):\n%s", diff)
			}
			if restoreBackups(t, dst) != 0 {
				t.Errorf("a restore backup was taken for an invalid dump")
			}
			if left, _ := filepath.Glob(filepath.Join(filepath.Dir(dst.path), ".fui-restore-*")); len(left) != 0 {
				t.Errorf("temp files left: %v", left)
			}
		})
	}
}

func TestLoadArchiveOlderDumpUsesColumnDefaults(t *testing.T) {
	ctx := context.Background()
	dst, _ := newTestStore(t)
	d := app.ArchiveDump{SchemaVersion: 1, Tables: []app.TableDump{
		{Name: "categories", Columns: []string{"id", "name", "kind"}, Rows: [][]any{{int64(7), "Food", "expense"}}},
	}}
	if err := dst.LoadArchive(ctx, d); err != nil {
		t.Fatalf("LoadArchive: %v", err)
	}
	if n := count(t, dst, "categories"); n != 1 {
		t.Errorf("categories = %d, want 1", n)
	}
}
