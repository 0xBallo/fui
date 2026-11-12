package connector

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"
	"time"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/importing"
	"github.com/0xballo/fui/internal/config"
	"github.com/0xballo/fui/internal/testutil/memstore"
	"github.com/0xballo/fui/pkg/fuiproto"
)

// perfTimingUnreliable reports whether the test binary was built with the race detector, or is
// running with coverage instrumentation. The non-functional 10 000-row timing budget is verified
// by BenchmarkImportTenThousandRows instead of this test in either case, because instrumentation
// inflates wall-clock time well past the budget on CI runners regardless of real performance.
func perfTimingUnreliable() bool {
	if testing.CoverMode() != "" {
		return true
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return false
	}
	for _, setting := range info.Settings {
		if setting.Key == "-race" && setting.Value == "true" {
			return true
		}
	}
	return false
}

type fixedClock struct{}

func (fixedClock) Now() time.Time { return time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC) }

type nopBackup struct{ n int }

func (b *nopBackup) Backup(context.Context, string) (string, error) { b.n++; return "backup.db", nil }
func (b *nopBackup) Restore(context.Context, string) error          { return nil }

func newService(reg importing.Connectors) (*importing.Service, *memstore.Store, *nopBackup) {
	store, backup := memstore.New(), &nopBackup{}
	return importing.New(store, fixedClock{}, backup, reg), store, backup
}

func TestPipelineAbortsWithoutWritingOnConnectorFailure(t *testing.T) {
	reg := Registry{Dir: fakeConnectorDir(t), Path: t.TempDir(), Timeout: 500 * time.Millisecond}
	for _, mode := range []string{"exit3", "fail-after", "truncated", "garbage", "bigline", "hang"} {
		t.Run(mode, func(t *testing.T) {
			svc, store, backup := newService(reg)
			before := store.Snapshot()
			_, err := svc.PlanImport(ctx(), importing.PlanRequest{Connector: "fake", Fetch: fetchMode(mode), CreateAccounts: true})
			if !errors.Is(err, importing.ErrConnectorFailed) && !errors.Is(err, importing.ErrInvalidStream) {
				t.Fatalf("err = %v, want a connector or stream failure", err)
			}
			if store.Snapshot() != before || backup.n != 0 {
				t.Fatal("a failed connector left a trace")
			}
		})
	}
}

func TestPipelineThroughExternalConnector(t *testing.T) {
	svc, _, backup := newService(Registry{Dir: fakeConnectorDir(t), Path: t.TempDir()})
	plan, err := svc.PlanImport(ctx(), importing.PlanRequest{Connector: "fake", Fetch: fetchMode("stderr"), CreateAccounts: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(plan.Stderr, "hello from stderr") || plan.Counts().New != 2 {
		t.Fatalf("plan stderr=%q counts=%+v", plan.Stderr, plan.Counts())
	}
	res, err := svc.ApplyImport(ctx(), plan)
	if err != nil || res.Transactions != 2 || backup.n != 1 {
		t.Fatalf("result=%+v err=%v backups=%d", res, err, backup.n)
	}
}

func TestTestConnectorWithExternalConnector(t *testing.T) {
	svc, store, _ := newService(Registry{Dir: fakeConnectorDir(t), Path: t.TempDir()})
	before := store.Snapshot()
	rep, err := svc.TestConnector(ctx(), "fake", fetchMode("ok"))
	if err != nil || rep.Transactions != 2 || len(rep.Accounts) != 1 {
		t.Fatalf("report=%+v err=%v", rep, err)
	}
	if store.Snapshot() != before {
		t.Fatal("test connector wrote")
	}
	rep, err = svc.TestConnector(ctx(), "fake", fetchMode("exit3"))
	var exit *ExitError
	if !errors.As(err, &exit) || !strings.Contains(rep.Stderr, "cannot read the source") {
		t.Fatalf("report=%+v err=%v", rep, err)
	}
	detail, err := svc.ShowConnector(ctx(), "fake")
	if err != nil || detail.Describe.Version != "1.2.3" || detail.Info.Kind != importing.ConnectorExternal {
		t.Fatalf("show = %+v, %v", detail, err)
	}
}

// writeBigCSV writes a statement of n rows, a fifth of them dated within the doubtful window of
// another row, and returns its path.
func writeBigCSV(tb testing.TB, n int) string {
	tb.Helper()
	var sb strings.Builder
	sb.WriteString("Date;Amount;Description;Ref\n")
	for i := range n {
		fmt.Fprintf(&sb, "2026-%02d-%02d;-%d,%02d;Shop %d;ref-%d\n", i%12+1, i%28+1, i%500+1, i%100, i%97, i)
	}
	path := filepath.Join(tb.TempDir(), "big.csv")
	if err := os.WriteFile(path, []byte(sb.String()), 0o600); err != nil {
		tb.Fatal(err)
	}
	return path
}

func bigRegistry() Registry {
	return Registry{
		Path: "", Currency: "EUR",
		Profiles: map[string]config.CSVProfile{"big": {
			DateColumn: "Date", AmountColumn: "Amount", DescriptionColumn: "Description", ExternalIDColumn: "Ref",
			Delimiter: ";", DateFormat: "YYYY-MM-DD", DecimalSeparator: ",", TargetAccount: "Main",
		}},
	}
}

func importBigCSV(tb testing.TB, path string) (importing.ImportResult, time.Duration) {
	tb.Helper()
	svc, _, _ := newService(bigRegistry())
	start := wall.Now()
	plan, err := svc.PlanImport(ctx(), importing.PlanRequest{
		Connector: "csv", CreateAccounts: true,
		Fetch: fuiproto.FetchRequest{File: &path, Config: map[string]any{"profile": "big"}},
	})
	if err != nil {
		tb.Fatal(err)
	}
	res, err := svc.ApplyImport(ctx(), plan)
	if err != nil {
		tb.Fatal(err)
	}
	return res, wall.Now().Sub(start)
}

func TestImportTenThousandRowsUnderFiveSeconds(t *testing.T) {
	res, took := importBigCSV(t, writeBigCSV(t, 10_000))
	if res.Transactions != 10_000 {
		t.Fatalf("imported %d rows", res.Transactions)
	}
	if perfTimingUnreliable() {
		t.Skip("timing budget skipped under the race detector or coverage instrumentation; see BenchmarkImportTenThousandRows")
	}
	if took > 5*time.Second {
		t.Fatalf("plan+apply of 10 000 rows took %s, want under 5s", took)
	}
}

func BenchmarkImportTenThousandRows(b *testing.B) {
	path := writeBigCSV(b, 10_000)
	b.ResetTimer()
	for range b.N {
		if res, _ := importBigCSV(b, path); res.Transactions != 10_000 {
			b.Fatalf("imported %d rows", res.Transactions)
		}
	}
}

func TestCSVImportRoundTripsThroughTheStore(t *testing.T) {
	svc, store, _ := newService(bigRegistry())
	path := writeBigCSV(t, 50)
	req := importing.PlanRequest{
		Connector: "csv", CreateAccounts: true,
		Fetch: fuiproto.FetchRequest{File: &path, Config: map[string]any{"profile": "big"}},
	}
	plan, err := svc.PlanImport(ctx(), req)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ApplyImport(ctx(), plan); err != nil {
		t.Fatal(err)
	}
	again, err := svc.PlanImport(ctx(), req)
	if err != nil {
		t.Fatal(err)
	}
	if c := again.Counts(); c.Included != 0 || c.Duplicate != 50 || again.AlreadyImported == nil {
		t.Fatalf("second plan counts=%+v already=%v", c, again.AlreadyImported)
	}
	if err := store.Read(ctx(), func(r app.Repos) error {
		accts, err := r.Accounts().List(ctx())
		if err != nil {
			return err
		}
		if len(accts) != 1 || accts[0].Name != "Main" || accts[0].Currency.Code() != "EUR" {
			return fmt.Errorf("accounts = %+v", accts)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
