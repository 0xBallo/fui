package connector

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/0xballo/fui/internal/app/importing"
	"github.com/0xballo/fui/internal/config"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/pkg/fuiproto"
)

func names(infos []importing.ConnectorInfo) []string {
	var out []string
	for _, i := range infos {
		out = append(out, i.Name)
	}
	return out
}

func TestRegistryList(t *testing.T) {
	dir, pathDir := t.TempDir(), t.TempDir()
	touch := func(d, name string, mode os.FileMode) {
		if err := os.WriteFile(filepath.Join(d, name), []byte("#!/bin/sh\n"), mode); err != nil {
			t.Fatal(err)
		}
	}
	touch(dir, "fui-connector-widiba", 0o700)
	touch(dir, "fui-connector-notexec", 0o600) // not executable
	touch(dir, "fui-connector-Bad Name", 0o700)
	touch(dir, "other-tool", 0o700)
	touch(dir, "fui-connector-csv", 0o700)         // cannot shadow a built-in
	touch(pathDir, "fui-connector-revolut", 0o700) // found in the PATH
	touch(pathDir, "fui-connector-widiba", 0o700)  // the config directory wins
	r := Registry{Dir: dir, Path: pathDir + string(os.PathListSeparator) + "/does/not/exist"}
	got, err := r.List(ctx())
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"csv", "fui", "revolut", "widiba"}; !slices.Equal(names(got), want) {
		t.Fatalf("names = %v, want %v", names(got), want)
	}
	for _, c := range got {
		switch c.Name {
		case "csv", "fui":
			if c.Kind != importing.ConnectorBuiltin || c.Path != "" {
				t.Errorf("%+v", c)
			}
		case "widiba":
			if c.Kind != importing.ConnectorExternal || c.Path != filepath.Join(dir, "fui-connector-widiba") {
				t.Errorf("%+v", c)
			}
		}
	}
}

func TestRegistryDescribe(t *testing.T) {
	r := Registry{Dir: fakeConnectorDir(t), Path: t.TempDir()}
	for _, name := range []string{"csv", "fui", "fake"} {
		d, err := r.Describe(ctx(), name)
		if err != nil || d.Protocol != fuiproto.ProtocolFUI1 || d.Name != name {
			t.Fatalf("describe %s = %+v, %v", name, d, err)
		}
	}
	if d, _ := r.Describe(ctx(), "csv"); len(d.ConfigParams) == 0 {
		t.Fatal("csv describes no parameters")
	}
	if _, err := r.Describe(ctx(), "nope"); err == nil {
		t.Fatal("unknown connector: no error")
	}
}

func TestRegistryOpen(t *testing.T) {
	prof := profile()
	r := Registry{Dir: fakeConnectorDir(t), Path: t.TempDir(), Timeout: 5 * time.Second, Profiles: map[string]config.CSVProfile{"bank": prof}, Currency: "EUR"}
	csvPath := write(t, "s.csv", "Date,Amount,Description\n2026-10-01,-1.00,x\n2026-10-03,-2.00,y\n")
	since := "2026-10-02"

	t.Run("csv", func(t *testing.T) {
		src, err := r.Open("csv", fuiproto.FetchRequest{File: &csvPath, Since: &since, Config: map[string]any{"profile": "bank", "account": "Card", "kind": "card"}})
		if err != nil {
			t.Fatal(err)
		}
		recs, err := collect(t, src.Records(ctx()))
		if err != nil || len(recs) != 4 || recs[1].Account.Ref != "Card" || recs[1].Account.Kind != fuiproto.AccountKindCard {
			t.Fatalf("records=%d err=%v", len(recs), err)
		}
	})
	t.Run("fui", func(t *testing.T) {
		path := write(t, "x.fui.jsonl", fuiFile)
		src, err := r.Open("fui", fuiproto.FetchRequest{File: &path})
		if err != nil {
			t.Fatal(err)
		}
		if recs, err := collect(t, src.Records(ctx())); err != nil || len(recs) != 4 {
			t.Fatalf("records=%d err=%v", len(recs), err)
		}
	})
	t.Run("external", func(t *testing.T) {
		src, err := r.Open("fake", fetchMode("ok"))
		if err != nil {
			t.Fatal(err)
		}
		if recs, err := collect(t, src.Records(ctx())); err != nil || len(recs) != 5 {
			t.Fatalf("records=%d err=%v", len(recs), err)
		}
	})
	t.Run("errors", func(t *testing.T) {
		if _, err := r.Open("csv", fuiproto.FetchRequest{}); err == nil {
			t.Error("csv without file: no error")
		}
		if _, err := r.Open("fui", fuiproto.FetchRequest{}); err == nil {
			t.Error("fui without file: no error")
		}
		if _, err := r.Open("csv", fuiproto.FetchRequest{File: &csvPath, Config: map[string]any{"profile": "nope"}}); err == nil {
			t.Error("unknown profile: no error")
		}
		_, err := r.Open("nope", fetchMode("ok"))
		if !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("unknown connector err = %v", err)
		}
	})
}
