package app

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/a-h/templ"
	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go-todo/internal/i18n"
)

func TestRenderFailureDoesNotSendPartialPage(t *testing.T) {
	component := templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		if _, err := io.WriteString(w, "partial page"); err != nil {
			return err
		}
		return errors.New("render failed")
	})
	translations, err := i18n.NewBundle()
	require.NoError(t, err)
	a := &App{translations: translations}
	a.router = a.routes()
	e := a.Echo()
	e.GET("/render-failure", func(c *echo.Context) error {
		return a.render(c, component, http.StatusOK)
	})
	w := httptest.NewRecorder()
	e.ServeHTTP(w, httptest.NewRequest("GET", "/render-failure", nil))
	assert.Equal(t, 500, w.Code)
	assert.NotContains(t, w.Body.String(), "partial page")
}
