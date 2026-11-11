// Package conformance runs connector protocol checks against fui/1 streams.
package conformance

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"slices"
	"testing"

	"github.com/0xballo/fui/pkg/fuiproto"
)

// RunOptions configures a connector conformance run.
type RunOptions struct {
	ConnectorPath string
	InputFiles    []string
	Config        map[string]any
	Since         *string
	BadConfig     map[string]any
}

// Run executes the connector conformance checks.
func Run(t *testing.T, opts RunOptions) {
	t.Helper()

	if err := opts.validate(); err != nil {
		t.Fatalf("invalid conformance options: %v", err)
	}

	describe := runDescribe(t, opts.ConnectorPath)
	if describe.Protocol != fuiproto.ProtocolFUI1 {
		t.Fatalf("describe protocol mismatch: got %q want %q", describe.Protocol, fuiproto.ProtocolFUI1)
	}

	for _, inputFile := range opts.InputFiles {
		first := runFetchAndValidate(t, opts.ConnectorPath, opts.Config, inputFile, opts.Since)
		second := runFetchAndValidate(t, opts.ConnectorPath, opts.Config, inputFile, opts.Since)
		if !slices.Equal(first, second) {
			t.Fatalf("transaction IDs are not stable for file %q: %v vs %v", inputFile, first, second)
		}
	}

	if opts.BadConfig != nil {
		assertBadConfigExit(t, opts)
	}
}

func (o RunOptions) validate() error {
	if o.ConnectorPath == "" {
		return fmt.Errorf("connector path is required")
	}
	if len(o.InputFiles) == 0 {
		return fmt.Errorf("at least one input file is required")
	}
	return nil
}

func runDescribe(t *testing.T, connectorPath string) fuiproto.DescribeResponse {
	t.Helper()

	stdout, stderr, exitCode, err := runCommand(connectorPath, []string{"describe"}, nil)
	if err != nil {
		t.Fatalf("run describe: %v", err)
	}
	if exitCode != fuiproto.ExitCodeOK {
		t.Fatalf("describe exit code = %d, stderr=%q", exitCode, string(stderr))
	}

	var describe fuiproto.DescribeResponse
	if err := json.Unmarshal(stdout, &describe); err != nil {
		t.Fatalf("decode describe output: %v", err)
	}
	return describe
}

func runFetchAndValidate(t *testing.T, connectorPath string, config map[string]any, inputFile string, since *string) []string {
	t.Helper()

	request := fuiproto.FetchRequest{
		Protocol: fuiproto.ProtocolFUI1,
		Config:   config,
		File:     &inputFile,
		Since:    since,
	}

	requestBytes, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("marshal fetch request: %v", err)
	}

	stdout, stderr, exitCode, err := runCommand(connectorPath, []string{"fetch"}, requestBytes)
	if err != nil {
		t.Fatalf("run fetch: %v", err)
	}
	if exitCode != fuiproto.ExitCodeOK {
		t.Fatalf("fetch exit code = %d, stderr=%q", exitCode, string(stderr))
	}

	result, err := fuiproto.ValidateStream(fuiproto.DecodeJSONL(bytes.NewReader(stdout)))
	if err != nil {
		t.Fatalf("validate stream: %v", err)
	}
	if len(result.RowErrors) > 0 {
		t.Fatalf("row-level stream errors: %v", result.RowErrors)
	}

	return transactionIDs(result.Records)
}

func assertBadConfigExit(t *testing.T, opts RunOptions) {
	t.Helper()

	request := fuiproto.FetchRequest{
		Protocol: fuiproto.ProtocolFUI1,
		Config:   opts.BadConfig,
		File:     &opts.InputFiles[0],
		Since:    opts.Since,
	}

	requestBytes, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("marshal bad-config fetch request: %v", err)
	}

	_, _, exitCode, err := runCommand(opts.ConnectorPath, []string{"fetch"}, requestBytes)
	if err != nil {
		t.Fatalf("run bad-config fetch: %v", err)
	}
	if exitCode != fuiproto.ExitCodeBadConfig {
		t.Fatalf("bad-config exit code = %d, want %d", exitCode, fuiproto.ExitCodeBadConfig)
	}
}

func transactionIDs(records []fuiproto.Record) []string {
	ids := make([]string, 0)
	for _, record := range records {
		if record.Type == fuiproto.RecordTypeTransaction && record.Transaction != nil {
			ids = append(ids, record.Transaction.Account+":"+record.Transaction.ID)
		}
	}
	return ids
}

func runCommand(binary string, args []string, stdin []byte) ([]byte, []byte, int, error) {
	// #nosec G204 -- connector path and arguments are explicit test/runtime inputs.
	cmd := exec.Command(binary, args...)
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		return stdout.Bytes(), stderr.Bytes(), fuiproto.ExitCodeOK, nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return stdout.Bytes(), stderr.Bytes(), exitErr.ExitCode(), nil
	}

	return nil, nil, 0, fmt.Errorf("start connector command %q: %w", cmd.String(), err)
}
