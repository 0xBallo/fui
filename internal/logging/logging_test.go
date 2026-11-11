package logging_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/0xballo/fui/internal/logging"
)

type fakeSensitive string

func (f fakeSensitive) LogValue() slog.Value {
	return slog.StringValue("[redacted]")
}

func (f fakeSensitive) String() string {
	return string(f)
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input string
		want  slog.Level
	}{
		{input: "debug", want: slog.LevelDebug},
		{input: "DEBUG", want: slog.LevelDebug},
		{input: "info", want: slog.LevelInfo},
		{input: "warn", want: slog.LevelWarn},
		{input: "warning", want: slog.LevelWarn},
		{input: "error", want: slog.LevelError},
		{input: "", want: slog.LevelWarn},
		{input: "unknown", want: slog.LevelWarn},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := logging.ParseLevel(tt.input)
			if got != tt.want {
				t.Errorf("ParseLevel(%q) = %v; want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestSensitiveRedactionDefault(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(logging.NewHandler(&buf, logging.Options{
		Level:        slog.LevelInfo,
		LogSensitive: false,
	}))

	val := fakeSensitive("123.45 EUR")
	logger.Info("processed payment", "amount", val)

	var record map[string]any
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatalf("failed to unmarshal log json: %v", err)
	}

	got, ok := record["amount"].(string)
	if !ok || got != "[redacted]" {
		t.Errorf("expected redacted amount, got %v", record["amount"])
	}
}

func TestSensitiveRedactionEnabled(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(logging.NewHandler(&buf, logging.Options{
		Level:        slog.LevelInfo,
		LogSensitive: true,
	}))

	val := fakeSensitive("123.45 EUR")
	logger.Info("processed payment", "amount", val)

	var record map[string]any
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatalf("failed to unmarshal log json: %v", err)
	}

	got, ok := record["amount"].(string)
	if !ok || got != "123.45 EUR" {
		t.Errorf("expected clear amount %q, got %v", "123.45 EUR", record["amount"])
	}
}

func TestSensitiveRedactionWithAttrsAndGroups(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(logging.NewHandler(&buf, logging.Options{
		Level:        slog.LevelInfo,
		LogSensitive: true,
	}))

	subLogger := logger.With("context_id", fakeSensitive("secret-ctx")).WithGroup("details")
	subLogger.Info("operation completed",
		"inner_secret", fakeSensitive("inner-val"),
		slog.Group("nested", slog.Any("sub", fakeSensitive("sub-val"))),
	)

	var record map[string]any
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatalf("failed to unmarshal log json: %v", err)
	}

	if record["context_id"] != "secret-ctx" {
		t.Errorf("expected clear context_id, got %v", record["context_id"])
	}

	group, ok := record["details"].(map[string]any)
	if !ok {
		t.Fatalf("missing group details in %v", record)
	}
	if group["inner_secret"] != "inner-val" {
		t.Errorf("expected clear inner_secret, got %v", group["inner_secret"])
	}
	nested, ok := group["nested"].(map[string]any)
	if !ok {
		t.Fatalf("missing nested group in %v", group)
	}
	if nested["sub"] != "sub-val" {
		t.Errorf("expected clear nested.sub, got %v", nested["sub"])
	}
}

func TestSetupFileAndPermissions(t *testing.T) {
	tmp := t.TempDir()
	logDir := filepath.Join(tmp, "state", "subdir")
	logPath := filepath.Join(logDir, "fui.log")

	logger, closer, err := logging.Setup(logPath, logging.Options{
		Level:        slog.LevelWarn,
		LogSensitive: false,
	})
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	defer func() {
		_ = closer.Close()
	}()

	logger.Warn("disk usage warning", "percent", 85)
	if err := closer.Close(); err != nil {
		t.Fatalf("closer.Close failed: %v", err)
	}

	dirInfo, err := os.Stat(logDir)
	if err != nil {
		t.Fatalf("stat log dir failed: %v", err)
	}
	// Verify directory permissions 0700
	if perm := dirInfo.Mode().Perm(); perm != 0o700 {
		t.Errorf("expected dir perms 0700, got %#o", perm)
	}

	fileInfo, err := os.Stat(logPath)
	if err != nil {
		t.Fatalf("stat log file failed: %v", err)
	}
	// Verify file permissions 0600
	if perm := fileInfo.Mode().Perm(); perm != 0o600 {
		t.Errorf("expected file perms 0600, got %#o", perm)
	}

	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log file failed: %v", err)
	}
	if !strings.Contains(string(content), "disk usage warning") {
		t.Errorf("expected log file to contain warning message, got %s", content)
	}
}

func TestSetupEmptyPathError(t *testing.T) {
	_, _, err := logging.Setup("", logging.Options{})
	if err == nil {
		t.Fatal("expected error for empty path, got nil")
	}
}

func TestHandlerEnabled(t *testing.T) {
	var buf bytes.Buffer
	handler := logging.NewHandler(&buf, logging.Options{
		Level:        slog.LevelWarn,
		LogSensitive: true,
	})

	if handler.Enabled(context.Background(), slog.LevelDebug) {
		t.Error("expected debug to be disabled when level is warn")
	}
	if !handler.Enabled(context.Background(), slog.LevelWarn) {
		t.Error("expected warn to be enabled when level is warn")
	}
	if !handler.Enabled(context.Background(), slog.LevelError) {
		t.Error("expected error to be enabled when level is warn")
	}
}

func TestPlainLogValuerWithoutStringer(t *testing.T) {
	// Test type that implements LogValuer but NOT fmt.Stringer
	var buf bytes.Buffer
	logger := slog.New(logging.NewHandler(&buf, logging.Options{
		Level:        slog.LevelInfo,
		LogSensitive: true,
	}))

	logger.Info("test", "opaque", customValuer{})
	var record map[string]any
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatalf("failed to unmarshal log json: %v", err)
	}
	if diff := cmp.Diff("opaque", record["opaque"]); diff != "" {
		t.Errorf("unexpected value (-want +got):\n%s", diff)
	}
}

type customValuer struct{}

func (customValuer) LogValue() slog.Value {
	return slog.StringValue("opaque")
}
