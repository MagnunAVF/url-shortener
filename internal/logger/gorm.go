package logger

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm/logger"
)

// GormLogger implements gorm.io/gorm/logger.Interface using slog with our schema
// Only time, level, msg at root. All attributes are under the top-level `data` group
// thanks to the default logger initialization.

type GormLogger struct {
	logLevel logger.LogLevel
	slowThreshold time.Duration
}

func NewGormLogger(level string) *GormLogger {
	var lvl logger.LogLevel
	switch level {
	case "silent":
		lvl = logger.Silent
	case "error":
		lvl = logger.Error
	case "warn", "warning":
		lvl = logger.Warn
	case "info", "":
		lvl = logger.Info
	default:
		lvl = logger.Info
	}
	return &GormLogger{logLevel: lvl, slowThreshold: 200 * time.Millisecond}
}

func (g *GormLogger) LogMode(level logger.LogLevel) logger.Interface {
	return &GormLogger{logLevel: level, slowThreshold: g.slowThreshold}
}

func (g *GormLogger) Info(ctx context.Context, msg string, data ...interface{}) {
	if g.logLevel >= logger.Info {
		FromContext(ctx).Info("gorm info", "msg_detail", msg, "data", data)
	}
}

func (g *GormLogger) Warn(ctx context.Context, msg string, data ...interface{}) {
	if g.logLevel >= logger.Warn {
		FromContext(ctx).Warn("gorm warn", "msg_detail", msg, "data", data)
	}
}

func (g *GormLogger) Error(ctx context.Context, msg string, data ...interface{}) {
	if g.logLevel >= logger.Error {
		FromContext(ctx).Error("gorm error", "msg_detail", msg, "data", data)
	}
}

// Trace logs SQL with its variables, rows affected and elapsed time.
func (g *GormLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if g.logLevel == logger.Silent {
		return
	}
	elapsed := time.Since(begin)
	sql, rows := fc()

	attrs := []any{
		"sql", sql,
		"rows", rows,
		"elapsed_ms", float64(elapsed.Microseconds()) / 1000.0,
	}

	if err != nil && !errors.Is(err, context.Canceled) {
		attrs = append(attrs, "err", err)
		if g.logLevel >= logger.Error {
			FromContext(ctx).Error("gorm trace", attrs...)
		}
		return
	}

	if g.slowThreshold > 0 && elapsed > g.slowThreshold {
		attrs = append(attrs, "slow", true, "threshold_ms", float64(g.slowThreshold.Microseconds())/1000.0)
		if g.logLevel >= logger.Warn {
			FromContext(ctx).Warn("gorm trace slow", attrs...)
		}
		return
	}

	if g.logLevel >= logger.Info {
		FromContext(ctx).Info("gorm trace", attrs...)
	}
}
