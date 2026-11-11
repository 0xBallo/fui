// Package main tests conformance of the Revolut connector binary.
package main

import (
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/0xballo/fui/pkg/fuiproto/conformance"
)

func TestConformance(t *testing.T) {
	connector := buildConnectorBinary(t)
	input := filepath.Join("testdata", "revolut-sample.csv")

	conformance.Run(t, conformance.RunOptions{
		ConnectorPath: connector,
		InputFiles:    []string{input},
		Config:        map[string]any{},
		BadConfig: map[string]any{
			"unexpected": true,
		},
	})
}

func buildConnectorBinary(t *testing.T) string {
	t.Helper()

	binary := filepath.Join(t.TempDir(), "fui-connector-revolut")
	cmd := exec.Command("go", "build", "-o", binary, ".")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build connector binary: %v\n%s", err, string(output))
	}
	return binary
}
