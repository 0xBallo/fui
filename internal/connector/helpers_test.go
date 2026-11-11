package connector

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"

	"github.com/0xballo/fui/internal/clock"
	"github.com/0xballo/fui/pkg/fuiproto"
)

// wall measures elapsed time in tests.
var wall = clock.NewSystem()

var (
	fakeOnce sync.Once
	fakeDir  string
	fakeErr  error
)

// fakeConnectorDir builds the fake connector once and returns the directory that holds
// fui-connector-fake, ready to be used as a Registry.Dir.
func fakeConnectorDir(t testing.TB) string {
	t.Helper()
	fakeOnce.Do(func() {
		fakeDir, fakeErr = os.MkdirTemp("", "fui-fake-connector-")
		if fakeErr != nil {
			return
		}
		cmd := exec.Command("go", "build", "-o", filepath.Join(fakeDir, "fui-connector-fake"), "./testdata/fakeconnector")
		if out, err := cmd.CombinedOutput(); err != nil {
			fakeErr = &buildError{err: err, out: string(out)}
		}
	})
	if fakeErr != nil {
		t.Fatalf("build fake connector: %v", fakeErr)
	}
	return fakeDir
}

type buildError struct {
	err error
	out string
}

func (e *buildError) Error() string { return e.err.Error() + ": " + e.out }

func TestMain(m *testing.M) {
	code := m.Run()
	if fakeDir != "" {
		_ = os.RemoveAll(fakeDir)
	}
	os.Exit(code)
}

func fakePath(t testing.TB) string { return filepath.Join(fakeConnectorDir(t), "fui-connector-fake") }

func fetchMode(mode string) fuiproto.FetchRequest {
	return fuiproto.FetchRequest{Protocol: fuiproto.ProtocolFUI1, Config: map[string]any{"mode": mode}}
}

// collect drains a source into its records and the error that ended it, if any.
func collect(t testing.TB, seq func(yield func(fuiproto.Record, error) bool)) ([]fuiproto.Record, error) {
	t.Helper()
	var recs []fuiproto.Record
	for r, err := range seq {
		if err != nil {
			return recs, err
		}
		recs = append(recs, r)
	}
	return recs, nil
}

func ctx() context.Context { return context.Background() }
