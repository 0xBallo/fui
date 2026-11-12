package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestBailRendersErrorAndExitCode(t *testing.T) {
	for _, asJSON := range []bool{false, true} {
		var buf bytes.Buffer
		code := bail(&buf, asJSON, errors.New("boom"))
		if code == 0 || !strings.Contains(buf.String(), "boom") {
			t.Fatalf("bail(json=%v): code=%d out=%q", asJSON, code, buf.String())
		}
	}
}
