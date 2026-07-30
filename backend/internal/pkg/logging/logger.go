package logging

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/trademind-ai/trademind/backend/internal/pkg/safefields"
)

// Config holds structured logger settings.
type Config struct {
	Format         string // json | console
	Level          string
	IncludeSource  bool
	MaxFieldLength int
	Service        string
	Version        string
	Environment    string
	FailSafe       bool
}

// Logger is the unified structured logger interface.
type Logger interface {
	Debug(ctx context.Context, message string, fields ...Field)
	Info(ctx context.Context, message string, fields ...Field)
	Warn(ctx context.Context, message string, fields ...Field)
	Error(ctx context.Context, message string, err error, fields ...Field)
	With(fields ...Field) Logger
	Underlying() *slog.Logger
}

type structuredLogger struct {
	base   *slog.Logger
	cfg    Config
	mu     sync.Mutex
	failed int64
	static []Field
}

// New creates a structured logger.
func New(cfg Config) Logger {
	if cfg.MaxFieldLength <= 0 {
		cfg.MaxFieldLength = 2048
	}
	level := parseLevel(cfg.Level)
	opts := &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Value.Kind() == slog.KindString {
				a.Value = slog.StringValue(TruncateString(a.Value.String(), cfg.MaxFieldLength))
			}
			return a
		},
	}
	if cfg.IncludeSource {
		opts.AddSource = true
	}
	var w io.Writer = os.Stdout
	var h slog.Handler
	switch strings.ToLower(strings.TrimSpace(cfg.Format)) {
	case "console", "text":
		h = slog.NewTextHandler(w, opts)
	default:
		h = slog.NewJSONHandler(w, opts)
	}
	base := slog.New(h)
	return &structuredLogger{base: base, cfg: cfg}
}

// Default returns a development-friendly logger.
func Default() Logger {
	return New(Config{
		Format:         "console",
		Level:          "debug",
		MaxFieldLength: 2048,
		Service:        "trademind-api",
		Environment:    "development",
		FailSafe:       true,
	})
}

func parseLevel(raw string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func (l *structuredLogger) Underlying() *slog.Logger {
	if l == nil {
		return slog.Default()
	}
	return l.base
}

func (l *structuredLogger) With(fields ...Field) Logger {
	if l == nil {
		return Default().With(fields...)
	}
	merged := append(append([]Field(nil), l.static...), fields...)
	return &structuredLogger{
		base:   l.base,
		cfg:    l.cfg,
		static: merged,
	}
}

func (l *structuredLogger) Debug(ctx context.Context, message string, fields ...Field) {
	l.log(ctx, slog.LevelDebug, message, nil, fields)
}

func (l *structuredLogger) Info(ctx context.Context, message string, fields ...Field) {
	l.log(ctx, slog.LevelInfo, message, nil, fields)
}

func (l *structuredLogger) Warn(ctx context.Context, message string, fields ...Field) {
	l.log(ctx, slog.LevelWarn, message, nil, fields)
}

func (l *structuredLogger) Error(ctx context.Context, message string, err error, fields ...Field) {
	l.log(ctx, slog.LevelError, message, err, fields)
}

func (l *structuredLogger) log(ctx context.Context, level slog.Level, message string, err error, fields []Field) {
	if l == nil || l.base == nil {
		return
	}
	defer func() {
		if recover() != nil && l.cfg.FailSafe {
			l.mu.Lock()
			l.failed++
			l.mu.Unlock()
		}
	}()

	attrs := l.buildAttrs(ctx, err, fields)
	l.base.LogAttrs(ctx, level, TruncateString(message, l.cfg.MaxFieldLength), attrs...)
}

func (l *structuredLogger) buildAttrs(ctx context.Context, err error, fields []Field) []slog.Attr {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	attrs := []slog.Attr{
		slog.String("timestamp", now),
		slog.String("service", l.cfg.Service),
		slog.String("version", l.cfg.Version),
		slog.String("environment", l.cfg.Environment),
	}
	for _, f := range CorrelationFields(ctx) {
		attrs = append(attrs, slog.Any(f.Key, SanitizeFieldValue(f.Key, f.Value)))
	}
	for _, f := range l.static {
		attrs = append(attrs, slog.Any(f.Key, SanitizeFieldValue(f.Key, f.Value)))
	}
	for _, f := range fields {
		attrs = append(attrs, slog.Any(f.Key, SanitizeFieldValue(f.Key, f.Value)))
	}
	if err != nil {
		attrs = append(attrs, slog.String("error_summary", safeErrorSummary(err)))
	}
	return attrs
}

func safeErrorSummary(err error) string {
	if err == nil {
		return ""
	}
	return TruncateString(safefields.RedactString(err.Error()), 512)
}
