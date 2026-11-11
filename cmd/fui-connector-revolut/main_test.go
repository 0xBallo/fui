package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/0xballo/fui/pkg/fuiproto"
)

var updateGolden = flag.Bool("update", false, "update golden files")

func TestDescribe(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"describe"}, bytes.NewReader(nil), &stdout, &stderr)
	if code != fuiproto.ExitCodeOK {
		t.Fatalf("exit code = %d, stderr=%q", code, stderr.String())
	}

	var response fuiproto.DescribeResponse
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		t.Fatalf("decode describe response: %v", err)
	}
	if response.Protocol != fuiproto.ProtocolFUI1 {
		t.Fatalf("protocol = %q, want %q", response.Protocol, fuiproto.ProtocolFUI1)
	}
	if response.Name != connectorName {
		t.Fatalf("name = %q, want %q", response.Name, connectorName)
	}
}

func TestFetchGolden(t *testing.T) {
	path := filepath.Join("testdata", "revolut-sample.csv")
	request := fuiproto.FetchRequest{
		Protocol: fuiproto.ProtocolFUI1,
		File:     &path,
	}

	stdout, stderr, code := runFetchForTest(t, request)
	if code != fuiproto.ExitCodeOK {
		t.Fatalf("exit code = %d, stderr=%q", code, stderr)
	}

	validateConnectorOutput(t, stdout)
	assertGolden(t, filepath.Join("testdata", "fetch.golden.jsonl"), stdout)
}

func TestFetchInvalidConfig(t *testing.T) {
	path := filepath.Join("testdata", "revolut-sample.csv")
	request := fuiproto.FetchRequest{
		Protocol: fuiproto.ProtocolFUI1,
		File:     &path,
		Config: map[string]any{
			"unexpected": true,
		},
	}

	_, _, code := runFetchForTest(t, request)
	if code != fuiproto.ExitCodeBadConfig {
		t.Fatalf("exit code = %d, want %d", code, fuiproto.ExitCodeBadConfig)
	}
}

func runFetchForTest(t *testing.T, request fuiproto.FetchRequest) ([]byte, string, int) {
	t.Helper()

	payload, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"fetch"}, bytes.NewReader(payload), &stdout, &stderr)
	return stdout.Bytes(), stderr.String(), code
}

func validateConnectorOutput(t *testing.T, output []byte) {
	t.Helper()

	result, err := fuiproto.ValidateStream(fuiproto.DecodeJSONL(bytes.NewReader(output)))
	if err != nil {
		t.Fatalf("validate stream: %v", err)
	}
	if len(result.RowErrors) > 0 {
		t.Fatalf("unexpected row errors: %v", result.RowErrors)
	}
}

func assertGolden(t *testing.T, goldenPath string, got []byte) {
	t.Helper()

	if *updateGolden {
		if err := os.WriteFile(goldenPath, got, 0o600); err != nil {
			t.Fatalf("write golden file: %v", err)
		}
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden file: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("golden mismatch for %s", goldenPath)
	}
}
