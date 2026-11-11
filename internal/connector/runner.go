package connector

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"iter"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/0xballo/fui/internal/app/importing"
	"github.com/0xballo/fui/pkg/fuiproto"
)

const (
	// DefaultTimeout is how long an external connector may run when the configuration says nothing.
	DefaultTimeout = 120 * time.Second
	// maxStderrBytes bounds the stderr kept for --verbose.
	maxStderrBytes = 64 << 10
	// killGrace is how long Wait lets a killed process close its pipes.
	killGrace = 2 * time.Second
)

// ExitError is a connector that ended with a non-zero exit code.
type ExitError struct {
	Code   int
	Stderr string
}

// Error implements error, naming the codes of the fui/1 contract.
func (e *ExitError) Error() string {
	what := "failed"
	switch e.Code {
	case fuiproto.ExitCodeBadConfig:
		what = "rejected its configuration"
	case fuiproto.ExitCodeUnreadableSource:
		what = "cannot read the source"
	}
	msg := fmt.Sprintf("connector %s (exit code %d)", what, e.Code)
	if tail := strings.TrimSpace(e.Stderr); tail != "" {
		msg += ": " + lastLine(tail)
	}
	return msg
}

func lastLine(s string) string {
	if i := strings.LastIndexByte(s, '\n'); i >= 0 {
		return s[i+1:]
	}
	return s
}

// Runner starts external connector executables.
type Runner struct {
	// Timeout bounds one fetch or describe; DefaultTimeout when zero.
	Timeout time.Duration
	// Environ is the environment of the parent; os.Environ when nil. Variables of FUI (FUI_*) are
	// never passed on: a connector gets only its configuration, on stdin.
	Environ []string
}

func (r Runner) timeout() time.Duration {
	if r.Timeout > 0 {
		return r.Timeout
	}
	return DefaultTimeout
}

// childEnv is the environment of a connector process: the parent's without FUI_*.
func (r Runner) childEnv() []string {
	env := r.Environ
	if env == nil {
		env = os.Environ()
	}
	out := make([]string, 0, len(env))
	for _, kv := range env {
		if !strings.HasPrefix(kv, "FUI_") {
			out = append(out, kv)
		}
	}
	return out
}

// Describe runs `<path> describe` and decodes its answer.
func (r Runner) Describe(ctx context.Context, path string) (fuiproto.DescribeResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout())
	defer cancel()
	dir, err := os.MkdirTemp("", "fui-connector-")
	if err != nil {
		return fuiproto.DescribeResponse{}, fmt.Errorf("create working directory: %w", err)
	}
	defer os.RemoveAll(dir) //nolint:errcheck // best-effort cleanup of a temporary directory
	var stdout limitedBuffer
	stdout.max = fuiproto.MaxLineBytes
	var stderr limitedBuffer
	stderr.max = maxStderrBytes
	cmd := exec.CommandContext(ctx, path, "describe")
	cmd.Dir, cmd.Env, cmd.Stdout, cmd.Stderr, cmd.WaitDelay = dir, r.childEnv(), &stdout, &stderr, killGrace
	if err := cmd.Run(); err != nil {
		return fuiproto.DescribeResponse{}, exitFailure(ctx, r.timeout(), err, stderr.String())
	}
	var d fuiproto.DescribeResponse
	if err := json.Unmarshal(stdout.Bytes(), &d); err != nil {
		return fuiproto.DescribeResponse{}, fmt.Errorf("decode describe output: %w", err)
	}
	if major, err := fuiproto.ParseProtocolMajor(d.Protocol); err != nil || major != 1 {
		return fuiproto.DescribeResponse{}, fmt.Errorf("connector speaks protocol %q, want %s", d.Protocol, fuiproto.ProtocolFUI1)
	}
	return d, nil
}

// Source returns the source that runs `<path> fetch` with req on stdin. Use it once.
func (r Runner) Source(path string, req fuiproto.FetchRequest) *ExternalSource {
	return &ExternalSource{runner: r, path: path, req: req}
}

// ExternalSource streams the records an external connector writes on stdout.
type ExternalSource struct {
	runner Runner
	path   string
	req    fuiproto.FetchRequest
	stderr limitedBuffer
}

var (
	_ importing.Source       = (*ExternalSource)(nil)
	_ importing.StderrSource = (*ExternalSource)(nil)
)

// Stderr returns what the connector wrote on stderr, the first 64 KiB at most.
func (s *ExternalSource) Stderr() string { return s.stderr.String() }

// Records implements importing.Source. A non-zero exit, a timeout or a decode failure end the
// sequence with an error; the caller treats a missing end record as a truncated stream.
func (s *ExternalSource) Records(ctx context.Context) iter.Seq2[fuiproto.Record, error] {
	return func(yield func(fuiproto.Record, error) bool) {
		timeout := s.runner.timeout()
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		dir, err := os.MkdirTemp("", "fui-connector-")
		if err != nil {
			yield(fuiproto.Record{}, fmt.Errorf("create working directory: %w", err))
			return
		}
		defer os.RemoveAll(dir) //nolint:errcheck // best-effort cleanup of a temporary directory
		input, err := json.Marshal(s.req)
		if err != nil {
			yield(fuiproto.Record{}, fmt.Errorf("encode fetch request: %w", err))
			return
		}
		s.stderr.max = maxStderrBytes
		cmd := exec.CommandContext(ctx, s.path, "fetch") //nolint:gosec // running the connector the user installed is the point
		cmd.Dir, cmd.Env, cmd.Stdin, cmd.Stderr, cmd.WaitDelay = dir, s.runner.childEnv(), bytes.NewReader(input), &s.stderr, killGrace
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			yield(fuiproto.Record{}, fmt.Errorf("connect to connector: %w", err))
			return
		}
		if err := cmd.Start(); err != nil {
			yield(fuiproto.Record{}, fmt.Errorf("start %s: %w", s.path, err))
			return
		}
		for rec, err := range fuiproto.DecodeJSONL(stdout) {
			if err != nil {
				cancel() // stop a connector that writes garbage or an oversized line
				_ = cmd.Wait()
				yield(fuiproto.Record{}, err)
				return
			}
			if !yield(rec, nil) {
				cancel()
				_ = cmd.Wait()
				return
			}
		}
		if err := cmd.Wait(); err != nil {
			yield(fuiproto.Record{}, exitFailure(ctx, timeout, err, s.stderr.String()))
		}
	}
}

// exitFailure explains why a process did not end cleanly.
func exitFailure(ctx context.Context, timeout time.Duration, err error, stderr string) error {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return fmt.Errorf("connector timed out after %s: %w", timeout, context.DeadlineExceeded)
	}
	if ctx.Err() != nil {
		return fmt.Errorf("connector stopped: %w", ctx.Err())
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return &ExitError{Code: exit.ExitCode(), Stderr: stderr}
	}
	return fmt.Errorf("run connector: %w", err)
}

// limitedBuffer keeps the first max bytes written and drops the rest, so that a chatty process
// cannot exhaust memory. It is safe for the concurrent use exec makes of it.
type limitedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
	max int
}

var _ io.Writer = (*limitedBuffer)(nil)

func (b *limitedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if room := b.max - b.buf.Len(); room > 0 {
		b.buf.Write(p[:min(room, len(p))])
	}
	return len(p), nil
}

func (b *limitedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func (b *limitedBuffer) Bytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	return bytes.Clone(b.buf.Bytes())
}
