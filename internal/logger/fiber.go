package logger

import (
	"time"

	"log/slog"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// FiberMiddleware returns a Fiber middleware that logs requests using slog
// with only time, level, msg at root and all other fields under `data`.
func FiberMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()
		// Extract or generate request ID
		reqID := c.Get("X-Request-Id")
		if reqID == "" {
			reqID = uuid.NewString()
		}
		// Make available to handlers and set response header
		c.Locals("request_id", reqID)
		c.Set("X-Request-Id", reqID)

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
			"request_id", reqID,
		}

		if err != nil {
			slog.Error("http request", append(attrs, "err", err.Error())...)
			return err
		}
		slog.Info("http request", attrs...)
		return nil
	}
}
