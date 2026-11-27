package app

import (
	"database/sql"
	"errors"
	"net/http"

	"go-todo/internal/i18n"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
)

const (
	adminContextKey     = "admin"
	localizerContextKey = "localizer"
)

func (a *App) localizer(c *echo.Context) i18n.Localizer {
	if localizer, ok := c.Get(localizerContextKey).(i18n.Localizer); ok {
		return localizer
	}
	return a.translations.For(c.Request().Header.Get("Accept-Language"))
}

func (a *App) localize(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		localizer := a.translations.For(c.Request().Header.Get("Accept-Language"))
		c.Response().Header().Set("Content-Language", localizer.Language())
		c.Response().Header().Set("Cache-Control", "no-store")
		c.Response().Header().Add("Vary", "Accept-Language")
		c.Set(localizerContextKey, localizer)
		return next(c)
	}
}

func (a *App) handleHTTPError(c *echo.Context, err error) {
	response, unwrapErr := echo.UnwrapResponse(c.Response())
	if unwrapErr == nil && response.Committed {
		return
	}
	status := echo.StatusCode(err)
	if status == 0 {
		status = http.StatusInternalServerError
	}
	message := a.message(c, "error.internal")
	var httpErr *echo.HTTPError
	switch {
	case errors.Is(err, middleware.ErrCSRFInvalid), errors.Is(err, echo.ErrBadRequest):
		message = a.message(c, "error.csrf")
	case status == http.StatusRequestEntityTooLarge:
		message = a.message(c, "error.invalid_input")
	case status == http.StatusNotFound:
		message = http.StatusText(status)
	case status < http.StatusInternalServerError:
		message = http.StatusText(status)
		if errors.As(err, &httpErr) {
			switch httpErr.Message {
			case "error.permission", "error.self_delete":
				message = a.message(c, httpErr.Message)
			default:
				message = httpErr.Message
			}
		}
	}
	if status >= http.StatusInternalServerError {
		c.Logger().ErrorContext(c.Request().Context(), "request failed", "error", err)
	}
	_ = c.String(status, message)
}

func (a *App) requireAdmin(super bool) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			cookie, err := c.Cookie("session")
			if err != nil {
				return c.Redirect(http.StatusSeeOther, "/auth/sign_in")
			}
			row, err := a.queries.FindSessionAdmin(c.Request().Context(), tokenHash(cookie.Value))
			if errors.Is(err, sql.ErrNoRows) {
				return c.Redirect(http.StatusSeeOther, "/auth/sign_in")
			}
			if err != nil {
				return err
			}
			admin := adminModel(row)
			if super && admin.Role != "super_admin" {
				return echo.NewHTTPError(http.StatusForbidden, "error.permission")
			}
			c.Set(adminContextKey, &admin)
			return next(c)
		}
	}
}
