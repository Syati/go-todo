package app

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"errors"
	"net/http"

	"go-todo/internal/i18n"

	"github.com/labstack/echo/v5"
)

type adminContextKey struct{}
type localizerContextKey struct{}

func (a *App) localizer(r *http.Request) i18n.Localizer {
	if localizer, ok := r.Context().Value(localizerContextKey{}).(i18n.Localizer); ok {
		return localizer
	}
	return a.translations.For(r.Header.Get("Accept-Language"))
}

func (a *App) localize(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		localizer := a.translations.For(c.Request().Header.Get("Accept-Language"))
		c.Response().Header().Set("Content-Language", localizer.Language())
		r := c.Request().WithContext(context.WithValue(c.Request().Context(), localizerContextKey{}, localizer))
		c.SetRequest(r)
		return next(c)
	}
}

func (a *App) securityHeaders(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		header := c.Response().Header()
		header.Set("X-Content-Type-Options", "nosniff")
		header.Set("X-Frame-Options", "DENY")
		header.Set("Referrer-Policy", "same-origin")
		header.Set("Content-Security-Policy", "default-src 'self'; style-src 'self'; script-src 'self'; form-action 'self'; frame-ancestors 'none'; base-uri 'self'")
		header.Set("Cache-Control", "no-store")
		header.Add("Vary", "Accept-Language")

		r := c.Request()
		r.Body = http.MaxBytesReader(c.Response(), r.Body, 64<<10)
		c.SetRequest(r)
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			cookie, err := r.Cookie("csrf")
			if err != nil || r.ParseForm() != nil || len(cookie.Value) != 64 || subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(r.PostForm.Get("csrf"))) != 1 {
				return c.String(http.StatusForbidden, a.message(r, "error.csrf"))
			}
		}
		return next(c)
	}
}

func (a *App) requireAdmin(super bool) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			r := c.Request()
			cookie, err := r.Cookie("session")
			if err != nil {
				return c.Redirect(http.StatusSeeOther, "/auth/sign_in")
			}
			row, err := a.queries.FindSessionAdmin(r.Context(), tokenHash(cookie.Value))
			if errors.Is(err, sql.ErrNoRows) {
				return c.Redirect(http.StatusSeeOther, "/auth/sign_in")
			}
			if err != nil {
				a.fail(c.Response(), r, err)
				return nil
			}
			admin := adminModel(row)
			if super && admin.Role != "super_admin" {
				return c.String(http.StatusForbidden, a.message(r, "error.permission"))
			}
			r = r.WithContext(context.WithValue(r.Context(), adminContextKey{}, &admin))
			c.SetRequest(r)
			return next(c)
		}
	}
}
