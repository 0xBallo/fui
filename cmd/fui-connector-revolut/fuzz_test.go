package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func FuzzParseRevolutCSV(f *testing.F) {
	seedPath := filepath.Join("testdata", "revolut-sample.csv")
	seed, err := os.ReadFile(seedPath)
	if err != nil {
		f.Fatalf("read seed file: %v", err)
	}

	f.Add(seed)
	f.Fuzz(func(_ *testing.T, data []byte) {
		_, _ = parseRevolutCSV(bytes.NewReader(data), nil)
	})
}
