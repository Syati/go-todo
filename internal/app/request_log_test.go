package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go-todo/internal/i18n"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestLog(t *testing.T) {
	translations, err := i18n.NewBundle()
	require.NoError(t, err)
	a := &App{translations: translations}
	e := a.routes()
	var output bytes.Buffer
	e.Logger = slog.New(slog.NewJSONHandler(&output, nil))
	e.GET("/log-test/:id", func(c *echo.Context) error {
		return c.String(http.StatusOK, "synthetic-response-secret")
	})
	e.GET("/log-error", func(c *echo.Context) error {
		return errors.New("synthetic failure")
	})
	e.GET("/log-panic", func(c *echo.Context) error {
		panic("synthetic panic")
	})

	for _, tc := range []struct {
		name, method, path, route, id string
		status                        int
		crossOrigin, oversized        bool
	}{
		{name: "success", method: "GET", path: "/log-test/private-path?search=private-query", route: "/log-test/:id", status: 200, id: "from-proxy_123"},
		{name: "redirect", method: "GET", path: "/", route: "/", status: 303},
		{name: "forbidden", method: "POST", path: "/auth/sign_in", route: "/auth/sign_in", status: 403, crossOrigin: true},
		{name: "not found", method: "GET", path: "/private-unmatched", route: "(unmatched)", status: 404, id: "malformed ID"},
		{name: "body limit", method: "POST", path: "/auth/sign_in", route: "/auth/sign_in", status: 413, oversized: true},
		{name: "error", method: "GET", path: "/log-error", route: "/log-error", status: 500},
		{name: "panic", method: "GET", path: "/log-panic", route: "/log-panic", status: 500},
	} {
		t.Run(tc.name, func(t *testing.T) {
			output.Reset()
			r := httptest.NewRequest(tc.method, tc.path, strings.NewReader("password=synthetic-password"))
			r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			if tc.name == "success" {
				r.AddCookie(&http.Cookie{Name: "session", Value: "synthetic-cookie"})
			}
			if tc.id != "" {
				r.Header.Set(echo.HeaderXRequestID, tc.id)
			}
			if tc.crossOrigin {
				r.Header.Set("Origin", "https://untrusted.example")
			}
			if tc.oversized {
				r.ContentLength = 65 << 10
			}

			w := httptest.NewRecorder()
			e.ServeHTTP(w, r)
			require.Equal(t, tc.status, w.Code)

			id := w.Header().Get(echo.HeaderXRequestID)
			require.Regexp(t, validRequestID, id)
			if validRequestID.MatchString(tc.id) {
				assert.Equal(t, tc.id, id)
			}

			for _, value := range []string{"synthetic-password", "synthetic-response-secret", "synthetic-cookie", "private-path", "private-query", "private-unmatched", "malformed ID"} {
				assert.NotContains(t, output.String(), value)
			}
			lines := bytes.Split(bytes.TrimSpace(output.Bytes()), []byte("\n"))
			if tc.status == 500 {
				require.Len(t, lines, 2)
				var cause map[string]any
				require.NoError(t, json.Unmarshal(lines[0], &cause))
				assert.Equal(t, "request failed", cause["msg"])
				assert.Equal(t, id, cause["request_id"])
				assert.NotEmpty(t, cause["error"])
			} else {
				require.Len(t, lines, 1)
			}
			var event map[string]any
			require.NoError(t, json.Unmarshal(lines[len(lines)-1], &event))
			assert.Equal(t, "http request", event["msg"])
			assert.Equal(t, id, event["request_id"])
			assert.Equal(t, tc.method, event["method"])
			assert.Equal(t, tc.route, event["route"])
			assert.Equal(t, float64(tc.status), event["status"])
			assert.Equal(t, float64(w.Body.Len()), event["response_bytes"])
			assert.Contains(t, event, "duration_us")
		})
	}
}
