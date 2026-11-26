package importing_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/importing"
	"github.com/0xballo/fui/internal/testutil/memstore"
)

// dumpBackup is a Backuper that also implements app.ArchiveDumper.
type dumpBackup struct {
	spyBackup
	dump app.ArchiveDump
	err  error
}

func (b *dumpBackup) DumpArchive(context.Context) (app.ArchiveDump, error) { return b.dump, b.err }

func fakeDump() app.ArchiveDump {
	return app.ArchiveDump{SchemaVersion: 7, Tables: []app.TableDump{
		{Name: "tags", Columns: []string{"id", "name"}, Rows: [][]any{{int64(1), "food"}, {int64(2), nil}}},
		{Name: "accounts", Columns: []string{"id"}, Rows: [][]any{{int64(1)}}},
		{Name: "budgets", Columns: []string{"id"}},
	}}
}

func dumpService(b app.Backuper) *importing.Service {
	return importing.New(memstore.New(), fixedClock{time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC)}, b, nil)
}

func fileNames(m memFiles) []string {
	var out []string
	for n := range m {
		out = append(out, n)
	}
	return out
}

func TestDumpAll(t *testing.T) {
	ctx := context.Background()
	typed := func(ext string) []string {
		var out []string
		for _, n := range []string{"accounts", "categories", "transactions", "transfers", "fx_rates", "import_batches", "saved_simulations", "dismissed_insights"} {
			out = append(out, n+ext)
		}
		return out
	}
	tests := []struct {
		name   string
		req    importing.DumpRequest
		want   []string
		absent []string
	}{
		{"json", importing.DumpRequest{Format: importing.DumpJSON, Config: "a = 1\n"}, append(typed(".json"), "dump.json"), []string{"config.toml", "tags.csv"}},
		{"csv", importing.DumpRequest{Format: importing.DumpCSV}, append(typed(".csv"), "tags.csv", "budgets.csv"), []string{"config.toml", "dump.json"}},
		{"csv config", importing.DumpRequest{Format: importing.DumpCSV, Config: "a = 1\n"}, append(typed(".csv"), "tags.csv", "config.toml"), nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			files := memFiles{}
			svc := dumpService(&dumpBackup{dump: fakeDump()})
			if err := svc.DumpAll(ctx, tc.req, files.create); err != nil {
				t.Fatal(err)
			}
			for _, n := range tc.want {
				if files[n] == nil {
					t.Errorf("missing %s in %v", n, fileNames(files))
				}
			}
			for _, n := range tc.absent {
				if files[n] != nil {
					t.Errorf("unexpected %s", n)
				}
			}
			if files["accounts.csv"] != nil && strings.Contains(files["accounts.csv"].String(), "\r\n1\r\n") {
				t.Error("accounts.csv must come from the typed export, not the generic table")
			}
			if tc.req.Format == importing.DumpCSV {
				if got := files["tags.csv"].String(); got != "id,name\r\n1,food\r\n2,\r\n" {
					t.Errorf("tags.csv = %q", got)
				}
			}
		})
	}

	t.Run("config embedded and deterministic", func(t *testing.T) {
		var out [2]memFiles
		for i := range out {
			out[i] = memFiles{}
			err := dumpService(&dumpBackup{dump: fakeDump()}).DumpAll(ctx,
				importing.DumpRequest{Format: importing.DumpJSON, Config: "theme = \"dark\"\n"}, out[i].create)
			if err != nil {
				t.Fatal(err)
			}
		}
		if out[0]["dump.json"].String() != out[1]["dump.json"].String() {
			t.Error("dump.json differs between runs")
		}
		d, cfg, err := importing.DecodeDump(out[0]["dump.json"])
		if err != nil {
			t.Fatal(err)
		}
		if cfg != "theme = \"dark\"\n" || d.SchemaVersion != 7 || len(d.Tables) != 3 || d.Tables[0].Name != "accounts" {
			t.Errorf("decoded %+v config %q", d, cfg)
		}
	})

	t.Run("dump() has no config", func(t *testing.T) {
		files := memFiles{}
		if err := dumpService(&dumpBackup{dump: fakeDump()}).Dump(ctx, importing.DumpJSON, files.create); err != nil {
			t.Fatal(err)
		}
		if _, cfg, err := importing.DecodeDump(files["dump.json"]); err != nil || cfg != "" {
			t.Fatalf("cfg=%q err=%v", cfg, err)
		}
	})

	t.Run("unsupported backuper", func(t *testing.T) {
		err := dumpService(&spyBackup{}).DumpAll(ctx, importing.DumpRequest{Format: importing.DumpJSON}, memFiles{}.create)
		if err == nil || !strings.Contains(err.Error(), "does not support") {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("dumper failure", func(t *testing.T) {
		boom := errors.New("boom")
		err := dumpService(&dumpBackup{err: boom}).DumpAll(ctx, importing.DumpRequest{Format: importing.DumpJSON}, memFiles{}.create)
		if !errors.Is(err, boom) {
			t.Fatalf("err = %v", err)
		}
	})
}
