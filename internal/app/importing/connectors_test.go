package importing_test

import (
	"context"
	"errors"
	"testing"

	"github.com/0xballo/fui/internal/app/importing"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/pkg/fuiproto"
)

type fakeConnectors struct {
	infos []importing.ConnectorInfo
	src   map[string]importing.Source
	asked fuiproto.FetchRequest
}

func (f *fakeConnectors) List(context.Context) ([]importing.ConnectorInfo, error) {
	return f.infos, nil
}

func (f *fakeConnectors) Describe(_ context.Context, name string) (fuiproto.DescribeResponse, error) {
	return fuiproto.DescribeResponse{Protocol: fuiproto.ProtocolFUI1, Name: name, Version: "1"}, nil
}

func (f *fakeConnectors) Open(name string, req fuiproto.FetchRequest) (importing.Source, error) {
	f.asked = req
	s, ok := f.src[name]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return s, nil
}

func newFake() *fakeConnectors {
	good := stream("bank", "sha", account("r1", "R1", "EUR"),
		txRec("r1", "a", "2026-10-01", -100, "x"), txRec("r1", "b", "2026-13-01", -100, "bad date"))
	good.stderr = "warming up"
	return &fakeConnectors{
		infos: []importing.ConnectorInfo{{Name: "bank", Kind: importing.ConnectorExternal, Path: "/x/fui-connector-bank"}, {Name: "csv", Kind: importing.ConnectorBuiltin}},
		src:   map[string]importing.Source{"bank": good},
	}
}

func TestListAndShowConnectors(t *testing.T) {
	e := newEnv(newFake())
	list, err := e.svc.ListConnectors(context.Background())
	if err != nil || len(list) != 2 {
		t.Fatalf("list = %+v, %v", list, err)
	}
	d, err := e.svc.ShowConnector(context.Background(), "bank")
	if err != nil || d.Info.Path == "" || d.Describe.Name != "bank" {
		t.Fatalf("show = %+v, %v", d, err)
	}
	if _, err := e.svc.ShowConnector(context.Background(), "nope"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("show unknown err = %v", err)
	}
}

func TestConnectorsWithoutRegistry(t *testing.T) {
	e := newEnv(nil)
	if list, err := e.svc.ListConnectors(context.Background()); err != nil || list != nil {
		t.Fatalf("list = %+v, %v", list, err)
	}
	if _, err := e.svc.ShowConnector(context.Background(), "x"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("show err = %v", err)
	}
	if _, err := e.svc.TestConnector(context.Background(), "x", fuiproto.FetchRequest{}); !errors.Is(err, domain.ErrMissingField) {
		t.Fatalf("test err = %v", err)
	}
}

func TestTestConnectorValidatesWithoutImporting(t *testing.T) {
	fake := newFake()
	e := newEnv(fake)
	before := e.store.Snapshot()
	rep, err := e.svc.TestConnector(context.Background(), "bank", fuiproto.FetchRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Connector != "bank" || len(rep.Accounts) != 1 || rep.Transactions != 1 || len(rep.Errors) != 1 || rep.Stderr != "warming up" {
		t.Fatalf("report = %+v", rep)
	}
	if fake.asked.Protocol != fuiproto.ProtocolFUI1 {
		t.Fatalf("fetch protocol = %q", fake.asked.Protocol)
	}
	if e.store.Snapshot() != before || len(e.backup.reasons) != 0 {
		t.Fatal("TestConnector wrote")
	}
	if _, err := e.svc.TestConnector(context.Background(), "missing", fuiproto.FetchRequest{}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("unknown connector err = %v", err)
	}
}

func TestPlanImportThroughConnector(t *testing.T) {
	e := newEnv(newFake())
	plan, err := e.svc.PlanImport(context.Background(), importing.PlanRequest{Connector: "bank", CreateAccounts: true})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Connector != "bank" || len(plan.Rows) != 1 || len(plan.Errors) != 1 || plan.Stderr != "warming up" {
		t.Fatalf("plan = %+v", plan)
	}
}
