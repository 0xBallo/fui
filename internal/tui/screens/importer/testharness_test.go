package importer_test

import (
	"context"
	"fmt"
	"iter"
	"time"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/importing"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/testutil/memstore"
	"github.com/0xballo/fui/pkg/fuiproto"
)

// fixedClock is a deterministic app.Clock: every test runs as if "now" were 2026-06-15 09:00 UTC.
type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

// stubBackup is a no-op app.Backuper.
type stubBackup struct{ n int }

func (b *stubBackup) Backup(context.Context, string) (string, error) {
	b.n++
	return "/backups/test.db", nil
}

func (b *stubBackup) Restore(context.Context, string) error { return nil }

// fakeConnectors is a minimal importing.Connectors for screen tests: it replays a fixed record
// set per connector name, set by the test, so tests can exercise the screen's flow without a
// real connector process (internal/tui may not import internal/connector, enforced by
// .golangci.yml depguard).
type fakeConnectors struct {
	fixtures map[string][]fuiproto.Record // connector name -> records of one fui/1 stream
}

func newFakeConnectors() *fakeConnectors {
	return &fakeConnectors{fixtures: map[string][]fuiproto.Record{}}
}

func (f *fakeConnectors) List(context.Context) ([]importing.ConnectorInfo, error) {
	return []importing.ConnectorInfo{
		{Name: "csv", Kind: importing.ConnectorBuiltin},
		{Name: "fui", Kind: importing.ConnectorBuiltin},
	}, nil
}

func (f *fakeConnectors) Describe(_ context.Context, name string) (fuiproto.DescribeResponse, error) {
	return fuiproto.DescribeResponse{Protocol: fuiproto.ProtocolFUI1, Name: name, Version: "test"}, nil
}

func (f *fakeConnectors) Open(name string, _ fuiproto.FetchRequest) (importing.Source, error) {
	recs, ok := f.fixtures[name]
	if !ok {
		return nil, &domain.Error{
			Code: domain.CodeNotFound, Message: fmt.Sprintf("no fixture for connector %q", name),
			Details: map[string]any{"query": name},
		}
	}
	return recordSource{records: recs}, nil
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

// fixture bundles an app.Service (for ListAccounts) and an importing.Service (for PlanImport,
// ApplyImport and ListConnectors) over the same memstore, forwarding to both so it satisfies
// importer.UseCases directly.
type fixture struct {
	*app.Service
	imp        *importing.Service
	store      *memstore.Store
	connectors *fakeConnectors
}

func newFixture() fixture {
	cur, _ := domain.ParseCurrency("EUR")
	store := memstore.New()
	clock := fixedClock{now: time.Date(2026, 6, 15, 9, 0, 0, 0, time.UTC)}
	backup := &stubBackup{}
	svc := app.New(store, clock, backup, app.Config{Locale: domain.LocaleENUS, Location: time.UTC, BaseCurrency: cur})
	connectors := newFakeConnectors()
	imp := importing.New(store, clock, backup, connectors)
	return fixture{Service: svc, imp: imp, store: store, connectors: connectors}
}

// ListConnectors, PlanImport and ApplyImport forward to the fixture's importing.Service, so
// fixture satisfies importer.UseCases alongside the embedded *app.Service.ListAccounts.
func (f fixture) ListConnectors(ctx context.Context) ([]importing.ConnectorInfo, error) {
	return f.imp.ListConnectors(ctx)
}

func (f fixture) PlanImport(ctx context.Context, req importing.PlanRequest) (importing.ImportPlan, error) {
	return f.imp.PlanImport(ctx, req)
}

func (f fixture) ApplyImport(ctx context.Context, p importing.ImportPlan) (importing.ImportResult, error) {
	return f.imp.ApplyImport(ctx, p)
}
