// Package logging provides file-based structured JSON logging for FUI.
//
// Log messages are written exclusively to a private log file, never to standard
// output, preserving machine-readable stdout for CLI scripts. By default,
// sensitive fields such as amounts, descriptions, and account identifiers are
// redacted via slog.LogValuer; sensitive diagnostics can be unlocked with
// FUI_LOG_SENSITIVE=1.
package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// DefaultLevel is the standard log level when none is explicitly configured.
const DefaultLevel = slog.LevelWarn

// Options specifies configuration for the structured logger.
type Options struct {
	// Level determines the minimum log severity recorded.
	Level slog.Level
	// LogSensitive, when true, unredacts fields implementing fmt.Stringer.
	LogSensitive bool
}

// ParseLevel converts a level name into its corresponding slog.Level.
// Supported names are "debug", "info", "warn" (or "warning"), and "error".
// If the string is empty or unrecognized, it falls back to DefaultLevel.
func ParseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return DefaultLevel
	}
}

// Setup initializes a JSON file logger at logPath, creating parent directories
// with permission 0700 and the log file with permission 0600.
// The caller is responsible for closing the returned io.Closer on shutdown.
func Setup(logPath string, opts Options) (*slog.Logger, io.Closer, error) {
	if logPath == "" {
		return nil, nil, fmt.Errorf("setup logger: log path must not be empty")
	}

	dir := filepath.Dir(logPath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, nil, fmt.Errorf("create log directory %q: %w", dir, err)
	}

	cleanPath := filepath.Clean(logPath)
	f, err := os.OpenFile(cleanPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, nil, fmt.Errorf("open log file %q: %w", logPath, err)
	}

	handler := NewHandler(f, opts)
	return slog.New(handler), f, nil
}

// NewHandler constructs a slog.Handler writing JSON records to w.
// If opts.LogSensitive is true, attributes implementing both slog.LogValuer
// and fmt.Stringer are preserved in clear text.
func NewHandler(w io.Writer, opts Options) slog.Handler {
	base := slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level: opts.Level,
	})

	if !opts.LogSensitive {
		return base
	}
	return &sensitiveHandler{next: base}
}

type sensitiveHandler struct {
	next slog.Handler
}

// Enabled delegates the severity check to the wrapped handler.
func (h *sensitiveHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

// Handle unredacts sensitive attributes before delegating to the wrapped handler.
func (h *sensitiveHandler) Handle(ctx context.Context, r slog.Record) error {
	unredacted := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
	r.Attrs(func(a slog.Attr) bool {
		unredacted.AddAttrs(unredactAttr(a))
		return true
	})

	if err := h.next.Handle(ctx, unredacted); err != nil {
		return fmt.Errorf("handle log record: %w", err)
	}
	return nil
}

// WithAttrs returns a new sensitiveHandler wrapping attributes with unredaction applied.
func (h *sensitiveHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	unredacted := make([]slog.Attr, len(attrs))
	for i, a := range attrs {
		unredacted[i] = unredactAttr(a)
	}
	return &sensitiveHandler{next: h.next.WithAttrs(unredacted)}
}

// WithGroup returns a new sensitiveHandler wrapping the group name.
func (h *sensitiveHandler) WithGroup(name string) slog.Handler {
	return &sensitiveHandler{next: h.next.WithGroup(name)}
}

func unredactAttr(a slog.Attr) slog.Attr {
	if a.Value.Kind() == slog.KindGroup {
		groupAttrs := a.Value.Group()
		unredacted := make([]slog.Attr, len(groupAttrs))
		for i, child := range groupAttrs {
			unredacted[i] = unredactAttr(child)
		}
		return slog.Attr{
			Key:   a.Key,
			Value: slog.GroupValue(unredacted...),
		}
	}

	if a.Value.Kind() == slog.KindLogValuer {
		if s, ok := a.Value.Any().(fmt.Stringer); ok {
			return slog.String(a.Key, s.String())
		}
	}
	return a
}
