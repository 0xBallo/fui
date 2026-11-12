package cli_test

import (
	"bytes"
	"context"
	"fmt"
	"iter"
	"os"
	"path/filepath"
	"testing"

	"github.com/0xballo/fui/internal/app/importing"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/pkg/fuiproto"
)

// fakeConnectors is a minimal importing.Connectors for CLI command tests. internal/cli may only
// import internal/app (never internal/connector, enforced by .golangci.yml), so it cannot reuse
// the real connector.Registry: "fui" decodes a real .fui.jsonl file from disk with the published
// pkg/fuiproto codec (exercising the CLI's default-connector-by-suffix logic genuinely); "csv"
// replays a fixed record set per --profile, set by the test, so command tests can exercise the
// CLI's flag plumbing without a real CSV parser.
type fakeConnectors struct {
	csvFixtures map[string][]fuiproto.Record // profile -> records of one fui/1 stream, meta and end included
	openErr     error                        // when set, the next Open fails with it instead
}

func (f *fakeConnectors) List(context.Context) ([]importing.ConnectorInfo, error) {
	return []importing.ConnectorInfo{
		{Name: "csv", Kind: importing.ConnectorBuiltin},
		{Name: "fui", Kind: importing.ConnectorBuiltin},
	}, nil
}

func (f *fakeConnectors) Describe(_ context.Context, name string) (fuiproto.DescribeResponse, error) {
	switch name {
	case "fui":
		return fuiproto.DescribeResponse{
			Protocol: fuiproto.ProtocolFUI1, Name: "fui", Version: "test", AcceptedFileTypes: []string{".fui.jsonl"},
		}, nil
	case "csv":
		return fuiproto.DescribeResponse{
			Protocol: fuiproto.ProtocolFUI1, Name: "csv", Version: "test", AcceptedFileTypes: []string{".csv"},
			ConfigParams: []fuiproto.ConfigParam{{Key: "profile", Type: "string", Required: true, Description: "test fixture key"}},
		}, nil
	}
	return fuiproto.DescribeResponse{}, &domain.Error{Code: domain.CodeNotFound, Message: fmt.Sprintf("no connector named %q", name)}
}

func (f *fakeConnectors) Open(name string, req fuiproto.FetchRequest) (importing.Source, error) {
	if f.openErr != nil {
		err := f.openErr
		f.openErr = nil
		return nil, err
	}
	if req.File == nil || *req.File == "" {
		return nil, &domain.Error{Code: domain.CodeMissingField, Message: name + " connector needs a file", Details: map[string]any{"field": "file"}}
	}
	if name == "fui" {
		data, err := os.ReadFile(*req.File)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", *req.File, err)
		}
		return fileSource{data: data}, nil
	}
	profile, _ := req.Config["profile"].(string)
	recs, ok := f.csvFixtures[profile]
	if !ok {
		return nil, &domain.Error{
			Code: domain.CodeNotFound, Message: fmt.Sprintf("no csv profile named %q", profile),
			Details: map[string]any{"query": profile},
		}
	}
	return recordSource{records: recs}, nil
}

// fileSource decodes the fui/1 JSONL content of data, read once from disk by fakeConnectors.
type fileSource struct{ data []byte }

func (s fileSource) Records(context.Context) iter.Seq2[fuiproto.Record, error] {
	return fuiproto.DecodeJSONL(bytes.NewReader(s.data))
}

// recordSource replays a fixed, already-valid slice of records.
type recordSource struct{ records []fuiproto.Record }

func (s recordSource) Records(context.Context) iter.Seq2[fuiproto.Record, error] {
	return func(yield func(fuiproto.Record, error) bool) {
		for _, r := range s.records {
			if !yield(r, nil) {
				return
			}
		}
	}
}

// fuiStream wraps recs with the meta and end records a fui/1 stream needs.
func fuiStream(connector, source, sha string, recs ...fuiproto.Record) []fuiproto.Record {
	out := []fuiproto.Record{{
		Type: fuiproto.RecordTypeMeta,
		Meta: &fuiproto.MetaRecord{
			Protocol: fuiproto.ProtocolFUI1, Connector: connector, Version: "1",
			Source: fuiproto.SourceMeta{File: source, SHA256: sha},
		},
	}}
	out = append(out, recs...)
	end := fuiproto.EndRecord{}
	for _, r := range recs {
		if r.Transaction != nil {
			end.Transactions++
		}
		if r.Error != nil {
			end.Errors++
		}
	}
	return append(out, fuiproto.Record{Type: fuiproto.RecordTypeEnd, End: &end})
}

// fuiAccount builds one checking account record.
func fuiAccount(ref, name, currency string) fuiproto.Record {
	return fuiproto.Record{Type: fuiproto.RecordTypeAccount, Account: &fuiproto.AccountRecord{
		Ref: ref, Name: name, Currency: currency, Kind: fuiproto.AccountKindChecking,
	}}
}

// fuiTx builds one booked payment transaction record.
func fuiTx(ref, id, date string, amount int64, desc string) fuiproto.Record {
	return fuiproto.Record{Type: fuiproto.RecordTypeTransaction, Transaction: &fuiproto.TransactionRecord{
		Account: ref, ID: id, Date: date, Amount: amount, Currency: "EUR", Description: desc,
		Status: fuiproto.TransactionStatusBooked, Kind: fuiproto.TransactionKindPayment,
	}}
}

// writeFUIFile encodes recs as fui/1 JSONL under dir/name and returns the full path.
func writeFUIFile(t *testing.T, dir, name string, recs ...fuiproto.Record) string {
	t.Helper()
	var buf bytes.Buffer
	if err := fuiproto.EncodeSliceJSONL(&buf, recs); err != nil {
		t.Fatalf("encode fui stream: %v", err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}
