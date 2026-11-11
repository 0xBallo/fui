// Package main implements the Widiba fui/1 connector binary.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/0xballo/fui/pkg/fuiproto"
)

const (
	connectorName    = "widiba"
	connectorVersion = "0.1.0"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		writeStderr(stderr, "usage: fui-connector-widiba <describe|fetch>\n")
		return 1
	}

	switch args[0] {
	case "describe":
		return runDescribe(stdout, stderr)
	case "fetch":
		return runFetch(stdin, stdout, stderr)
	default:
		writeStderr(stderr, "unknown command %q\n", args[0])
		return 1
	}
}

func runDescribe(stdout, stderr io.Writer) int {
	response := fuiproto.DescribeResponse{
		Protocol:          fuiproto.ProtocolFUI1,
		Name:              connectorName,
		Version:           connectorVersion,
		AcceptedFileTypes: []string{".xlsx"},
	}

	if err := json.NewEncoder(stdout).Encode(response); err != nil {
		writeStderr(stderr, "encode describe response: %v\n", err)
		return 1
	}
	return fuiproto.ExitCodeOK
}

func runFetch(stdin io.Reader, stdout, stderr io.Writer) int {
	request, code := decodeFetchRequest(stdin, stderr)
	if code != fuiproto.ExitCodeOK {
		return code
	}

	content, code := readSourceFile(request, stderr)
	if code != fuiproto.ExitCodeOK {
		return code
	}

	result, err := parseWidibaWorkbook(content, request.Since)
	if err != nil {
		writeStderr(stderr, "parse source: %v\n", err)
		return fuiproto.ExitCodeUnreadableSource
	}

	if err := emitStream(stdout, *request.File, content, result); err != nil {
		writeStderr(stderr, "emit stream: %v\n", err)
		return 1
	}
	return fuiproto.ExitCodeOK
}

func decodeFetchRequest(stdin io.Reader, stderr io.Writer) (fuiproto.FetchRequest, int) {
	var request fuiproto.FetchRequest
	if err := json.NewDecoder(stdin).Decode(&request); err != nil {
		writeStderr(stderr, "decode fetch request: %v\n", err)
		return fuiproto.FetchRequest{}, fuiproto.ExitCodeBadConfig
	}

	if err := validateFetchRequest(request); err != nil {
		writeStderr(stderr, "%v\n", err)
		return fuiproto.FetchRequest{}, fuiproto.ExitCodeBadConfig
	}
	return request, fuiproto.ExitCodeOK
}

func validateFetchRequest(request fuiproto.FetchRequest) error {
	if request.Protocol != fuiproto.ProtocolFUI1 {
		return fmt.Errorf("unsupported protocol %q", request.Protocol)
	}

	if len(request.Config) > 0 {
		return fmt.Errorf("this connector does not accept config parameters")
	}

	if request.Since != nil {
		if err := fuiproto.ValidateDate(*request.Since); err != nil {
			return fmt.Errorf("invalid since date: %w", err)
		}
	}

	if request.File == nil || *request.File == "" {
		return fmt.Errorf("missing source file")
	}
	return nil
}

func readSourceFile(request fuiproto.FetchRequest, stderr io.Writer) ([]byte, int) {
	content, err := os.ReadFile(*request.File)
	if err != nil {
		writeStderr(stderr, "read source file: %v\n", err)
		return nil, fuiproto.ExitCodeUnreadableSource
	}
	return content, fuiproto.ExitCodeOK
}

func writeStderr(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...)
}
