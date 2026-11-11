package main

import (
	"bytes"
	"testing"
)

func FuzzParseWidibaWorkbook(f *testing.F) {
	f.Add([]byte("not-an-xlsx"))

	f.Fuzz(func(_ *testing.T, data []byte) {
		_, _ = parseWidibaWorkbook(bytes.Clone(data), nil)
	})
}
