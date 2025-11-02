package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
)

type Config struct {
	Level     string
	Format    string
	AddSource bool
	Service   string
	Env       string
	Output    string
}

type ctxKey int

const (
	ctxKeyLogger ctxKey = iota
	ctxKeyRequestID
)

var (
	levelVar      slog.LevelVar
	defaultLogger *slog.Logger
)

func Default() *slog.Logger {
	if defaultLogger != nil {
		return defaultLogger
	}
	return slog.Default()
}

func Init(cfg Config) *slog.Logger {
	lvl := strings.ToLower(strings.TrimSpace(cfg.Level))
	switch lvl {
	case "debug":
		levelVar.Set(slog.LevelDebug)
	case "info", "":
		levelVar.Set(slog.LevelInfo)
	case "warn", "warning":
		levelVar.Set(slog.LevelWarn)
	case "error":
		levelVar.Set(slog.LevelError)
	default:
		levelVar.Set(slog.LevelInfo)
	}

	w := resolveWriter(cfg.Output)
	format := strings.ToLower(strings.TrimSpace(cfg.Format))
	// Keep only time, level, msg at root: avoid handler-level source field
	opts := &slog.HandlerOptions{Level: &levelVar, AddSource: false}

	var h slog.Handler
	if format == "text" {
		h = slog.NewTextHandler(w, opts)
	} else {
		h = slog.NewJSONHandler(w, opts)
	}

	service := firstNonEmpty(cfg.Service, os.Getenv("SERVICE_NAME"))
	env := firstNonEmpty(cfg.Env, os.Getenv("ENV"), os.Getenv("APP_ENV"))
	version := os.Getenv("VERSION")

	// Ensure service is always set so `data` group always exists in logs
	if strings.TrimSpace(service) == "" {
		service = defaultServiceName()
	}

	// Create logger and scope all attributes into top-level `data` group
	base := slog.New(h).WithGroup("data").With("service", service)
	if env != "" {
		base = base.With("env", env)
	}
	if version != "" {
		base = base.With("version", version)
	}

	defaultLogger = base
	slog.SetDefault(defaultLogger)
	return defaultLogger
}

func SetLevel(level string) {
	lvl := strings.ToLower(strings.TrimSpace(level))
	switch lvl {
	case "debug":
		levelVar.Set(slog.LevelDebug)
	case "info", "":
		levelVar.Set(slog.LevelInfo)
	case "warn", "warning":
		levelVar.Set(slog.LevelWarn)
	case "error":
		levelVar.Set(slog.LevelError)
	}
}

func With(args ...any) *slog.Logger {
	return Default().With(args...)
}

func IntoContext(ctx context.Context, l *slog.Logger) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, ctxKeyLogger, l)
}

func FromContext(ctx context.Context) *slog.Logger {
	l := Default()
	if ctx == nil {
		return l
	}
	if v := ctx.Value(ctxKeyLogger); v != nil {
		if lg, ok := v.(*slog.Logger); ok && lg != nil {
			l = lg
		}
	}
	if v := ctx.Value(ctxKeyRequestID); v != nil {
		if id, ok := v.(string); ok && id != "" {
			l = l.With("request_id", id)
		}
	}
	return l
}

func WithRequestID(ctx context.Context, requestID string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, ctxKeyRequestID, requestID)
}

func resolveWriter(output string) io.Writer {
	o := strings.ToLower(strings.TrimSpace(output))
	switch o {
	case "", "stdout":
		return os.Stdout
	case "stderr":
		return os.Stderr
	default:
		f, err := os.OpenFile(output, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return os.Stdout
		}
		return f
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func defaultServiceName() string {
	exe := os.Args
	if len(exe) > 0 {
		// last segment of argv[0]
		path := exe[0]
		// trim path separators manually to avoid extra imports
		i := strings.LastIndexByte(path, '/')
		if i >= 0 && i+1 < len(path) {
			return path[i+1:]
		}
		i = strings.LastIndexByte(path, '\\')
		if i >= 0 && i+1 < len(path) {
			return path[i+1:]
		}
		if path != "" {
			return path
		}
	}
	return "app"
}
