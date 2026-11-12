package main

import (
	"bytes"
	"context"
	"iter"
	"path/filepath"
	"testing"
	"time"

	"github.com/0xballo/fui/internal/app/importing"
	"github.com/0xballo/fui/internal/testutil/memstore"
	"github.com/0xballo/fui/pkg/fuiproto"
)

type importingFixedClock struct{}

func (importingFixedClock) Now() time.Time { return time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC) }

type importingNopBackup struct{}

func (importingNopBackup) Backup(context.Context, string) (string, error) { return "backup.db", nil }

func (importingNopBackup) Restore(context.Context, string) error { return nil }

// replayConnectors is both the importing.Connectors and the importing.Source: Open always
// serves the same fixed slice of records, so the test below can feed importing.Service the
// exact bytes this connector's own "fetch" produced.
type replayConnectors struct{ recs []fuiproto.Record }

func (c replayConnectors) List(context.Context) ([]importing.ConnectorInfo, error) { return nil, nil }

func (c replayConnectors) Describe(context.Context, string) (fuiproto.DescribeResponse, error) {
	return fuiproto.DescribeResponse{}, nil
}

func (c replayConnectors) Open(string, fuiproto.FetchRequest) (importing.Source, error) {
	return c, nil
}

func (c replayConnectors) Records(context.Context) iter.Seq2[fuiproto.Record, error] {
	return func(yield func(fuiproto.Record, error) bool) {
		for _, r := range c.recs {
			if !yield(r, nil) {
				return
			}
		}
	}
}

// TestImportingRevolutSampleHasNoDiscrepancies feeds the connector's own fetch output (the exact
// bytes "fui import" would consume) into importing.Service.PlanImport: the fixture's running
// Balance column must be internally coherent, once pending movements are excluded from the
// chain, for every booked row.
func TestImportingRevolutSampleHasNoDiscrepancies(t *testing.T) {
	path := filepath.Join("testdata", "revolut-sample.csv")
	stdout, stderr, code := runFetchForTest(t, fuiproto.FetchRequest{Protocol: fuiproto.ProtocolFUI1, File: &path})
	if code != fuiproto.ExitCodeOK {
		t.Fatalf("exit code = %d, stderr=%q", code, stderr)
	}

	var recs []fuiproto.Record
	for rec, err := range fuiproto.DecodeJSONL(bytes.NewReader(stdout)) {
		if err != nil {
			t.Fatalf("decode fetch output: %v", err)
		}
		recs = append(recs, rec)
	}

	svc := importing.New(memstore.New(), importingFixedClock{}, importingNopBackup{}, replayConnectors{recs: recs})
	plan, err := svc.PlanImport(context.Background(), importing.PlanRequest{
		Connector: connectorName, CreateAccounts: true,
	})
	if err != nil {
		t.Fatalf("plan import: %v", err)
	}
	if len(plan.Discrepancies) != 0 {
		t.Fatalf("discrepancies = %+v, want none", plan.Discrepancies)
	}
}
