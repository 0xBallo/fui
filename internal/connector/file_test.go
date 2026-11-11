package connector

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const fuiFile = `{"type":"meta","protocol":"fui/1","connector":"fui","version":"1"}
{"type":"account","ref":"a","name":"A","currency":"EUR","kind":"checking"}
{"type":"transaction","account":"a","id":"1","date":"2026-10-01","amount":-100,"currency":"EUR","description":"x","status":"booked"}
{"type":"end","transactions":1,"errors":0}
`

func write(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestFileSourceFillsMissingSource(t *testing.T) {
	path := write(t, "movements.fui.jsonl", fuiFile)
	recs, err := collect(t, FileSource{Path: path}.Records(ctx()))
	if err != nil || len(recs) != 4 {
		t.Fatalf("records=%d err=%v", len(recs), err)
	}
	sum := sha256.Sum256([]byte(fuiFile))
	if got := recs[0].Meta.Source; got.File != "movements.fui.jsonl" || got.SHA256 != hex.EncodeToString(sum[:]) {
		t.Fatalf("source = %+v", got)
	}
}

func TestFileSourceKeepsDeclaredSource(t *testing.T) {
	declared := strings.Replace(fuiFile, `"version":"1"}`, `"version":"1","source":{"file":"bank.csv","sha256":"cafe"}}`, 1)
	recs, err := collect(t, FileSource{Path: write(t, "x.fui.jsonl", declared)}.Records(ctx()))
	if err != nil {
		t.Fatal(err)
	}
	if got := recs[0].Meta.Source; got.File != "bank.csv" || got.SHA256 != "cafe" {
		t.Fatalf("source = %+v", got)
	}
}

func TestFileSourceErrors(t *testing.T) {
	if _, err := collect(t, FileSource{Path: filepath.Join(t.TempDir(), "missing")}.Records(ctx())); err == nil {
		t.Fatal("missing file: no error")
	}
	if _, err := collect(t, FileSource{Path: write(t, "bad.jsonl", "not json\n")}.Records(ctx())); err == nil {
		t.Fatal("garbage: no error")
	}
}

func TestFileSourceStopsEarly(t *testing.T) {
	n := 0
	for _, err := range (FileSource{Path: write(t, "x.fui.jsonl", fuiFile)}).Records(ctx()) {
		if err != nil {
			t.Fatal(err)
		}
		n++
		break
	}
	if n != 1 {
		t.Fatalf("saw %d records", n)
	}
}
