package connector

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/0xballo/fui/pkg/fuiproto"
)

func descriptions(recs []fuiproto.Record) map[string]string {
	out := map[string]string{}
	for _, r := range recs {
		if r.Transaction != nil {
			out[r.Transaction.ID] = r.Transaction.Description
		}
	}
	return out
}

func TestRunnerFetchStreamsRecords(t *testing.T) {
	src := Runner{}.Source(fakePath(t), fetchMode("ok"))
	recs, err := collect(t, src.Records(ctx()))
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 5 || recs[4].End == nil || recs[4].End.Transactions != 2 {
		t.Fatalf("records = %d, last %+v", len(recs), recs[len(recs)-1])
	}
	if _, err := fuiproto.ValidateStream(func(yield func(fuiproto.Record, error) bool) {
		for _, r := range recs {
			if !yield(r, nil) {
				return
			}
		}
	}); err != nil {
		t.Fatalf("stream invalid: %v", err)
	}
}

func TestRunnerPassesRequestOnStdin(t *testing.T) {
	file, since := "/tmp/in.csv", "2026-01-01"
	req := fetchMode("stdin")
	req.Config["echo"], req.File, req.Since = "hi", &file, &since
	recs, err := collect(t, Runner{}.Source(fakePath(t), req).Records(ctx()))
	if err != nil {
		t.Fatal(err)
	}
	if got := descriptions(recs)["stdin"]; got != "file=/tmp/in.csv since=2026-01-01 echo=hi" {
		t.Fatalf("connector saw %q", got)
	}
}

func TestRunnerEnvironmentAndWorkingDirectory(t *testing.T) {
	r := Runner{Environ: []string{"FUI_DB=/secret.db", "FUI_LOG=debug", "FAKE_EXTRA=kept", "PATH=" + os.Getenv("PATH")}}
	recs, err := collect(t, r.Source(fakePath(t), fetchMode("env")).Records(ctx()))
	if err != nil {
		t.Fatal(err)
	}
	d := descriptions(recs)
	if d["env"] != "fui vars: -" {
		t.Fatalf("FUI variables leaked: %q", d["env"])
	}
	if d["extra"] != "extra: kept" {
		t.Fatalf("other variables must pass through: %q", d["extra"])
	}
	wd, _ := os.Getwd()
	if cwd := strings.TrimPrefix(d["cwd"], "cwd: "); !strings.HasPrefix(cwd, "fui-connector-") || strings.Contains(wd, cwd) {
		t.Fatalf("connector ran in %q, want a temporary directory", cwd)
	}
}

func TestRunnerDefaultEnvironmentDropsFUIVariables(t *testing.T) {
	t.Setenv("FUI_DB", "/secret.db")
	recs, err := collect(t, Runner{}.Source(fakePath(t), fetchMode("env")).Records(ctx()))
	if err != nil {
		t.Fatal(err)
	}
	if got := descriptions(recs)["env"]; got != "fui vars: -" {
		t.Fatalf("FUI variables leaked: %q", got)
	}
}

func TestRunnerExitCodes(t *testing.T) {
	src := Runner{}.Source(fakePath(t), fetchMode("exit3"))
	_, err := collect(t, src.Records(ctx()))
	var exit *ExitError
	if !errors.As(err, &exit) || exit.Code != fuiproto.ExitCodeUnreadableSource {
		t.Fatalf("err = %v, want exit code 3", err)
	}
	if !strings.Contains(err.Error(), "cannot read the source") {
		t.Fatalf("error does not carry the stderr: %v", err)
	}
	if src.Stderr() == "" {
		t.Fatal("stderr not captured")
	}
}

func TestRunnerFailureAfterCompleteStreamIsAnError(t *testing.T) {
	recs, err := collect(t, Runner{}.Source(fakePath(t), fetchMode("fail-after")).Records(ctx()))
	var exit *ExitError
	if !errors.As(err, &exit) || exit.Code != 1 || len(recs) != 4 {
		t.Fatalf("records=%d err=%v, want the full stream then exit code 1", len(recs), err)
	}
}

func TestRunnerTruncatedStreamHasNoEnd(t *testing.T) {
	recs, err := collect(t, Runner{}.Source(fakePath(t), fetchMode("truncated")).Records(ctx()))
	if err != nil || len(recs) != 3 {
		t.Fatalf("records=%d err=%v", len(recs), err)
	}
	_, err = fuiproto.ValidateStream(func(yield func(fuiproto.Record, error) bool) {
		for _, r := range recs {
			yield(r, nil)
		}
	})
	if !errors.Is(err, fuiproto.ErrTruncatedStream) {
		t.Fatalf("validation err = %v, want truncated", err)
	}
}

func TestRunnerTimeoutKillsTheProcess(t *testing.T) {
	start := wall.Now()
	src := Runner{Timeout: 300 * time.Millisecond}.Source(fakePath(t), fetchMode("hang"))
	_, err := collect(t, src.Records(ctx()))
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want deadline exceeded", err)
	}
	if wall.Now().Sub(start) > 10*time.Second {
		t.Fatalf("timeout took %s", wall.Now().Sub(start))
	}
}

func TestRunnerContextCancel(t *testing.T) {
	c, cancel := context.WithCancel(ctx())
	src := Runner{}.Source(fakePath(t), fetchMode("hang"))
	time.AfterFunc(200*time.Millisecond, cancel)
	if _, err := collect(t, src.Records(c)); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want canceled", err)
	}
}

func TestRunnerRejectsGarbageAndOversizedLines(t *testing.T) {
	for _, mode := range []string{"garbage", "bigline"} {
		if _, err := collect(t, Runner{}.Source(fakePath(t), fetchMode(mode)).Records(ctx())); err == nil {
			t.Errorf("%s: no error", mode)
		}
	}
}

func TestRunnerStderr(t *testing.T) {
	src := Runner{}.Source(fakePath(t), fetchMode("stderr"))
	if _, err := collect(t, src.Records(ctx())); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(src.Stderr(), "hello from stderr") {
		t.Fatalf("stderr = %q", src.Stderr())
	}
	noisy := Runner{}.Source(fakePath(t), fetchMode("noisy"))
	if _, err := collect(t, noisy.Records(ctx())); err != nil {
		t.Fatal(err)
	}
	if len(noisy.Stderr()) != maxStderrBytes {
		t.Fatalf("stderr kept %d bytes, want the %d cap", len(noisy.Stderr()), maxStderrBytes)
	}
}

func TestRunnerStopsWhenTheConsumerStops(t *testing.T) {
	src := Runner{}.Source(fakePath(t), fetchMode("hang"))
	n := 0
	for _, err := range src.Records(ctx()) {
		if err != nil {
			t.Fatal(err)
		}
		n++
		break
	}
	if n != 1 {
		t.Fatalf("saw %d records", n)
	}
}

func TestRunnerMissingExecutable(t *testing.T) {
	if _, err := collect(t, Runner{}.Source("/nonexistent/fui-connector-x", fetchMode("ok")).Records(ctx())); err == nil {
		t.Fatal("no error for a missing executable")
	}
}

func TestRunnerDescribe(t *testing.T) {
	d, err := Runner{}.Describe(ctx(), fakePath(t))
	if err != nil || d.Name != "fake" || d.Version != "1.2.3" {
		t.Fatalf("describe = %+v, %v", d, err)
	}
	for _, mode := range []string{"fail", "badproto", "garbage"} {
		if _, err := (Runner{Environ: []string{"FAKE_MODE=" + mode}}).Describe(ctx(), fakePath(t)); err == nil {
			t.Errorf("describe in mode %s: no error", mode)
		}
	}
	_, err = Runner{Timeout: 200 * time.Millisecond, Environ: []string{"FAKE_MODE=hang"}}.Describe(ctx(), fakePath(t))
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("describe timeout err = %v", err)
	}
}

func TestExitErrorMessages(t *testing.T) {
	for code, want := range map[int]string{2: "rejected its configuration", 3: "cannot read the source", 9: "failed"} {
		if got := (&ExitError{Code: code}).Error(); !strings.Contains(got, want) {
			t.Errorf("code %d: %q does not contain %q", code, got, want)
		}
	}
}
