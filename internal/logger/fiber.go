package logger

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"log/slog"
)

// FiberMiddleware returns a Fiber middleware that logs requests using slog
// with only time, level, msg at root and all other fields under `data`.
func FiberMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()
		err := c.Next()
		latency := time.Since(start)

		status := c.Response().StatusCode()
		method := c.Method()
		path := c.OriginalURL()
		ip := c.IP()
		ua := c.Get("User-Agent")
		route := ""
		if r := c.Route(); r != nil {
			route = r.Path
		}

		attrs := []any{
			"status", status,
			"method", method,
			"path", path,
			"route", route,
			"ip", ip,
			"user_agent", ua,
			"latency_ms", float64(latency.Microseconds()) / 1000.0,
		}

		if err != nil {
			slog.Error("http request", append(attrs, "err", err.Error())...)
			return err
		}
		slog.Info("http request", attrs...)
		return nil
	}
}
