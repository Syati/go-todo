package app

import (
	"crypto/rand"
	"log/slog"
	"regexp"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
)

var validRequestID = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

func setRequestLogger(c *echo.Context, id string) {
	if !validRequestID.MatchString(id) {
		id = rand.Text()
		c.Response().Header().Set(echo.HeaderXRequestID, id)
	}
	c.Request().Header.Set(echo.HeaderXRequestID, id)
	c.SetLogger(c.Logger().With("request_id", id))
}

func logRequests() echo.MiddlewareFunc {
	return middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		HandleError:     true,
		LogMethod:       true,
		LogRoutePath:    true,
		LogStatus:       true,
		LogLatency:      true,
		LogResponseSize: true,
		LogValuesFunc: func(c *echo.Context, v middleware.RequestLoggerValues) error {
			route := v.RoutePath
			if route == "" || c.RouteInfo().Method == echo.RouteNotFound {
				route = "(unmatched)"
			}
			c.Logger().LogAttrs(c.Request().Context(), slog.LevelInfo, "http request",
				slog.String("method", v.Method),
				slog.String("route", route),
				slog.Int("status", v.Status),
				slog.Int64("duration_us", v.Latency.Microseconds()),
				slog.Int64("response_bytes", v.ResponseSize),
			)
			return nil
		},
	})
}
